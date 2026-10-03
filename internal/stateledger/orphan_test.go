package stateledger

import (
	"strings"
	"testing"
)

// A check recorded against a claim that does not exist was accepted and then
// never rendered: Render walks claims and prints the checks attached to each,
// so a check whose ClaimID matches nothing vanished without a word. The same
// for a revision of an unknown claim, which returned silently and recorded
// nothing. Both are the failure the package exists to prevent, wearing a
// different hat: evidence that was observed and then lost.
//
// The repair keeps the API (callers hand in ids they already hold) and makes
// the loss visible instead: Render ends with an ORPHANED section naming every
// check and every revision that found no claim.
func TestRenderShowsAnOrphanedCheck(t *testing.T) {
	l := New("orphan")
	l.Claim("the only claim", "test")
	l.Check("c9", Executable, "count the records", "1,040 records", Inconclusive)

	out := l.Render()
	for _, want := range []string{"ORPHANED", "c9", "count the records"} {
		if !strings.Contains(out, want) {
			t.Errorf("a check on an unknown claim was dropped from the rendering; missing %q:\n%s", want, out)
		}
	}
}

func TestRevisingAnUnknownClaimIsVisible(t *testing.T) {
	l := New("orphan")
	l.Claim("the only claim", "test")
	k := l.Check("c1", Dispositive, "grok usage <session>", "Error: No usage recorded", Contradicts)
	l.Revise("c7", k, Contradicted, nil)

	if !l.HasOpenQuestion("c7") {
		t.Error("revising a claim that does not exist left no trace; the revision was lost")
	}
	out := l.Render()
	for _, want := range []string{"ORPHANED", "c7"} {
		if !strings.Contains(out, want) {
			t.Errorf("a revision of an unknown claim is not rendered; missing %q:\n%s", want, out)
		}
	}
}
