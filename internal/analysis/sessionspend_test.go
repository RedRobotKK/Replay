package analysis

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// What a session cost is what every lane of it cost.
//
// `replay cost` priced MainLane(session) and nothing else. On the corpus this
// was found on that dropped 21,854 of 60,401 requests — 36.2% — concentrated in
// 8 files out of 1,812, which are the fan-out sessions and therefore the
// expensive ones. `replay cost` reported $3,721 where `replay burn`, pricing
// every lane, reported $10,499.
//
// The dropped requests are not duplicates. A session's in-file lanes and its
// subagents/*.jsonl files share zero request ids, checked on the largest
// session in the corpus against a 200-file sample of its lanes. What IS
// duplicated is small and known: 428 ids of 59,967 appear in more than one
// lane, the re-render `replay cost` already discloses.
//
// So the fix has two halves and both are load-bearing: count every lane, and
// count each request once.

func laneWithIDs(id string, sidechain bool, ids ...string) *transcript.Lane {
	l := &transcript.Lane{ID: id, Sidechain: sidechain}
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, rid := range ids {
		l.Requests = append(l.Requests, &transcript.Request{
			ID:        rid,
			Model:     "claude-opus-5",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Usage:     transcript.Usage{Input: 1000, CacheRead: 4000, Output: 100},
		})
	}
	return l
}

// SS1: every lane is counted, not just the one MainLane picks.
func TestSS1_EveryLaneIsPriced(t *testing.T) {
	s := &transcript.Session{ID: "sess", Source: transcript.SourceTranscript,
		Lanes: []*transcript.Lane{
			laneWithIDs("main", false, "a", "b"),
			laneWithIDs("sub-1", true, "c", "d", "e"),
			laneWithIDs("sub-2", true, "f"),
		}}

	main := AsRun(MainLane(s))
	all := AsRunSession(s)

	if all.Requests != 6 {
		t.Errorf("a session of 6 requests across 3 lanes priced %d", all.Requests)
	}
	if main.Requests != 2 {
		t.Fatalf("the fixture's main lane holds %d requests, want 2; this test cannot "+
			"show the gap it is about", main.Requests)
	}
	if all.CostUSD <= main.CostUSD {
		t.Errorf("all lanes cost $%.6f, the main lane alone $%.6f. Pricing every lane "+
			"must cost more than pricing one of them", all.CostUSD, main.CostUSD)
	}
	// Six identical requests, so the total is exactly three times the two-request
	// main lane. An aggregate that merely rose is not evidence it summed.
	if got, want := all.CostUSD, main.CostUSD*3; !nearly(got, want) {
		t.Errorf("six identical requests cost $%.6f; three times the main lane's two "+
			"is $%.6f", got, want)
	}
}

// SS2: a request appearing in more than one lane is counted once.
//
// A sub-agent lane re-renders some of its parent's requests. Summing lanes
// naively bills those twice, which would replace an undercount with an
// overcount and be harder to notice because the number moved the popular way.
func TestSS2_ARequestInTwoLanesIsCountedOnce(t *testing.T) {
	s := &transcript.Session{ID: "sess", Source: transcript.SourceTranscript,
		Lanes: []*transcript.Lane{
			laneWithIDs("main", false, "a", "b", "c"),
			laneWithIDs("sub", true, "b", "c", "d"), // b and c re-rendered
		}}
	got := AsRunSession(s)
	if got.Requests != 4 {
		t.Errorf("four distinct requests across two lanes priced %d", got.Requests)
	}
	if got.Duplicated != 2 {
		t.Errorf("two re-rendered requests reported as %d duplicates", got.Duplicated)
	}
	single := AsRunSession(&transcript.Session{ID: "s", Source: transcript.SourceTranscript,
		Lanes: []*transcript.Lane{laneWithIDs("main", false, "a", "b", "c", "d")}})
	if !nearly(got.CostUSD, single.CostUSD) {
		t.Errorf("four requests over two lanes cost $%.6f, over one lane $%.6f",
			got.CostUSD, single.CostUSD)
	}
}

// SS3: a request with no id cannot be deduplicated and is not silently dropped.
//
// Absence is not a duplicate. A request the transcript did not identify is
// still a request the provider served, and treating unidentifiable as
// already-seen would undercount exactly the traffic nobody can check.
func TestSS3_AnUnidentifiedRequestIsStillCounted(t *testing.T) {
	s := &transcript.Session{ID: "sess", Source: transcript.SourceTranscript,
		Lanes: []*transcript.Lane{
			laneWithIDs("main", false, "", ""),
			laneWithIDs("sub", true, ""),
		}}
	got := AsRunSession(s)
	if got.Requests != 3 {
		t.Errorf("three unidentified requests priced %d", got.Requests)
	}
	if got.Duplicated != 0 {
		t.Errorf("unidentified requests reported as %d duplicates", got.Duplicated)
	}
}

// SS4: a single-lane session is unchanged, so every flat corpus prices exactly
// as it did before.
func TestSS4_ASingleLaneSessionIsUnchanged(t *testing.T) {
	s := &transcript.Session{ID: "sess", Source: transcript.SourceTranscript,
		Lanes: []*transcript.Lane{laneWithIDs("main", false, "a", "b", "c")}}
	if got, want := AsRunSession(s), AsRun(MainLane(s)); !nearly(got.CostUSD, want.CostUSD) ||
		got.Requests != want.Requests {
		t.Errorf("a one-lane session priced $%.6f over %d requests, was $%.6f over %d",
			got.CostUSD, got.Requests, want.CostUSD, want.Requests)
	}
}

// SS5: and it reports how many lanes it covered, because a total whose scope a
// reader cannot see is the defect this replaces.
func TestSS5_TheTotalSaysHowManyLanesItCovers(t *testing.T) {
	s := &transcript.Session{ID: "sess", Source: transcript.SourceTranscript,
		Lanes: []*transcript.Lane{
			laneWithIDs("main", false, "a"),
			laneWithIDs("sub-1", true, "b"),
			laneWithIDs("sub-2", true, "c"),
		}}
	if got := AsRunSession(s).Lanes; got != 3 {
		t.Errorf("a three-lane session reports %d lanes", got)
	}
}

func nearly(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

// SS6: no session is not a crash and not a cost.
//
// forEachSession hands its callback a nil session when a transcript fails to
// parse — 9 of 1,812 files on the corpus this was written against — so a nil
// here is an ordinary first-class state rather than a defensive flourish, and
// it is the state in which returning a cost would be worst.
func TestSS6_ANilSessionCostsNothingAndSaysSo(t *testing.T) {
	got := AsRunSession(nil)
	if got.CostUSD != 0 || got.Requests != 0 || got.Lanes != 0 {
		t.Errorf("a nil session priced $%.6f over %d requests in %d lanes",
			got.CostUSD, got.Requests, got.Lanes)
	}
	if got.Name != "as-run" {
		t.Errorf("a nil session produced a result named %q, which a caller "+
			"matching on the name would skip", got.Name)
	}
	if reps := AnalyzeEveryLane(nil); reps != nil {
		t.Errorf("a nil session produced %d lane report(s)", len(reps))
	}
}

// SS7: a session with no lanes at all is zero, not a panic.
//
// ParseClaudeCode refuses a transcript with no lanes, so this arrives only
// from a caller building one — which the corpus tooling does.
func TestSS7_ASessionWithNoLanesIsZero(t *testing.T) {
	got := AsRunSession(&transcript.Session{ID: "empty", Source: transcript.SourceTranscript})
	if got.Requests != 0 || got.Lanes != 0 || got.CostUSD != 0 {
		t.Errorf("an empty session priced $%.6f over %d requests in %d lanes",
			got.CostUSD, got.Requests, got.Lanes)
	}
}

// One labelled epoch beside an unlabelled request is one epoch.
//
// toolsWireHash returns "" for any body with no tool set — a first request, a
// tool-free sub-agent lane — and the empty string is absence, not a third
// epoch. Counting it as one reports a session as spanning two epochs because
// one request carried no tools, which would put a "not one as-run" warning on
// the most ordinary session there is.
func TestAsRunSession_AnUnlabelledRequestIsNotASecondEpoch(t *testing.T) {
	s := &transcript.Session{Lanes: []*transcript.Lane{{ID: "main", Requests: []*transcript.Request{
		{ID: "a", Epoch: "", Usage: transcript.Usage{Input: 10}},
		{ID: "b", Epoch: "abc123", Usage: transcript.Usage{Input: 10}},
	}}}}
	if AsRunSession(s).MixedEpochs {
		t.Fatal("one labelled epoch and one unlabelled request reported as two epochs; " +
			"absence is not a value (ADR-0018)")
	}
}

// Two different labels really are two epochs.
//
// The control for the test above. Without it, skipping the empty string could
// be widened until nothing is ever mixed and the field silently stops meaning
// anything.
func TestAsRunSession_TwoLabelsAreMixedEpochs(t *testing.T) {
	s := &transcript.Session{Lanes: []*transcript.Lane{{ID: "main", Requests: []*transcript.Request{
		{ID: "a", Epoch: "abc123", Usage: transcript.Usage{Input: 10}},
		{ID: "b", Epoch: "def456", Usage: transcript.Usage{Input: 10}},
	}}}}
	if !AsRunSession(s).MixedEpochs {
		t.Fatal("two different tool-set labels reported as one epoch; the sum is then " +
			"presented as one as-run when it is not")
	}
}

// The same label on every request is one epoch, however many requests carry it.
func TestAsRunSession_OneLabelRepeatedIsOneEpoch(t *testing.T) {
	s := &transcript.Session{Lanes: []*transcript.Lane{{ID: "main", Requests: []*transcript.Request{
		{ID: "a", Epoch: "abc123", Usage: transcript.Usage{Input: 10}},
		{ID: "b", Epoch: "abc123", Usage: transcript.Usage{Input: 10}},
		{ID: "c", Epoch: "abc123", Usage: transcript.Usage{Input: 10}},
	}}}}
	if AsRunSession(s).MixedEpochs {
		t.Fatal("one label on three requests reported as mixed")
	}
}

// A session nothing labelled is not mixed, and this is the common case.
//
// --freeze-prefix is off by default, so every ledger written without it has no
// epoch anywhere. If absence counted, every existing ledger would read as
// mixed the moment one request differed from another in having no tools.
func TestAsRunSession_NoLabelsAnywhereIsNotMixed(t *testing.T) {
	s := &transcript.Session{Lanes: []*transcript.Lane{{ID: "main", Requests: []*transcript.Request{
		{ID: "a", Usage: transcript.Usage{Input: 10}},
		{ID: "b", Usage: transcript.Usage{Input: 10}},
	}}}}
	if AsRunSession(s).MixedEpochs {
		t.Fatal("a session with no epoch labels at all reported as mixed; that is every " +
			"ledger written before --freeze-prefix existed")
	}
}
