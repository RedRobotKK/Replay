package probe

import (
	"testing"
)

// Deliberate probes: finding a model's caching floor on purpose rather than
// waiting for ordinary traffic to happen to reveal it.
//
// Passive measurement gave one bound from four sessions — floor at most 36,635
// — because real traffic never sends a small prompt with a breakpoint on it.
// Nobody caches a 600-token prefix by accident. The evidence that would tighten
// the bound has to be manufactured, and manufacturing it costs real money at
// the provider, which shapes every decision here.
//
// A probe is a request with a cache breakpoint at a known prefix size. If the
// provider writes a cache entry, the floor is at or below that size. If it
// writes nothing, the floor is above it. Bisect between the two.
//
//	B1  the search brackets before it bisects
//	B2  each answer halves the remaining interval
//	B3  it stops at the resolution asked for, and not before
//	B4  it never exceeds the spend the operator authorised
//	B5  a probe that read from cache is inconclusive, not evidence
//	B6  it reports the bracket it reached, never a point it did not establish
//	B7  a contradictory provider is reported, not smoothed

func TestB1_SearchBracketsBeforeBisecting(t *testing.T) {
	// PASS: the first probes establish an interval containing the floor.
	// FAIL: bisecting an interval nobody has shown contains anything, which
	// converges confidently on a number with no evidence under it.
	s := New(Config{Min: 64, Max: 65536, Resolution: 128})
	first := s.Next()
	if first == 0 {
		t.Fatal("a fresh search must propose a probe")
	}
	if first < 64 || first > 65536 {
		t.Errorf("first probe %d is outside the stated range", first)
	}
}

func TestB2_EachAnswerHalvesTheInterval(t *testing.T) {
	// PASS: the interval shrinks monotonically and the probe count is
	// logarithmic.
	// FAIL: linear search, which is affordable in tests and not at $5 a probe.
	s := New(Config{Min: 0, Max: 65536, Resolution: 64})
	const floor = 1024
	probes := 0
	for {
		n := s.Next()
		if n == 0 {
			break
		}
		probes++
		if probes > 32 {
			t.Fatal("search did not converge; it must halve, not walk")
		}
		s.Record(Result{PrefixTokens: n, Wrote: n >= floor})
	}
	lo, hi := s.Bracket()
	if lo >= floor || hi < floor {
		t.Errorf("bracket (%d, %d] does not contain the true floor %d", lo, hi, floor)
	}
	if probes > 12 {
		t.Errorf("%d probes for a 65,536-token range; bisection needs about 10", probes)
	}
}

func TestB3_StopsAtTheResolutionAsked(t *testing.T) {
	// PASS: it stops once the interval is no wider than the resolution.
	// FAIL: spending on precision nobody asked for. Cache granularity is
	// coarse, so resolving a floor to the token is money spent on noise.
	s := New(Config{Min: 0, Max: 4096, Resolution: 512})
	for i := 0; ; i++ {
		if i > 64 {
			// A cap, because a mutant that removes a stop condition should
			// fail rather than hang. PM3 ran for five minutes before this
			// existed, which is a worse failure mode than a red test.
			t.Fatal("search did not terminate; the resolution stop condition is gone")
		}
		n := s.Next()
		if n == 0 {
			break
		}
		s.Record(Result{PrefixTokens: n, Wrote: n >= 2000})
	}
	lo, hi := s.Bracket()
	if hi-lo > 512 {
		t.Errorf("interval (%d, %d] is wider than the 512 resolution", lo, hi)
	}
}

func TestB4_NeverExceedsTheAuthorisedSpend(t *testing.T) {
	// PASS: the search stops when the next probe would cost more than remains.
	// FAIL: one probe over. This spends the operator's money at their provider,
	// and a cap that is approximately respected is not a cap.
	s := New(Config{Min: 0, Max: 65536, Resolution: 1, MaxProbes: 3})
	n := 0
	for s.Next() != 0 {
		n++
		s.Record(Result{PrefixTokens: 1000, Wrote: true})
		if n > 10 {
			t.Fatal("MaxProbes was ignored")
		}
	}
	if n != 3 {
		t.Errorf("ran %d probes, want exactly the 3 authorised", n)
	}
	if !s.StoppedEarly() {
		t.Error("a search cut short by its budget must say so; its bracket is wider than the resolution implies")
	}
}

func TestB5_AReadIsInconclusive(t *testing.T) {
	// A probe that READ from cache tells us nothing about the floor: it found
	// an existing entry, so the provider never had to decide whether this
	// prefix was cacheable. Counting a read as a write would place the floor
	// below a size never actually tested.
	// PASS: the result is rejected and the same size is proposed again.
	// FAIL: treating it as evidence, or silently advancing past an untested
	// size.
	s := New(Config{Min: 0, Max: 4096, Resolution: 64})
	first := s.Next()
	if err := s.Record(Result{PrefixTokens: first, Wrote: true, Read: true}); err == nil {
		t.Error("a probe that read from cache must be refused as inconclusive")
	}
	if again := s.Next(); again != first {
		t.Errorf("after an inconclusive probe the same size must be retried; got %d want %d", again, first)
	}
}

func TestB6_ReportsTheBracketNotAPoint(t *testing.T) {
	// PASS: an interval, and Floor() reporting the upper bound as "at most".
	// FAIL: naming a single number. The search establishes that the floor is
	// above one size and at or below another; the exact value inside that gap
	// was never tested, and claiming it is the same overreach as reading a
	// warm write as a prefix size.
	s := New(Config{Min: 0, Max: 2048, Resolution: 256})
	for i := 0; ; i++ {
		if i > 64 {
			t.Fatal("search did not terminate")
		}
		n := s.Next()
		if n == 0 {
			break
		}
		s.Record(Result{PrefixTokens: n, Wrote: n >= 1000})
	}
	lo, hi := s.Bracket()
	if lo == hi {
		t.Error("a bisection to a resolution of 256 cannot produce a point")
	}
	if s.AtMost() != hi {
		t.Errorf("AtMost = %d, want the upper bound %d", s.AtMost(), hi)
	}
}

func TestB7_AContradictoryProviderIsReported(t *testing.T) {
	// A provider that caches at 500 and then refuses at 2000 has contradicted
	// the premise that there is a single floor. That is a real result — block
	// granularity, a per-account difference, a change mid-run — and it is the
	// interesting part.
	// PASS: the contradiction is flagged.
	// FAIL: continuing to bisect an interval the answers have already made
	// impossible, and reporting a clean number from dirty data.
	s := New(Config{Min: 0, Max: 4096, Resolution: 64})
	s.Record(Result{PrefixTokens: 500, Wrote: true})
	s.Record(Result{PrefixTokens: 2000, Wrote: false})
	if !s.Contradicted() {
		t.Error("caching at 500 and not at 2000 cannot both hold for one floor; that must be reported")
	}
}

// The bisection above assumes one deterministic floor. Both halves of that
// assumption are worth attacking, because both are probably false.
//
//	B8   cache writes land on a block boundary, and the block size is inferable
//	B9   a boundary answer is confirmed before it is believed
//	B10  a non-deterministic boundary is a finding, not noise to average out

// B8: PASS: the granularity is the GCD of observed write sizes, and it is
// reported as inferred rather than measured.
// FAIL: claiming a floor finer than the block size. If a provider caches in
// 128-token blocks, no floor between multiples of 128 is observable, and
// bisecting past that resolution buys nothing but billable requests.
func TestB8_BlockGranularityIsInferred(t *testing.T) {
	s := New(Config{Min: 0, Max: 65536, Resolution: 1})
	for _, n := range []int{1024, 2048, 512, 4096} {
		s.Record(Result{PrefixTokens: n, Wrote: true, CachedTokens: n})
	}
	if g := s.Granularity(); g != 512 {
		t.Errorf("granularity = %d, want 512 (the GCD of the observed writes)", g)
	}

	// One odd size collapses it, which is correct: a single write at 700 means
	// the boundary is not on 512s.
	s2 := New(Config{Min: 0, Max: 65536, Resolution: 1})
	for _, n := range []int{1024, 2048, 700} {
		s2.Record(Result{PrefixTokens: n, Wrote: true, CachedTokens: n})
	}
	if g := s2.Granularity(); g != 4 {
		t.Errorf("granularity = %d, want 4 (GCD of 1024, 2048, 700)", g)
	}
}

// B9: PASS: the decisive probe is repeated, and agreement across repeats is
// required before the bracket narrows to the resolution.
// FAIL: believing one answer at the boundary. A single request decides the
// published claim, and a single request is exactly where a transient — a
// routing change, a warm entry, a partial write — does the most damage.
func TestB9_TheBoundaryIsConfirmed(t *testing.T) {
	s := New(Config{Min: 0, Max: 4096, Resolution: 256, Confirm: 3})
	seen := map[int]int{}
	for i := 0; ; i++ {
		if i > 128 {
			t.Fatal("search did not terminate")
		}
		n := s.Next()
		if n == 0 {
			break
		}
		seen[n]++
		s.Record(Result{PrefixTokens: n, Wrote: n >= 1000, CachedTokens: n})
	}
	// The last size probed is the decisive one and must have been asked more
	// than once.
	var maxRepeat int
	for _, c := range seen {
		if c > maxRepeat {
			maxRepeat = c
		}
	}
	if maxRepeat < 3 {
		t.Errorf("no size was probed more than %d time(s); a boundary decided by one request is decided by luck", maxRepeat)
	}
}

// B10: PASS: disagreement at one size is reported as non-deterministic.
// FAIL: majority-voting it into a clean answer. A floor that holds sometimes
// is not a floor, and averaging hides the only interesting thing the probe
// found.
func TestB10_ANonDeterministicBoundaryIsAFinding(t *testing.T) {
	s := New(Config{Min: 0, Max: 4096, Resolution: 64, Confirm: 3})
	s.Record(Result{PrefixTokens: 1024, Wrote: true, CachedTokens: 1024})
	s.Record(Result{PrefixTokens: 1024, Wrote: false})
	if !s.NonDeterministic() {
		t.Error("the same prefix caching once and not the next time must be reported, not resolved by vote")
	}
	if s.Next() != 0 {
		t.Error("a search whose premise has failed must stop rather than spend more money bisecting")
	}
}

// B11: resolution can be relative, because 128 tokens means something very
// different at 512 than at 65,536.
//
// A search over two orders of magnitude with a fixed token resolution either
// stops far too early at the top of the range or spends a fortune resolving
// the bottom. A relative stop condition — "within 10 percent" — is the same
// statement at every scale, which is what a floor that could be anywhere in
// [64, 65536] actually needs.
//
// PASS: the search stops when the bracket is within the stated ratio, and the
// stated ratio holds at both ends of the range.
// FAIL: an absolute interpretation, which is wrong by a factor of a thousand
// across this range.
func TestB11_ResolutionCanBeRelative(t *testing.T) {
	for _, floor := range []int{128, 40000} {
		s := New(Config{Min: 64, Max: 65536, RelativeResolution: 0.10})
		for i := 0; ; i++ {
			if i > 64 {
				t.Fatalf("floor %d: search did not terminate", floor)
			}
			n := s.Next()
			if n == 0 {
				break
			}
			s.Record(Result{PrefixTokens: n, Wrote: n >= floor, CachedTokens: n})
		}
		lo, hi := s.Bracket()
		if lo >= floor || hi < floor {
			t.Errorf("floor %d: bracket (%d, %d] does not contain it", floor, lo, hi)
		}
		// Within ten percent, measured the way the config states it.
		if float64(hi-lo)/float64(hi) > 0.10001 {
			t.Errorf("floor %d: bracket (%d, %d] is wider than the 10%% asked for", floor, lo, hi)
		}
	}
}

// B12: a relative resolution never bisects below the inferred block size.
// PASS: the search stops once the bracket is inside one block.
// FAIL: probing for precision the provider cannot express, which is money
// spent to distinguish sizes that are the same size.
func TestB12_RelativeResolutionRespectsGranularity(t *testing.T) {
	s := New(Config{Min: 0, Max: 65536, RelativeResolution: 0.001})
	for i := 0; ; i++ {
		if i > 128 {
			t.Fatal("search did not terminate; granularity must floor the resolution")
		}
		n := s.Next()
		if n == 0 {
			break
		}
		// The provider rounds every write up to a 1024-token block.
		cached := ((n + 1023) / 1024) * 1024
		s.Record(Result{PrefixTokens: n, Wrote: n >= 4096, CachedTokens: cached})
	}
	if g := s.Granularity(); g != 1024 {
		t.Errorf("granularity = %d, want 1024", g)
	}
	lo, hi := s.Bracket()
	if hi-lo < 1024 && hi-lo > 0 {
		t.Errorf("bracket (%d, %d] is narrower than one block; that precision is not observable", lo, hi)
	}
}
