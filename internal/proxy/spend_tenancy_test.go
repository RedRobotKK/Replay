package proxy

import (
	"strings"
	"testing"
)

// Characterization: the day cap is process-wide, and refuses without saying
// who spent it.
//
// This pins current behaviour rather than asserting desired behaviour. On one
// developer's machine it is correct and cheap - there is one budget because
// there is one person. Behind a shared proxy the same code makes `--max-day-usd`
// an organisation-wide cap: the first developer to run a loop consumes the
// budget, and every other developer is then refused by a message that names no
// tenant, so an operator reading it has nothing to act on. The guard that makes
// the product worth buying becomes an outage with no attribution.
//
// SP-5 through SP-8 in docs/requirements.md specify the fix and ADR-0015 gates
// it. This test exists so that claim is mechanically checkable instead of
// merely written down, and so the day it stops being true, the suite says so
// rather than letting a tenancy change land silently.
//
// PASS today: one session exhausts the day cap and an unrelated session is
// refused, with no identity in the reason.
// FAIL: the behaviour changed. That is the SP-6 work landing, not a defect -
// read the message, then replace this test with the two-tenant isolation
// assertion SP-6 names as its acceptance criterion.
func TestSpendGuard_DayCapIsProcessWideAndUnattributed(t *testing.T) {
	// One cap value, deliberately small, so the arithmetic is not the subject.
	g := NewSpendGuard(SpendLimits{DayTokens: 100})

	// Two sessions that share nothing but this process. Behind a team proxy
	// these are two different people.
	const noisy, quiet = "session-of-developer-a", "session-of-developer-b"

	if g.Check(quiet) != "" {
		t.Fatal("the second session must start unrefused, or this test proves nothing")
	}

	// The first burns the whole day budget on its own.
	g.Record(noisy, 100, 0)

	reason := g.Check(quiet)
	if reason == "" {
		t.Fatal("the day cap is no longer process-wide: a session that spent nothing was " +
			"allowed after another exhausted the budget. If SP-6 landed, this is the " +
			"expected outcome - replace this characterization with SP-6's acceptance test " +
			"(tenant A exhausting its cap refuses A and not B).")
	}

	// The second half, and the one that turns a wrong number into an outage:
	// the refusal carries no identity, so nobody can be told whose loop did it.
	if strings.Contains(reason, noisy) {
		t.Fatalf("the refusal now names the spender, which is SP-7 landing. Update this "+
			"test to assert attribution rather than its absence. Got: %q", reason)
	}
	t.Logf("current behaviour, pinned: an unrelated session is refused with %q", reason)
}
