package advisor

import (
	"testing"
	"time"
)

// What the reader said and what the verifier computed are two facts with
// different authors, and they were one field until 2026-09-23.
//
// The consequence, measured rather than argued: `appliedIDs` read Verified and
// NotVerified back as `applied=true`, and those two statuses are produced by
// track and by nothing else. A status the pre-8f32a31 verifier had inferred
// from corpus drift therefore stood in for a human decision forever after. On
// one real advice.json, 147 records with ZERO reader dispositions ever
// recorded, regenerating moved a suggestion from "not verified" to "verified"
// with nobody having marked anything.
//
// ApplyDecisions is the display half of the split: the decision is what
// persists, and this puts it back on the status where the verifier is silent.
// See ADR-0027.

// TD1: the reader's decision fills the gap where track said nothing.
func TestTD1_ADecisionShowsWhereTheVerifierIsSilent(t *testing.T) {
	got := ApplyDecisions([]Suggestion{
		{ID: "marked", Status: Pending},
		{ID: "refused", Status: Pending},
		{ID: "untouched", Status: Pending},
	}, map[string]Decision{
		"marked":  DecisionApplied,
		"refused": DecisionDismissed,
	})
	want := map[string]Status{"marked": Applied, "refused": Dismissed, "untouched": Pending}
	if len(got) != len(want) {
		t.Fatalf("got %d suggestions, want %d", len(got), len(want))
	}
	for _, s := range got {
		if s.Status != want[s.ID] {
			t.Errorf("%s is %q, want %q", s.ID, s.Status, want[s.ID])
		}
	}
}

// TD2: a measurement is never overwritten by a decision.
//
// Verified and NotVerified are what the reader's own mark produced, so
// replacing them with "applied" would walk the lifecycle backwards. AdviceOnly
// is a statement about what this tool can observe at all, true whatever
// anybody did, and `advise`'s coverage footer counts it off these strings: "N
// of M are advice only" stops being a coverage figure the moment a keystroke
// can change N.
func TestTD2_ADecisionDoesNotOverwriteAMeasurement(t *testing.T) {
	in := []Suggestion{
		{ID: "verified", Status: Verified, RealizedShare: 0.2},
		{ID: "notverified", Status: NotVerified, RealizedShare: 0.01},
		{ID: "unobservable", Status: AdviceOnly},
	}
	d := map[string]Decision{
		"verified":     DecisionApplied,
		"notverified":  DecisionApplied,
		"unobservable": DecisionDismissed,
	}
	want := map[string]Status{"verified": Verified, "notverified": NotVerified, "unobservable": AdviceOnly}
	for _, s := range ApplyDecisions(in, d) {
		if s.Status != want[s.ID] {
			t.Errorf("%s is %q, want %q: the verifier's own answer was overwritten by a keystroke", s.ID, s.Status, want[s.ID])
		}
	}
}

// TD3: no decisions leaves every status exactly as computed.
//
// The common case, and the one the whole defect lived in: with nobody having
// marked anything, nothing may look marked.
func TestTD3_NoDecisionsChangesNothing(t *testing.T) {
	// Snapshotted, not read back off the input. ApplyDecisions edits the slice
	// it is given, so comparing the result against in[i] would be comparing a
	// value with itself and would pass however badly the function behaved.
	want := []Status{Pending, Verified}
	for _, d := range []map[string]Decision{nil, {}, {"other": DecisionApplied}} {
		in := []Suggestion{{ID: "a", Status: Pending}, {ID: "b", Status: Verified}}
		for i, s := range ApplyDecisions(in, d) {
			if s.Status != want[i] {
				t.Errorf("with decisions %+v, %s became %q, was %q", d, s.ID, s.Status, want[i])
			}
		}
	}
}

// TD4: the lifecycle survives a round trip.
//
// The narrow repair to this defect is to restrict the applied set to
// Status == Applied, and it regresses exactly here. `advise` recomputes every
// status each run, so the moment track turns the reader's Applied into
// Verified, the next run reads Verified, decides it is not Applied, passes
// applied=false, and track returns Pending. The reader's keystroke is destroyed
// by the verifier acting on it.
//
// Here the decision is what persists, so run two is handed the same map run one
// was, and the suggestion holds its ground.
func TestTD4_PendingToAppliedToVerifiedSurvivesTheRoundTrip(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tools := []string{"Bash", "Idle1", "Idle2", "Idle3", "Idle4", "Idle5", "Idle6"}
	var obs []Observation
	add := func(i int, defined []string) {
		ob, ok := Observe(synthetic(base.AddDate(0, 0, i), defined, []string{"Bash", "Bash", "Bash"}, nil, 200))
		if !ok {
			t.Fatal("must calibrate")
		}
		obs = append(obs, ob)
	}
	for i := 0; i < 4; i++ {
		add(i, tools)
	}
	sid := id(KindUnusedTools, "built-in")

	// Run one: the reader has just pressed `a`. Nothing has moved yet, so
	// track has nothing to say, and the mark must still be visible.
	decisions := map[string]Decision{sid: DecisionApplied}
	applied := map[string]bool{sid: true}
	s := kinds(ApplyDecisions(Suggest(obs, applied), decisions))[KindUnusedTools]
	if s.Status != Applied {
		t.Fatalf("a suggestion the reader just marked reads back as %q; the keystroke is invisible from the next run onward", s.Status)
	}

	// Run two: they did the work, and the same decision map is what is on
	// disk. This is the step the narrow repair loses.
	add(4, []string{"Bash", "Idle1"})
	add(5, []string{"Bash", "Idle1"})
	s = kinds(ApplyDecisions(Suggest(obs, applied), decisions))[KindUnusedTools]
	if s.Status != Verified {
		t.Fatalf("the lifecycle did not reach verified: %q. The reader's decision was lost between runs", s.Status)
	}
	if s.RealizedShare <= 0 {
		t.Fatalf("verified with no realized saving: %+v", s)
	}

	// Run three: verified stays verified. Nothing about re-reading the same
	// decision may demote it.
	s = kinds(ApplyDecisions(Suggest(obs, applied), decisions))[KindUnusedTools]
	if s.Status != Verified {
		t.Fatalf("re-reading the same decision demoted a verified suggestion to %q", s.Status)
	}
}

// TD5: a dismissal never reaches the verifier.
//
// A dismissal is a decision and it is not an application. The reader said they
// were not doing it, so nothing about the corpus moved, and handing it to track
// as applied=true would rebuild this defect out of a different constant. The
// caller is what enforces this, so the assertion here is the consequence: with
// a dismissal and no application, the drop that would otherwise verify does not.
func TestTD5_ADismissalDoesNotVerifyAnything(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tools := []string{"Bash", "Idle1", "Idle2", "Idle3", "Idle4", "Idle5", "Idle6"}
	var obs []Observation
	add := func(i int, defined []string) {
		ob, ok := Observe(synthetic(base.AddDate(0, 0, i), defined, []string{"Bash", "Bash", "Bash"}, nil, 200))
		if !ok {
			t.Fatal("must calibrate")
		}
		obs = append(obs, ob)
	}
	for i := 0; i < 4; i++ {
		add(i, tools)
	}
	add(4, []string{"Bash", "Idle1"})
	add(5, []string{"Bash", "Idle1"})
	sid := id(KindUnusedTools, "built-in")

	// Positive control: this corpus DOES verify when the reader applied it.
	// Without it, the assertion below would pass over a suggestion that could
	// never have verified for reasons of its own.
	if s := kinds(Suggest(obs, map[string]bool{sid: true}))[KindUnusedTools]; s.Status != Verified {
		t.Fatalf("control: this corpus should verify when applied, got %q", s.Status)
	}

	s := kinds(ApplyDecisions(Suggest(obs, nil), map[string]Decision{sid: DecisionDismissed}))[KindUnusedTools]
	if s.Status != Dismissed {
		t.Fatalf("a dismissed suggestion reads as %q; the reader answered and was not heard", s.Status)
	}
	if s.RealizedShare != 0 {
		t.Errorf("a dismissed suggestion carries a realized saving of %.3f: nothing was applied, so nothing was realized", s.RealizedShare)
	}
}
