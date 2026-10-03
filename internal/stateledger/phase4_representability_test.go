package stateledger

import (
	"fmt"
	"testing"
)

// Campaign Phase 4 asks whether Replay can represent partial, evolving, stale,
// contradictory and inferred work state "without collapsing it into a false
// binary", and instructs: do NOT invent a new state taxonomy unless the
// existing semantics demonstrably cannot express the distinction.
//
// This is that demonstration, written before any taxonomy work. It takes the
// eleven states the campaign names and asks, for each, whether this package can
// express it AND distinguish it from all ten others. A state that cannot be
// told apart from another state is not represented, it is merely spelled.
//
// NOT a test of whether the distinctions are USEFUL. Expressibility only.

// fingerprint is everything an outside consumer can read about one claim.
// Two states that produce the same fingerprint are indistinguishable.
func fingerprint(l *Ledger, id string) string {
	c := l.ClaimByID(id)
	if c == nil {
		return "NO-SUCH-CLAIM"
	}
	repl := "nil"
	if c.Replacement != nil {
		repl = "set:" + *c.Replacement
	}
	best := "none"
	for _, k := range l.checks {
		if k.ClaimID == id {
			best = fmt.Sprintf("%s/%s", k.Kind, k.Outcome)
		}
	}
	return fmt.Sprintf("standing=%s replacement=%s open=%v history=%d check=%s",
		c.Standing, repl, l.HasOpenQuestion(id), len(c.History), best)
}

func TestPhase4_TheElevenStatesAreExpressibleAndDistinct(t *testing.T) {
	l := New("phase4")

	// Each builder returns the claim id for one of the campaign's states.
	states := []struct {
		name  string
		build func() string
	}{
		{"directly observed / known", func() string {
			id := l.Claim("the binary writes a cost index", "observed in cost.go")
			k := l.Check(id, Dispositive, "read the write site", "found", Supports)
			if err := l.Settle(id, k); err != nil {
				t.Fatalf("a dispositive check could not settle: %v", err)
			}
			l.Revise(id, k, Supported, nil)
			return id
		}},
		{"unknown / never investigated", func() string {
			return l.Claim("the index survives a model rename", "asserted at plan time")
		}},
		{"unresolved / investigated, unsettled", func() string {
			id := l.Claim("the warm path discloses what the cold path does", "plan")
			k := l.Check(id, Executable, "ran the warm suite", "no signal either way", Inconclusive)
			l.Revise(id, k, Unresolved, nil)
			l.Open(id, "needs a dispositive check")
			return id
		}},
		{"pending / action taken, outcome not yet observable", func() string {
			id := l.Claim("the repair closes the disclosure gap", "post-repair")
			l.Check(id, Dispositive, "submitted a corpus re-run", "not back yet", NotRun)
			return id
		}},
		{"contradicted", func() string {
			id := l.Claim("every figure carries a truth tier", "README")
			k := l.Check(id, Dispositive, "read the JSON surface", "no tier field", Contradicts)
			l.Revise(id, k, Contradicted, nil)
			return id
		}},
		{"superseded, replacement known", func() string {
			id := l.Claim("avoidableUsd is the field name", "v1 schema")
			k := l.Check(id, Dispositive, "read the v2 schema", "renamed", Supports)
			repl := "rebilledUsd"
			l.Revise(id, k, Superseded, &repl)
			return id
		}},
		{"stale, formerly established, replacement unknown", func() string {
			id := l.Claim("the corpus is 1363 sessions", "evidence README")
			k := l.Check(id, Dispositive, "recounted", "that was a file count", Contradicts)
			l.Revise(id, k, Superseded, nil)
			return id
		}},
		{"abandoned / dropped without settling", func() string {
			id := l.Claim("a tenant dimension is needed", "a direction memo")
			l.Open(id, "nobody is pursuing this")
			return id
		}},
	}

	ids := map[string]string{}
	for _, s := range states {
		ids[s.name] = s.build()
	}

	// History must survive every revision: a state that was once asserted must
	// not read as though it never was.
	for name, id := range ids {
		if n := len(l.ClaimByID(id).History); n < 1 {
			t.Errorf("%s: history is empty, so the earlier standing is gone", name)
		}
	}

	seen := map[string][]string{}
	for name, id := range ids {
		fp := fingerprint(l, id)
		seen[fp] = append(seen[fp], name)
		t.Logf("%-52s %s", name, fp)
	}

	for fp, names := range seen {
		if len(names) > 1 {
			t.Errorf("INDISTINGUISHABLE: %v all produce %s. The campaign's Phase 4 "+
				"asks for these as separate states and this package cannot tell "+
				"them apart, so a consumer reading a claim cannot either.", names, fp)
		}
	}
}

// A dispositive check is required to settle. An executable one that ran
// cleanly must not close a question it could never answer. This is the
// action-is-not-outcome invariant inside the state model.
func TestPhase4_RunningACheckIsNotAnsweringTheQuestion(t *testing.T) {
	l := New("phase4b")
	id := l.Claim("the cap is enforced", "guards.go")
	k := l.Check(id, Executable, "ran the guard", "no refusal fired", Supports)
	if err := l.Settle(id, k); err == nil {
		t.Fatal("an EXECUTABLE check settled a claim: running a check was treated " +
			"as answering the question, which is the action-equals-outcome defect")
	}
	d := l.Check(id, Dispositive, "read the arming condition", "unreachable", Contradicts)
	if err := l.Settle(id, d); err != nil {
		t.Fatalf("a dispositive check could not settle: %v", err)
	}
}
