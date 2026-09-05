package cachemodel

import "fmt"

// Every published provider number is a claim, and Replay is the one tool in
// this space positioned to check them.
//
// The 2026-09-03 corpus did it by accident: the rules said the minimum
// cacheable prefix was 512 tokens, and observation bounded it at at most
// 40,563, with nothing uncached ever seen below. Either the published figure is
// wrong, or stale, or right but untestable from that sample. A disagreement
// between documentation and behaviour, found on eleven sessions from one
// laptop.
//
// So a rules document carries, per field, both what the provider says and what
// has been observed. The verdict is computed from the two. It is never written
// into the file: a hand-written "consistent" is another claim wearing a
// verdict's clothes, and the whole point is to have one field in this system
// that nobody can simply assert.

// ClaimStatus is the verdict on a documented figure.
type ClaimStatus string

const (
	// StatusUntested means no traffic has bounded this figure yet.
	StatusUntested ClaimStatus = "untested"
	// StatusUnverified means the evidence is one-sided: it agrees with the
	// documented figure but does not close the interval around it.
	StatusUnverified ClaimStatus = "unverified"
	// StatusConsistent means the documented figure sits inside a closed
	// observed interval.
	StatusConsistent ClaimStatus = "consistent"
	// StatusContradicted means real traffic disagrees with the provider's own
	// documentation. It is the most valuable value in this design: no provider
	// dashboard will ever show it, and it is only reachable by replaying real
	// requests.
	StatusContradicted ClaimStatus = "contradicted"
)

// Observation is what replaying sessions bounded a figure to.
//
// For a minimum cacheable prefix: UpperBound is the smallest prompt ever seen
// served from cache, so the true minimum is at most that. LowerBound is the
// largest prompt ever seen uncached, so the true minimum is above it. The truth
// lies in (LowerBound, UpperBound].
type Observation struct {
	UpperBound *int `json:"upperBound,omitempty"`
	LowerBound *int `json:"lowerBound,omitempty"`
	// Sessions and Machines say how much and how varied the evidence is.
	// They do not affect a contradiction, which needs only one counterexample,
	// but they are what a reader weighs when the verdict is agreement.
	Sessions int `json:"sessions,omitempty"`
	Machines int `json:"machines,omitempty"`
}

// Claim pairs a documented figure with what was observed about it.
type Claim struct {
	Documented int          `json:"documented"`
	Observed   *Observation `json:"observed,omitempty"`
	// Status_ exists only to catch a file that tries to declare a verdict.
	// validate refuses any document that sets it. Read Status() instead.
	Status_ string `json:"status,omitempty"`
}

// Status derives the verdict.
//
// Falsification is asymmetric and this encodes that. One machine seeing a
// prompt cached below the published minimum refutes the figure outright, and no
// sample size is required for that. One machine agreeing with it proves very
// little, because what is in question is the sampling rather than the sample
// size, so agreement over an open interval is reported as unverified rather
// than as confirmation.
func (c Claim) Status() ClaimStatus {
	o := c.Observed
	if o == nil || (o.UpperBound == nil && o.LowerBound == nil) {
		return StatusUntested
	}
	// A single counterexample is enough, either side.
	if o.UpperBound != nil && c.Documented > *o.UpperBound {
		return StatusContradicted
	}
	if o.LowerBound != nil && c.Documented <= *o.LowerBound {
		return StatusContradicted
	}
	if o.UpperBound != nil && o.LowerBound != nil {
		return StatusConsistent
	}
	return StatusUnverified
}

// Interval returns the closed bounds when both are known.
func (c Claim) Interval() (lower, upper int, ok bool) {
	if c.Observed == nil || c.Observed.LowerBound == nil || c.Observed.UpperBound == nil {
		return 0, 0, false
	}
	return *c.Observed.LowerBound, *c.Observed.UpperBound, true
}

// IntervalWidth is how much room is left in the answer.
//
// It travels with the verdict because "consistent" across 39,000 tokens and
// "consistent" across 8 are different statements, and the word alone cannot
// tell them apart. More machines narrow this monotonically; more sessions from
// one machine mostly do not.
func (c Claim) IntervalWidth() (int, bool) {
	lo, hi, ok := c.Interval()
	if !ok {
		return 0, false
	}
	return hi - lo, true
}

// Describe is the one line a report prints for a claim.
//
// It prints whichever bounds exist and never invents the other one. An earlier
// version formatted "(lower, upper]" unconditionally and printed "(0, 0]" for a
// one-sided observation, which is a fabricated interval on the one line whose
// job is to be checkable.
func (c Claim) Describe(field string) string {
	s := c.Status()
	if s == StatusUntested {
		return fmt.Sprintf("%s: %d documented, untested", field, c.Documented)
	}
	return fmt.Sprintf("%s: %d documented, %s, observed %s (%s)", field, c.Documented, s, c.boundsText(), c.evidence())
}

// boundsText states exactly what was seen and nothing more.
func (c Claim) boundsText() string {
	if c.Observed == nil {
		return "nothing"
	}
	lo, hi := c.Observed.LowerBound, c.Observed.UpperBound
	switch {
	case lo != nil && hi != nil:
		return fmt.Sprintf("in (%d, %d]", *lo, *hi)
	case hi != nil:
		return fmt.Sprintf("at most %d, with nothing uncached seen below it", *hi)
	case lo != nil:
		return fmt.Sprintf("above %d", *lo)
	default:
		return "nothing"
	}
}

func (c Claim) evidence() string {
	if c.Observed == nil {
		return "no evidence"
	}
	if c.Observed.Machines > 1 {
		return fmt.Sprintf("%d sessions, %d machines", c.Observed.Sessions, c.Observed.Machines)
	}
	return fmt.Sprintf("%d sessions, 1 machine", c.Observed.Sessions)
}

// validate refuses a claim that cannot be true or that tries to assert its own
// verdict.
func (c *Claim) validate(field string) error {
	if c.Status_ != "" {
		return fmt.Errorf("%s: the file declares status %q, but a status is derived from the observation and never written; remove it", field, c.Status_)
	}
	if c.Documented < 0 {
		return fmt.Errorf("%s: documented value is negative", field)
	}
	if o := c.Observed; o != nil {
		if o.LowerBound != nil && o.UpperBound != nil && *o.LowerBound > *o.UpperBound {
			return fmt.Errorf("%s: observed lower bound %d is above the upper bound %d, which no traffic can produce", field, *o.LowerBound, *o.UpperBound)
		}
		if o.Sessions < 0 || o.Machines < 0 {
			return fmt.Errorf("%s: negative evidence counts", field)
		}
	}
	return nil
}
