package probe

import (
	"path/filepath"
	"testing"
	"time"
)

// Reading the series back: what changed, and when.
//
// This is the payoff for keeping one. A floor is a fact anyone can copy; a
// dated change in a floor can only be produced by someone who was measuring
// before it. Everything here is about not overstating that.
//
//	T1  a change is only claimed when the brackets cannot both be true
//	T2  readings taken with different methods are never compared
//	T3  an inconclusive reading is not a data point
//	T4  a recent reading suppresses a re-measurement
//	T5  a stale reading does not

func at(days int) string {
	return time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
}

func TestT1_AChangeNeedsDisjointBrackets(t *testing.T) {
	// PASS: overlapping brackets report no change; disjoint ones do, with the
	// date of the first reading that could not agree.
	// FAIL: calling any difference in numbers a change. Two brackets that
	// overlap are both consistent with one unchanged floor, and reporting that
	// as movement is how a series manufactures news.
	overlapping := []Reading{
		{Model: "m", Method: MethodVersion, TakenAt: at(9), Above: 508, AtMost: 512},
		{Model: "m", Method: MethodVersion, TakenAt: at(5), Above: 505, AtMost: 514},
		{Model: "m", Method: MethodVersion, TakenAt: at(1), Above: 509, AtMost: 512},
	}
	if ch := Changes(overlapping); len(ch) != 0 {
		t.Errorf("reported %d change(s) between brackets that all overlap: %+v", len(ch), ch)
	}

	moved := []Reading{
		{Model: "m", Method: MethodVersion, TakenAt: at(9), Above: 508, AtMost: 512},
		{Model: "m", Method: MethodVersion, TakenAt: at(2), Above: 1020, AtMost: 1024},
	}
	ch := Changes(moved)
	if len(ch) != 1 {
		t.Fatalf("reported %d changes across a floor that clearly moved", len(ch))
	}
	if ch[0].At != moved[1].TakenAt {
		t.Errorf("change dated %s, want the first reading that disagreed, %s", ch[0].At, moved[1].TakenAt)
	}
}

func TestT2_DifferentMethodsAreNotCompared(t *testing.T) {
	// The method changed four times in one day and each change moved the
	// numbers. A difference across that boundary says nothing about the world.
	// PASS: no change reported, and the method break is surfaced instead.
	// FAIL: reporting an instrument change as a provider change — the exact
	// error a series exists to prevent.
	// The real pair. On 2026-09-05 the superseded method read fable-5-1 as at
	// most 440 — which looked like a refutation of a documented 512 — and the
	// corrected method reads (508, 512]. Those brackets are disjoint, so
	// comparing across the break would report a floor that moved when nothing
	// moved but the instrument.
	rs := []Reading{
		{Model: "m", Method: "2026-09-05.1", TakenAt: at(9), Above: 387, AtMost: 440},
		{Model: "m", Method: MethodVersion, TakenAt: at(1), Above: 508, AtMost: 512},
	}
	if !disjoint(rs[0], rs[1]) {
		t.Fatal("this fixture is meant to be disjoint; overlapping brackets would pass whatever the code did")
	}
	if ch := Changes(rs); len(ch) != 0 {
		t.Errorf("compared readings taken with different methods: %+v", ch)
	}
	if b := MethodBreaks(rs); len(b) != 1 {
		t.Errorf("a method change between readings must be reported; got %d", len(b))
	}
}

func TestT3_AnInconclusiveReadingIsNotADataPoint(t *testing.T) {
	// PASS: outcomes other than a clean bracket are excluded from comparison.
	// FAIL: treating a failed run as a measurement, which would report a
	// change every time a run went wrong.
	rs := []Reading{
		{Model: "m", Method: MethodVersion, TakenAt: at(9), Above: 508, AtMost: 512},
		{Model: "m", Method: MethodVersion, TakenAt: at(5), Outcome: "non-deterministic"},
		{Model: "m", Method: MethodVersion, TakenAt: at(1), Above: 508, AtMost: 512},
	}
	if ch := Changes(rs); len(ch) != 0 {
		t.Errorf("an inconclusive run was treated as a measurement: %+v", ch)
	}
}

func TestT4_ARecentReadingSuppressesAReMeasurement(t *testing.T) {
	// Probing costs real money at the provider. Measuring again an hour after
	// the last identical reading buys nothing.
	// PASS: a reading inside the window is found and returned.
	// FAIL: paying to rediscover what is already recorded.
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := AppendReading(path, Reading{Model: "m", Above: 508, AtMost: 512}); err != nil {
		t.Fatal(err)
	}
	got, ok := RecentReading(path, "m", 24*time.Hour)
	if !ok {
		t.Fatal("a reading taken seconds ago must be found")
	}
	if got.AtMost != 512 {
		t.Errorf("atMost = %d, want the recorded 512", got.AtMost)
	}
}

func TestT5_AStaleOrForeignReadingDoesNot(t *testing.T) {
	// PASS: nothing returned for another model, an older method, an
	// inconclusive run, or a reading outside the window.
	// FAIL: skipping a measurement on the strength of a reading that does not
	// apply — which silently freezes the series at whatever it last saw.
	path := filepath.Join(t.TempDir(), "s.jsonl")
	for _, r := range []Reading{
		{Model: "other", TakenAt: at(0), Above: 1, AtMost: 2},
		{Model: "m", TakenAt: at(30), Above: 508, AtMost: 512},
		{Model: "m", TakenAt: at(0), Method: "2026-09-05.1", Above: 508, AtMost: 512},
		{Model: "m", TakenAt: at(0), Outcome: "stalled"},
	} {
		if err := AppendReading(path, r); err != nil {
			t.Fatal(err)
		}
	}
	if got, ok := RecentReading(path, "m", 24*time.Hour); ok {
		t.Errorf("suppressed a measurement on a reading that does not apply: %+v", got)
	}
}
