// Package analysis turns parsed requests into calibrated findings: whether
// the provider's cache behavior is reproduced, where it broke and why, which
// content costs the most, and what alternative layouts would have cost.
//
// Every figure is either measured (taken from provider usage) or estimated
// (derived through the byte-to-token fit); the Tokens type carries that
// distinction with the number, and reports print it.
package analysis

import (
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
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

// HasEvidence reports whether any turn was actually checked.
//
// A lane's first request is always ReadFirst — there is nothing before it to
// compare against — so a single-request lane offers no evidence at all. That
// is not a failure and not a success; it is an absence, and it needs its own
// name so a caller cannot mistake it for either.
func (c *Calibration) HasEvidence() bool { return c.Compared() > 0 }

// MatchRate is the share of compared turns whose read was reproduced or
// exceeded. Exceeded counts as a match because the provider served at least
// the prefix the model predicted.
//
// Zero when nothing was compared. It returned 1 until 2026-09-06, which made
// every threshold test on it pass for free on exactly the lanes that had
// tested nothing: 18 of 1450 lanes in the real corpus scored a perfect 100%
// for having no turn to check, and every one of them was admitted to
// alternative scoring. An absent measurement must not read as a good one.
//
// Callers wanting the gate should use Passes, which asks for evidence first.
func (c *Calibration) MatchRate() float64 {
	if !c.HasEvidence() {
		return 0
	}
	return float64(c.Reproduced+c.Exceeded) / float64(c.Compared())
}

// Passes reports whether alternatives may be scored for this lane.
//
// "The check never ran" must not be inside the passing case, and it is kept
// out structurally rather than by a second guard here: MatchRate reports 0
// without evidence, and 0 is below any threshold worth having. An explicit
// `HasEvidence() &&` was tried and removed — with the threshold a constant
// 0.95 its removal changed nothing observable, which makes it dead code by
// ADR-0014's own standard rather than defence in depth.
//
// That makes this gate depend on MatchRate's behaviour at zero, so
// TestE2_NoEvidenceIsNotAPerfectScore pins exactly that: the empty rate must
// sit below CalibrationThreshold, not merely differ from 1.
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
