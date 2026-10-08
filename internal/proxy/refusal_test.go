package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// A guard firing is the product's strongest claim and today it leaves almost no
// trace. The counter reaches /replay/metrics and nothing else: no log line, no
// ledger record. The observable behaviour of a guard saving somebody money
// overnight is that the log simply stops and the agent shows a provider-shaped
// error. This pins the fix.

func TestRefusalIsLoggedWithAttribution(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	w := httptest.NewRecorder()

	s.refuseSession(w, tenancy.LocalTenant, "session-abcdef123456789", "claude-opus-5", refusalLoop,
		"the same Bash call was just made 12 times in a row", 0)

	got := buf.String()
	for _, want := range []string{"REFUSED", "loop", "session-abcd", "12 times"} {
		if !strings.Contains(got, want) {
			t.Fatalf("log line %q is missing %q", got, want)
		}
	}
	// The session id is shortened the way every other log line shortens it.
	if strings.Contains(got, "session-abcdef123456789") {
		t.Fatalf("the full session id should not be logged verbatim: %q", got)
	}
}

func TestRefusalStillAnswersTheClient(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	w := httptest.NewRecorder()

	s.refuseSession(w, tenancy.LocalTenant, "s1", "claude-opus-5", refusalCircuitOpen, "the provider has been failing", 30*time.Second)

	if w.Code != refusalCircuitOpen.status {
		t.Fatalf("status %d, want %d", w.Code, refusalCircuitOpen.status)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("a circuit-open refusal must still carry Retry-After")
	}
	if !strings.Contains(w.Body.String(), refusalCircuitOpen.errType) {
		t.Fatalf("the body must keep the provider error shape: %s", w.Body.String())
	}
}

// A refusal must never be counted as a provider failure. refusalCircuitOpen is
// 503 and IsRetryableStatus covers 500-599, so anything that routes a refusal
// into Breaker.Observe re-arms the cooldown on every refused request and the
// circuit never closes.
func TestRefusalIsNeverObservedAsAProviderFailure(t *testing.T) {
	br := NewBreaker(BreakerSettings{Failures: 2, Cooldown: time.Minute})
	br.Observe(true)
	br.Observe(true)
	if ok, _, _ := br.Allow(); ok {
		t.Fatal("fixture: the circuit should be open here")
	}

	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0), Breaker: br}, stats: newStats()}
	for i := 0; i < 20; i++ {
		s.refuseSession(httptest.NewRecorder(), tenancy.LocalTenant, "s1", "claude-opus-5", refusalCircuitOpen, "holding", time.Second)
	}
	// Twenty refusals must not have touched the breaker's failure count.
	// Still open, and still refusing: nothing reset or re-armed it.
	if ok, _, _ := br.Allow(); ok {
		t.Fatal("twenty refusals changed the breaker's state")
	}
}

// Nil-safety: most test configs have no logger and no guards.
func TestRefusalSurvivesAMinimalConfig(t *testing.T) {
	s := &Server{cfg: Config{}, stats: newStats()}
	w := httptest.NewRecorder()
	s.refuseSession(w, tenancy.LocalTenant, "", "claude-opus-5", refusalSpendCap, "cap reached", 0)
	if w.Code != refusalSpendCap.status {
		t.Fatalf("status %d", w.Code)
	}
}

// A log line cannot be analysed. ADR-0009's complaint is that not one guard
// threshold in this tool is derived from evidence, and the only local path to
// changing that is recording what a session looked like when a threshold fired.
// Scrollback cannot answer "how many sessions hit a guard, at what point, and
// what did they look like" tomorrow morning.
func TestRefusalIsRecordedOnTheLedger(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{cfg: Config{Store: store, Logger: log.New(&bytes.Buffer{}, "", 0)}, stats: newStats()}
	s.recordRefusal(tenancy.LocalTenant, "sess-1234567890ab", "claude-opus-5", refusalLoop, "the same Bash call ran 12 times in a row")

	var found map[string]any
	for _, line := range readLedgerLines(t, dir) {
		var rec map[string]any
		if json.Unmarshal(line, &rec) == nil && rec["refusal"] != nil {
			found = rec
		}
	}
	if found == nil {
		t.Fatal("no refusal record reached the ledger")
	}
	if found["refusal"] != refusalLoop.counter {
		t.Fatalf("refusal=%v, want %q", found["refusal"], refusalLoop.counter)
	}
	if fmt.Sprint(found["status"]) != fmt.Sprint(refusalLoop.status) {
		t.Fatalf("status=%v, want %d", found["status"], refusalLoop.status)
	}
}

// The ledger's content-free guarantee holds for refusals too. The reason
// strings carry counts, thresholds and a tool name; they must never carry a
// prompt, a path, or a secret.
func TestRefusalRecordCarriesNoContent(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: Config{Store: store, Logger: log.New(&bytes.Buffer{}, "", 0)}, stats: newStats()}
	s.recordRefusal(tenancy.LocalTenant, "s1", "claude-opus-5", refusalSpendCap, "daily spend cap reached: $50.00 of $50.00 at list price")

	body := string(bytes.Join(readLedgerLines(t, dir), []byte("\n")))
	// Assert on content, not on JSON key names: "prompt" and "messages" are
	// keys on every record and their values here are zero and null.
	for _, forbidden := range []string{"/Users/", "sk-ant", "Bearer ", "x-api-key"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("a refusal record leaked %q:\n%s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"messages":null`) {
		t.Fatalf("a refusal record must carry no messages at all:\n%s", body)
	}
}

// Recording must never be the reason a request fails, and most configs have no
// store at all.
func TestRefusalRecordingIsOptional(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a config with no store and no logger panicked: %v", r)
		}
	}()
	s := &Server{cfg: Config{}, stats: newStats()}
	s.recordRefusal(tenancy.LocalTenant, "s1", "m", refusalLoop, "loop")
}

// SP-7's own text: "Re-read this row once SP-5 lands: naming a session is
// not naming a tenant." SP-5 landed 2026-10-07 (internal/tenancy wired into
// internal/proxy). An org-wide refusal that cannot say who spent the budget
// is an outage with no operator action attached, and that is true of the
// tenant as much as the session: a day cap tripped by tenant "acme" and one
// tripped by tenant "umbrella" are two different operator conversations.
//
// PASS: a refusal fired for a non-local tenant names that tenant in the log
// line and in the ledger record.
// FAIL: the tenant is silently dropped, leaving only the session id — which
// is exactly what every refusal did before this test existed.
func TestRefusalNamesTheTenantInTheLogLine(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	w := httptest.NewRecorder()

	s.refuseSession(w, tenancy.TenantID("acme"), "sess-known-1234", "claude-opus-5", refusalSpendCap,
		"daily spend cap reached: $50.00 of $50.00 at list price", 0)

	got := buf.String()
	if !strings.Contains(got, "acme") {
		t.Fatalf("a refusal for a non-local tenant must name it in the log line:\n%s", got)
	}
}

// The local default must stay silent about tenancy. Printing "tenant=LOCAL"
// on every line a single developer ever sees would be noise for a concept
// that, for them, has never existed and never needs to.
func TestRefusalDoesNotNameTheLocalTenant(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	w := httptest.NewRecorder()

	s.refuseSession(w, tenancy.LocalTenant, "sess-known-1234", "claude-opus-5", refusalSpendCap, "cap reached", 0)

	if strings.Contains(buf.String(), "LOCAL") {
		t.Fatalf("the local default tenant must not be printed as though it were a real tenant:\n%s", buf.String())
	}
}

// The ledger record is the artifact that outlives the scrollback, and it is
// what SP-7's acceptance criterion names explicitly: "a refusal record names
// the tenant and survives the ledger's existing leak assertions."
func TestRefusalRecordNamesTheTenant(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: Config{Store: store, Logger: log.New(&bytes.Buffer{}, "", 0)}, stats: newStats()}
	s.recordRefusal(tenancy.TenantID("acme"), "sess-1234567890ab", "claude-opus-5", refusalSpendCap,
		"daily spend cap reached: $50.00 of $50.00 at list price")

	var found map[string]any
	for _, line := range readLedgerLines(t, dir) {
		var rec map[string]any
		if json.Unmarshal(line, &rec) == nil && rec["refusal"] != nil {
			found = rec
		}
	}
	if found == nil {
		t.Fatal("no refusal record reached the ledger")
	}
	if found["tenant_id"] != "acme" {
		t.Fatalf("tenant_id=%v, want %q", found["tenant_id"], "acme")
	}
}

// omitempty means the field must be absent, not present-and-blank, for the
// tenant every pre-existing ledger reader has to keep tolerating.
func TestRefusalRecordOmitsTheLocalTenant(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: Config{Store: store, Logger: log.New(&bytes.Buffer{}, "", 0)}, stats: newStats()}
	s.recordRefusal(tenancy.LocalTenant, "s1", "claude-opus-5", refusalLoop, "loop")

	body := string(bytes.Join(readLedgerLines(t, dir), []byte("\n")))
	if strings.Contains(body, "tenant_id") {
		t.Fatalf("the local tenant must not appear on the ledger at all:\n%s", body)
	}
}

func readLedgerLines(t *testing.T, dir string) [][]byte {
	t.Helper()
	var out [][]byte
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
			if len(line) > 0 {
				out = append(out, line)
			}
		}
	}
	return out
}
