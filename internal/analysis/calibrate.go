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
	// Correlation is how firmly Previous can be called this request's
	// predecessor. Calibrate pairs request i with request i-1 in the lane
	// slice, and that slice is in the order the records were written, which
	// is the order the responses finished. Where those two requests were in
	// flight together the pairing is a coin toss, and every per-event claim
	// built on it inherits that.
	Correlation string
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

// ExactRate is the share of compared turns whose read was reproduced exactly,
// with Exceeded left out of the numerator.
//
// MatchRate above keeps its published meaning and this sits beside it, because
// the two answer different questions and the headline was only ever reporting
// one of them. An exceeded read is the provider serving MORE cached prefix
// than the model predicted — usually a concurrent sibling lane extended it —
// and "the provider served at least what we predicted" is a weaker claim than
// "we predicted the read". A larger-than-predicted read is still a prediction
// that was wrong, in the other direction, and until 2026-09-11 every surface
// folded it into the number that sells the tool without saying so.
//
// On the corpus measured 2026-09-11 — 37925 compared turns, 1816 transcripts,
// 118 distinct sessions — the headline 97.87% match is 94.10% exact: 1433
// exceeded turns, 3.86% of everything counted as a match. Redefining MatchRate
// would have moved a number other documents quote; reporting both moves
// nothing and hides nothing.
//
// Zero when nothing was compared, for the reason MatchRate gives: an absent
// measurement must not read as a good one.
func (c *Calibration) ExactRate() float64 {
	if !c.HasEvidence() {
		return 0
	}
	return float64(c.Reproduced) / float64(c.Compared())
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

// correlationOf says how firmly this turn's request can be joined to the one
// the lane order puts before it.
//
// The proxy takes the reading directly — it is the only thing that can, since
// it is the only thing that sees both requests open at once — and puts it on
// the record. A transcript carries no such field and never will, so the
// fallback is the one thing a transcript does say: a request that began before
// its supposed predecessor had answered was in flight beside it. That is a
// weaker instrument than the proxy's counter and it is not nothing, and where
// it says neither, the answer is that nobody looked.
func correlationOf(prev, cur *transcript.Request) string {
	if cur.Correlation != transcript.CorrelationUnmeasured {
		return cur.Correlation
	}
	if prev.Output != nil && !prev.Output.Timestamp.IsZero() && cur.Timestamp.Before(prev.Output.Timestamp) {
		return transcript.CorrelationLaneOverlap
	}
	return transcript.CorrelationUnmeasured
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
		t.Correlation = correlationOf(prev, req)
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
