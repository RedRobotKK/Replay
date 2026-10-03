// Command simexp counts the pre-registered primary outcome of the simulate
// experiment over a frozen, participant-reported dataset.
//
// It is an experiment measurement, not a Replay product primitive, and it is
// not part of the shipped binary. The rule it applies is element 13 of
// docs/evidence/simulate-experiment-prereg-2026-10-03.md; the row shape is
// element 12; the way it treats UNKNOWN is element 8; the window is element
// 11. Nothing here reads a ledger, a refusal reason or a filesystem for
// evidence of a cap change: every value is what the participant reported
// (elements 2 and 15, and the gatekeeper's Decisions 2 and 3).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const (
	datasetSchema   = "replay.simexp.dataset.v1"
	resultSchema    = "replay.simexp.result.v1"
	preregistration = "docs/evidence/simulate-experiment-prereg-2026-10-03.md"

	// The frozen numbers. Element 14: not adjusted after any observation.
	denominator = 10
	threshold   = 3
	windowDays  = 30
	graceDays   = 3

	rule         = "fewer than 3 of 10 enrolled participants changed their spend cap because of the simulation they were shown: killed; 3 or more: survives"
	decisionKill = "PRO HYPOTHESIS KILLED / REDESIGN REQUIRED"
	decisionLive = "PRO HYPOTHESIS SURVIVES THIS EXPERIMENT"

	unknown = "UNKNOWN"
)

// The four serve settings a cap change can be a change of (element 7).
var capKeys = map[string]bool{
	"maxSessionTokens": true, "maxDayTokens": true, "maxSessionUsd": true, "maxDayUsd": true,
}

// dataset is the frozen file, exactly as element 12 describes it.
type dataset struct {
	Schema          string        `json:"schema"`
	Preregistration string        `json:"preregistration"`
	FrozenOn        string        `json:"frozenOn"`
	Window          window        `json:"window"`
	Participants    []participant `json:"participants"`
}

type window struct {
	Mode string `json:"mode"`          // "cohort" or "per-participant"
	End  string `json:"end,omitempty"` // cohort only: the pre-registered end date
}

// participant is one row. Caps are the four serve settings as the participant
// reported them, or the string UNKNOWN. A tri-state field is "yes", "no" or
// "UNKNOWN". Dates are YYYY-MM-DD or UNKNOWN.
type participant struct {
	ID             string `json:"id"`
	ExposedOn      string `json:"exposedOn"`
	Exposures      int    `json:"exposures"`
	CapBefore      caps   `json:"capBefore"`
	AlternativeCap caps   `json:"alternativeCap"`
	Changed        string `json:"changed"`
	CapAfter       *caps  `json:"capAfter,omitempty"`
	Influenced     string `json:"influenced"`
	AnsweredOn     string `json:"answeredOn"`
	// IneligibleAfterEnrolment is element 9: a participant found ineligible
	// after enrolment is reported as UNKNOWN and stays in the ten.
	IneligibleAfterEnrolment bool `json:"ineligibleAfterEnrolment,omitempty"`
}

// caps is either a set of the four serve cap values or UNKNOWN.
type caps struct {
	Unknown bool
	Values  map[string]float64
}

func (c *caps) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte(`"UNKNOWN"`)) {
		*c = caps{Unknown: true}
		return nil
	}
	var m map[string]float64
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("a cap is an object of the four serve settings or %q: %v", unknown, err)
	}
	for k, v := range m {
		if !capKeys[k] {
			return fmt.Errorf("%s is not one of the four serve cap settings", k)
		}
		if v < 0 {
			return fmt.Errorf("%s is negative", k)
		}
	}
	if m == nil {
		m = map[string]float64{}
	}
	*c = caps{Values: m}
	return nil
}

func (c caps) MarshalJSON() ([]byte, error) {
	if c.Unknown {
		return json.Marshal(unknown)
	}
	if c.Values == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(c.Values)
}

// positive reports whether any cap is set: zero or absent is no cap
// (element 7).
func (c caps) positive() bool {
	for _, v := range c.Values {
		if v > 0 {
			return true
		}
	}
	return false
}

// same compares two known cap sets with zero and absent treated alike.
func (c caps) same(o caps) bool {
	keys := map[string]bool{}
	for k := range c.Values {
		keys[k] = true
	}
	for k := range o.Values {
		keys[k] = true
	}
	for k := range keys {
		if c.Values[k] != o.Values[k] {
			return false
		}
	}
	return true
}

type result struct {
	Schema          string           `json:"schema"`
	Preregistration string           `json:"preregistration"`
	DatasetSHA256   string           `json:"datasetSha256"`
	Denominator     int              `json:"denominator"`
	Primary         primary          `json:"primary"`
	Secondary       secondary        `json:"secondary"`
	Participants    []participantRow `json:"participants"`
}

type primary struct {
	Qualifying int    `json:"qualifying"`
	Rule       string `json:"rule"`
	Decision   string `json:"decision"`
}

// secondary carries the descriptive figures the gatekeeper allowed, and
// nothing decides on any of them (pre-registration section 8).
type secondary struct {
	EnabledACap          int `json:"enabledACap"`
	ChangedAnExistingCap int `json:"changedAnExistingCap"`
	ChangedNotAttributed int `json:"changedNotAttributed"`
	Unknown              int `json:"unknown"`
	NotExposed           int `json:"notExposed"`
	CapUnchanged         int `json:"capUnchanged"`
	CapValuesUnknown     int `json:"capValuesUnknown"`
}

type participantRow struct {
	ID             string `json:"id"`
	Qualifying     bool   `json:"qualifying"`
	Outcome        string `json:"outcome"`
	CapBefore      caps   `json:"capBefore"`
	AlternativeCap caps   `json:"alternativeCap"`
	CapAfter       *caps  `json:"capAfter,omitempty"`
}

// The outcomes a row can have. The first is the only one that counts.
const (
	outQualifies     = "changed a cap because of the simulation"
	outNotAttributed = "changed a cap, not attributed to the simulation"
	outUnchanged     = "cap unchanged"
	outNotExposed    = "not exposed"
	outUnkChanged    = "UNKNOWN: whether the cap changed"
	outUnkInfluenced = "UNKNOWN: whether the simulation influenced the change"
	outUnkNoAnswer   = "UNKNOWN: no end-of-window answer"
	outUnkLate       = "UNKNOWN: answer after the window"
	outUnkIneligible = "UNKNOWN: ineligible after enrolment"
)

func rejected(format string, a ...any) error {
	return fmt.Errorf("dataset rejected: "+format, a...)
}

const day = "2006-01-02"

// date parses YYYY-MM-DD or UNKNOWN; ok is false for UNKNOWN.
func date(field, s string) (t time.Time, ok bool, err error) {
	if s == unknown {
		return time.Time{}, false, nil
	}
	t, err = time.Parse(day, s)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%s %q is not YYYY-MM-DD or %s", field, s, unknown)
	}
	return t, true, nil
}

func tristate(field, s string) error {
	switch s {
	case "yes", "no", unknown:
		return nil
	}
	return fmt.Errorf("%s %q is not yes, no or %s", field, s, unknown)
}

// count applies the pre-registered rule to the bytes of a frozen dataset.
// The digest in the result is of exactly those bytes.
func count(raw []byte) (result, error) {
	var d dataset
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return result{}, rejected("not the dataset shape: %v", err)
	}
	if d.Schema != datasetSchema {
		return result{}, rejected("schema %q; want %s", d.Schema, datasetSchema)
	}
	if d.Preregistration != preregistration {
		return result{}, rejected("preregistration %q; this counter applies %s", d.Preregistration, preregistration)
	}
	frozen, ok, err := date("frozenOn", d.FrozenOn)
	if err != nil || !ok {
		return result{}, rejected("frozenOn %q must be the date the file was frozen", d.FrozenOn)
	}
	var cohortEnd time.Time
	switch d.Window.Mode {
	case "cohort":
		cohortEnd, ok, err = date("window.end", d.Window.End)
		if err != nil || !ok {
			return result{}, rejected("window: a cohort needs its pre-registered end date, got %q", d.Window.End)
		}
	case "per-participant":
		if d.Window.End != "" {
			return result{}, rejected("window: per-participant mode takes no end date")
		}
	default:
		return result{}, rejected("window mode %q; want cohort or per-participant", d.Window.Mode)
	}
	if len(d.Participants) != denominator {
		return result{}, rejected("%d participants; the denominator is %d and cannot change", len(d.Participants), denominator)
	}
	seen := map[string]bool{}
	for _, p := range d.Participants {
		if p.ID == "" {
			return result{}, rejected("a participant has no id")
		}
		if seen[p.ID] {
			return result{}, rejected("duplicate participant id %s", p.ID)
		}
		seen[p.ID] = true
		if err := validate(p); err != nil {
			return result{}, rejected("%s: %v", p.ID, err)
		}
	}

	sum := sha256.Sum256(raw)
	r := result{
		Schema:          resultSchema,
		Preregistration: preregistration,
		DatasetSHA256:   hex.EncodeToString(sum[:]),
		Denominator:     denominator,
		Primary:         primary{Rule: rule},
	}
	for _, p := range d.Participants {
		exposed, _, _ := date("exposedOn", p.ExposedOn)
		var end time.Time
		if d.Window.Mode == "cohort" {
			end = cohortEnd
		} else if p.ExposedOn != unknown {
			end = exposed.AddDate(0, 0, windowDays)
		}
		if !end.IsZero() && frozen.Before(end) {
			return result{}, rejected("window not closed for %s (ends %s, frozen %s)", p.ID, end.Format(day), d.FrozenOn)
		}
		// Element 11: the end-of-window question is asked on or after the
		// end. An answer dated earlier is not that observation, and a row
		// carrying one is an operator error to correct before the freeze.
		if answered, ok, _ := date("answeredOn", p.AnsweredOn); ok && !end.IsZero() && answered.Before(end) {
			return result{}, rejected("%s: answeredOn %s is before the window end %s; the end-of-window question is asked on or after the end", p.ID, p.AnsweredOn, end.Format(day))
		}
		row := participantRow{ID: p.ID, CapBefore: p.CapBefore, AlternativeCap: p.AlternativeCap, CapAfter: p.CapAfter}
		row.Outcome = classify(p, end)
		switch row.Outcome {
		case outQualifies:
			row.Qualifying = true
			r.Primary.Qualifying++
			switch {
			case p.CapBefore.Unknown || p.CapAfter == nil || p.CapAfter.Unknown:
				r.Secondary.CapValuesUnknown++
			case p.CapBefore.positive():
				r.Secondary.ChangedAnExistingCap++
			default:
				r.Secondary.EnabledACap++
			}
		case outNotAttributed:
			r.Secondary.ChangedNotAttributed++
		case outUnchanged:
			r.Secondary.CapUnchanged++
		case outNotExposed:
			r.Secondary.NotExposed++
		default:
			r.Secondary.Unknown++
		}
		r.Participants = append(r.Participants, row)
	}
	if r.Primary.Qualifying < threshold {
		r.Primary.Decision = decisionKill
	} else {
		r.Primary.Decision = decisionLive
	}
	return r, nil
}

// validate checks one row's shape and internal consistency. It never decides
// an outcome; a row that contradicts itself is a rejection, not an inference.
func validate(p participant) error {
	exposed, exposedKnown, err := date("exposedOn", p.ExposedOn)
	if err != nil {
		return err
	}
	answered, answeredKnown, err := date("answeredOn", p.AnsweredOn)
	if err != nil {
		return err
	}
	if p.Exposures < 0 {
		return fmt.Errorf("exposures %d is negative", p.Exposures)
	}
	if err := tristate("changed", p.Changed); err != nil {
		return err
	}
	if err := tristate("influenced", p.Influenced); err != nil {
		return err
	}
	switch p.Changed {
	case "yes":
		if p.CapAfter == nil {
			return fmt.Errorf("changed is yes but capAfter is missing")
		}
		if !p.CapBefore.Unknown && !p.CapAfter.Unknown && p.CapBefore.same(*p.CapAfter) {
			return fmt.Errorf("changed is yes but capBefore and capAfter are identical")
		}
	case "no":
		if p.CapAfter != nil && !p.CapBefore.Unknown && !p.CapAfter.Unknown && !p.CapBefore.same(*p.CapAfter) {
			return fmt.Errorf("changed is no but capAfter differs from capBefore")
		}
	}
	if exposedKnown && answeredKnown && answered.Before(exposed) {
		return fmt.Errorf("answeredOn %s is before exposedOn %s", p.AnsweredOn, p.ExposedOn)
	}
	return nil
}

// classify is the chain of element 8, in order: eligibility (element 9),
// exposure, answer, change, attribution. The first UNKNOWN or negative stops
// it.
func classify(p participant, end time.Time) string {
	if p.IneligibleAfterEnrolment {
		return outUnkIneligible
	}
	if p.ExposedOn == unknown {
		return outNotExposed
	}
	answered, ok, _ := date("answeredOn", p.AnsweredOn)
	if !ok {
		return outUnkNoAnswer
	}
	if answered.After(end.AddDate(0, 0, graceDays)) {
		return outUnkLate
	}
	switch p.Changed {
	case unknown:
		return outUnkChanged
	case "no":
		return outUnchanged
	}
	switch p.Influenced {
	case unknown:
		return outUnkInfluenced
	case "no":
		return outNotAttributed
	}
	return outQualifies
}
