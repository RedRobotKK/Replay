package e005

import "testing"

func load(t *testing.T) ([]Event, map[string]int) {
	t.Helper()
	ev, err := LoadEvents("testdata/events.json")
	if err != nil {
		t.Fatal(err)
	}
	br, err := LoadBreaks("testdata/breaks.json")
	if err != nil {
		t.Fatal(err)
	}
	return ev, br
}

// The frozen corpus accounting, to the event.
//
// docs/evidence/detector-observables-2026-09-26.md is frozen and these are its
// numbers. A change here means either the artifacts moved or the recovered
// procedure drifted, and both are defects rather than new findings.
func TestFrozenCorpusAccounting(t *testing.T) {
	ev, br := load(t)
	r := Score(ev, br)

	want := map[string]int{
		"tools_changed": 1006, "messages_changed": 591,
		"previous_message_not_found": 255, "system_changed": 119,
		"unavailable": 48, "model_changed": 45,
	}
	for k, v := range want {
		if got := r.ClassCounts[k]; got != v {
			t.Errorf("%s = %d, want %d", k, got, v)
		}
	}
	if r.Total != 2064 {
		t.Errorf("total = %d, want 2064", r.Total)
	}
	if got := r.A + r.B + r.D; got != 2064 {
		t.Errorf("A+B+D = %d, want 2064: the partition must exhaust the corpus", got)
	}
	if r.A != 1143 || r.B != 618 || r.D != 303 {
		t.Errorf("A/B/D = %d/%d/%d, want 1143/618/303", r.A, r.B, r.D)
	}
	// 1,761 structural and 303 indeterminate, as the frozen file states.
	if got := r.A + r.B; got != 1761 {
		t.Errorf("structural = %d, want 1761", got)
	}
}

// The load-bearing column. Median change in provider-reported cache-creation
// tokens: +5,005 for tools_changed and 0 for the other three.
//
// This is the result the whole study rests on, and it is the one quantity here
// that is a provider-usage measurement rather than an instrument agreement.
func TestFrozenMedianDeltaWrite(t *testing.T) {
	ev, br := load(t)
	r := Score(ev, br)
	want := map[string]float64{
		"tools_changed": 5005, "messages_changed": 0,
		"system_changed": 0, "model_changed": 0,
	}
	for c, v := range want {
		if got := r.PerClass[c].MedianDWrite; got != v {
			t.Errorf("%s median delta-write = %v, want %v", c, got, v)
		}
	}
}

// The missed-token column, which E005 shows is NOT a materiality proxy: the
// class with the largest median carries no write movement at all.
func TestFrozenMedianMissed(t *testing.T) {
	ev, br := load(t)
	r := Score(ev, br)
	// The frozen table shows 13,482 for tools_changed. The true median is
	// 13,482.5; Python's "%.0f" rounds half to even and printed 13,482. The
	// underlying value is asserted here and the rendering below it, so a
	// future reader does not "correct" one to match the other.
	want := map[string]float64{
		"tools_changed": 13482.5, "messages_changed": 54125,
		"system_changed": 82484, "model_changed": 243095,
	}
	rendered := map[string]string{
		"tools_changed": "13482", "messages_changed": "54125",
		"system_changed": "82484", "model_changed": "243095",
	}
	for c, want := range rendered {
		if got := RenderMedian(r.PerClass[c].MedianMissed); got != want {
			t.Errorf("%s median missed renders as %s, want %s (the frozen table's figure)", c, got, want)
		}
	}
	for c, v := range want {
		if got := r.PerClass[c].MedianMissed; got != v {
			t.Errorf("%s median missed = %v, want %v", c, got, v)
		}
	}
}

// The column labelled "economic" in the frozen table.
//
// It counts oracle events whose (file, second) collided with a Replay break.
// That is agreement between two instruments, not a provider-usage measure, and
// the erratum in the frozen file says so. The numbers are pinned here under
// their real definition so that nobody re-derives them from a different one:
// computing it as "cur.write > 0" yields 1006/590/119/45 and not these.
func TestFrozenAgreementColumn(t *testing.T) {
	ev, br := load(t)
	r := Score(ev, br)
	want := map[string]int{
		"tools_changed": 932, "messages_changed": 163,
		"system_changed": 35, "model_changed": 13,
	}
	for c, v := range want {
		if got := r.PerClass[c].Economic; got != v {
			t.Errorf("%s agreement count = %d, want %d", c, got, v)
		}
	}
}

// The conservative alignment bounds, which survive the 375 same-second
// collisions. The frozen claim is the ORDERING: tools_changed at its lower
// bound beats every other class at its upper bound.
func TestFrozenAmbiguityBounds(t *testing.T) {
	ev, br := load(t)
	// Both layers are pinned: the exact counts the aligner produced, and the
	// percentages the frozen table renders from them.
	type want struct {
		n, lower, upper int
		lo, hi          string
	}
	cases := map[string]want{
		"tools_changed":    {1006, 587, 932, "58%", "93%"},
		"messages_changed": {591, 140, 163, "24%", "28%"},
		"system_changed":   {119, 29, 35, "24%", "29%"},
		"model_changed":    {45, 12, 13, "27%", "29%"},
	}
	lowest := 101
	highestOther := 0
	for c, w := range cases {
		b := Bounds(ev, br, c)
		if b.N != w.n || b.Lower != w.lower || b.Upper != w.upper {
			t.Errorf("%s counts = n%d lo%d up%d, want n%d lo%d up%d",
				c, b.N, b.Lower, b.Upper, w.n, w.lower, w.upper)
		}
		if got := Pct(b.Lower, b.N); got != w.lo {
			t.Errorf("%s lower renders as %s, want %s", c, got, w.lo)
		}
		if got := Pct(b.Upper, b.N); got != w.hi {
			t.Errorf("%s upper renders as %s, want %s", c, got, w.hi)
		}
		lo := 100 * b.Lower / b.N
		hi := 100 * b.Upper / b.N
		if c == "tools_changed" {
			lowest = lo
		} else if hi > highestOther {
			highestOther = hi
		}
	}
	if lowest <= highestOther {
		t.Errorf("tools_changed lower bound %d%% does not exceed every other class's "+
			"upper bound %d%%; the frozen ordering claim has stopped holding",
			lowest, highestOther)
	}
}

// Python's statistics.median averages the two middle values on an even input.
// Taking the lower middle instead would move published numbers, so the
// convention is pinned rather than assumed.
func TestMedianMatchesPythonConvention(t *testing.T) {
	if got := median([]float64{1, 2, 3, 4}); got != 2.5 {
		t.Errorf("median of 1,2,3,4 = %v, want 2.5", got)
	}
	if got := median([]float64{1, 2, 3}); got != 2 {
		t.Errorf("median of 1,2,3 = %v, want 2", got)
	}
	if got := median(nil); got != 0 {
		t.Errorf("median of nothing = %v, want 0", got)
	}
}

// Indeterminate classes are excluded from the per-class table and counted in
// D. Folding them in would change every denominator.
func TestIndeterminateClassesAreExcluded(t *testing.T) {
	ev, br := load(t)
	r := Score(ev, br)
	for _, c := range Indeterminate {
		if _, ok := r.PerClass[c]; ok {
			t.Errorf("%s appears in the per-class table; it is indeterminate and "+
				"E005 excludes it", c)
		}
	}
	if r.D != 255+48 {
		t.Errorf("D = %d, want %d", r.D, 255+48)
	}
}

// The lower bound caps at the class's own event count, and the real corpus
// never exercises that cap.
//
// Mutation M5 removed `min(len(mine), nb-others)` and every frozen number
// still reproduced, because in this corpus the break count in an ambiguous
// second never exceeds the events of the class being bounded. A guard that
// cannot fire on the data is not evidence, so it is pinned here on a synthetic
// second built to reach it: one oracle event of the class, no other structural
// class present, and three breaks in the same second.
//
// Without the cap the lower bound would be 3, which is more matches than there
// are events to match, and the class would be credited above its own n.
func TestLowerBoundCannotExceedTheClassEventCount(t *testing.T) {
	ev := []Event{{File: "f0001", Second: "10:00:00", Oracle: "tools_changed"}}
	breaks := map[string]int{"f0001\t10:00:00": 3}
	b := Bounds(ev, breaks, "tools_changed")
	if b.N != 1 {
		t.Fatalf("n = %d, want 1", b.N)
	}
	if b.Lower > b.N {
		t.Errorf("lower = %d with n = %d: a class cannot match more events than "+
			"it has; the min() cap in the recovered procedure is not being applied",
			b.Lower, b.N)
	}
	if b.Lower != 1 || b.Upper != 1 {
		t.Errorf("bounds = %d/%d, want 1/1", b.Lower, b.Upper)
	}
}
