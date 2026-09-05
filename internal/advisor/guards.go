package advisor

import (
	"math"
	"sort"
)

// MinGuardSessions is how many sessions a guard threshold needs before it is
// offered. A fence from three sessions is a fence from noise, and it would be
// printed with exactly the same confidence as one from three hundred.
const MinGuardSessions = 10

// Fence is a Tukey upper fence and the derivation behind it, because a
// threshold a user cannot check is a threshold they have to take on faith,
// which is the thing this tool exists not to ask for.
type Fence struct {
	Q1, Q3, IQR float64
	// Upper is Q3 + 1.5*IQR: the value above which a session is an outlier
	// against this user's own history.
	Upper float64
	// Median is carried so the report can say what an ordinary session looks
	// like beside the cap being suggested.
	Median float64
	N      int
}

// UpperFence computes Q3 + 1.5*IQR over the samples, or reports that it will
// not.
//
// Quartiles use linear interpolation between order statistics (the method R
// calls type 7 and most spreadsheets use), so the answer matches what a person
// gets checking it by hand. It refuses three ways: too few samples, no usable
// samples, and zero spread. The last is the subtle one, because a fence over
// identical sessions sits exactly on the typical session, and a cap there
// would refuse ordinary work while looking like it came from evidence.
func UpperFence(samples []float64, floor int) (Fence, bool) {
	xs := make([]float64, 0, len(samples))
	for _, v := range samples {
		// Not data: a negative cost or a NaN is an upstream bug, and letting
		// either into a quartile moves a threshold that refuses live traffic.
		if v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			xs = append(xs, v)
		}
	}
	if len(xs) < floor || len(xs) < 4 {
		return Fence{}, false
	}
	sort.Float64s(xs)

	q1, q3 := quantile(xs, 0.25), quantile(xs, 0.75)
	iqr := q3 - q1
	if iqr <= 0 {
		return Fence{}, false
	}
	return Fence{
		Q1: q1, Q3: q3, IQR: iqr,
		Upper:  q3 + 1.5*iqr,
		Median: quantile(xs, 0.5),
		N:      len(xs),
	}, true
}

// quantile interpolates between order statistics. xs must be sorted.
func quantile(xs []float64, p float64) float64 {
	idx := p * float64(len(xs)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return xs[lo]
	}
	return xs[lo] + (idx-float64(lo))*(xs[hi]-xs[lo])
}
