package main

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"time"
)

// Summary is one distribution of durations, in nanoseconds so the JSON keeps
// full precision and a reader converts once.
type Summary struct {
	N      int   `json:"n"`
	MinNS  int64 `json:"min_ns"`
	P50NS  int64 `json:"p50_ns"`
	P95NS  int64 `json:"p95_ns"`
	P99NS  int64 `json:"p99_ns"`
	MaxNS  int64 `json:"max_ns"`
	MeanNS int64 `json:"mean_ns"`
}

// Spread describes one metric across repeats: its median, its range, and its
// coefficient of variation, which is the repeatability figure. CV is NaN when
// it is undefined (fewer than two values, or a mean of zero), never zero.
type Spread struct {
	N      int     `json:"n"`
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	CV     float64 `json:"cv"`
}

// Percentile is the nearest-rank percentile of an already sorted slice:
// index ceil(p*n)-1, clamped. It always returns an observed sample, never an
// interpolation between two. internal/proxy's BenchmarkAddedLatency uses
// floor(p*(n-1)) instead; on small n the two can pick different samples, so
// figures from the two are not interchangeable at the tail.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	i := int(math.Ceil(p*float64(n))) - 1
	i = max(0, min(i, n-1))
	return sorted[i]
}

// Summarize sorts a copy of samples and reads its percentiles.
func Summarize(samples []time.Duration) Summary {
	if len(samples) == 0 {
		return Summary{}
	}
	s := slices.Clone(samples)
	slices.Sort(s)
	var sum time.Duration
	for _, d := range s {
		sum += d
	}
	return Summary{
		N:      len(s),
		MinNS:  s[0].Nanoseconds(),
		P50NS:  Percentile(s, 0.50).Nanoseconds(),
		P95NS:  Percentile(s, 0.95).Nanoseconds(),
		P99NS:  Percentile(s, 0.99).Nanoseconds(),
		MaxNS:  s[len(s)-1].Nanoseconds(),
		MeanNS: (sum / time.Duration(len(s))).Nanoseconds(),
	}
}

// PairedOverhead summarizes proxied[i]-direct[i] over pairs taken back to
// back. Because each pair shares its moment, drift that moves both halves
// cancels in the difference; subtracting two separately-taken percentiles,
// the 2026-09-03 method, cancels nothing. Negative differences are kept.
func PairedOverhead(direct, proxied []time.Duration) (Summary, error) {
	if len(direct) != len(proxied) {
		return Summary{}, errors.New("paired overhead needs the same number of samples in each arm")
	}
	diff := make([]time.Duration, len(direct))
	for i := range direct {
		diff[i] = proxied[i] - direct[i]
	}
	return Summarize(diff), nil
}

// PercentileDifference is the 2026-09-03 method, kept so the two can be
// compared on the same data: each proxied percentile minus the direct one.
func PercentileDifference(direct, proxied Summary) Summary {
	return Summary{
		N:      min(direct.N, proxied.N),
		P50NS:  proxied.P50NS - direct.P50NS,
		P95NS:  proxied.P95NS - direct.P95NS,
		P99NS:  proxied.P99NS - direct.P99NS,
		MeanNS: proxied.MeanNS - direct.MeanNS,
	}
}

// SpreadOf reports the median, range and coefficient of variation (sample
// standard deviation over mean) of xs.
func SpreadOf(xs []float64) Spread {
	if len(xs) == 0 {
		return Spread{Median: math.NaN(), Min: math.NaN(), Max: math.NaN(), CV: math.NaN()}
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	n := len(s)
	median := s[n/2]
	if n%2 == 0 {
		median = (s[n/2-1] + s[n/2]) / 2
	}
	out := Spread{N: n, Median: median, Min: s[0], Max: s[n-1], CV: math.NaN()}
	if n < 2 {
		return out
	}
	var sum float64
	for _, x := range s {
		sum += x
	}
	mean := sum / float64(n)
	if mean == 0 {
		return out
	}
	var ss float64
	for _, x := range s {
		ss += (x - mean) * (x - mean)
	}
	out.CV = math.Sqrt(ss/float64(n-1)) / math.Abs(mean)
	return out
}

// spreadJSON is Spread on the wire. JSON has no NaN, and a sentinel number
// could collide with a real overhead, so an undefined statistic is null.
type spreadJSON struct {
	N      int      `json:"n"`
	Median *float64 `json:"median"`
	Min    *float64 `json:"min"`
	Max    *float64 `json:"max"`
	CV     *float64 `json:"cv"`
}

func defined(x float64) *float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return nil
	}
	return &x
}

func orNaN(p *float64) float64 {
	if p == nil {
		return math.NaN()
	}
	return *p
}

// MarshalJSON writes undefined statistics as null.
func (s Spread) MarshalJSON() ([]byte, error) {
	return json.Marshal(spreadJSON{N: s.N, Median: defined(s.Median), Min: defined(s.Min), Max: defined(s.Max), CV: defined(s.CV)})
}

// UnmarshalJSON reads null back as NaN.
func (s *Spread) UnmarshalJSON(b []byte) error {
	var w spreadJSON
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*s = Spread{N: w.N, Median: orNaN(w.Median), Min: orNaN(w.Min), Max: orNaN(w.Max), CV: orNaN(w.CV)}
	return nil
}
