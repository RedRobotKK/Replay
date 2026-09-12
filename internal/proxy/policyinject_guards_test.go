package proxy

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
)

// The failure arms of policy injection.
//
// Four conditionals here were reported UNREACHED by guard-reachability (#236),
// all four older than the server.go split that surfaced them. They share the
// request-side half of the rule masking keeps on the response side: EVERY
// FAILURE RETURNS THE ORIGINAL BODY AND SAYS WHY.
//
// That matters more here than it reads. A policy that half-applies, or that
// silently does nothing, is worse than one that refuses: the ledger would
// record a policy name against bytes it did not change, and every figure
// derived from that session would be attributed to a policy that never ran.

// A policy the edit declines to apply leaves the body alone and names the
// reason.
func TestPolicyInject_AnUnappliedEditReturnsTheOriginalBody(t *testing.T) {
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	s := &Server{cfg: Config{
		Logger:      log.New(&logs, "", 0),
		Store:       store,
		ContextEdit: &policy.ContextEdit{TriggerTokens: 1, KeepLast: 1},
	}}
	s.stats = newStats()

	// A body Apply declines: it splices the parameter into a top-level object
	// and there is none here, so it returns SkipNotAnObject. My first fixture
	// was an object with an empty messages array, which Apply handles fine —
	// the guard is about the body's SHAPE, not its contents.
	body := []byte(`[1,2,3]`)
	rec := &ledger.Record{SessionID: "sess-unapplied"}
	rec.Model = "claude-opus-5"
	req, rerr := http.NewRequest(http.MethodPost, "http://x/v1/messages", nil)
	if rerr != nil {
		t.Fatal(rerr)
	}
	req.Header.Set("anthropic-beta", policy.BetaFeature)

	out := s.applyPolicy(req, rec, body, true)

	if !bytes.Equal(out, body) {
		t.Fatalf("an edit that did not apply still changed the body:\n  in:  %s\n  out: %s", body, out)
	}
	if rec.Policy != "" {
		t.Fatalf("the ledger records policy %q against a body the policy did not change. Every "+
			"figure from this session would then be attributed to a policy that never ran", rec.Policy)
	}
}

// A pin that cannot be persisted fails OPEN: the session still gets its
// decision, from memory, and the operator is told the disk did not take it.
//
// Failing closed here would be worse than the thing it guards against. The pin
// exists so a session's policy does not change under it mid-run; losing the
// file loses that guarantee across a restart, and refusing the request loses
// the request.
func TestPolicyInject_APinThatCannotBePersistedFailsOpen(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Only now: a directory where the pins FILE goes, so OpenFile cannot make
	// a regular file there on any platform — no dependence on permissions that
	// root ignores. Done after Open because Open reads the pins file itself and
	// would fail first, which is a different failure from the one under test.
	if err := os.Mkdir(filepath.Join(dir, ".pins"), 0o755); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	s := &Server{cfg: Config{
		Logger:      log.New(&logs, "", 0),
		Store:       store,
		ContextEdit: &policy.ContextEdit{TriggerTokens: 1, KeepLast: 1},
	}}
	s.stats = newStats()

	edit, decision, _ := s.decidePolicy("sess-nopin-0000-longer-than-twelve", policy.BetaFeature, false, "claude-opus-5", 4096)

	if edit == nil && decision == policy.Applied {
		t.Fatal("a decision of Applied with no edit is not a state the caller can use")
	}
	if !strings.Contains(logs.String(), "not persisted") {
		t.Fatalf("a pin that could not be written was not reported:\n%s", logs.String())
	}
	// Longer than short()'s twelve-character cut, or this assertion passes on
	// an id that was never truncated. I made exactly this mistake in the
	// masking tests an hour before writing these, which is the argument for
	// asserting both halves rather than only the absence.
	if strings.Contains(logs.String(), "sess-nopin-0000-longer-than-twelve") {
		t.Errorf("the full session id reached the log:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "sess-nopin-0") {
		t.Errorf("the truncated id is absent too, so the line cannot be traced:\n%s", logs.String())
	}
}

// A policy file that cannot be read leaves the session with NO policy, rather
// than a guessed one, and says so.
func TestPolicyInject_AnUnreadablePolicyFileLeavesNoPolicy(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{cfg: Config{
		Logger:     log.New(&logs, "", 0),
		PolicyFile: filepath.Join(t.TempDir(), "does-not-exist.json"),
	}}

	edit, generated := s.policyFromFile("sess-nofile", "cli")

	if edit != nil {
		t.Fatal("an unreadable policy file produced a policy. A policy the operator did not " +
			"choose, applied to their traffic, is the thing a policy file exists to prevent")
	}
	if !generated.IsZero() {
		t.Errorf("a generation time was reported for a file that was not read: %v", generated)
	}
	if !strings.Contains(logs.String(), "not read") {
		t.Fatalf("the failure was not reported:\n%s", logs.String())
	}
}

// withUsageReporting's marshal error is UNREACHABLE, and that is recorded
// rather than worked around.
//
// `raw` is a map[string]json.RawMessage populated by a successful Unmarshal,
// so every value in it is already valid JSON, and the one key this function
// adds is a literal. json.Marshal has nothing to fail on. Probed with a
// number too large for float64, invalid UTF-8, and deep nesting: all three
// marshal cleanly.
//
// Kept rather than deleted — it guards the standard library's error return and
// the signature promises one. What is testable is the contract it protects:
// whatever happens, the caller gets a usable body back and a truthful bool.
func TestPolicyInject_UsageReportingNeverReturnsABrokenBody(t *testing.T) {
	for _, in := range []string{
		`{"model":"gpt-x","stream":true,"messages":[]}`,
		`{"model":"gpt-x","stream":false}`,
		`{"model":"gpt-x","stream":true,"stream_options":{"include_usage":false}}`,
		`not json at all`,
		``,
	} {
		out, changed := withUsageReporting([]byte(in))
		if !changed && string(out) != in {
			t.Errorf("reported no change and returned different bytes for %q:\n  %s", in, out)
		}
		if changed && !bytes.Contains(out, []byte(`"include_usage":true`)) {
			t.Errorf("reported a change that did not add the parameter for %q:\n  %s", in, out)
		}
	}
}
