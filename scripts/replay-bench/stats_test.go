package main

import (
	"math"
	"testing"
	"time"
)

// The harness reports numbers other documents will quote, so the arithmetic
// under them is tested before anything is measured with it. Each case is
// worked by hand in its comment so a reader can check the expectation without
// trusting the code.

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestPercentileIsNearestRank(t *testing.T) {
	// Ten sorted samples 1..10ms. Nearest rank: index ceil(p*n)-1.
	//   p50 -> ceil(5)-1   = 4 -> 5ms
	//   p95 -> ceil(9.5)-1 = 9 -> 10ms
	//   p99 -> ceil(9.9)-1 = 9 -> 10ms
	//   p10 -> ceil(1)-1   = 0 -> 1ms
	sorted := []time.Duration{ms(1), ms(2), ms(3), ms(4), ms(5), ms(6), ms(7), ms(8), ms(9), ms(10)}
	for _, c := range []struct {
		p    float64
		want time.Duration
	}{{0.50, ms(5)}, {0.95, ms(10)}, {0.99, ms(10)}, {0.10, ms(1)}, {1.0, ms(10)}, {0, ms(1)}} {
		if got := Percentile(sorted, c.p); got != c.want {
			t.Errorf("Percentile(p=%v) = %v, want %v", c.p, got, c.want)
		}
	}
	if got := Percentile(nil, 0.5); got != 0 {
		t.Errorf("Percentile(empty) = %v, want 0", got)
	}
}

func TestSummarizeDoesNotRequireSortedInputAndDoesNotMutateIt(t *testing.T) {
	in := []time.Duration{ms(9), ms(1), ms(5), ms(3), ms(7)}
	s := Summarize(in)
	// Sorted: 1 3 5 7 9. p50 = idx ceil(2.5)-1 = 2 -> 5ms. Mean 25/5 = 5ms.
	if s.N != 5 || s.MinNS != ms(1).Nanoseconds() || s.MaxNS != ms(9).Nanoseconds() ||
		s.P50NS != ms(5).Nanoseconds() || s.MeanNS != ms(5).Nanoseconds() {
		t.Errorf("Summarize = %+v", s)
	}
	if in[0] != ms(9) {
		t.Errorf("Summarize sorted its caller's slice: %v", in)
	}
}

func TestPairedOverheadIsTheMedianOfPerPairDifferences(t *testing.T) {
	// Pairs (direct, proxied): (10,13) (10,12) (20,23) (10,11) (10,30)
	// Differences: 3 2 3 1 20 -> sorted 1 2 3 3 20 -> median (p50) 3ms.
	// The subtraction-of-percentiles method on the same data would read
	// p50(proxied)=13 - p50(direct)=10 = 3 too, but p99 differs: 30-20 = 10
	// against a paired p99 of 20. They are different quantities; both are kept.
	direct := []time.Duration{ms(10), ms(10), ms(20), ms(10), ms(10)}
	proxied := []time.Duration{ms(13), ms(12), ms(23), ms(11), ms(30)}
	got, err := PairedOverhead(direct, proxied)
	if err != nil {
		t.Fatal(err)
	}
	if got.P50NS != ms(3).Nanoseconds() || got.MaxNS != ms(20).Nanoseconds() || got.MinNS != ms(1).Nanoseconds() {
		t.Errorf("PairedOverhead = %+v", got)
	}
	if _, err := PairedOverhead(direct, proxied[:4]); err == nil {
		t.Error("unequal arms must be refused: a pair needs both halves")
	}
}

func TestPairedOverheadKeepsNegativeDifferences(t *testing.T) {
	// A proxied sample faster than its direct partner is noise, and dropping
	// it would bias the overhead upward. It must survive as a negative value.
	got, err := PairedOverhead([]time.Duration{ms(5)}, []time.Duration{ms(3)})
	if err != nil {
		t.Fatal(err)
	}
	if got.P50NS != -ms(2).Nanoseconds() {
		t.Errorf("negative difference lost: %+v", got)
	}
}

func TestPercentileDifference(t *testing.T) {
	d := Summary{P50NS: 100, P95NS: 200, P99NS: 300}
	p := Summary{P50NS: 150, P95NS: 260, P99NS: 290}
	got := PercentileDifference(d, p)
	if got.P50NS != 50 || got.P95NS != 60 || got.P99NS != -10 {
		t.Errorf("PercentileDifference = %+v", got)
	}
}

func TestSpreadMedianAndCoefficientOfVariation(t *testing.T) {
	// 2 4 4 4 5 5 7 9: mean 5, sample variance 32/7, sd 2.138..., CV 0.4276.
	// Median of an even count is the mean of the middle two: (4+5)/2 = 4.5.
	s := SpreadOf([]float64{9, 2, 4, 4, 4, 5, 5, 7})
	if s.N != 8 || s.Median != 4.5 || s.Min != 2 || s.Max != 9 {
		t.Errorf("SpreadOf = %+v", s)
	}
	if want := math.Sqrt(32.0/7.0) / 5.0; math.Abs(s.CV-want) > 1e-9 {
		t.Errorf("CV = %v, want %v", s.CV, want)
	}
	// One value has no spread to report: CV is undefined, reported as NaN
	// rather than a reassuring zero.
	if one := SpreadOf([]float64{3}); !math.IsNaN(one.CV) || one.Median != 3 {
		t.Errorf("SpreadOf(single) = %+v, want CV NaN", one)
	}
	if empty := SpreadOf(nil); empty.N != 0 || !math.IsNaN(empty.Median) {
		t.Errorf("SpreadOf(empty) = %+v", empty)
	}
}

func TestCoefficientOfVariationOfAZeroMeanIsUndefined(t *testing.T) {
	// Overheads can straddle zero. Dividing by a mean of zero would print an
	// infinite or enormous CV; it is reported as undefined instead.
	if s := SpreadOf([]float64{-1, 1}); !math.IsNaN(s.CV) {
		t.Errorf("CV with zero mean = %v, want NaN", s.CV)
	}
}
