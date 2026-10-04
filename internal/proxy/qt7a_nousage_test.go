package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// QT-7a. What happens when a response carries no usage?
//
// The qualification pass (docs/evidence/replay-1.0-qualification-2026-10-03.md)
// sent thirty well-formed responses with no `usage` object through a proxy
// with a dollar cap: none refused, cost $0, spend_cap_not_enforced false. The
// cap could not see the traffic and nothing said so.
//
// The path, traced on 2026-10-03: ledger.ParseResponse leaves Response.Usage
// nil when the key is absent; passthrough.go calls the spend guard only when
// Usage is non-nil; stats.observe returns before counting anything when it
// is nil; no counter, status field, metric, doctor line or TUI line exists
// for the case. The ledger record keeps the distinction (Usage nil, not a
// zero struct), which is why replay cost and replay simulate can report
// "without usage" after the fact; the running proxy cannot.
//
// The oracle below is a signal, not a sentence: the status and metrics
// surfaces must carry a count of responses that carried no usage, and the
// count must distinguish a missing usage object from a measured zero.

const messageWithoutUsage = `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}]}`
const messageWithZeroUsage = `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0}}`

func jsonUpstream(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	})
}

func statusRaw(t *testing.T, base string) map[string]any {
	t.Helper()
	resp, err := http.Get(base + "/replay/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func metricsText(t *testing.T, base string) string {
	t.Helper()
	resp, err := http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func postN(t *testing.T, base string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		resp := post(t, base, "/v1/messages", nil)
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status %d", i, resp.StatusCode)
		}
	}
}

// The signal. Five 200 responses with no usage object under a dollar cap of
// a tenth of a cent: all forwarded, nothing counted, and the operator must
// be able to see both facts on the status endpoint and on /replay/metrics.
func TestQT7a_ResponsesWithoutUsageAreCountedAndSurfaced(t *testing.T) {
	base, dir, _ := startProxyWith(t, jsonUpstream(messageWithoutUsage), Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 0.001})})
	postN(t, base, 5)
	recs := waitLedger(t, dir, 5)
	for i, r := range recs {
		if r.Response.Usage != nil {
			t.Fatalf("fixture: record %d has usage; the upstream was meant to send none", i)
		}
	}
	st := getStatus(t, base)
	if st.CostUSD != 0 {
		t.Fatalf("fixture: $%.4f was counted from responses that carried no usage", st.CostUSD)
	}
	raw := statusRaw(t, base)
	got, ok := raw["responses_without_usage"]
	if !ok {
		t.Errorf("/replay/status carries no count of responses without usage; five 200s were forwarded under a $0.001 day cap and nothing counted them")
	} else if n, _ := got.(float64); n != 5 {
		t.Errorf("responses_without_usage = %v, want 5", got)
	}
	if m := metricsText(t, base); !strings.Contains(m, "replay_responses_without_usage_total 5") {
		t.Errorf("/replay/metrics carries no replay_responses_without_usage_total line, or not 5")
	}
}

// Missing is not zero. A provider that says zero tokens has measured
// something; a provider that says nothing has not. The count must not fold
// the second into the first.
func TestQT7a_AMeasuredZeroIsNotAMissingUsage(t *testing.T) {
	base, dir, _ := startProxyWith(t, jsonUpstream(messageWithZeroUsage), Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 0.001})})
	postN(t, base, 3)
	recs := waitLedger(t, dir, 3)
	if recs[0].Response.Usage == nil {
		t.Fatal("fixture: a zero usage object parsed as no usage")
	}
	raw := statusRaw(t, base)
	got, ok := raw["responses_without_usage"]
	if !ok {
		t.Errorf("/replay/status carries no responses_without_usage field, so a measured zero and a missing usage cannot be told apart from it")
	} else if n, _ := got.(float64); n != 0 {
		t.Errorf("responses_without_usage = %v after three responses with a measured zero usage, want 0", got)
	}
}

// Positive control: the same harness, status read and cap configuration do
// observe the one signal the guard already has. An unpriced model under a
// dollar cap arms spend_cap_not_enforced and is counted as unpriced on
// /replay/metrics. If this ever fails, the RED above is not evidence of
// anything.
func TestQT7a_PositiveControl_TheExistingUnpricedSignalIsObservedByThisHarness(t *testing.T) {
	base, _, _ := startProxyWith(t, &upstream{t: t}, Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 1})})
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(`{"model":"qt7a-model-no-table-carries","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", secret)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set(HeaderSessionID, "session-qt7a")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	st := getStatus(t, base)
	if !st.SpendCapNotEnforced {
		t.Fatalf("control: an unpriced model under a dollar cap did not arm spend_cap_not_enforced; the harness cannot see the signal that exists, so it cannot be trusted to see one that does not")
	}
	if m := metricsText(t, base); !strings.Contains(m, "replay_cost_unpriced_requests_total 1") {
		t.Fatalf("control: /replay/metrics does not count the unpriced request:\n%s", m)
	}
}

// Negative control: priced usage present. Nothing is flagged, and the cost
// is counted. A no-usage signal that fired here would be noise.
func TestQT7a_NegativeControl_PricedUsageRaisesNoSignal(t *testing.T) {
	base, dir, _ := startProxyWith(t, &upstream{t: t}, Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 1})})
	postN(t, base, 2)
	waitLedger(t, dir, 2)
	st := getStatus(t, base)
	if st.CostUSD <= 0 || st.SpendCapNotEnforced {
		t.Fatalf("control: cost $%.4f, spend_cap_not_enforced %v; priced usage must be counted and flag nothing", st.CostUSD, st.SpendCapNotEnforced)
	}
	if n, ok := statusRaw(t, base)["responses_without_usage"]; ok && n.(float64) != 0 {
		t.Errorf("responses_without_usage = %v with usage present on every response", n)
	}
}
