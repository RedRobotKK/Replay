package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

// QT-7c. What happens when a 2xx body cannot be read at all?
//
// The qualification pass sent ten malformed bodies: ten 200s forwarded, ten
// ledger records, none with usage, the cap never advanced, no signal. After
// QT-7a (commit 40bed0a) those ten are counted, but under the same name as a
// well-formed message that simply carried no usage object, because
// ledger.ParseResponse returns an empty Response for a body that is not JSON
// or not a message, the tap returns an empty Response for a dropped or
// truncated body, and nothing downstream can tell an empty Response from a
// parsed one whose usage pointer is nil. The record keeps content blocks for
// the parsed case, but a message with no content and no usage is legal, so
// blocks are not the distinction.
//
// The two states need different operator actions: a provider that omits
// usage needs a usage-reporting option or a price-table fix; a body that
// does not parse is a client, proxy or provider defect. The oracle below
// asks for the split at the status and metrics surfaces, with the sibling
// signal from QT-7a as the positive control.

const errorObjectAt200 = `{"type":"error","error":{"type":"overloaded_error","message":"try again"}}`

// A 200 whose body is not JSON at all.
func TestQT7c_ANonJSONBodyIsCountedAsUnparsedNotAsMissingUsage(t *testing.T) {
	base, dir, _ := startProxyWith(t, jsonUpstream("<html>502 from a load balancer that said 200</html>"), Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 0.001})})
	postN(t, base, 4)
	recs := waitLedger(t, dir, 4)
	for i, r := range recs {
		if r.Response.Usage != nil || len(r.Response.Blocks) != 0 {
			t.Fatalf("fixture: record %d parsed something from a body that is not JSON: %+v", i, r.Response)
		}
	}
	raw := statusRaw(t, base)
	unparsed, ok := raw["responses_unparsed"]
	if !ok {
		t.Errorf("/replay/status carries no count of 2xx bodies that could not be read; four were forwarded under a dollar cap")
	} else if n, _ := unparsed.(float64); n != 4 {
		t.Errorf("responses_unparsed = %v, want 4", unparsed)
	}
	if n, _ := raw["responses_without_usage"].(float64); n != 0 {
		t.Errorf("responses_without_usage = %v for bodies that did not parse; want 0, this count is for messages that parsed and carried no usage", n)
	}
	if m := metricsText(t, base); !strings.Contains(m, "replay_responses_unparsed_total 4") {
		t.Errorf("/replay/metrics carries no replay_responses_unparsed_total line, or not 4")
	}
}

// A 200 whose body is a well-formed JSON error object: parses, is not a
// message, carries no usage. The provider's own shape for "no result".
func TestQT7c_AnErrorObjectBehindA200IsUnparsedNotMissingUsage(t *testing.T) {
	base, dir, _ := startProxyWith(t, jsonUpstream(errorObjectAt200), Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 0.001})})
	postN(t, base, 2)
	waitLedger(t, dir, 2)
	raw := statusRaw(t, base)
	if n, _ := raw["responses_unparsed"].(float64); n != 2 {
		t.Errorf("responses_unparsed = %v after two error objects behind 200, want 2", raw["responses_unparsed"])
	}
	if n, _ := raw["responses_without_usage"].(float64); n != 0 {
		t.Errorf("responses_without_usage = %v; an error object is not a message that forgot its usage", n)
	}
}

// A 200 whose JSON ends early.
func TestQT7c_ATruncatedBodyIsUnparsed(t *testing.T) {
	base, dir, _ := startProxyWith(t, jsonUpstream(messageResponse[:len(messageResponse)/2]), Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 0.001})})
	postN(t, base, 3)
	waitLedger(t, dir, 3)
	raw := statusRaw(t, base)
	if n, _ := raw["responses_unparsed"].(float64); n != 3 {
		t.Errorf("responses_unparsed = %v after three truncated bodies, want 3", raw["responses_unparsed"])
	}
	if n, _ := raw["responses_without_usage"].(float64); n != 0 {
		t.Errorf("responses_without_usage = %v for truncated bodies, want 0", n)
	}
}

// Positive control: the sibling signal. A well-formed message with no usage
// object is counted under responses_without_usage and NOT under the unparsed
// count. This is the QT-7a path and must keep passing; if it ever fails the
// RED above is measuring the wrong thing.
func TestQT7c_PositiveControl_AParsedMessageWithoutUsageIsNotUnparsed(t *testing.T) {
	base, dir, _ := startProxyWith(t, jsonUpstream(messageWithoutUsage), Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 0.001})})
	postN(t, base, 3)
	waitLedger(t, dir, 3)
	raw := statusRaw(t, base)
	if n, _ := raw["responses_without_usage"].(float64); n != 3 {
		t.Fatalf("control: responses_without_usage = %v, want 3; the QT-7a signal this test leans on is gone", raw["responses_without_usage"])
	}
	if n, _ := raw["responses_unparsed"].(float64); n != 0 {
		t.Errorf("responses_unparsed = %v for messages that parsed, want 0", n)
	}
}

// Negative control: priced usage present raises neither count.
func TestQT7c_NegativeControl_PricedUsageRaisesNeitherCount(t *testing.T) {
	base, dir, _ := startProxyWith(t, &upstream{t: t}, Config{Spend: NewSpendGuard(SpendLimits{DayUSD: 1})})
	postN(t, base, 2)
	waitLedger(t, dir, 2)
	st := getStatus(t, base)
	if st.CostUSD <= 0 {
		t.Fatalf("control: priced usage was not counted")
	}
	raw := statusRaw(t, base)
	for _, k := range []string{"responses_without_usage", "responses_unparsed"} {
		if n, _ := raw[k].(float64); n != 0 {
			t.Errorf("%s = %v with priced usage on every response", k, n)
		}
	}
}

// The record. After the fact, replay cost and replay simulate read the
// ledger, and today both states are one state there: Usage nil. The split
// is only worth having if the record can carry it too, so a reader of the
// ledger can say "N not parsed" apart from "N without usage". This reads
// the record's JSON generically, so it compiles before any field exists.
func TestQT7c_TheLedgerRecordSaysWhyThereIsNoUsage(t *testing.T) {
	base, dir, _ := startProxyWith(t, jsonUpstream("not json"), Config{})
	postN(t, base, 1)
	recs := waitLedger(t, dir, 1)
	b, err := json.Marshal(recs[0].Response)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["unparsed"]; !ok {
		t.Errorf("the ledger record for a body that did not parse carries no mark of it: %s", b)
	}
	base2, dir2, _ := startProxyWith(t, jsonUpstream(messageWithoutUsage), Config{})
	postN(t, base2, 1)
	recs2 := waitLedger(t, dir2, 1)
	b2, _ := json.Marshal(recs2[0].Response)
	var m2 map[string]any
	_ = json.Unmarshal(b2, &m2)
	if v, ok := m2["unparsed"]; ok && v != false {
		t.Errorf("a message that parsed and carried no usage is marked unparsed: %s", b2)
	}
}
