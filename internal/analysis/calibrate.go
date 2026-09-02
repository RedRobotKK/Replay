// Package analysis turns parsed requests into calibrated findings: whether
// the provider's cache behavior is reproduced, where it broke and why, which
// content costs the most, and what alternative layouts would have cost.
//
// Every figure is either measured (taken from provider usage) or estimated
// (derived through the byte-to-token fit) and reports say which.
package analysis

import (
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// CalibrationThreshold is the share of turns whose cache read must be
// reproduced (or exceeded) before alternative layouts are scored.
const CalibrationThreshold = 0.95

// Turn is one request's cache read compared with the expectation from the
// request before it.
type Turn struct {
	Index    int
	Request  *transcript.Request
	Previous *transcript.Request
	Outcome  cachemodel.ReadOutcome
	Expected int
	Actual   int
	// Gap is the time since the previous request started.
	Gap time.Duration
}

// Calibration is the per-lane result of checking every turn.
type Calibration struct {
	Lane       *transcript.Lane
	Turns      []Turn
	Reproduced int
	Exceeded   int
	Broken     int
}

// Compared is the number of turns that had a predecessor to compare with.
func (c *Calibration) Compared() int {
	return c.Reproduced + c.Exceeded + c.Broken
}

// MatchRate is the share of compared turns whose read was reproduced or
// exceeded. Exceeded counts as a match because the provider served at least
// the prefix the model predicted.
func (c *Calibration) MatchRate() float64 {
	if c.Compared() == 0 {
		return 1
	}
	return float64(c.Reproduced+c.Exceeded) / float64(c.Compared())
}

// Passes reports whether alternatives may be scored for this lane.
func (c *Calibration) Passes() bool {
	return c.MatchRate() >= CalibrationThreshold
}

// Calibrate checks every turn of a lane against the expected-read invariant.
func Calibrate(lane *transcript.Lane) *Calibration {
	cal := &Calibration{Lane: lane}
	for i, req := range lane.Requests {
		t := Turn{Index: i, Request: req, Actual: req.Usage.CacheRead}
		if i == 0 {
			t.Outcome = cachemodel.ReadFirst
			cal.Turns = append(cal.Turns, t)
			continue
		}
		prev := lane.Requests[i-1]
		t.Previous = prev
		t.Gap = req.Timestamp.Sub(prev.Timestamp)
		t.Outcome, t.Expected = cachemodel.ClassifyRead(prev.Usage, req.Usage)
		switch t.Outcome {
		case cachemodel.ReadReproduced:
			cal.Reproduced++
		case cachemodel.ReadExceeded:
			cal.Exceeded++
		case cachemodel.ReadBroken:
			cal.Broken++
		}
		cal.Turns = append(cal.Turns, t)
	}
	return cal
}
