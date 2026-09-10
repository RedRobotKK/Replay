package analysis

import "testing"

// The comparison a reader can act on at first run, without a population.
//
// The first version of this compared the peak session to the median and, on a
// real corpus, produced "1363.2x your median session" — exact and useless,
// because on a skewed corpus the median is a two-minute question and the peak
// is a day's work. Share of the reader's own total is interpretable on sight.
// These tests pin that choice and the refusals around it.

// OL1: a share is cost over total, and it carries what it divided by.
func TestOL1_ShareIsCostOverTotal(t *testing.T) {
	o, ok := CompareToTotal(1056.14, 3424.33, 116)
	if !ok {
		t.Fatal("a 116-session corpus with a positive total produced no comparison")
	}
	if o.Share < 0.30 || o.Share > 0.31 {
		t.Errorf("share %.4f, want ~0.308", o.Share)
	}
	if o.Total != 3424.33 || o.Cost != 1056.14 || o.N != 116 {
		t.Errorf("the comparison does not carry what it compared: %+v", o)
	}
}

// OL2: too few sessions is a refusal.
//
// With one session the share is 100% by construction; with two, 50% is the
// even split. Neither is a statement about the distribution.
func TestOL2_TooFewSessionsRefuses(t *testing.T) {
	for _, n := range []int{0, 1, 2} {
		if _, ok := CompareToTotal(10, 10, n); ok {
			t.Errorf("n=%d produced a comparison; concentration needs at least %d "+
				"sessions to mean anything", n, minSessions)
		}
	}
	if _, ok := CompareToTotal(5, 10, minSessions); !ok {
		t.Errorf("n=%d was refused; that is the stated floor", minSessions)
	}
}

// OL3: a zero total is refused rather than divided by.
func TestOL3_AZeroTotalIsRefused(t *testing.T) {
	for _, tot := range []float64{0, -1} {
		if o, ok := CompareToTotal(3.40, tot, 100); ok {
			t.Errorf("total %v produced share %v instead of refusing", tot, o.Share)
		}
	}
}

// OL4: notable scales with the corpus, so a tiny corpus never fires.
//
// This is the property a flat percentage would not have. On three sessions an
// even share is 33%, so a 20% session is BELOW average and must not be called
// concentrated — while on 116 sessions, 20% is twenty-three times an even
// share and plainly is.
func TestOL4_NotableScalesWithTheCorpus(t *testing.T) {
	small, _ := CompareToTotal(2.0, 10.0, 3) // 20% of a 3-session corpus: below even
	if small.Notable() {
		t.Error("20% of a three-session corpus was called notable; an even share " +
			"there is 33%, so this session is below average")
	}
	big, _ := CompareToTotal(1056.14, 3424.33, 116) // 31% of 116: 36x even share
	if !big.Notable() {
		t.Errorf("%.1f%% of a 116-session corpus was not called notable", big.Share*100)
	}
	// The absolute floor: on a large corpus, 3x an even share can still be a
	// rounding error nobody should be interrupted for.
	tiny, _ := CompareToTotal(4.0, 100.0, 1000) // 4% — 40x even share, under the floor
	if tiny.Notable() {
		t.Errorf("%.1f%% was called notable despite being under the %.0f%% floor",
			tiny.Share*100, minShareFloor*100)
	}
}

// OL-N: an Outlier assembled by hand still honours the session minimum.
//
// CompareToTotal refuses n < minSessions before it builds anything, so through
// that path Notable's own check never fires. Outlier is exported, though, and
// the check is not redundant for a value someone constructs directly: with two
// sessions where one carries the whole bill, Share is 1.0 and the evenness
// multiple is 2 x 0.5 = 1.0, so the concentration test passes and Notable
// would call a two-session corpus notable — under a floor the package states
// is three.
//
// The concentration arithmetic is what makes the smallest corpora look most
// extreme, which is exactly why the floor exists rather than being implied.
func TestOutlierNotableHonoursTheMinimumWhenBuiltDirectly(t *testing.T) {
	// One of two sessions holds every dollar: the most concentrated a corpus
	// can be, and still too few sessions to mean anything.
	o := Outlier{Share: 1.0, Total: 100, Cost: 100, N: 2}

	if got := o.Share; got < concentrationMultiple/float64(o.N) {
		t.Fatalf("the fixture does not reach the concentration threshold (share %v), "+
			"so this test would pass without the guard it exists to observe", got)
	}
	if o.Notable() {
		t.Errorf("a %d-session corpus was called notable; the package requires %d",
			o.N, minSessions)
	}
}
