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
	// Gap is the difference between this request's timestamp and the previous
	// one's. What that measures depends on which reader supplied them, and
	// the two do not agree.
	//
	// From the ledger it is start to start: the proxy stamps a record when the
	// request arrives (proxy/server.go:490) and carries the latency separately,
	// so this is the interval between two request starts.
	//
	// From a Claude Code transcript it is completion to completion. The
	// timestamp is the assistant line's, which approximates when the response
	// finished — docs/design-review-2026-09-02.md:33 records that as risk R6
	// and accepts it — and the parser gives the request and its output THE
	// SAME line's instant, so no duration is recoverable to correct with. The
	// gap therefore carries the difference of the two responses' durations:
	// a long generation after a short one inflates it, and the reverse deflates
	// it. TestTranscriptRequestsCarryNoDuration pins that.
	//
	// This matters because cachemodel.ClassifyBreak calls a gap longer than the
	// TTL a certain cause. Near the threshold that verdict inherits the error
	// above, in whichever direction the two durations happen to fall.
	//
	// It is NOT corrected here, and re-anchoring it on the previous response
	// would be the wrong correction anyway: the provider refreshes a cache
	// entry when it READS the prefix, which is at the start of a request, not
	// at the end of the one before. The ledger already measures that interval.
	// A transcript cannot, and no arithmetic on one clock recovers what the
	// other clock never wrote down.
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
// its supposed predecessor had answered was in flight beside it. Where neither
// says anything, the answer is that nobody looked.
//
// On the Claude Code reader that fallback is weaker than it reads. The parser
// gives a request and its output the same assistant line's timestamp
// (claudecode.go:228, :317, :452), so prev.Output.Timestamp IS prev.Timestamp
// and the test below reduces to asking whether the lane's timestamps run
// backwards. That is a test for a file written out of order, not for two
// requests open at once. It stays because it is correct on any source that does
// record a response instant — the ledger does — and because removing it would
// leave the ledger's own transcript-shaped path with nothing.
//
// TestTranscriptRequestsCarryNoDuration pins the equality it turns on, so
// whoever makes a transcript carry a response instant will be told that this
// fallback becomes real at the same moment.
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
