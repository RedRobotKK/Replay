package proxy

import (
	"strings"
	"testing"
)

// Characterization: the day cap is process-wide. It now says who spent it.
//
// This pins current behaviour rather than asserting desired behaviour. On one
// developer's machine it is correct and cheap - there is one budget because
// there is one person. Behind a shared proxy the same code makes `--max-day-usd`
// an organisation-wide cap: the first developer to run a loop consumes the
// budget and every other developer is refused.
//
// SP-7 has since landed, so the refusal now names the lane that spent it, and
// this file asserts both halves at once: the budget is still shared (SP-6 is
// NOT done, and this is what pins that), and the refusal is attributed (SP-7 IS
// done, and this is its regression guard). Splitting them would let the first
// silently stop being true.
//
// PASS today: one session exhausts the day cap, an unrelated session is
// refused, and the reason names the spender.
// FAIL on the first assertion: SP-6 landed. Replace this with SP-6's own
// acceptance criterion - tenant A exhausting its cap refuses A and not B.
func TestSpendGuard_DayCapIsProcessWideAndAttributed(t *testing.T) {
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

	// SP-7: the refusal names the lane that spent the budget, not the one being
	// refused. Without this an org-wide refusal is an outage with no operator
	// action attached, and with a solo developer it is still the difference
	// between "something stopped" and "the indexing lane did it".
	if !strings.Contains(reason, noisy) {
		t.Errorf("the refusal does not name the lane that spent the budget (SP-7): %q", reason)
	}
	if strings.Contains(reason, quiet) {
		t.Errorf("the refusal blames the lane being refused rather than the one that spent: %q", reason)
	}
	t.Logf("current behaviour, pinned: an unrelated session is refused with %q", reason)
}
