package proxy

import (
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

// Regression guard (measured PASS on 924d3be): under default local mode the
// day cap holds across session IDs. Must stay green after the tenant-header fix.
func TestSP8_SessionRotationDoesNotEscapeTheDayCap(t *testing.T) {
	var hits atomic.Int32
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
	})
	base, dir, _ := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{SessionTokens: 336, DayTokens: 336})})

	first := post(t, base, "/v1/messages", map[string]string{HeaderSessionID: "session-1"})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("precondition: first request must be forwarded: status %d", first.StatusCode)
	}
	waitLedger(t, dir, 1)

	rotated := post(t, base, "/v1/messages", map[string]string{HeaderSessionID: "session-2"})
	_ = rotated.Body.Close()
	if rotated.StatusCode != http.StatusBadRequest {
		t.Fatalf("rotated session: status %d, want 400 spend_cap refusal", rotated.StatusCode)
	}
	recs := waitLedger(t, dir, 2)
	if recs[1].Refusal != "spend_cap" {
		t.Fatalf("rotated session refusal %q, want spend_cap", recs[1].Refusal)
	}
	if recs[1].TenantID != "" {
		t.Fatalf("a local-tenant refusal must omit the tenant (SP-7), got %q", recs[1].TenantID)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("upstream hit %d times, want 1", n)
	}
}
