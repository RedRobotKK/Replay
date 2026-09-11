package analysis

// Self-referential comparison: how a session sits against the reader's own
// total, and nothing else.
//
// A bare figure is not actionable. "This session cost $3.40" leaves the reader
// asking whether that is high, and neither they nor this tool can answer from a
// population — the pooled corpus has one member, and publishing a population
// figure derived from one machine is the shape of claim this project has
// already retracted twice.
//
// The comparison that IS available at first run is the reader against
// themselves. Their own total needs no population, no key, no network, and no
// second surface; it works identically on a metered API account, a subscription
// seat and a local model where there is no money at all. It is the only
// interpretable number this tool can honestly put in front of somebody who has
// just installed it.
//
// The unit is a SHARE OF THE READER'S OWN TOTAL, and that choice was made by
// running the first version against a real corpus. Comparing the peak session
// to the median produced "1363.2x your median session" — arithmetically exact
// and useless. On a skewed corpus the median session is a two-minute question
// and the peak is a day's work, so the ratio between them is a category error
// wearing a number. A share of the total is interpretable on sight: one
// session was a third of everything you spent.

// minSessions is how many sessions must exist before concentration means
// anything. With one session it is 100% by construction; with two, 50% is the
// even split. Three is the floor at which "one of these is disproportionate"
// is a statement about the distribution rather than about its size.
const minSessions = 3

// Outlier is one session measured against the reader's own total.
type Outlier struct {
	// Share is this session's cost divided by the total. 0.31 is a session
	// that was 31% of everything.
	Share float64
	// Total is what it was divided by, carried so a reader can check the
	// arithmetic rather than take it.
	Total float64
	// Cost is this session's own figure.
	Cost float64
	// N is how many sessions stood behind the total. A share without its n is
	// not checkable, which is the rule the pooled corpus already enforces.
	N int
}

// CompareToTotal places one cost against the total across n sessions.
//
// The bool is the refusal. It is false when there are too few sessions for
// concentration to mean anything, or when the total is zero — dividing by
// which yields an infinity that renders as a confident-looking number about a
// corpus where nothing was priced.
func CompareToTotal(cost, total float64, n int) (Outlier, bool) {
	if n < minSessions || total <= 0 {
		return Outlier{}, false
	}
	return Outlier{Share: cost / total, Total: total, Cost: cost, N: n}, true
}

// Notable reports whether this session is concentrated enough to be worth
// showing unprompted.
//
// Two conditions, both required. The session must hold at least
// concentrationMultiple times its even share (1/n), which keeps a tiny corpus
// from firing spuriously — with three sessions an even share is already 33%.
// And it must clear an absolute floor, because on a large corpus twice an even
// share can be a rounding error nobody should be interrupted for.
//
// Which of the two binds depends on n, and it is worth being exact because the
// comment here used to claim the test "scales with the corpus rather than being
// a flat percentage" — which is true only below n = 21. The multiple binds
// while 2/n > minShareFloor, i.e. for n <= 20. At n >= 21 the floor is always
// the larger of the two and Notable is exactly Share >= minShareFloor: a flat
// percentage, on every corpus this tool has been run against.
//
// Both numbers are judgements rather than measurements, and ADR-0009 says that
// of every threshold in this tool. They are named here rather than buried at a
// call site so they can be argued with, and pinned by tests that go red when
// either moves, so they cannot be changed without somebody deciding to.
func (o Outlier) Notable() bool {
	if o.N < minSessions {
		return false
	}
	even := 1.0 / float64(o.N)
	return o.Share >= concentrationMultiple*even && o.Share >= minShareFloor
}

const (
	concentrationMultiple = 2.0
	minShareFloor         = 0.10
)
