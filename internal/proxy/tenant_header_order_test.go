package proxy

import (
	"net/http"
	"sync/atomic"
	"testing"
)

// Ordering guard. PASSES on 924d3be by design: localOnly runs before the
// tenant-header rule, so an unauthenticated request carrying the header gets
// 401, not 400. Fails if a later change moves the reject ahead of localOnly.
func TestLocalMode_UnauthenticatedHeaderGets401(t *testing.T) {
	var hits atomic.Int32
	base, _, _ := startProxyWith(t, countingUpstream(&hits), Config{Token: "tok", Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	r := sendWithHeaders(t, base, http.Header{HeaderTenantID: {"widget-co"}})
	_ = r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated header-bearing request: status %d, want 401 from localOnly", r.StatusCode)
	}
	if hits.Load() != 0 {
		t.Fatalf("unauthenticated request reached upstream %d times, want 0", hits.Load())
	}
}
