package main

import (
	"testing"
)

// The p90 is the nearest rank: the smallest value with at least nine tenths
// of the sample at or below it. percentile took index int(0.9*(n-1)), one rank
// low whenever n mod 10 was 2 to 9, so over the costs [1, 100] the p90 was 1
// and printed beneath a median of 50.5 (2026-10-14).

// nearestRankByCounting finds the value by counting, not by the index formula
// the fix uses, so the two cannot share a mistake.
func nearestRankByCounting(sorted []float64, p float64) float64 {
	for _, v := range sorted {
		at := 0
		for _, w := range sorted {
			if w <= v {
				at++
			}
		}
		if float64(at) >= p*float64(len(sorted)) {
			return v
		}
	}
	return sorted[len(sorted)-1]
}

func TestCP1_TheP90IsTheNearestRank(t *testing.T) {
	for n := 1; n <= 21; n++ {
		s := make([]float64, n)
		for i := range s {
			s[i] = float64(i + 1)
		}
		if got, want := percentile(s, 0.9), nearestRankByCounting(s, 0.9); got != want {
			t.Errorf("n=%d: p90 %v, want the nearest rank %v", n, got, want)
		}
	}
	if got := percentile([]float64{1, 100}, 0.9); got != 100 {
		t.Errorf("p90 of [1 100] is %v, want 100", got)
	}
}

func TestCP2_TheP90IsNeverBelowTheMedian(t *testing.T) {
	cases := [][]float64{
		{1, 100},
		{5, 5, 5, 5},
		{1, 1, 1, 50, 50},
		{0.01, 0.02, 0.03, 9, 10, 11, 900},
		{3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 40, 400},
	}
	for n := 1; n <= 40; n++ {
		s := make([]float64, n)
		for i := range s {
			s[i] = float64(i*i + 1)
		}
		cases = append(cases, s)
	}
	for _, s := range cases {
		if p90, med := percentile(s, 0.9), percentile(s, 0.5); p90 < med {
			t.Errorf("%v: p90 %v below the median %v", s, p90, med)
		}
	}
	// Through the summary the report and the share card print.
	s := summarise([]costUnit{{CostUSD: 1}, {CostUSD: 100}})
	if s.P90USD < s.MedianUSD {
		t.Errorf("summary p90 %v below its median %v", s.P90USD, s.MedianUSD)
	}
}

// What the fix must not move: the even-count median is the midpoint, the odd
// one the middle value, and the ends are the ends.
func TestCP3_TheMedianAndTheBoundsAreUnchanged(t *testing.T) {
	cases := []struct {
		s    []float64
		p    float64
		want float64
	}{
		{nil, 0.9, 0},
		{nil, 0.5, 0},
		{[]float64{7}, 0.9, 7},
		{[]float64{7}, 0.5, 7},
		{[]float64{1, 2, 3, 4}, 0.5, 2.5},
		{[]float64{1, 2, 3}, 0.5, 2},
		{[]float64{2, 2, 2, 9}, 0.5, 2},
		{[]float64{1, 2, 3, 4, 5}, 0, 1},
		{[]float64{1, 2, 3, 4, 5}, 1, 5},
	}
	for _, c := range cases {
		if got := percentile(c.s, c.p); got != c.want {
			t.Errorf("percentile(%v, %v) = %v, want %v", c.s, c.p, got, c.want)
		}
	}
}
