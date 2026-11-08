package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// Local-mode retarget of TestR1_UnresolvedTenantHeaderBytesNeverReachTheLedger.
// In local mode the reject runs before tenant resolution, so the unresolvable
// classes no longer apply: every header-bearing value gets the same fixed
// refusal. The property the table existed for survives: the header bytes
// reach no response, ledger record or log line.
//
// Scope: local mode. Hosted tenant resolution keeps the original table, which
// is skipped until hosted tenancy lands.
func TestLocalMode_HeaderBytesNeverReachTheLedgerOrLog(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"quotes", `sk-ant-"` + r1Marker + `"`},
		{"backslashes", `C:\` + r1Marker + `\`},
		{"tab", r1Marker + "\twith tab"},
		{"obs-text bytes", r1Marker + "\xff\xfe"},
		{"oversized", strings.Repeat(r1Marker, 300)},
		{"json field bait", `x","tenant_id":"` + r1Marker},
		{"unicode", r1Marker + "\u79d8\u5bc6"},
		{"leading separator", "-" + r1Marker},
		{"reserved local", string(tenancy.LocalTenant)},
		{"reserved unknown", string(tenancy.TenantUnknown)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			base, dir, logs := startProxyWith(t, countingUpstream(&hits), Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 1_000_000})})

			resp := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: tc.value})
			body := readAll(t, resp)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", resp.StatusCode, body)
			}
			if !strings.Contains(body, "replay_tenant_header_not_allowed") {
				t.Fatalf("refusal body does not name the local reject: %s", body)
			}
			if strings.Contains(body, r1Marker) {
				t.Fatalf("response body reflects the header bytes: %s", body)
			}

			recs := waitLedger(t, dir, 1)
			if recs[0].Refusal != "tenant_header_not_allowed" {
				t.Fatalf("ledger refusal %q, want tenant_header_not_allowed", recs[0].Refusal)
			}
			if recs[0].TenantID != "" {
				t.Fatalf("a local-mode refusal must omit the tenant (SP-7), got %q", recs[0].TenantID)
			}
			raw, err := json.Marshal(recs)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), r1Marker) {
				t.Fatalf("ledger record contains the header bytes: %s", raw)
			}
			if strings.Contains(logs.String(), r1Marker) {
				t.Fatalf("log contains the header bytes:\n%s", logs.String())
			}
			if hits.Load() != 0 {
				t.Fatalf("refused request reached upstream %d times, want 0", hits.Load())
			}
		})
	}
}
