package proxy

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
)

func breachingReads() analysis.ReReads {
	return analysis.ReReads{ReadsAfterClear: 10, RepeatedAfterClear: 9}
}

func trialOf(revertAfter int) TrialSettings {
	return TrialSettings{Share: 1, ReReadRate: 0.5, RevertAfter: revertAfter}
}

// pinSession puts a session in the state noteBreach expects: pinned to a
// policy, with the parameters that policy was generated for.
func pinSession(s *stats, id string, edit *policy.ContextEdit, gen time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.session(id)
	st.policy, st.edit, st.generated = policy.Applied, edit, gen
}

// The revert record names a specific trigger and keep-last. Counting breaches
// in one process-wide total means evidence gathered against one set of
// parameters reverts a different set: `replay learn` writes a new policy, one
// session breaches on it, and the old policy's breach carries it over the line.
func TestBreachesDoNotCarryAcrossPolicies(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := newStats()
	old := &policy.ContextEdit{TriggerTokens: 20_000, KeepLast: 3}
	next := &policy.ContextEdit{TriggerTokens: 50_000, KeepLast: 5}
	genOld := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	genNew := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)

	pinSession(s, "s-old", old, genOld)
	s.noteBreach(store, trialOf(2), "s-old", old, breachingReads(), genOld)

	pinSession(s, "s-new", next, genNew)
	line := s.noteBreach(store, trialOf(2), "s-new", next, breachingReads(), genNew)

	if strings.Contains(line, "reverted") {
		t.Fatalf("one breach on the new policy reverted it using the old policy's evidence: %s", line)
	}
	if rev, ok := store.Revert(); ok {
		t.Fatalf("a revert was written on one session's evidence: %+v", rev)
	}
}

// Two breaches against the same parameters still revert. Scoping the counter
// must not switch the guardrail off.
func TestTwoBreachesOnOnePolicyStillRevert(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := newStats()
	edit := &policy.ContextEdit{TriggerTokens: 20_000, KeepLast: 3}
	gen := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	pinSession(s, "s1", edit, gen)
	s.noteBreach(store, trialOf(2), "s1", edit, breachingReads(), gen)
	pinSession(s, "s2", edit, gen)
	line := s.noteBreach(store, trialOf(2), "s2", edit, breachingReads(), gen)

	if !strings.Contains(line, "reverted") {
		t.Fatalf("two breaches on one policy must revert it: %s", line)
	}
	rev, ok := store.Revert()
	if !ok {
		t.Fatal("revert not persisted")
	}
	if rev.Trigger != 20_000 {
		t.Fatalf("the revert names the wrong policy: %+v", rev)
	}
}

// Reverting one policy must not make the next one un-revertable. `reverted`
// was a single flag, so once any policy had been reverted a newer one could
// breach forever with the guardrail silently disarmed.
func TestANewerPolicyCanStillBeReverted(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := newStats()
	old := &policy.ContextEdit{TriggerTokens: 20_000, KeepLast: 3}
	next := &policy.ContextEdit{TriggerTokens: 50_000, KeepLast: 5}
	genOld := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	genNew := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)

	for _, id := range []string{"a1", "a2"} {
		pinSession(s, id, old, genOld)
		s.noteBreach(store, trialOf(2), id, old, breachingReads(), genOld)
	}
	for _, id := range []string{"b1", "b2"} {
		pinSession(s, id, next, genNew)
		s.noteBreach(store, trialOf(2), id, next, breachingReads(), genNew)
	}
	rev, ok := store.Revert()
	if !ok {
		t.Fatal("revert not persisted")
	}
	if rev.Trigger != 50_000 {
		t.Fatalf("the newer policy breached twice and was never reverted; the record still names %d", rev.Trigger)
	}
}

// A session breaching twice is still one breach: the guardrail counts sessions.
func TestOneSessionCountsOnce(t *testing.T) {
	dir := t.TempDir()
	store, _ := ledger.Open(dir)
	s := newStats()
	edit := &policy.ContextEdit{TriggerTokens: 20_000, KeepLast: 3}
	gen := time.Now()
	pinSession(s, "s1", edit, gen)
	s.noteBreach(store, trialOf(2), "s1", edit, breachingReads(), gen)
	if line := s.noteBreach(store, trialOf(2), "s1", edit, breachingReads(), gen); line != "" {
		t.Fatalf("the same session breached twice and was counted twice: %s", line)
	}
	if _, ok := store.Revert(); ok {
		t.Fatal("one session reverted a policy on its own")
	}
}
