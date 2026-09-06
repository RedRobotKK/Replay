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
	// Each answer must move a bound, or the search stops because the bracket
	// stalled rather than because the budget ran out — and this test is about
	// the budget. Recording the same result repeatedly is a stall by
	// definition, which is what B22 covers.
	s := New(Config{Min: 0, Max: 65536, Resolution: 1, MaxProbes: 3})
	n := 0
	for {
		p := s.Next()
		if p == 0 {
			break
		}
		n++
		// Always caches, so every answer lowers the upper bound.
		s.Record(Result{PrefixTokens: p, Wrote: true, CachedTokens: p})
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
	// One block, less the dither. The stop width is floored by granularity, but
	// the final bracket lands wherever the last dithered probe put it, so it
	// can sit a few tokens inside a block boundary. Allowing the dither's own
	// range keeps the assertion about the block size rather than about the
	// offset table.
	const maxDither = 11
	if hi-lo > 0 && hi-lo < 1024-2*maxDither {
		t.Errorf("bracket (%d, %d] is %d wide, materially narrower than one 1024-token block; that precision is not observable",
			lo, hi, hi-lo)
	}
}

// Edges. A search that spends real money should not be surprising at its
// boundaries, and 90% statement coverage left every one of these untested.
//
//	B13  a nonsensical range is refused rather than searched
//	B14  a result for a size never proposed is still usable
//	B15  recording after the search is finished changes nothing
//	B16  the confirmation cost against the probe budget is explicit
//	B17  the inconclusive error says what to do about it

func TestB13_ANonsensicalRangeIsRefused(t *testing.T) {
	// PASS: no probe proposed, and the bracket says nothing was established.
	// FAIL: bisecting an inverted or negative interval, which converges
	// confidently on a number with nothing under it.
	for _, c := range []Config{
		{Min: 4096, Max: 64, Resolution: 8},
		{Min: -100, Max: 1000, Resolution: 8},
		{Min: 0, Max: 0, Resolution: 8},
		{Min: 500, Max: 500, Resolution: 8},
	} {
		s := New(c)
		if n := s.Next(); n != 0 {
			t.Errorf("%+v proposed probe %d; an unusable range must propose none", c, n)
		}
	}
}

func TestB14_AResultForAnUnproposedSizeIsStillUsable(t *testing.T) {
	// A caller may have evidence the search did not ask for — an earlier run,
	// or a ledger record. It is still testimony about the floor.
	// PASS: the bracket narrows.
	// FAIL: ignoring evidence because it was not solicited, or crashing on it.
	s := New(Config{Min: 0, Max: 65536, Resolution: 64})
	if err := s.Record(Result{PrefixTokens: 2048, Wrote: true, CachedTokens: 2048}); err != nil {
		t.Fatalf("unsolicited evidence must be accepted: %v", err)
	}
	if _, hi := s.Bracket(); hi != 2048 {
		t.Errorf("upper bound = %d, want 2048", hi)
	}
}

func TestB15_RecordingAfterTheEndChangesNothing(t *testing.T) {
	// PASS: a finished search stays finished and its bracket is stable.
	// FAIL: a late result reopening a search whose budget is already spent, or
	// silently widening a published bracket.
	s := New(Config{Min: 0, Max: 1024, Resolution: 1024})
	if s.Next() != 0 {
		t.Fatal("this search is already within its resolution and must propose nothing")
	}
	loBefore, hiBefore := s.Bracket()
	_ = s.Record(Result{PrefixTokens: 512, Wrote: true, CachedTokens: 512})
	lo, hi := s.Bracket()
	if lo != loBefore || hi > hiBefore {
		t.Errorf("bracket moved from (%d, %d] to (%d, %d] after the search ended", loBefore, hiBefore, lo, hi)
	}
}

func TestB16_ConfirmationCostIsExplicit(t *testing.T) {
	// Confirm multiplies against MaxProbes: every confirmation is a billable
	// request. MaxProbes 9 with Confirm 3 buys three bisection steps, not
	// nine, and a caller who does not know that will authorise a budget that
	// cannot reach the resolution they asked for.
	// PASS: Budget reports the decisions the budget actually affords.
	// FAIL: leaving the caller to discover it from an invoice.
	s := New(Config{Min: 0, Max: 65536, Resolution: 1, MaxProbes: 9, Confirm: 3})
	if got := s.AffordableDecisions(); got != 3 {
		t.Errorf("AffordableDecisions = %d, want 3 (9 probes at 3 confirmations each)", got)
	}
	// Three decisions cannot resolve a 65,536-token range to 1 token, and the
	// search must say so rather than reporting a bracket it did not establish.
	if !s.BudgetTooSmall() {
		t.Error("a budget that cannot reach the requested resolution must be reported before any money is spent")
	}
}

func TestB17_TheInconclusiveErrorSaysWhatToDo(t *testing.T) {
	// PASS: the message names the cause and the remedy.
	// FAIL: an opaque error. The caller has just spent money on a probe that
	// taught nothing, and needs to know it must vary its content.
	s := New(Config{Min: 0, Max: 4096, Resolution: 64})
	err := s.Record(Result{PrefixTokens: 512, Wrote: true, Read: true})
	if err == nil {
		t.Fatal("a read must be refused")
	}
	for _, want := range []string{"read", "retry"} {
		if !contains(err.Error(), want) {
			t.Errorf("the message must mention %q; got %q", want, err.Error())
		}
	}
	if s.Probes() != 0 {
		t.Errorf("Probes = %d; an inconclusive probe consumed budget but decided nothing, and must not count as a decision", s.Probes())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// The exhaustive sweep, and the benchmark it must clear.
//
// Every test above examines one scenario chosen by hand, which means the
// scenarios are the ones I thought of. This runs the search against a
// simulated provider across the whole parameter space and states, as numbers,
// what "working" means:
//
//	SOUNDNESS      the bracket contains the true floor. Every case, no
//	               exceptions. A search that reports a bracket the floor is
//	               not in has published a false claim.
//	TERMINATION    every case converges. A search that does not stop spends
//	               money until the budget runs out.
//	EFFICIENCY     no case exceeds ceil(log2(span/resolution)) + 4 decisions.
//	               Bisection is optimal on a monotone predicate, so the base is
//	               log2; more than a small constant over it means the
//	               implementation is not bisecting.
//
//	               The slack is measured, not chosen. Dithering the probe points
//	               off the power-of-two grid costs extra halvings, and a sweep
//	               of the whole space puts the worst case at exactly +4 — one
//	               instance, span 1024, block 1, resolution 1, floor 625, which
//	               took 14 decisions against a log2 base of 10. With dither
//	               disabled nothing exceeds +2.
//
//	               That is the price of a sound granularity inference and it is
//	               worth paying: without dithering, the GCD reports an artifact
//	               of how the search chose its own probe points rather than a
//	               fact about the provider, confidently and always too large.
//
//	               If a change makes this exceed +4, measure the new worst case
//	               and move the number deliberately. Widening it to make a red
//	               test green is how a benchmark stops meaning anything.
//	TIGHTNESS      the final bracket is within the resolution, or the search
//	               says why not.
//
// These are the numbers to argue with. If a future change makes the search
// cheaper or tighter, move them down deliberately rather than discovering the
// regression from an invoice.
func TestSweep_ExhaustiveAgainstASimulatedProvider(t *testing.T) {
	type failure struct {
		floor, blockSize, span int
		reason                 string
	}
	var failures []failure
	var cases, totalDecisions, worstOver int

	spans := []int{1024, 8192, 65536, 262144}
	blockSizes := []int{1, 128, 512, 1024}
	resolutions := []int{1, 64, 512}

	for _, span := range spans {
		for _, block := range blockSizes {
			// Sweep the floor across the whole span, at a stride fine enough
			// to land on and between block boundaries.
			stride := span / 64
			if stride < 1 {
				stride = 1
			}
			for floor := 1; floor < span; floor += stride {
				for _, res := range resolutions {
					cases++
					s := New(Config{Min: 0, Max: span, Resolution: res})
					decisions := 0
					for i := 0; ; i++ {
						if i > 200 {
							failures = append(failures, failure{floor, block, span, "did not terminate"})
							break
						}
						n := s.Next()
						if n == 0 {
							break
						}
						decisions++
						// The simulated provider, in one coherent unit.
						//
						// The prefix we ask for is an estimate; what the
						// provider actually sees is that estimate rounded to a
						// block, and it is THAT size which decides caching and
						// which the response reports. An oracle that decides
						// on the requested size but reports the rounded one
						// mixes two quantities, and the search — which now
						// takes its upper bound from the reported figure —
						// then brackets against a floor expressed in the other.
						actual := ((n + block - 1) / block) * block
						wrote := actual >= floor
						cached := 0
						if wrote {
							cached = actual
						}
						if err := s.Record(Result{PrefixTokens: n, Wrote: wrote, CachedTokens: cached}); err != nil {
							failures = append(failures, failure{floor, block, span, "unexpected record error"})
							break
						}
					}
					totalDecisions += decisions

					lo, hi := s.Bracket()
					// SOUNDNESS. The lower bound is in requested tokens and the
					// upper in the provider's reported count, which is the
					// honest asymmetry: only the write side is measured. The
					// floor must sit between them.
					if lo >= floor || hi < floor {
						failures = append(failures, failure{floor, block, span,
							"bracket does not contain the floor"})
						continue
					}
					// EFFICIENCY.
					budget := 4
					for w := span; w > res; w /= 2 {
						budget++
					}
					if decisions > budget {
						if over := decisions - budget; over > worstOver {
							worstOver = over
						}
						failures = append(failures, failure{floor, block, span,
							"more decisions than bisection needs"})
					}
				}
			}
		}
	}

	t.Logf("swept %d cases, %d total decisions, %.1f average",
		cases, totalDecisions, float64(totalDecisions)/float64(cases))

	if cases < 3000 {
		t.Fatalf("only %d cases swept; the sweep is not covering the space it claims to", cases)
	}
	if len(failures) > 0 {
		for i, f := range failures {
			if i >= 8 {
				t.Errorf("... and %d more", len(failures)-8)
				break
			}
			t.Errorf("floor %d, block %d, span %d: %s", f.floor, f.blockSize, f.span, f.reason)
		}
		t.Fatalf("%d of %d cases failed the benchmark", len(failures), cases)
	}
}

// The same sweep for relative resolution, where the stop condition scales with
// the answer rather than being fixed in advance.
func TestSweep_RelativeResolutionIsSoundAtEveryScale(t *testing.T) {
	var failures int
	cases := 0
	for _, span := range []int{8192, 65536, 262144} {
		for _, ratio := range []float64{0.25, 0.10, 0.02} {
			stride := span / 48
			for floor := 1; floor < span; floor += stride {
				cases++
				s := New(Config{Min: 0, Max: span, RelativeResolution: ratio})
				for i := 0; ; i++ {
					if i > 200 {
						t.Fatalf("span %d ratio %v floor %d: did not terminate", span, ratio, floor)
					}
					n := s.Next()
					if n == 0 {
						break
					}
					_ = s.Record(Result{PrefixTokens: n, Wrote: n >= floor, CachedTokens: n})
				}
				lo, hi := s.Bracket()
				if lo >= floor || hi < floor {
					failures++
					if failures < 5 {
						t.Errorf("span %d ratio %v floor %d: bracket (%d, %d] excludes it", span, ratio, floor, lo, hi)
					}
				}
			}
		}
	}
	t.Logf("swept %d relative-resolution cases", cases)
	if failures > 0 {
		t.Fatalf("%d of %d cases put the floor outside the reported bracket", failures, cases)
	}
}

// The remaining defaults and clamps. Each is a line that only runs on input no
// earlier test sends, which is exactly where a wrong default hides.
func TestB18_DefaultsAndClamps(t *testing.T) {
	// A config with neither resolution stated falls back to one token, rather
	// than to zero — which would divide the interval forever.
	s := New(Config{Min: 0, Max: 16})
	for i := 0; ; i++ {
		if i > 64 {
			t.Fatal("a config with no resolution must still terminate")
		}
		n := s.Next()
		if n == 0 {
			break
		}
		s.Record(Result{PrefixTokens: n, Wrote: n >= 9, CachedTokens: n})
	}
	if lo, hi := s.Bracket(); lo >= 9 || hi < 9 {
		t.Errorf("bracket (%d, %d] excludes the floor 9", lo, hi)
	}

	// A bracket two wide has no interior midpoint, so the clamps decide the
	// probe. Without them the search proposes an endpoint it has already
	// answered and spends money learning nothing.
	tight := New(Config{Min: 100, Max: 102, Resolution: 1})
	if n := tight.Next(); n != 101 {
		t.Errorf("probe = %d, want the only untested size 101", n)
	}

	// A relative resolution small enough to round to zero must still stop.
	fine := New(Config{Min: 0, Max: 64, RelativeResolution: 0.0001})
	for i := 0; ; i++ {
		if i > 128 {
			t.Fatal("a sub-token relative resolution must clamp to one token, not loop")
		}
		n := fine.Next()
		if n == 0 {
			break
		}
		fine.Record(Result{PrefixTokens: n, Wrote: n >= 33, CachedTokens: n})
	}

	// With no probe budget stated there is nothing to be short of.
	unbudgeted := New(Config{Min: 0, Max: 4096, Resolution: 64})
	if unbudgeted.AffordableDecisions() != 0 {
		t.Error("no budget means no affordable-decision count to report")
	}
	if unbudgeted.BudgetTooSmall() {
		t.Error("a search with no budget cannot have too small a one")
	}
	// Nor can an unusable range.
	if New(Config{Min: 500, Max: 100, MaxProbes: 1}).BudgetTooSmall() {
		t.Error("an unusable range is refused before its budget is judged")
	}
	// A relative resolution is what the budget is measured against.
	rel := New(Config{Min: 0, Max: 65536, RelativeResolution: 0.001, MaxProbes: 2, Confirm: 1})
	if !rel.BudgetTooSmall() {
		t.Error("two probes cannot resolve 65,536 to a tenth of a percent")
	}
	// A ratio so fine it rounds to less than a token clamps to one token,
	// rather than dividing by zero when the needed depth is computed.
	subToken := New(Config{Min: 0, Max: 64, RelativeResolution: 0.0001, MaxProbes: 100, Confirm: 1})
	if subToken.BudgetTooSmall() {
		t.Error("100 probes is plenty to resolve 64 tokens to one token")
	}
}

// B19: a proposed probe is always strictly inside the bracket.
//
// Both endpoints are already answered, so proposing one spends money to learn
// nothing. This was previously defended by two clamps that could never
// execute — the stop condition always returned first — so they were dead code
// shaped like a safety check. The invariant is real; the clamps were not.
//
// PASS: across the sweep space, every proposal is in (lo, hi).
// FAIL: a proposal on a boundary, which is a billable request for an answer
// already held.
func TestB19_EveryProposalIsStrictlyInsideTheBracket(t *testing.T) {
	checked := 0
	for _, span := range []int{2, 3, 9, 1024, 65536} {
		for _, res := range []int{1, 7, 64} {
			for floor := 1; floor < span; floor += max(1, span/32) {
				s := New(Config{Min: 0, Max: span, Resolution: res})
				for i := 0; i < 100; i++ {
					lo, hi := s.Bracket()
					n := s.Next()
					if n == 0 {
						break
					}
					checked++
					if n <= lo || n >= hi {
						t.Fatalf("span %d res %d floor %d: proposed %d on the boundary of (%d, %d]",
							span, res, floor, n, lo, hi)
					}
					s.Record(Result{PrefixTokens: n, Wrote: n >= floor, CachedTokens: n})
				}
			}
		}
	}
	if checked < 500 {
		t.Fatalf("only %d proposals checked; the invariant is not being exercised", checked)
	}
	t.Logf("checked %d proposals, all strictly interior", checked)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// B20: confirmations land on the same size, so a decision can be reached.
//
// Found by the first live run against a real provider. With the dither keyed
// on the probe count, every confirmation of one decision landed on a different
// size, the repeat count never reached Confirm, and nine of ten probes bought
// a single decision. The sweep missed it entirely because it runs Confirm at
// 1, where one answer always resolves — a whole parameter left at its default
// across 3,072 cases.
//
// PASS: with Confirm > 1, the search still converges within a bisection budget.
// FAIL: burning probes on sizes that can never accumulate agreement.
func TestB20_ConfirmationsAgreeOnASize(t *testing.T) {
	for _, confirm := range []int{2, 3} {
		s := New(Config{Min: 0, Max: 8192, Resolution: 256, Confirm: confirm})
		const floor = 1024
		sizes := map[int]int{}
		probes := 0
		for i := 0; ; i++ {
			if i > 200 {
				t.Fatalf("confirm %d: did not converge", confirm)
			}
			n := s.Next()
			if n == 0 {
				break
			}
			probes++
			sizes[n]++
			s.Record(Result{PrefixTokens: n, Wrote: n >= floor, CachedTokens: n})
		}
		lo, hi := s.Bracket()
		if lo >= floor || hi < floor {
			t.Errorf("confirm %d: bracket (%d, %d] excludes the floor %d", confirm, lo, hi, floor)
		}
		// Bisecting 8192 to 256 needs 5 decisions; each costs `confirm`
		// probes. Anything beyond that plus slack means probes are being spent
		// on sizes that never accumulate agreement.
		budget := (5 + 2) * confirm
		if probes > budget {
			t.Errorf("confirm %d: %d probes for 5 decisions, budget %d — sizes are not repeating",
				confirm, probes, budget)
		}
		// Every size probed must have been asked exactly `confirm` times, or
		// it was asked once and abandoned.
		for size, n := range sizes {
			if n != confirm && n != 1 {
				t.Errorf("confirm %d: size %d asked %d times, want %d", confirm, size, n, confirm)
			}
		}
	}
}

// B21: the upper bound uses the provider's own count, not our estimate.
//
// Found by the first live runs. The probe builds filler from a chars-per-token
// approximation, so the size it asked for is a guess — but the response
// carries `cache_creation_input_tokens`, which IS the number of tokens cached.
// Recording the guess when the exact figure is in hand makes every published
// bracket only as good as the approximation, and the first fable-5-1 run would
// have declared a documented 512 refuted on the strength of an estimate.
//
// The asymmetry is real and stays: a request that cached nothing reports no
// token count, so the lower bound has only the estimate behind it.
//
// PASS: the upper bound is the reported cached size; the lower bound is the
// requested size.
// FAIL: an upper bound denominated in a quantity nobody measured.
func TestB21_TheUpperBoundIsTheProvidersOwnCount(t *testing.T) {
	s := New(Config{Min: 0, Max: 8192, Resolution: 64})
	// Asked for 1000, and the provider says it actually cached 1177.
	if err := s.Record(Result{PrefixTokens: 1000, Wrote: true, CachedTokens: 1177}); err != nil {
		t.Fatal(err)
	}
	_, hi := s.Bracket()
	if hi != 1177 {
		t.Errorf("upper bound = %d, want 1177: the provider's count, not our estimate of 1000", hi)
	}

	// A write with no reported count falls back to the estimate, because
	// something is better than nothing — but only then.
	s2 := New(Config{Min: 0, Max: 8192, Resolution: 64})
	_ = s2.Record(Result{PrefixTokens: 1000, Wrote: true})
	if _, h := s2.Bracket(); h != 1000 {
		t.Errorf("with no reported count the estimate is all there is; got %d", h)
	}

	// The lower bound has only the estimate: a request that cached nothing
	// reports no token count at all.
	s3 := New(Config{Min: 0, Max: 8192, Resolution: 64})
	_ = s3.Record(Result{PrefixTokens: 900, Wrote: false})
	if lo, _ := s3.Bracket(); lo != 900 {
		t.Errorf("lower bound = %d, want the requested 900", lo)
	}
}

// B22: a bracket that cannot narrow further stops, rather than paying to
// rediscover that.
//
// Found by making the sweep's oracle unit-consistent. When a provider rounds a
// prefix up to a block, asking for 131 tokens against a 128-token block caches
// 256 — so the upper bound sticks at 256 while nothing caches below it to move
// the lower one. The proposal is a function of the bracket, so the same probe
// is proposed and the same answer returned, forever, at full price.
//
// PASS: the search stops and reports the stall; the bracket still contains the
// floor.
// FAIL: looping, which in production is an unbounded bill for no information.
func TestB22_AStalledBracketStops(t *testing.T) {
	const block, floor = 128, 1
	s := New(Config{Min: 0, Max: 1024, Resolution: 1})
	probes := 0
	for i := 0; ; i++ {
		if i > 100 {
			t.Fatal("a bracket that cannot narrow must stop, not spend the budget rediscovering it")
		}
		n := s.Next()
		if n == 0 {
			break
		}
		probes++
		actual := ((n + block - 1) / block) * block
		_ = s.Record(Result{PrefixTokens: n, Wrote: actual >= floor, CachedTokens: actual})
	}
	if !s.Stalled() {
		t.Error("a search that stopped because the provider's granularity will not express a finer answer must say so")
	}
	lo, hi := s.Bracket()
	if lo >= floor || hi < floor {
		t.Errorf("bracket (%d, %d] excludes the floor %d", lo, hi, floor)
	}
	if probes > 12 {
		t.Errorf("%d probes before noticing the bracket had stopped moving", probes)
	}
}
