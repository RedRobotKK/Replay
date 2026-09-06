// Package probe finds a model's minimum cacheable prefix on purpose.
//
// Passive measurement only learns what ordinary traffic happens to reveal, and
// ordinary traffic never sends a small prompt with a cache breakpoint on it —
// nobody caches a 600-token prefix by accident. Against a real ledger that
// yielded one loose bound from four sessions: floor at most 36,635, which is
// consistent with a documented 512 and confirms nothing.
//
// The evidence that would tighten it has to be manufactured. A probe is a
// request carrying a cache breakpoint at a known prefix size: if the provider
// writes a cache entry the floor is at or below that size, and if it writes
// nothing the floor is above it. Bisect.
//
// Every probe is a real request the operator pays for at their own provider,
// and that shapes the whole design. The search is logarithmic rather than
// linear, it stops at a stated resolution instead of chasing precision nobody
// asked for, it refuses to exceed an authorised number of probes, and it
// reports the interval it actually established rather than a point inside it.
package probe

// ditherOffsets nudge probe points off the power-of-two grid. Small, odd, and
// mixed in sign so the sequence does not drift in one direction.
var ditherOffsets = []int{3, -5, 7, -3, 11, -7, 5, -11}

// Config bounds the search before it begins.
type Config struct {
	// Min and Max bracket where the floor could be. The search assumes
	// nothing outside them.
	Min, Max int
	// Resolution is how narrow an interval is narrow enough, in tokens.
	// Cache granularity is coarse, so resolving to the token spends money on
	// noise.
	Resolution int
	// RelativeResolution stops the search when the bracket is within this
	// fraction of its upper bound. Zero means use Resolution.
	//
	// It exists because a fixed token width is not one statement across a
	// range spanning two orders of magnitude: 128 tokens is a quarter of 512
	// and two thousandths of 65,536. A search that does not know where the
	// floor is cannot pick a sensible absolute width in advance, and "within
	// ten percent" means the same thing wherever the answer turns out to be.
	RelativeResolution float64
	// MaxProbes caps the spend. Zero means the resolution decides.
	MaxProbes int
	// Confirm is how many agreeing answers a size needs before the search
	// acts on it. Zero or one means take the first answer.
	//
	// The decisive probe is the one that fixes the published claim, and it is
	// exactly where a transient does the most damage: a routing change, a
	// warm entry, a partial write. One request is not a measurement.
	Confirm int
}

// Result is what one probe observed.
type Result struct {
	PrefixTokens int
	// Wrote is true when the provider reported creating a cache entry.
	Wrote bool
	// Read is true when it reported reading one. A probe that read found an
	// existing entry, so the provider never had to decide whether this prefix
	// was cacheable — the result says nothing about the floor.
	Read bool
	// CachedTokens is how many tokens the provider actually cached, which is
	// not always what was asked for: writes land on block boundaries. The
	// sizes it takes are what Granularity is inferred from.
	CachedTokens int
}

// Search is a bisection over prefix sizes.
type Search struct {
	cfg Config
	// lo is the largest size seen NOT to cache; hi the smallest seen to cache.
	lo, hi       int
	probes       int
	pending      int
	stoppedEarly bool
	contradicted bool
	// answers accumulates repeats at one size until Confirm is satisfied.
	answers map[int][]bool
	// writes are the cached sizes observed, for inferring block granularity.
	writes           []int
	nonDeterministic bool
	stalled          bool
}

// New starts a search.
func New(cfg Config) *Search {
	if cfg.Resolution < 1 && cfg.RelativeResolution <= 0 {
		cfg.Resolution = 1
	}
	if cfg.Confirm < 1 {
		cfg.Confirm = 1
	}
	return &Search{cfg: cfg, lo: cfg.Min, hi: cfg.Max, answers: map[int][]bool{}}
}

// Next proposes the next prefix size to probe, or zero when the search is done.
//
// Done means one of three things, and Bracket and StoppedEarly distinguish
// them: the interval is within the resolution, the probe budget is spent, or
// the answers have contradicted each other.
func (s *Search) Next() int {
	if s.contradicted || s.nonDeterministic || s.stalled || !s.usableRange() {
		return 0
	}
	if s.cfg.MaxProbes > 0 && s.probes >= s.cfg.MaxProbes {
		s.stoppedEarly = true
		return 0
	}
	if s.hi-s.lo <= s.stopWidth() {
		return 0
	}
	// Midpoint, dithered off the power-of-two grid.
	//
	// A clean bisection over a power-of-two range proposes 32768, 16384, 8192
	// and so on, and the GCD of those is 8192 whatever the provider's real
	// block size is. The inference would then report a block size that is
	// merely an artifact of how the search chose its own probe points —
	// confidently, and wrongly, and larger than the truth every time.
	//
	// Offsetting each probe by a small varying odd amount breaks that
	// alignment. The provider still rounds each write up to a real block, so
	// the cached sizes it reports carry the true divisor, and their GCD
	// converges on it. The offset is a deterministic function of the probe
	// count so a run is reproducible.
	mid := s.lo + (s.hi-s.lo)/2
	if span := s.hi - s.lo; span > 8 {
		// Keyed on the bracket, not the probe count.
		//
		// The first live run found this: with the offset varying per probe,
		// every confirmation of the same decision landed on a different size —
		// 4099, 4091, 4103, 4093 — so the repeat count never reached Confirm
		// and the bracket never moved. Nine of ten probes bought one decision,
		// and a probe that did cache at 2044 was discarded as unconfirmed
		// while the report said "at most 4099".
		//
		// Keying on the bracket makes the offset constant for as long as the
		// decision is unresolved, so confirmations agree on a size, while
		// still varying between decisions — which is all the granularity
		// inference needs. 3,072 simulated cases missed this because the sweep
		// runs Confirm at 1, where a single answer always resolves.
		dither := ditherOffsets[(s.lo+s.hi)%len(ditherOffsets)]
		if mid+dither > s.lo && mid+dither < s.hi {
			mid += dither
		}
	}
	// No clamp here. The stop condition above already returned unless hi-lo
	// exceeds stopWidth, which is at least one, so the midpoint of an interval
	// at least two wide is strictly interior — and the dither only applies
	// when it stays inside. Clamps guarding that could never execute: dead
	// code shaped like a safety check, which is the thing ADR-0014 is about.
	// TestB19 asserts the invariant across the sweep space instead.
	s.pending = mid
	return mid
}

// Record takes the answer to a probe.
//
// It returns an error for a result that cannot be used, and does not advance:
// the same size will be proposed again. A probe that read from cache is the
// case that matters — treating it as a write would place the floor below a
// size that was never actually tested.
func (s *Search) Record(r Result) error {
	if r.Read {
		return errInconclusive{}
	}
	s.probes++
	if r.Wrote && r.CachedTokens > 0 {
		s.writes = append(s.writes, r.CachedTokens)
	}

	// Repeats at one size must agree before the search acts on them.
	got := append(s.answers[r.PrefixTokens], r.Wrote)
	s.answers[r.PrefixTokens] = got
	for _, a := range got {
		if a != got[0] {
			// The same prefix cached once and not the next time. No single
			// floor explains that, and a majority vote would bury the only
			// interesting thing this run found.
			s.nonDeterministic = true
			return nil
		}
	}
	if len(got) < s.cfg.Confirm {
		// Not yet confirmed. Do not move the bracket; Next will propose the
		// same size again.
		return nil
	}

	beforeLo, beforeHi := s.lo, s.hi
	if r.Wrote {
		// The provider's own count, not the size we asked for. Probe filler is
		// built from a chars-per-token approximation, so the requested size is
		// an estimate; `cache_creation_input_tokens` is the number of tokens
		// actually cached. Recording the estimate would make every published
		// bracket only as good as that approximation — and the first live
		// fable-5-1 run would have declared a documented 512 refuted on the
		// strength of a guess.
		//
		// There is no equivalent for the lower bound: a request that cached
		// nothing reports no token count, so that side stays estimated.
		bound := r.CachedTokens
		if bound <= 0 {
			bound = r.PrefixTokens
		}
		if bound < s.hi {
			s.hi = bound
		}
	} else if r.PrefixTokens > s.lo {
		s.lo = r.PrefixTokens
	}
	// The answers now say the floor is above lo and at or below hi. If those
	// have crossed, no single floor explains them.
	if s.lo >= s.hi {
		s.contradicted = true
		return nil
	}

	// A confirmed decision that moved neither bound cannot be improved on.
	//
	// This happens whenever the provider rounds a prefix up to a block: ask
	// for 131 tokens against a 128-token block and it caches 256, so the upper
	// bound stays at 256 while nothing caches below it to move the lower one.
	// The proposal is a function of the bracket, so the next probe is the same
	// probe and the answer is the same answer — forever, at full price.
	//
	// The bracket is then as tight as the provider's granularity allows, which
	// is a result rather than a failure, and the search says so instead of
	// spending the rest of its budget rediscovering it.
	if s.lo == beforeLo && s.hi == beforeHi {
		s.stalled = true
	}
	return nil
}

type errInconclusive struct{}

func (errInconclusive) Error() string {
	return "the probe read from an existing cache entry, so it never tested whether this prefix is cacheable; " +
		"retry with content that cannot already be cached"
}

// stopWidth is how narrow the bracket has to get before the search stops.
//
// It never goes below the inferred block granularity. If writes land on
// 1024-token blocks then no floor between multiples of 1024 is observable, and
// bisecting past that spends real money distinguishing sizes the provider
// treats as the same size.
func (s *Search) stopWidth() int {
	w := s.cfg.Resolution
	if s.cfg.RelativeResolution > 0 {
		w = int(float64(s.hi) * s.cfg.RelativeResolution)
	}
	if g := s.Granularity(); g > w {
		w = g
	}
	if w < 1 {
		w = 1
	}
	return w
}

// Bracket is what the search established: the floor is above lo and at or
// below hi.
func (s *Search) Bracket() (lo, hi int) { return s.lo, s.hi }

// AtMost is the tightest upper bound reached. It is the only figure worth
// publishing as a claim, and it is an upper bound rather than a value: the
// exact floor inside the bracket was never tested.
func (s *Search) AtMost() int { return s.hi }

// StoppedEarly reports a search cut short by its probe budget, whose bracket
// is therefore wider than the resolution suggests.
func (s *Search) StoppedEarly() bool { return s.stoppedEarly }

// Contradicted reports answers that no single floor explains: a prefix cached
// at one size and refused at a larger one. That is a real finding rather than
// an error — block granularity, a per-account difference, or a change during
// the run — and it is more interesting than the number would have been.
func (s *Search) Contradicted() bool { return s.contradicted }

// Stalled reports a bracket that stopped narrowing before reaching the
// requested resolution, because the provider's own granularity will not
// express a finer answer. The bracket is valid; it is just as tight as it can
// get.
func (s *Search) Stalled() bool { return s.stalled }

// Probes is how many billable requests have decided something. An
// inconclusive probe is not counted: it cost money and taught nothing, and
// counting it would make the budget look spent on progress that did not happen.
func (s *Search) Probes() int { return s.probes }

// usableRange reports whether the configured bounds can contain a floor.
// An inverted, empty or negative range is refused rather than searched: a
// bisection over it converges confidently on a number with nothing under it.
func (s *Search) usableRange() bool {
	return s.cfg.Min >= 0 && s.cfg.Max > s.cfg.Min
}

// AffordableDecisions is how many bisection steps the probe budget actually
// buys, which is not MaxProbes.
//
// Every confirmation is a billable request, so Confirm multiplies against the
// budget: nine probes at three confirmations each is three decisions. A caller
// who does not know that authorises a budget which cannot reach the resolution
// they asked for, and finds out from an invoice.
func (s *Search) AffordableDecisions() int {
	if s.cfg.MaxProbes <= 0 {
		return 0
	}
	return s.cfg.MaxProbes / s.cfg.Confirm
}

// BudgetTooSmall reports a budget that cannot reach the requested resolution,
// before any money is spent.
//
// Bisection needs about log2(range/resolution) decisions. If the budget affords
// fewer, the search will stop early with a bracket wider than asked for — which
// is fine, and worth knowing in advance rather than after paying for it.
func (s *Search) BudgetTooSmall() bool {
	if s.cfg.MaxProbes <= 0 || !s.usableRange() {
		return false
	}
	span := s.cfg.Max - s.cfg.Min
	target := s.cfg.Resolution
	if s.cfg.RelativeResolution > 0 {
		target = int(float64(s.cfg.Max) * s.cfg.RelativeResolution)
	}
	if target < 1 {
		target = 1
	}
	needed := 0
	for w := span; w > target; w /= 2 {
		needed++
	}
	return s.AffordableDecisions() < needed
}

// Granularity is the block size cache writes appear to land on, inferred as
// the greatest common divisor of the sizes actually cached.
//
// It is inferred, not measured, and the distinction matters: a GCD over few
// samples is easily coincidence, and one odd size collapses it to something
// meaningless. Its use is to stop the search resolving finer than the provider
// can express — if writes land on 512-token blocks, no floor between multiples
// of 512 is observable, and bisecting past that buys only billable requests.
func (s *Search) Granularity() int {
	// Fewer than three writes is not an inference, it is a coincidence. One
	// write's GCD is itself, which would report a 32,768-token "block size"
	// from a single large probe — and because the stop width is floored by
	// granularity, that halts the search immediately on its own first answer.
	// Zero means unknown, and callers treat unknown as no constraint.
	if len(s.writes) < 3 {
		return 0
	}
	g := s.writes[0]
	for _, w := range s.writes[1:] {
		g = gcd(g, w)
	}
	return g
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// NonDeterministic reports the same prefix size answering differently across
// repeats. The premise of the search — that there is one floor — has failed,
// and that is a finding rather than an error to retry past.
func (s *Search) NonDeterministic() bool { return s.nonDeterministic }
