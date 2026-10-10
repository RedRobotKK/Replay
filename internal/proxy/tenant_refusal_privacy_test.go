package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// R1 (PR #336 review): the x-replay-tenant-id header is client-controlled
// bytes. Until this test existed, a value that failed tenancy.ResolveTenant
// was formatted with %q into the validator's error, that error was pasted
// into the refusal message, and the message was written to three places:
// the REFUSED log line, the HTTP error body, and ledger.Record.RefusalReason,
// whose contract (internal/ledger/record.go) is "counts and thresholds,
// never content". A client could therefore write any string it liked into
// the operator's ledger by sending it as a tenant id that does not validate.
//
// The marker below is alphanumeric on purpose: %q escaping leaves it
// intact, so a test that only checked for the quotes or the backslashes
// would pass against the defect. These tests look for the marker itself, in
// every byte the proxy persists or prints.
const r1Marker = "R1SECRETMARKER7f3a"

// r1Surfaces reads everything the proxy wrote: every file under the ledger
// directory (not only the session .jsonl, so a future pins or state file is
// covered too) and the log sink.
func r1Surfaces(t *testing.T, dir string, logs *syncBuffer) string {
	t.Helper()
	var sb strings.Builder
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sb.Write(b)
		sb.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sb.WriteString(logs.String())
	return sb.String()
}

// r1ReasonPattern is the whole of what a tenant refusal may say about the
// header: a failure class from a fixed set and a byte count. Anything else
// in the reason is content, and content is what the ledger contract
// forbids.
var r1ReasonPattern = regexp.MustCompile(
	`^tenant identity could not be resolved: ` + regexp.QuoteMeta(HeaderTenantID) +
		` is (a reserved word|not a legal identity) \([0-9]+ bytes\); omit it to run as the local default, or send a valid identity \(ASCII letters, digits, -\._: only, 1-128 chars, first character alphanumeric\)$`)

// TestR1_UnresolvedTenantHeaderBytesNeverReachTheLedger sends adversarial
// tenant values through the real request path (loopback server, fake
// upstream) and proves the bytes stop at the validator.
//
// Every value here is one Go's HTTP client and server both accept in a
// header (VCHAR, SP, HTAB and obs-text per RFC 7230), so each one reaches
// handle. Control characters, which the server rejects before the handler,
// are covered by the raw-socket test below.
//
// PASS: each value is refused with 400 replay_tenant_unresolved, the ledger
// record names the tenant guard, its refusal_reason is a bounded
// classification, and the marker appears nowhere the proxy wrote.
// FAIL: the marker is found in the ledger directory, the log line or the
// response body, or the reason carries anything but the class and a count.
func TestR1_UnresolvedTenantHeaderBytesNeverReachTheLedger(t *testing.T) {
	cases := []struct {
		name  string
		value string
		class string
	}{
		{"quotes", `sk-ant-"` + r1Marker + `"`, "not a legal identity"},
		{"backslashes", `C:\` + r1Marker + `\`, "not a legal identity"},
		{"tab", r1Marker + "\twith tab", "not a legal identity"},
		{"obs-text bytes", r1Marker + "\xff\xfe", "not a legal identity"},
		{"oversized", strings.Repeat(r1Marker, 300), "not a legal identity"},
		{"json field bait", `x","tenant_id":"` + r1Marker, "not a legal identity"},
		{"unicode", r1Marker + "\u79d8\u5bc6", "not a legal identity"},
		{"leading separator", "-" + r1Marker, "not a legal identity"},
		{"reserved local", string(tenancy.LocalTenant), "a reserved word"},
		{"reserved unknown", string(tenancy.TenantUnknown), "a reserved word"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := &upstream{t: t}
			base, dir, logs := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 1_000_000})})

			resp := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: tc.value})
			defer func() { _ = resp.Body.Close() }()
			body := readAll(t, resp)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", resp.StatusCode, body)
			}
			if !strings.Contains(body, "replay_tenant_unresolved") {
				t.Fatalf("refusal body does not name the tenant guard: %s", body)
			}
			if strings.Contains(body, r1Marker) {
				t.Fatalf("the response body reflects the header bytes: %s", body)
			}

			recs := waitLedger(t, dir, 1)
			rec := recs[0]
			if rec.Refusal != "tenant_unresolved" {
				t.Fatalf("ledger record does not attribute the refusal to the tenant guard: %+v", rec)
			}
			if !r1ReasonPattern.MatchString(rec.RefusalReason) {
				t.Fatalf("refusal_reason is not a bounded classification:\n%q\nwant to match\n%s", rec.RefusalReason, r1ReasonPattern)
			}
			if !strings.Contains(rec.RefusalReason, tc.class) {
				t.Fatalf("refusal_reason classifies %q as something other than %q: %q", tc.name, tc.class, rec.RefusalReason)
			}
			wantBytes := fmt.Sprintf("(%d bytes)", len(tc.value))
			if !strings.Contains(rec.RefusalReason, wantBytes) {
				t.Fatalf("refusal_reason does not carry the header length %s: %q", wantBytes, rec.RefusalReason)
			}
			if rec.TenantID != string(tenancy.TenantUnknown) {
				t.Fatalf("an unresolved tenant must be recorded as the unknown sentinel, got %q", rec.TenantID)
			}

			wrote := r1Surfaces(t, dir, logs)
			if strings.Contains(wrote, r1Marker) {
				t.Fatalf("the header bytes reached something the proxy wrote:\n%s", wrote)
			}
			if up.seen().requests != 0 {
				t.Fatal("a request whose tenant could not be resolved must never reach the upstream")
			}
		})
	}
}

// TestR1_ControlCharactersInTheTenantHeaderNeverReachTheLedger covers the
// bytes Go's HTTP client refuses to send: a raw socket writes them. Go's
// server rejects such a header before the handler runs, and this test does
// not depend on that staying true; whatever answers, the marker must not be
// persisted or logged, and the upstream must not be reached.
func TestR1_ControlCharactersInTheTenantHeaderNeverReachTheLedger(t *testing.T) {
	up := &upstream{t: t}
	base, dir, logs := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 1_000_000})})

	conn, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	raw := "POST /v1/messages HTTP/1.1\r\n" +
		"Host: " + strings.TrimPrefix(base, "http://") + "\r\n" +
		"Content-Type: application/json\r\n" +
		"x-api-key: " + secret + "\r\n" +
		HeaderSessionID + ": session-abc\r\n" +
		HeaderTenantID + ": " + r1Marker + "\x01\x7f\x00\r\n" +
		"Content-Length: " + strconv.Itoa(len(requestBody)) + "\r\n" +
		"Connection: close\r\n\r\n" + requestBody
	if _, err := io.WriteString(conn, raw); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("no HTTP response to a header with control characters: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", resp.StatusCode, body)
	}
	if strings.Contains(body, r1Marker) {
		t.Fatalf("the response body reflects the header bytes: %s", body)
	}
	if strings.Contains(r1Surfaces(t, dir, logs), r1Marker) {
		t.Fatal("the header bytes reached something the proxy wrote")
	}
	if up.seen().requests != 0 {
		t.Fatal("a request whose tenant could not be resolved must never reach the upstream")
	}
}

// TestR1_ValidTenantStillResolvesAndIsStrippedOutbound is the positive
// control: tightening what a refusal says must not change what a valid
// identity does. A legal tenant is forwarded, the header never reaches the
// provider, and a refusal for that tenant still names it (SP-7).
func TestR1_ValidTenantStillResolvesAndIsStrippedOutbound(t *testing.T) {
	var gotTenant []string
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = append(gotTenant, r.Header.Get(HeaderTenantID))
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
	})
	// messageResponse's usage totals 336 tokens; one request exhausts the cap.
	base, dir, logs := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	first := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "acme-co"})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("a valid tenant must be forwarded: status %d", first.StatusCode)
	}
	waitLedger(t, dir, 1)
	if len(gotTenant) != 1 || gotTenant[0] != "" {
		t.Fatalf("the tenant header must be stripped before the provider, upstream saw %q", gotTenant)
	}

	second := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "acme-co"})
	defer func() { _ = second.Body.Close() }()
	if second.StatusCode != http.StatusBadRequest {
		t.Fatalf("the exhausted tenant's next request must be refused: status %d", second.StatusCode)
	}
	recs := waitLedger(t, dir, 2)
	var refused bool
	for _, rec := range recs {
		if rec.Refusal == "spend_cap" {
			refused = true
			if rec.TenantID != "acme-co" {
				t.Fatalf("a refusal for a valid tenant must still name it (SP-7), got %q", rec.TenantID)
			}
		}
	}
	if !refused {
		t.Fatalf("no spend_cap refusal reached the ledger: %+v", recs)
	}
	if !strings.Contains(logs.String(), "tenant=acme-co") {
		t.Fatalf("the log line must still name a valid tenant:\n%s", logs.String())
	}
}

// TestR1_TenantUnresolvedMessageIsBuiltFromTheClassNotTheError pins the
// builder at the unit level: the class is taken from errors.Is, the error's
// own text is never copied, and an error from neither class is still
// bounded. The marker is placed in the error text, which is where the raw
// header used to travel.
func TestR1_TenantUnresolvedMessageIsBuiltFromTheClassNotTheError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"reserved", fmt.Errorf("tenant id %q %w", r1Marker, tenancy.ErrIdentityReserved), "a reserved word"},
		{"illegal", fmt.Errorf("tenant id %q %w", r1Marker, tenancy.ErrIdentityIllegal), "not a legal identity"},
		{"unclassified", fmt.Errorf("something new about %q", r1Marker), "not a recognised identity"},
	}
	for _, tc := range cases {
		got := tenantUnresolvedMessage(len(r1Marker), tc.err)
		if strings.Contains(got, r1Marker) {
			t.Fatalf("%s: the error text was copied into the refusal: %q", tc.name, got)
		}
		if !strings.Contains(got, tc.want) {
			t.Fatalf("%s: want class %q in %q", tc.name, tc.want, got)
		}
		if !strings.Contains(got, fmt.Sprintf("(%d bytes)", len(r1Marker))) {
			t.Fatalf("%s: the byte count is missing from %q", tc.name, got)
		}
		if tc.name != "unclassified" && !r1ReasonPattern.MatchString(got) {
			t.Fatalf("%s: %q does not match the bounded pattern", tc.name, got)
		}
	}
}
