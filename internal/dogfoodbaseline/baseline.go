// Package dogfoodbaseline pins what survives of
// docs/evidence/dogfood-baseline-2026-09-25.md.
//
// # Why there is no reproduction harness here
//
// The baseline is `replay cost ~/.claude/projects` run on 2026-09-25. Its
// input is the operator's live transcript tree, which grows continuously. The
// measurement is a dated observation of a population that no longer exists,
// and re-running the command today answers a different question.
//
// That is not speculation. The documented command was run again on 2026-09-26
// and returned 482 sessions and $18,105.79 against the frozen 127 sessions and
// $17,738.82. **That reading is recorded as evidence of divergence and is not
// a reproduction, not a correction, and not a forward comparison.** The
// forward comparison the document pre-registers remains unrun.
//
// So this package deliberately reads nothing from disk. Building a harness
// that pointed at ~/.claude/projects would manufacture a reproduction out of a
// different population, which is the precise error the document was written to
// avoid: comparing totals across time measures the calendar.
//
// # What is still guaranteed
//
// Internal consistency. The published components sum to the published total,
// and the three rate metrics the document permits a forward comparison to use
// are arithmetically entailed by that block rather than independently
// asserted. Those hold forever, without the tree, and they are what this
// package tests.
package dogfoodbaseline

import "math"

// Baseline is the frozen block, transcribed from the evidence file.
//
// Transcribed rather than computed: there is nothing left to compute it from.
// The figures are the artifact.
type Baseline struct {
	Sessions   int
	AgentLanes int
	Requests   int

	CacheWrite float64
	CacheRead  float64
	Uncached   float64
	Output     float64
	Total      float64
	Rebilled   float64

	// SecondaryTotalAsWritten is the second, disagreeing total the document
	// prints beside the dominant session. It is carried so the discrepancy is
	// visible in code rather than only in prose.
	SecondaryTotalAsWritten float64

	// SourceReproducible is false and cannot honestly be otherwise. The input
	// population is gone.
	SourceReproducible bool
}

// Frozen returns the block exactly as published on 2026-09-25.
func Frozen() Baseline {
	return Baseline{
		Sessions: 127, AgentLanes: 2229, Requests: 94466,
		CacheWrite: 2660.30, CacheRead: 13880.70, Uncached: 2.04,
		Output: 1195.78, Total: 17738.82, Rebilled: 668.43,
		SecondaryTotalAsWritten: 17738.65,
		SourceReproducible:      false,
	}
}

// ComponentSum is what the four priced components add up to.
func (b Baseline) ComponentSum() float64 {
	return b.CacheWrite + b.CacheRead + b.Uncached + b.Output
}

// RebilledShare is the first permitted metric, as a percentage.
func (b Baseline) RebilledShare() float64 { return b.Rebilled / b.Total * 100 }

// WriteShareOfCached is the second: write against write plus read, the spend
// that passed through the cache at all.
func (b Baseline) WriteShareOfCached() float64 {
	return b.CacheWrite / (b.CacheWrite + b.CacheRead) * 100
}

// ReadToWrite is the third, and the one whose improvement direction is up.
func (b Baseline) ReadToWrite() float64 { return b.CacheRead / b.CacheWrite }

// Observed is a later reading of the same command, kept only to show that the
// source population changed.
//
// It is NOT a baseline, NOT a comparison, and must not be differenced against
// Frozen(). The document's own rule is that totals across time measure the
// calendar, and these two readings are a day and 355 sessions apart.
type Observed struct {
	Sessions   int
	AgentLanes int
	Total      float64
}

// Divergence2026_09_26 is what the documented command returned a day later.
func Divergence2026_09_26() Observed {
	return Observed{Sessions: 482, AgentLanes: 2587, Total: 18105.79}
}

func sameCents(a, b float64) bool { return math.Abs(a-b) < 0.005 }

// closeTo compares against a figure the document prints to three decimals.
func closeTo(got, want float64) bool { return math.Abs(got-want) < 0.005 }
