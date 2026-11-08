package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// Local-mode tenant-header rejection. RED on 924d3be: the header is accepted
// and a rotated tenant gets a fresh day cap. Run with --token unset.

const tenantHeaderBody = `{"model":"claude-test","max_tokens":1,"messages":[]}`

func countingUpstream(hits *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
	})
}

// sendWithHeaders writes the header map exactly as given, so repeated values
// and non-canonical key spellings reach the proxy unchanged.
func sendWithHeaders(t *testing.T, base string, h http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(tenantHeaderBody))
	if err != nil {
		t.Fatal(err)
	}
	for k, vs := range h {
		req.Header[k] = vs
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestLocalMode_TenantHeaderIsRefusedWithNamedReason(t *testing.T) {
	var hits atomic.Int32
	base, dir, _ := startProxyWith(t, countingUpstream(&hits), Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	r := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "widget-co"})
	_ = r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", r.StatusCode)
	}
	recs := waitLedger(t, dir, 1)
	if recs[0].Refusal != "tenant_header_not_allowed" {
		t.Fatalf("ledger refusal %q, want tenant_header_not_allowed", recs[0].Refusal)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("refused request reached upstream %d times, want 0", n)
	}
}

func TestLocalMode_CapCannotBeEvadedByRotatingTenantID(t *testing.T) {
	var hits atomic.Int32
	base, dir, _ := startProxyWith(t, countingUpstream(&hits), Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	first := post(t, base, "/v1/messages", map[string]string{})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("precondition: header-less request forwarded, got %d", first.StatusCode)
	}
	waitLedger(t, dir, 1)

	evade := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "widget-co"})
	_ = evade.Body.Close()
	if evade.StatusCode != http.StatusBadRequest {
		t.Fatalf("rotated tenant: status %d, want 400", evade.StatusCode)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("rotated tenant reached upstream: hits %d, want 1", n)
	}
}

func TestLocalMode_EmptyTenantHeaderIsRefused(t *testing.T) {
	var hits atomic.Int32
	base, _, _ := startProxyWith(t, countingUpstream(&hits), Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	r := sendWithHeaders(t, base, http.Header{HeaderTenantID: {""}})
	_ = r.Body.Close()
	if r.StatusCode != http.StatusBadRequest || hits.Load() != 0 {
		t.Fatalf("empty header: status %d, upstream hits %d; want 400 and 0", r.StatusCode, hits.Load())
	}
}

func TestLocalMode_DuplicateTenantHeaderIsRefused(t *testing.T) {
	var hits atomic.Int32
	base, _, _ := startProxyWith(t, countingUpstream(&hits), Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	r := sendWithHeaders(t, base, http.Header{HeaderTenantID: {"widget-co", "gadget-co"}})
	_ = r.Body.Close()
	if r.StatusCode != http.StatusBadRequest || hits.Load() != 0 {
		t.Fatalf("duplicate header: status %d, upstream hits %d; want 400 and 0", r.StatusCode, hits.Load())
	}
}

func TestLocalMode_CaseVariedTenantHeaderIsRefused(t *testing.T) {
	var hits atomic.Int32
	base, _, _ := startProxyWith(t, countingUpstream(&hits), Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	r := sendWithHeaders(t, base, http.Header{"X-REPLAY-TENANT-ID": {"widget-co"}})
	_ = r.Body.Close()
	if r.StatusCode != http.StatusBadRequest || hits.Load() != 0 {
		t.Fatalf("case-varied header: status %d, upstream hits %d; want 400 and 0", r.StatusCode, hits.Load())
	}
}

func TestLocalMode_RefusalNeverEchoesTheHeaderValue(t *testing.T) {
	const marker = "marker-7f3a9c"
	var hits atomic.Int32
	base, dir, logs := startProxyWith(t, countingUpstream(&hits), Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	r := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: marker})
	body := readAll(t, r)
	_ = r.Body.Close()

	recs := waitLedger(t, dir, 1)
	if recs[0].Refusal != "tenant_header_not_allowed" {
		t.Fatalf("privacy test did not exercise a refusal: ledger refusal %q", recs[0].Refusal)
	}
	raw, err := json.Marshal(recs)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(marker)) {
		t.Fatalf("ledger record contains the header value: %s", raw)
	}
	if strings.Contains(logs.String(), marker) {
		t.Fatalf("log contains the header value:\n%s", logs.String())
	}
	if strings.Contains(body, marker) {
		t.Fatalf("response body contains the header value: %s", body)
	}
}
