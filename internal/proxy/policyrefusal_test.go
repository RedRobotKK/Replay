package proxy

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/learn"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
)

// Three policy refusals that no test in this package executed.
//
// Each one is a place where the proxy decides NOT to apply a policy because
// the input it was handed is not something it will act on. All three sat at
// zero statement coverage as of this audit: server.go:842-846 (a persisted pin
// whose parameters do not validate), server.go:924-926 (a selection from the
// policy file that is a client setting rather than something the proxy can
// apply) and server.go:929-932 (a selection whose parameters do not validate).
//
// The risk in each is the same and it is not "a log line is missing". A refusal
// that cannot be observed is a refusal that can be deleted, and deleting any of
// these three sends invalid or inapplicable parameters into a live request:
// TriggerTokens=0 is a context-edit that clears at every prompt.

// policyServer is the minimum Server that decidePolicy and policyFromFile need.
func policyServer(t *testing.T, cfg Config) (*Server, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	cfg.Store = mustStore(t)
	cfg.Logger = log.New(logs, "", 0)
	return &Server{cfg: cfg, stats: newStats()}, logs
}

// writeSelection writes a policy file whose selection is exactly c.
func writeSelection(t *testing.T, path string, c *learn.Candidate) {
	t.Helper()
	res := learn.Result{Schema: learn.PolicyFileSchema, Rules: cachemodel.RulesVersion, Selected: c, Generated: time.Now()}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// PR1: a persisted pin with parameters that do not validate is refused.
//
// The pin file is written by an earlier process and read by this one. A pin
// naming context-edit with TriggerTokens=0 is a policy that clears tool
// results at every prompt; the comment on the guard says a pin another process
// wrote is data, not an order, and this is what makes that true.
//
// PASS: decidePolicy returns a nil edit and policy.NotConfigured, and logs that
// the pin had invalid parameters.
// FAIL: a non-nil *ContextEdit comes back, which is the invalid parameters
// being handed to Apply and put on the wire.
func TestPR1_APersistedPinWithInvalidParametersIsRefused(t *testing.T) {
	s, logs := policyServer(t, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 200000, KeepLast: 6}})
	if err := s.cfg.Store.SetPin(ledger.Pin{
		SessionID: "sess-bad", Policy: policy.Name,
		Trigger: 0, Keep: 6, Decision: string(policy.Applied), At: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	edit, decision, _ := s.decidePolicy("sess-bad", policy.BetaFeature, false, "claude-opus-5", 200000)

	if edit != nil {
		t.Fatalf("a pin with trigger=0 must not become a live policy, got %+v", edit)
	}
	if decision != policy.NotConfigured {
		t.Fatalf("an unusable pin must leave the session with no policy, got %q", decision)
	}
	if !strings.Contains(logs.String(), "invalid parameters") {
		t.Fatalf("the refusal must say why the pin was not honoured:\n%s", logs.String())
	}
}

// PR1b: a valid persisted pin still applies, or PR1 would pass for the wrong
// reason — a guard that refuses everything is indistinguishable from a guard
// that works, and ADR-0014's shadowed-payee defect is exactly that shape.
//
// PASS: a pin with usable parameters comes back as a live edit carrying them.
// FAIL: nil, meaning the refusal above swallowed the legitimate case too.
func TestPR1b_AValidPersistedPinIsStillHonoured(t *testing.T) {
	s, _ := policyServer(t, Config{})
	if err := s.cfg.Store.SetPin(ledger.Pin{
		SessionID: "sess-ok", Policy: policy.Name,
		Trigger: 150000, Keep: 3, Decision: string(policy.Applied), At: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	edit, decision, _ := s.decidePolicy("sess-ok", policy.BetaFeature, false, "claude-opus-5", 200000)
	if edit == nil || edit.TriggerTokens != 150000 || edit.KeepLast != 3 {
		t.Fatalf("a valid pin must be restored exactly: %+v", edit)
	}
	if decision != policy.Applied {
		t.Fatalf("a restored pin keeps its decision, got %q", decision)
	}
}

// PR2: a selection the proxy cannot apply is refused and named.
//
// The learner's catalog holds a TTL family whose Live value is a client
// setting. Nothing in the proxy can set it on the client's behalf, so the
// session must run with no proxy policy — and the operator has to be told,
// because a policy file that named a winner and a proxy that applies nothing
// otherwise look identical from outside.
//
// PASS: policyFromFile returns nil and logs that the selection is a client
// setting, naming the candidate.
// FAIL: a non-nil edit, or silence. Silence is the worse half: the operator
// ran replay learn, got a selection, and has no way to know it never ran.
func TestPR2_AClientSideSelectionIsRefusedAndNamed(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writeSelection(t, file, &learn.Candidate{
		Name: "ttl(1h)", Family: learn.FamilyTTL, Live: "cache_control ttl on the client",
		TTL: time.Hour,
	})
	s, logs := policyServer(t, Config{PolicyFile: file})

	edit, generated := s.policyFromFile("sess-ttl", "")

	if edit != nil {
		t.Fatalf("a TTL selection is not something this proxy can apply, got %+v", edit)
	}
	if !generated.IsZero() {
		t.Fatalf("a refused selection carries no generation time, got %s", generated)
	}
	got := logs.String()
	if !strings.Contains(got, "client setting") || !strings.Contains(got, "ttl(1h)") {
		t.Fatalf("the refusal must name the candidate and say it is a client setting:\n%s", got)
	}
}

// PR3: a selection whose parameters do not validate is rejected.
//
// The policy file is JSON on disk. A hand-edit, a truncated write or a future
// learner that emits a different shape can put TriggerTokens=0 in it, and that
// value applied live is a context-edit that clears at every prompt.
//
// PASS: policyFromFile returns nil and logs the rejection.
// FAIL: the edit comes back, which is the file's contents being trusted as
// parameters without being checked.
func TestPR3_AnInvalidSelectionFromTheFileIsRejected(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writeSelection(t, file, &learn.Candidate{
		Name: "context-edit(keep=6,trigger=0)", Family: learn.FamilyContextEdit,
		ContextEdit: &analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: 0},
	})
	s, logs := policyServer(t, Config{PolicyFile: file})

	edit, generated := s.policyFromFile("sess-invalid", "")

	if edit != nil {
		t.Fatalf("trigger=0 must never become a live policy, got %+v", edit)
	}
	if !generated.IsZero() {
		t.Fatalf("a rejected selection carries no generation time, got %s", generated)
	}
	if !strings.Contains(logs.String(), "rejected") {
		t.Fatalf("the rejection must be logged:\n%s", logs.String())
	}
}

// PR3b: a valid selection from the same path still comes through, with the
// file's generation time attached. Without this, PR2 and PR3 are satisfied by
// a policyFromFile that returns nil unconditionally.
//
// PASS: the edit carries the file's parameters and a non-zero generation time.
// FAIL: nil, or a zero generation time, which is what ties a revert to the file
// it happened under.
func TestPR3b_AValidSelectionFromTheFileIsApplied(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writeSelection(t, file, &learn.Candidate{
		Name: "context-edit(keep=6,trigger=200000)", Family: learn.FamilyContextEdit,
		ContextEdit: &analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: 200000},
	})
	s, _ := policyServer(t, Config{PolicyFile: file})

	edit, generated := s.policyFromFile("sess-good", "")

	if edit == nil || edit.TriggerTokens != 200000 || edit.KeepLast != 6 {
		t.Fatalf("a valid selection must be applied exactly: %+v", edit)
	}
	if generated.IsZero() {
		t.Fatal("a valid selection must carry the file's generation time, or a revert cannot be tied to it")
	}
}
