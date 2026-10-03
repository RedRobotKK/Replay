package stateledger

import "testing"

// Contradicting a claim does not establish its replacement.
//
// This is the invariant the whole package exists for. An agent that finds a
// claim false has learned one thing, not two: the old reading is dead, and
// what is true instead is a separate question that may stay open for a long
// time. A ledger that fills the gap in order to have something to display is
// worse than no ledger, because it launders an open question into a finding.
func TestContradictionDoesNotInventAReplacement(t *testing.T) {
	l := New("s1")
	c := l.Claim("updates.jsonl may be the authoritative usage record", "inherited")
	chk := l.Check(c, Dispositive, "grok usage <session>", "Error: No usage recorded", Contradicts)
	l.Revise(c, chk, Contradicted, nil) // nil replacement: still unknown

	got := l.ClaimByID(c)
	if got.Standing != Contradicted {
		t.Errorf("standing = %q, want %q", got.Standing, Contradicted)
	}
	if got.Replacement != nil {
		t.Errorf("a replacement was recorded (%q) from a contradiction alone", *got.Replacement)
	}
	if !l.HasOpenQuestion(c) {
		t.Error("contradicting a claim with no replacement did not leave an open question; " +
			"the ledger has quietly closed something it never established")
	}
}

// A check that is merely executable is not dispositive.
//
// Running a command and getting output is not the same as settling a
// proposition. The ledger must refuse to mark a claim resolved on the strength
// of a check that cannot settle it, however cleanly that check ran.
func TestOnlyADispositiveCheckCanSettleAClaim(t *testing.T) {
	l := New("s1")
	c := l.Claim("no other reader exists on any branch", "inherited")

	weak := l.Check(c, Executable, "git branch -a", "a list of branch names", Supports)
	if err := l.Settle(c, weak); err == nil {
		t.Error("an Executable check was allowed to settle a claim; running a command " +
			"is not proof that the command could answer the question")
	}
	loc := l.Check(c, Locates, "cmd/replay/grok.go header comment", "prose about ticks", Supports)
	if err := l.Settle(c, loc); err == nil {
		t.Error("a Locates check was allowed to settle a claim")
	}
	strong := l.Check(c, Dispositive, "git log --all -- cmd/replay/grok.go", "commit 6074075", Contradicts)
	if err := l.Settle(c, strong); err != nil {
		t.Errorf("a Dispositive check could not settle a claim: %v", err)
	}
}

// Provenance survives revision. The superseded standing is still readable.
func TestRevisionPreservesTheEarlierStanding(t *testing.T) {
	l := New("s1")
	c := l.Claim("records are cumulative", "inherited")
	k := l.Check(c, Dispositive, "count within-session decreases", "359 of 1040 decrease", Contradicts)
	l.Revise(c, k, Contradicted, nil)

	h := l.History(c)
	if len(h) < 2 {
		t.Fatalf("history has %d entries, want at least the original standing and the revision", len(h))
	}
	if h[0].Standing != Asserted {
		t.Errorf("the original standing was overwritten; first entry is %q", h[0].Standing)
	}
	if h[len(h)-1].Standing != Contradicted {
		t.Errorf("latest standing = %q, want %q", h[len(h)-1].Standing, Contradicted)
	}
	if h[len(h)-1].Because != k {
		t.Error("the revision does not name the check that caused it")
	}
}

// An unresolved replacement is rendered as unresolved, not omitted.
//
// The failure this guards is a renderer that shows only what is known and so
// reads as though nothing is outstanding.
func TestRenderShowsTheOpenQuestion(t *testing.T) {
	l := New("s1")
	c := l.Claim("updates.jsonl may be the authoritative usage record", "inherited")
	k := l.Check(c, Dispositive, "grok usage <session>", "Error: No usage recorded", Contradicts)
	l.Revise(c, k, Contradicted, nil)
	l.Open(c, "what updates.jsonl records instead is unresolved")

	out := l.Render()
	for _, want := range []string{"CONTRADICTED", "UNRESOLVED", "grok usage", "No usage recorded"} {
		if !contains(out, want) {
			t.Errorf("rendering omits %q:\n%s", want, out)
		}
	}
}

// A claim with no origin is still recordable, but says so.
func TestUnknownOriginIsExplicit(t *testing.T) {
	l := New("s1")
	c := l.Claim("something believed", "")
	if got := l.ClaimByID(c).Origin; got != OriginUnknown {
		t.Errorf("origin = %q, want %q; an unrecorded origin must be visible, not blank", got, OriginUnknown)
	}
}

// A claim that was resolved can be re-opened by later evidence.
//
// Mutation M6 made Revise keep an existing replacement whenever the new one
// was nil, so a claim settled early kept its answer even after later evidence
// contradicted it. Every other test still passed, because none of them revised
// the same claim twice. Knowledge moves in both directions and the ledger has
// to let it.
func TestLaterEvidenceCanReopenASettledClaim(t *testing.T) {
	l := New("s1")
	c := l.Claim("the scale is 10^10 ticks per USD", "inherited")

	first := l.Check(c, Dispositive, "read the vendor guide", "the guide states 10^10", Supports)
	repl := "10^10 ticks per USD, per the guide"
	l.Revise(c, first, Superseded, &repl)
	if l.ClaimByID(c).Replacement == nil {
		t.Fatal("the first revision did not record its replacement")
	}

	second := l.Check(c, Dispositive, "compare one session's ticks against the invoice", "no invoice available", Contradicts)
	l.Revise(c, second, Contradicted, nil)

	got := l.ClaimByID(c)
	if got.Replacement != nil {
		t.Errorf("a stale replacement survived a later contradiction: %q. "+
			"Later evidence must be able to re-open a claim, or the ledger "+
			"records only the first answer it ever had.", *got.Replacement)
	}
	if !l.HasOpenQuestion(c) {
		t.Error("re-opening a claim left no open question")
	}
	if len(l.History(c)) != 3 {
		t.Errorf("history has %d entries, want 3: asserted, superseded, contradicted", len(l.History(c)))
	}
}
