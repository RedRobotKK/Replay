package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The measurement series.
//
// A single floor is a fact anyone can copy the day it is published. "The floor
// changed on this date" can only be produced by someone who was measuring
// before the change, and it cannot be backfilled at any price — a competitor
// can only start their own clock, however long ago ours started. That is the
// only thing here that compounds, and it exists only if readings are stored.
//
// The file is append-only JSON Lines: one reading per line, oldest first,
// never rewritten. Losing an earlier entry costs exactly the part that cannot
// be rebuilt.

// MethodVersion fingerprints how a reading was taken.
//
// Bump it whenever a change would move the numbers, and say what changed. The
// method changed four times on 2026-09-05 and each change moved them: sizing
// the prefix by estimate rather than by the provider's own count, measuring
// the whole request rather than the cacheable prefix, English filler rather
// than varied CJK, and reading a warm cache write as a prefix size. Readings
// taken either side of any of those are not comparable, and nothing in a bare
// number says so.
//
// A series that silently mixes methods is worse than no series: a change in
// the numbers cannot be told from a change in how they were produced, which is
// precisely the question the series exists to answer.
const MethodVersion = "2026-09-06.1"

// Reading is one measurement of one model at one time.
type Reading struct {
	TakenAt string `json:"takenAt"`
	Method  string `json:"method"`

	// Model is what was asked for; AnsweredBy what the provider said answered.
	// They differ whenever an alias is used, and the provider may not name a
	// snapshot at all — in which case only the date distinguishes readings.
	Model       string `json:"model"`
	AnsweredBy  string `json:"answeredBy,omitempty"`
	ServiceTier string `json:"serviceTier,omitempty"`
	Geo         string `json:"geo,omitempty"`

	// Above and AtMost bracket the caching floor. Both are omitted when the
	// run did not establish one — see Outcome.
	Above  int `json:"above,omitempty"`
	AtMost int `json:"atMost,omitempty"`

	// Documented is the published figure at the time of the reading, so a
	// later change to the documentation is visible against the measurement.
	Documented int `json:"documented,omitempty"`

	// Outcome names anything other than a plain bracket: "non-deterministic",
	// "contradicted", "stalled", "budget-exhausted". Empty means a clean
	// bracket.
	Outcome string `json:"outcome,omitempty"`

	Probes int `json:"probes,omitempty"`
	// Confirm is how many agreeing answers each boundary needed, which is what
	// separates a measurement from a single observation.
	Confirm int `json:"confirm,omitempty"`
}

// AppendReading adds one reading to the series.
//
// Append-only and owner-only. The series says which models an account measured
// and when — not secret, but not the world's business, and the rest of
// ~/.replay is owner-only too.
func AppendReading(path string, r Reading) error {
	if r.TakenAt == "" {
		r.TakenAt = time.Now().UTC().Format(time.RFC3339)
	}
	if r.Method == "" {
		r.Method = MethodVersion
	}
	// An inconclusive run carries no bounds. Storing the untouched search
	// range as though it were a measurement is how a series stops being able
	// to tell a result from a failure.
	if r.Outcome != "" && r.Outcome != "budget-exhausted" {
		r.Above, r.AtMost = 0, 0
	}

	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open the measurement series: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// ReadingFrom builds a reading from a finished search.
func ReadingFrom(model string, documented int, s *Search, p Provenance, confirm int) Reading {
	lo, hi := s.Bracket()
	r := Reading{
		Model:       model,
		AnsweredBy:  p.ResolvedModel,
		ServiceTier: p.ServiceTier,
		Geo:         p.Geo,
		Above:       lo,
		AtMost:      hi,
		Documented:  documented,
		Probes:      s.Probes(),
		Confirm:     confirm,
	}
	switch {
	case p.Mixed:
		r.Outcome = "mixed-provenance"
	case s.NonDeterministic():
		r.Outcome = "non-deterministic"
	case s.Contradicted():
		r.Outcome = "contradicted"
	case s.StoppedEarly():
		r.Outcome = "budget-exhausted"
	case s.Stalled():
		r.Outcome = "stalled"
	}
	return r
}
