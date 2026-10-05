package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// applyPlan is one setting this tool is willing to change on a user's behalf.
//
// Exactly one setting qualifies today: the prompt cache TTL. It qualifies
// because it is a documented client setting, because changing it alters no
// content, and because the corpus can say what it would have cost. Every other
// suggestion the advisor makes is a change to how somebody works, and a tool
// that rewrites your instruction files because it judged them too long would be
// a worse tool than one that tells you and stops.
type applyPlan struct {
	Setting string
	// Want is the value the corpus supports; Have is what is set today.
	Want string
	Have string
	// Trustworthy gates the write. Numbers this tool would not stand behind
	// must not become changes to somebody's configuration.
	Trustworthy bool
	Reason      string
	// Evidence is the one line a person needs to judge the change themselves.
	Evidence string
	// PredictedShare is the margin the evidence line states, as a fraction:
	// the simulator's prediction, carried as a number so the record of the
	// write holds the figure the write was justified by.
	PredictedShare float64
	// Log is where the record of an applied change is appended. Empty means
	// no record is kept, which only a test should want.
	Log string
	// verify reads the setting back after a write. Nil means readSettingValue.
	verify func(path, setting string) (string, bool)
}

// readSettingValue opens the settings file again and reports the value it
// holds for the setting, UNSET when the key is absent, or false when the
// file cannot be read as JSON.
func readSettingValue(path, setting string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", false
	}
	v, ok := m[setting].(string)
	if !ok {
		return "UNSET", true
	}
	return v, true
}

// interventionLogPath is the provenance log under the reader's home.
func interventionLogPath(home string) string {
	return filepath.Join(home, ".replay", "interventions.jsonl")
}

// interventionRecord is one request to change a user's configuration, as
// it ended: INTERVENTION_APPLIED when the written value was read back from
// the file, APPLY_ATTEMPTED when it was not, INTERVENTION_REFUSED when the
// plan refused under --yes.
//
// Every field is either the fact as it was or the word UNAVAILABLE. The
// prediction is a number with its basis named; the realized effect is not
// a number until a measurement exists, and nothing here is one. The
// register in docs/evidence/ttl-register-2026-10-05.md says what a
// measurement would have to be.
type interventionRecord struct {
	Schema       string `json:"schema"`
	Event        string `json:"event"`
	Intervention string `json:"intervention"`
	Policy       string `json:"policy"`
	Setting      string `json:"setting"`
	At           string `json:"at"`
	// ApplyRequested is true for every record: a dry run writes none.
	// ApplyAttempted is false for a refusal. StateChange is VERIFIED only
	// when the value was read back from the file after the write,
	// UNVERIFIED when it was not, NOT_ATTEMPTED for a refusal.
	ApplyRequested       bool    `json:"apply_requested"`
	ApplyAttempted       bool    `json:"apply_attempted"`
	PriorValue           string  `json:"prior_value"`
	IntendedValue        string  `json:"intended_value"`
	AppliedValue         string  `json:"applied_value"`
	ActualValue          string  `json:"actual_value"`
	StateChange          string  `json:"state_change"`
	Reason               string  `json:"reason,omitempty"`
	SettingsPath         string  `json:"settings_path"`
	BackupPath           string  `json:"backup_path"`
	BackupNote           string  `json:"backup_note,omitempty"`
	PredictedEffect      float64 `json:"predicted_effect"`
	PredictedEffectBasis string  `json:"predicted_effect_basis"`
	PredictedEffectText  string  `json:"predicted_effect_text"`
	RealizedEffect       string  `json:"realized_effect"`
	Outcome              string  `json:"outcome"`
	QualityOutcome       string  `json:"quality_outcome"`
	Block                string  `json:"block"`
	Surface              string  `json:"surface"`
	Model                string  `json:"model"`
}

// transition is what happened to the setting, as it happened.
type transition struct {
	event        string
	at           time.Time
	settingsPath string
	backupPath   string
	backupNote   string
	intended     string
	applied      string
	actual       string
	stateChange  string
	attempted    bool
	reason       string
}

// record appends the provenance line for a request to apply, whether it
// ended in a verified change, an unconfirmed write, or a refusal.
func (p applyPlan) record(tr transition) error {
	if p.Log == "" {
		return nil
	}
	prior := p.Have
	if prior == "" {
		prior = "UNSET"
	}
	actual := tr.actual
	if actual == "" {
		actual = "UNSET"
	}
	policy := p.Setting + "=" + p.Want
	if p.Want == "" {
		policy = p.Setting + "=UNDECIDED"
	}
	rec := interventionRecord{
		Schema:               "replay.intervention.v1",
		Event:                tr.event,
		Intervention:         "client-setting",
		Policy:               policy,
		Setting:              p.Setting,
		At:                   tr.at.Format(time.RFC3339),
		ApplyRequested:       true,
		ApplyAttempted:       tr.attempted,
		PriorValue:           prior,
		IntendedValue:        tr.intended,
		AppliedValue:         tr.applied,
		ActualValue:          actual,
		StateChange:          tr.stateChange,
		Reason:               tr.reason,
		SettingsPath:         tr.settingsPath,
		BackupPath:           tr.backupPath,
		BackupNote:           tr.backupNote,
		PredictedEffect:      p.PredictedShare,
		PredictedEffectBasis: "SIMULATOR",
		PredictedEffectText:  p.Evidence,
		RealizedEffect:       "UNAVAILABLE",
		Outcome:              "NOT_YET_MEASURED",
		QualityOutcome:       "UNAVAILABLE",
		Block:                "UNAVAILABLE",
		Surface:              "claude-code",
		Model:                "UNAVAILABLE",
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.Log), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(p.Log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(body, '\n'))
	return errors.Join(werr, f.Close())
}

// write applies the plan, or explains why it will not.
//
// The file belongs to the user and predates this tool, so: refuse on
// untrustworthy input, keep every key we did not set, back up what was there,
// and write owner-only.
func (p applyPlan) write(path string, out io.Writer, commit bool) error {
	if !p.Trustworthy {
		reason := p.Reason
		if reason == "" {
			reason = "the calibration for this corpus is not good enough to act on"
		}
		// A refusal under --yes is a request to apply that was not carried
		// out. On the machine this was built on it is the only truthful
		// record of the intervention, so it is written as what it is.
		if commit {
			if err := p.record(transition{event: "INTERVENTION_REFUSED", at: time.Now().UTC(), settingsPath: path,
				backupPath: "NONE", backupNote: "nothing was written", intended: "UNDECIDED", applied: "NONE",
				actual: p.Have, stateChange: "NOT_ATTEMPTED", reason: reason}); err != nil {
				return fmt.Errorf("refusing to change %s: %s (and the refusal was not recorded: %v)", p.Setting, reason, err)
			}
		}
		return fmt.Errorf("refusing to change %s: %s", p.Setting, reason)
	}
	if p.Want == p.Have {
		_, err := fmt.Fprintf(out, "%s is already %s. Nothing to change.\n", p.Setting, p.Want)
		return err
	}

	have := p.Have
	if have == "" {
		have = "(unset)"
	}
	if !commit {
		_, err := fmt.Fprintf(out, "would set %s: %s -> %s in %s\n  %s\nRe-run with --apply --yes to write it.\n",
			p.Setting, have, p.Want, path, p.Evidence)
		return err
	}

	settings := map[string]any{}
	// One instant names the backup and dates the record of the write.
	now := time.Now().UTC()
	backupPath, backupNote := "NONE", "no settings file existed before this write"
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		// A settings file that does not parse is a file to leave alone. The
		// user has something in there we do not understand, and guessing is
		// how a tool destroys a config it was asked to improve.
		if err := json.Unmarshal(existing, &settings); err != nil {
			return fmt.Errorf("%s is not valid JSON, so it will not be modified: %w", path, err)
		}
		backup := fmt.Sprintf("%s.bak-%s", path, now.Format("20060102T150405Z"))
		if err := os.WriteFile(backup, existing, 0o600); err != nil {
			return fmt.Errorf("write backup: %w", err)
		}
		if _, err := fmt.Fprintf(out, "backed up %s\n", filepath.Base(backup)); err != nil {
			return err
		}
		backupPath, backupNote = backup, ""
	case !os.IsNotExist(err):
		return err
	default:
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
	}

	settings[p.Setting] = p.Want
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return err
	}
	// INTERVENTION_APPLIED is earned by reading the value back, not by the
	// write returning nil. Something else may own the file, or the write
	// may not have landed as JSON the client reads.
	verify := p.verify
	if verify == nil {
		verify = readSettingValue
	}
	actual, readable := verify(path, p.Setting)
	tr := transition{event: "INTERVENTION_APPLIED", at: now, settingsPath: path, backupPath: backupPath,
		backupNote: backupNote, intended: p.Want, applied: p.Want, actual: actual, stateChange: "VERIFIED", attempted: true}
	switch {
	case !readable:
		tr.event, tr.stateChange, tr.actual = "APPLY_ATTEMPTED", "UNVERIFIED", "UNREADABLE"
	case actual != p.Want:
		tr.event, tr.stateChange = "APPLY_ATTEMPTED", "UNVERIFIED"
	}
	if err := p.record(tr); err != nil {
		return fmt.Errorf("the setting was written but its record was not: %w", err)
	}
	if tr.stateChange != "VERIFIED" {
		_, err = fmt.Fprintf(out, "wrote %s: %s -> %s in %s, but the change was not confirmed: the file now reads %s\n",
			p.Setting, have, p.Want, path, tr.actual)
		return err
	}
	_, err = fmt.Fprintf(out, "set %s: %s -> %s in %s\n", p.Setting, have, p.Want, path)
	return err
}

// ttlObservation is one session's cost under each TTL, in effective tokens.
type ttlObservation struct {
	Short float64 // ttl-5m
	Long  float64 // ttl-1h
}

// chooseTTL decides whether one TTL is worth switching to, weighted by cost.
//
// Sessions are the wrong unit. Most are short enough that no idle gap exceeds
// either TTL, so the two tie exactly, and counting those ties as votes let a
// few hundred trivial sessions outvote the handful that actually cost money.
// Summing effective tokens gives every session precisely the weight of its
// bill, which is the thing being optimised.
//
// The margin exists because the offline tier is an estimate with error bars
// wider than a percent. A difference this tool cannot distinguish from noise is
// not a reason to edit somebody's configuration.
func chooseTTL(obs []ttlObservation, have string) applyPlan {
	// With no wider corpus to compare against, what was scored is all there is.
	return chooseTTLWithCoverage(obs, have, 1)
}

// chooseTTLWithCoverage is chooseTTL plus the size of the bill it did not see.
//
// The engine refuses to score alternatives for a model whose behaviour has
// drifted, which is right, and which silently removes those sessions from this
// decision. On a real corpus that removed every one of the largest sessions,
// and the remainder recommended the opposite policy. So a recommendation is
// only offered when the sessions it could price account for most of the spend.
// coverage is the share of the corpus's as-run spend that could be scored at
// all, between 0 and 1.
func chooseTTLWithCoverage(obs []ttlObservation, have string, coverage float64) applyPlan {
	const minCoverage = 0.5
	const minMargin = 0.01

	plan := applyPlan{Setting: "promptCacheTtl", Have: have}
	if len(obs) == 0 {
		plan.Reason = "no session in this corpus reproduced well enough to act on"
		return plan
	}

	var short, long float64
	differing := 0
	for _, o := range obs {
		short += o.Short
		long += o.Long
		if o.Short != o.Long {
			differing++
		}
	}
	if differing == 0 {
		plan.Reason = fmt.Sprintf("5m and 1h cost the same across all %d sessions: no idle gap in this corpus outlives either", len(obs))
		return plan
	}

	want, winner, loser := "1h", long, short
	if short < long {
		want, winner, loser = "5m", short, long
	}
	if loser == 0 {
		plan.Reason = "no priced TTL policy in this corpus"
		return plan
	}
	margin := (loser - winner) / loser
	if margin < minMargin {
		plan.Reason = fmt.Sprintf("%s leads by only %.2f%% across %d sessions, which is inside the fit's own error bars", want, margin*100, len(obs))
		return plan
	}

	// The sessions that dominate a bill can prefer the opposite policy to the
	// many small ones, and a token-weighted total hides that rather than
	// resolving it. If the top decile by cost disagrees with the total, the
	// honest output is the disagreement.
	if big := heaviestDecile(obs); big != "" && big != want {
		plan.Reason = fmt.Sprintf("no single setting is right for this corpus: %s is cheaper in total, but %s is cheaper on the largest sessions, which is where the spend is. Set %s if your next sessions are long with idle gaps, %s if they are short and bursty",
			want, big, big, want)
		return plan
	}

	if coverage < minCoverage {
		plan.Reason = fmt.Sprintf("only %.0f%% of this corpus's effective tokens could be scored, so the sessions that cost the most are not represented; %s led on what was scored, which is not a finding",
			coverage*100, want)
		return plan
	}

	plan.Trustworthy = true
	plan.Want = want
	plan.PredictedShare = margin
	plan.Evidence = fmt.Sprintf("%s costs %.1f%% fewer effective tokens across %d sessions that reproduced at or above 95%%, covering %.0f%% of the corpus's scored spend (%d sessions differ between the two TTLs)",
		want, margin*100, len(obs), coverage*100, differing)
	return plan
}

// ttlInput is one lane's cost under each TTL, with the as-run spend it stands for.
type ttlInput struct {
	asRun float64
	obs   ttlObservation
}

// ttlInputOf is what a lane contributes to the TTL decision: nothing at
// all when it is not eligible, its as-run spend toward coverage when it
// is, and its observation only when both TTLs were priced.
//
// A sidechain is refused first: promptCacheTtl governs the main-thread
// query sources and subagentPromptCacheTtl the rest, so a sub-agent lane
// is outside what this plan can change, however well it reproduced.
func ttlInputOf(r *analysis.LaneReport) ttlInput {
	if !ttlEligible(r) {
		return ttlInput{}
	}
	var in ttlInput
	for _, pol := range r.Policies() {
		switch pol.Name {
		case "as-run":
			in.asRun = pol.EffectiveTokens
		case "ttl-5m0s":
			in.obs.Short = pol.EffectiveTokens
		case "ttl-1h0m0s":
			in.obs.Long = pol.EffectiveTokens
		}
	}
	return in
}

// ttlEligible is the population gate: a main-thread lane that reproduced
// at or above the match rate. The lane is consulted before the
// calibration, because a sub-agent is outside the setting's reach however
// well it reproduced.
func ttlEligible(r *analysis.LaneReport) bool {
	const minMatchRate = 0.95
	if r == nil || r.Lane == nil || r.Lane.Sidechain {
		return false
	}
	if r.Calibration == nil || r.Calibration.Compared() == 0 {
		return false
	}
	return float64(r.Calibration.Reproduced)/float64(r.Calibration.Compared()) >= minMatchRate
}

// ttlPlan turns scored reports into observations and asks chooseTTL.
//
// A session only counts when the engine reproduced it well enough to believe.
// Sessions it could not follow are excluded rather than averaged in, because a
// confident recommendation built on turns we could not reproduce is exactly the
// failure this tool exists to point at.
func ttlPlan(reports []*analysis.LaneReport, have string) applyPlan {
	inputs := make([]ttlInput, 0, len(reports))
	for _, r := range reports {
		inputs = append(inputs, ttlInputOf(r))
	}
	return ttlPlanFromInputs(inputs, have)
}

// ttlPlanFromInputs aggregates eligible lanes and asks chooseTTL.
//
// Coverage is a share of as-run spend: what could be scored over what was
// trusted. Leaving the unscoreable sessions out of the denominator is how a
// recommendation ends up describing only the cheap tail of a corpus.
func ttlPlanFromInputs(inputs []ttlInput, have string) applyPlan {
	var obs []ttlObservation
	var trustedTokens, scoredTokens float64
	for _, in := range inputs {
		trustedTokens += in.asRun
		if in.obs.Short > 0 && in.obs.Long > 0 {
			scoredTokens += in.asRun
			obs = append(obs, in.obs)
		}
	}
	coverage := 1.0
	if trustedTokens > 0 {
		coverage = scoredTokens / trustedTokens
	}
	return chooseTTLWithCoverage(obs, have, coverage)
}

// heaviestDecile reports which TTL wins across the costliest tenth of sessions,
// or "" when that tenth is too small to mean anything.
func heaviestDecile(obs []ttlObservation) string {
	if len(obs) < 10 {
		return ""
	}
	sorted := append([]ttlObservation(nil), obs...)
	// Stable, and with a tiebreak: this ordering decides the decile boundary,
	// which decides whether the tool edits somebody's settings file. An
	// unstable sort could shuffle ties between runs and flip that decision on
	// identical input.
	sort.SliceStable(sorted, func(i, j int) bool {
		mi, mj := math.Min(sorted[i].Short, sorted[i].Long), math.Min(sorted[j].Short, sorted[j].Long)
		if mi != mj {
			return mi > mj
		}
		return sorted[i].Short > sorted[j].Short
	})
	var short, long float64
	for _, o := range sorted[:len(sorted)/10] {
		short += o.Short
		long += o.Long
	}
	if short == long {
		return ""
	}
	if short < long {
		return "5m"
	}
	return "1h"
}
