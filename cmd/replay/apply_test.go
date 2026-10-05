package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// `replay advise --apply` writes one setting: the prompt cache TTL. Everything
// else the advisor suggests is a change to how a person works, and a tool that
// edits your instruction files because it thinks they are too long would be a
// worse tool than one that tells you.
//
// The rules below exist because this writes to a config file the user did not
// open. It must never act on numbers it cannot stand behind, never lose a
// setting it did not put there, and never leave the file worse than it found it.

func writeSettings(t *testing.T, dir string, v map[string]any) string {
	t.Helper()
	p := filepath.Join(dir, "settings.json")
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readSettings(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("settings.json is no longer valid JSON: %v\n%s", err, b)
	}
	return m
}

func TestApplyRefusesWhenCalibrationIsUntrustworthy(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "5m"})
	plan := applyPlan{Setting: "promptCacheTtl", Want: "1h", Have: "5m", Trustworthy: false, Reason: "provider behaviour changed"}

	var out strings.Builder
	if err := plan.write(p, &out, false); err == nil {
		t.Fatal("must refuse to write when the calibration cannot be trusted")
	}
	if got := readSettings(t, p)["promptCacheTtl"]; got != "5m" {
		t.Fatalf("settings were changed despite the refusal: %v", got)
	}
}

func TestApplyIsANoOpWhenAlreadyOptimal(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "1h"})
	plan := applyPlan{Setting: "promptCacheTtl", Want: "1h", Have: "1h", Trustworthy: true}

	var out strings.Builder
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	if len(backupsIn(t, dir)) != 0 {
		t.Fatal("a no-op must not leave a backup file behind")
	}
	if !strings.Contains(out.String(), "already") {
		t.Fatalf("a no-op should say so: %q", out.String())
	}
}

func TestApplyPreservesEverySettingItDidNotSet(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{
		"promptCacheTtl": "5m",
		"env":            map[string]any{"FOO": "bar"},
		"permissions":    map[string]any{"allow": []any{"Bash(ls:*)"}},
	})
	plan := applyPlan{Setting: "promptCacheTtl", Want: "1h", Have: "5m", Trustworthy: true}

	var out strings.Builder
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	got := readSettings(t, p)
	if got["promptCacheTtl"] != "1h" {
		t.Fatalf("the setting was not applied: %v", got["promptCacheTtl"])
	}
	if _, ok := got["env"]; !ok {
		t.Fatal("an unrelated setting was dropped")
	}
	if _, ok := got["permissions"]; !ok {
		t.Fatal("permissions were dropped, which is the worst thing this could do")
	}
	// The previous file is recoverable.
	b := backupsIn(t, dir)
	if len(b) != 1 {
		t.Fatalf("want exactly one backup, got %v", b)
	}
	if !strings.Contains(string(mustRead(t, filepath.Join(dir, b[0]))), `"5m"`) {
		t.Fatal("the backup does not hold the previous value")
	}
}

func TestApplyDryRunChangesNothing(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "5m"})
	plan := applyPlan{Setting: "promptCacheTtl", Want: "1h", Have: "5m", Trustworthy: true}

	var out strings.Builder
	if err := plan.write(p, &out, false); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(t, p)["promptCacheTtl"]; got != "5m" {
		t.Fatalf("a dry run wrote to the file: %v", got)
	}
	if len(backupsIn(t, dir)) != 0 {
		t.Fatal("a dry run must not create a backup")
	}
	if !strings.Contains(out.String(), "would") {
		t.Fatalf("a dry run must say what it would do: %q", out.String())
	}
}

func TestApplyCreatesSettingsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	plan := applyPlan{Setting: "promptCacheTtl", Want: "1h", Have: "", Trustworthy: true}

	var out strings.Builder
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(t, p)["promptCacheTtl"]; got != "1h" {
		t.Fatalf("want 1h, got %v", got)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no POSIX permission model: Go's Chmod there toggles only the
	// read-only bit, so a file created 0600 reports 0666 and this cannot hold.
	// Skipped rather than weakened, because the assertion is the security
	// property on the platforms that have one, and pretending Windows enforces
	// it would be worse than saying it does not.
	if runtime.GOOS == "windows" {
		t.Skip("file permissions are not POSIX on this platform; see docs/SURFACES.md")
	}
	if runtime.GOOS == "windows" {
		// No Unix mode bits here: Go synthesises 0666 for any writable
		// file, so this would assert against a value the platform never
		// set. Skipped rather than loosened to something that passes
		// everywhere and checks nothing.
		t.Skip("file permissions are not mode bits on this platform")
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("settings must be owner-only, got %v", info.Mode().Perm())
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func backupsIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak") {
			out = append(out, e.Name())
		}
	}
	return out
}

// Counting sessions is the wrong unit.
//
// Most sessions are short enough that 5m and 1h cost exactly the same: no idle
// gap ever exceeds either TTL, so the two policies tie. Counting a tie as a win
// for whichever policy is listed first, and then counting sessions rather than
// tokens, produced a confident recommendation to set 5m across a corpus whose
// only expensive sessions measurably preferred 1h. The tool would have told a
// user to make their bill worse, in the one place it edits their config.
func TestChooseTTLWeighsTokensNotSessions(t *testing.T) {
	var obs []ttlObservation
	for i := 0; i < 500; i++ {
		obs = append(obs, ttlObservation{Short: 1000, Long: 1000})
	}
	// One session that actually costs money, where 1h is plainly cheaper.
	obs = append(obs, ttlObservation{Short: 300_000_000, Long: 200_000_000})

	plan := chooseTTL(obs, "")
	if !plan.Trustworthy {
		t.Fatalf("expected a recommendation, got refusal: %s", plan.Reason)
	}
	if plan.Want != "1h" {
		t.Fatalf("want 1h, the policy that is cheaper where the money is; got %q (%s)", plan.Want, plan.Evidence)
	}
}

// When the two policies genuinely cost the same, there is nothing to recommend
// and the tool must not edit a config to express a coin flip.
func TestChooseTTLRefusesWhenThePoliciesTie(t *testing.T) {
	var obs []ttlObservation
	for i := 0; i < 50; i++ {
		obs = append(obs, ttlObservation{Short: 10_000, Long: 10_000})
	}
	if plan := chooseTTL(obs, ""); plan.Trustworthy {
		t.Fatalf("a tie is not a recommendation: %s", plan.Evidence)
	}
}

// A margin too small to survive the fit's own error bars is not a finding.
func TestChooseTTLRefusesAMarginInsideTheNoise(t *testing.T) {
	obs := []ttlObservation{{Short: 1_000_000, Long: 999_000}} // 0.1% apart
	if plan := chooseTTL(obs, ""); plan.Trustworthy {
		t.Fatalf("0.1%% is not a reason to edit a config: %s", plan.Evidence)
	}
}

func TestChooseTTLRefusesWithNoTrustedSessions(t *testing.T) {
	if plan := chooseTTL(nil, ""); plan.Trustworthy {
		t.Fatal("no data must never produce a recommendation")
	}
}

// A recommendation must disclose how much of the bill it actually looked at.
//
// The engine declines to score alternatives for a model whose behaviour has
// drifted. That is correct, and it has a consequence nobody would guess: the
// sessions it drops can be the expensive ones. On a real corpus every one of
// the six largest sessions preferred 1h, while the scoreable remainder
// preferred 5m, and an aggregate over "sessions it could score" cheerfully
// recommended 5m. The arithmetic was right and the answer was worthless.
func TestChooseTTLRefusesWhenItPricedOnlyAThinSliceOfTheSpend(t *testing.T) {
	obs := []ttlObservation{{Short: 1_000_000, Long: 1_200_000}}
	// The corpus is far larger than what could be priced.
	plan := chooseTTLWithCoverage(obs, "", 0.04) // 4% of spend scored
	if plan.Trustworthy {
		t.Fatalf("must refuse when most of the spend was never scored: %s", plan.Evidence)
	}
	if !strings.Contains(plan.Reason, "%") {
		t.Fatalf("the refusal must say how thin the coverage was: %q", plan.Reason)
	}
}

func TestChooseTTLProceedsWhenCoverageIsBroad(t *testing.T) {
	obs := []ttlObservation{{Short: 1_000_000, Long: 1_200_000}}
	plan := chooseTTLWithCoverage(obs, "", 0.92) // 92% of spend scored
	if !plan.Trustworthy {
		t.Fatalf("broad coverage should still recommend: %s", plan.Reason)
	}
	if !strings.Contains(plan.Evidence, "%") {
		t.Fatalf("the evidence should quantify coverage: %q", plan.Evidence)
	}
}

// When the corpus total and the expensive sessions disagree, there is no single
// right setting and the tool must not pretend otherwise.
//
// This is what a real corpus looked like: 744 mid-sized sessions preferred 5m
// by a little, six very large ones preferred 1h by a lot, and the token-weighted
// total said 5m. Applying 5m would have made the only sessions that cost real
// money 8% to 40% worse. The setting is not the answer; the shape of the work
// is, and only the person can say which shape they are about to have.
func TestChooseTTLRefusesWhenTheLargestSessionsDisagree(t *testing.T) {
	var obs []ttlObservation
	for i := 0; i < 744; i++ {
		obs = append(obs, ttlObservation{Short: 8_000_000, Long: 12_000_000}) // 5m cheaper on the many
	}
	for i := 0; i < 6; i++ {
		obs = append(obs, ttlObservation{Short: 500_000_000, Long: 300_000_000}) // 1h much cheaper on the few
	}
	plan := chooseTTLWithCoverage(obs, "", 1)
	if plan.Trustworthy {
		t.Fatalf("must not pick a side when the expensive sessions disagree: %s", plan.Evidence)
	}
	for _, want := range []string{"5m", "1h", "largest"} {
		if !strings.Contains(plan.Reason, want) {
			t.Fatalf("the refusal must name both policies and why; missing %q in %q", want, plan.Reason)
		}
	}
}

// The hazard this used to pin: hoisting moved a flag ahead of the paths and
// left its value behind to be read as one. `replay rules --update <file>
// --dry-run` consumed "--dry-run" as the filename. hoistFlagsFor asks the
// FlagSet which flags take a value, so the pair travels together.
func TestHoistingKeepsAStringFlagWithItsValue(t *testing.T) {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	update := fs.String("update", "", "")
	dry := fs.Bool("dry-run", false, "")
	if err := fs.Parse(hoistFlagsFor(fs, []string{"--update", "rules.json", "--dry-run"})); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if *update != "rules.json" {
		t.Fatalf("--update took %q, the old defect took the next flag", *update)
	}
	if !*dry {
		t.Fatal("--dry-run was swallowed as a value")
	}
	// And the value-blind helper must stay gone rather than linger as a
	// loaded gun for the next command that takes a flag with a value.
	if src := string(mustRead(t, "main.go")); strings.Contains(src, "func hoistFlags(") {
		t.Fatal("the value-blind hoistFlags is back; every value-taking flag after a path breaks again")
	}
}

// readInterventions parses the provenance log, one JSON object per line.
func readInterventions(t *testing.T, p string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("no intervention record: %v", err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("record is not JSON: %v\n%s", err, line)
		}
		out = append(out, m)
	}
	return out
}

// The one intervention Replay performs itself leaves a record.
//
// `advise --apply --yes` writes promptCacheTtl into the user's settings and
// records nothing: not that it did it, not when, not what it predicted. The
// control-thesis panel of 2026-10-05 made the record the first condition of
// any measurement, and the register written before this test says what it
// must hold. Every field below is either the fact as it is, or the word
// UNAVAILABLE.
//
// PASS: one record, the prior and applied values as they were, the instant
// of the write, the backup that was made, the predicted effect as a number
// with SIMULATOR as its basis, and no realized figure of any kind.
// FAIL: no record, which is what shipped.
func TestApplyRecordsTheInterventionItPerformed(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "1h", "env": map[string]any{"FOO": "bar"}})
	log := filepath.Join(dir, "interventions.jsonl")
	plan := applyPlan{Setting: "promptCacheTtl", Want: "5m", Have: "1h", Trustworthy: true,
		PredictedShare: 0.28, Evidence: "5m costs 28.0% fewer effective tokens across 755 sessions", Log: log}
	before := time.Now().UTC().Add(-time.Second)
	var out strings.Builder
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	recs := readInterventions(t, log)
	if len(recs) != 1 {
		t.Fatalf("want exactly one record, got %d", len(recs))
	}
	r := recs[0]
	for k, want := range map[string]any{
		"schema":                 "replay.intervention.v1",
		"event":                  "INTERVENTION_APPLIED",
		"intervention":           "client-setting",
		"policy":                 "promptCacheTtl=5m",
		"setting":                "promptCacheTtl",
		"apply_requested":        true,
		"apply_attempted":        true,
		"prior_value":            "1h",
		"intended_value":         "5m",
		"applied_value":          "5m",
		"actual_value":           "5m",
		"state_change":           "VERIFIED",
		"settings_path":          p,
		"predicted_effect":       0.28,
		"predicted_effect_basis": "SIMULATOR",
		"predicted_effect_text":  plan.Evidence,
		"realized_effect":        "UNAVAILABLE",
		"outcome":                "NOT_YET_MEASURED",
		"quality_outcome":        "UNAVAILABLE",
		"block":                  "UNAVAILABLE",
		"surface":                "claude-code",
		"model":                  "UNAVAILABLE",
	} {
		if r[k] != want {
			t.Errorf("%s = %v (%T), want %v", k, r[k], r[k], want)
		}
	}
	at, err := time.Parse(time.RFC3339, fmt.Sprint(r["at"]))
	if err != nil {
		t.Fatalf("at is not an RFC3339 instant: %v", r["at"])
	}
	if at.Before(before) || at.After(time.Now().UTC().Add(time.Second)) {
		t.Errorf("at = %v is not the instant of this write", at)
	}
	b := backupsIn(t, dir)
	if len(b) != 1 || r["backup_path"] != filepath.Join(dir, b[0]) {
		t.Errorf("backup_path = %v, want the backup that was made: %v", r["backup_path"], b)
	}
	if !strings.Contains(string(mustRead(t, filepath.Join(dir, b[0]))), `"1h"`) {
		t.Error("the backup the record points at does not hold the prior value")
	}
}

// A dry run and a no-op record nothing: nothing was requested, or nothing
// needed to change.
func TestApplyRecordsNothingWhenItChangesNothing(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "1h"})
	log := filepath.Join(dir, "interventions.jsonl")
	var out strings.Builder
	dry := applyPlan{Setting: "promptCacheTtl", Want: "5m", Have: "1h", Trustworthy: true, Log: log}
	if err := dry.write(p, &out, false); err != nil {
		t.Fatal(err)
	}
	noop := applyPlan{Setting: "promptCacheTtl", Want: "1h", Have: "1h", Trustworthy: true, Log: log}
	if err := noop.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("a record was written although nothing changed: %v", err)
	}
}

// A refusal under --yes is recorded as a refusal: apply was requested, not
// attempted, the state did not change, and the reason is the predictor's.
//
// On the machine this was built on, `advise --apply --yes` refuses ("no
// single setting is right for this corpus"), so the only truthful record
// of the intervention here is this one. It is not INTERVENTION_APPLIED.
func TestApplyRecordsARefusalWhenApplyWasRequested(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "1h"})
	log := filepath.Join(dir, "interventions.jsonl")
	var out strings.Builder
	refused := applyPlan{Setting: "promptCacheTtl", Have: "1h", Reason: "no single setting is right for this corpus", Log: log}
	if err := refused.write(p, &out, true); err == nil {
		t.Fatal("an untrustworthy plan must refuse")
	}
	recs := readInterventions(t, log)
	if len(recs) != 1 {
		t.Fatalf("want one refusal record, got %d", len(recs))
	}
	r := recs[0]
	for k, want := range map[string]any{
		"event":           "INTERVENTION_REFUSED",
		"apply_requested": true,
		"apply_attempted": false,
		"prior_value":     "1h",
		"intended_value":  "UNDECIDED",
		"applied_value":   "NONE",
		"actual_value":    "1h",
		"state_change":    "NOT_ATTEMPTED",
		"reason":          "no single setting is right for this corpus",
		"realized_effect": "UNAVAILABLE",
		"outcome":         "NOT_YET_MEASURED",
	} {
		if r[k] != want {
			t.Errorf("%s = %v, want %v", k, r[k], want)
		}
	}
	if got := readSettings(t, p)["promptCacheTtl"]; got != "1h" {
		t.Errorf("a refusal must not touch the file: %v", got)
	}
	// A dry run of a refused plan is not a request to apply.
	dry := applyPlan{Setting: "promptCacheTtl", Have: "1h", Reason: "x", Log: filepath.Join(dir, "other.jsonl")}
	_ = dry.write(p, &out, false)
	if _, err := os.Stat(filepath.Join(dir, "other.jsonl")); !os.IsNotExist(err) {
		t.Error("a dry run records nothing, refused or not")
	}
}

// INTERVENTION_APPLIED is earned by reading the value back from disk, not by
// the write call returning nil. When the file does not hold the intended
// value afterwards, the record says APPLY_ATTEMPTED and the state change
// UNVERIFIED, and the actual value is what was read.
func TestApplyDoesNotClaimAppliedWithoutReadingTheValueBack(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "1h"})
	log := filepath.Join(dir, "interventions.jsonl")
	plan := applyPlan{Setting: "promptCacheTtl", Want: "5m", Have: "1h", Trustworthy: true, Log: log,
		// Something else owns the file: the value read back is not the one written.
		verify: func(_, _ string) (string, bool) { return "1h", true }}
	var out strings.Builder
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	r := readInterventions(t, log)[0]
	for k, want := range map[string]any{
		"event":           "APPLY_ATTEMPTED",
		"apply_attempted": true,
		"applied_value":   "5m",
		"actual_value":    "1h",
		"state_change":    "UNVERIFIED",
	} {
		if r[k] != want {
			t.Errorf("%s = %v, want %v", k, r[k], want)
		}
	}
	if !strings.Contains(out.String(), "not confirmed") {
		t.Errorf("the reader must be told the change was not confirmed: %q", out.String())
	}

	// An unreadable file after the write is UNVERIFIED too, with the value UNREADABLE.
	plan.verify = func(_, _ string) (string, bool) { return "", false }
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	r = readInterventions(t, log)[1]
	if r["state_change"] != "UNVERIFIED" || r["actual_value"] != "UNREADABLE" {
		t.Errorf("unreadable after write: %v / %v", r["state_change"], r["actual_value"])
	}
}

// readSettingValue is the independent check: it opens the file again and
// reports what it holds, or that it could not be read.
func TestReadSettingValueReportsWhatTheFileHolds(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "5m"})
	if v, ok := readSettingValue(p, "promptCacheTtl"); !ok || v != "5m" {
		t.Errorf("got %q %v, want 5m true", v, ok)
	}
	if v, ok := readSettingValue(p, "absent"); !ok || v != "UNSET" {
		t.Errorf("an absent key reads as UNSET, got %q %v", v, ok)
	}
	if _, ok := readSettingValue(filepath.Join(dir, "missing.json"), "promptCacheTtl"); ok {
		t.Error("a missing file is not readable")
	}
}

// An absent prior value is recorded as UNSET, and an absent backup as none,
// never as an empty string that reads as a value.
func TestApplyRecordsAnAbsentPriorValueAndBackupExplicitly(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nested", "settings.json")
	log := filepath.Join(dir, "interventions.jsonl")
	plan := applyPlan{Setting: "promptCacheTtl", Want: "5m", Have: "", Trustworthy: true, PredictedShare: 0.28, Log: log}
	var out strings.Builder
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	r := readInterventions(t, log)[0]
	if r["prior_value"] != "UNSET" {
		t.Errorf("prior_value = %v, want UNSET", r["prior_value"])
	}
	if r["event"] != "INTERVENTION_APPLIED" || r["state_change"] != "VERIFIED" || r["actual_value"] != "5m" {
		t.Errorf("a write into a fresh file is verified by reading it back: %v / %v / %v", r["event"], r["state_change"], r["actual_value"])
	}
	if r["backup_path"] != "NONE" || r["backup_note"] != "no settings file existed before this write" {
		t.Errorf("an absent backup must be named as absent: %v / %v", r["backup_path"], r["backup_note"])
	}
}

// The record never manufactures a realized result. Prediction is a number
// with a stated basis; realization is the word UNAVAILABLE until a
// measurement exists, and no field in the record carries a saving.
func TestApplyRecordDoesNotManufactureARealizedResult(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{})
	log := filepath.Join(dir, "interventions.jsonl")
	plan := applyPlan{Setting: "promptCacheTtl", Want: "5m", Have: "", Trustworthy: true, PredictedShare: 0.28, Log: log}
	var out strings.Builder
	if err := plan.write(p, &out, true); err != nil {
		t.Fatal(err)
	}
	r := readInterventions(t, log)[0]
	if _, isNumber := r["predicted_effect"].(float64); !isNumber {
		t.Errorf("predicted_effect must be the simulator's number, got %v (%T)", r["predicted_effect"], r["predicted_effect"])
	}
	if _, isNumber := r["realized_effect"].(float64); isNumber {
		t.Fatalf("realized_effect is a number before any measurement: %v", r["realized_effect"])
	}
	for k := range r {
		if strings.Contains(k, "saving") || strings.Contains(k, "measured") {
			t.Errorf("record carries a field that reads as a result: %s", k)
		}
	}
}

// chooseTTL hands its margin to the record as the prediction, so the number
// the write was justified by is the number that gets tested.
func TestChooseTTLCarriesItsMarginAsThePrediction(t *testing.T) {
	obs := make([]ttlObservation, 12)
	for i := range obs {
		obs[i] = ttlObservation{Short: 70, Long: 100}
	}
	plan := chooseTTLWithCoverage(obs, "", 1)
	if !plan.Trustworthy || plan.Want != "5m" {
		t.Fatalf("setup: 5m wins by 30%%, got %+v", plan)
	}
	if plan.PredictedShare != 0.3 {
		t.Errorf("PredictedShare = %v, want the 0.30 margin the evidence line states", plan.PredictedShare)
	}
}

// The log lives with Replay's other stores, and the store registry names it
// as provenance that a retention window does not remove.
func TestInterventionLogIsARegisteredStore(t *testing.T) {
	home := t.TempDir()
	if got, want := interventionLogPath(home), filepath.Join(home, ".replay", "interventions.jsonl"); got != want {
		t.Errorf("interventionLogPath = %s, want %s", got, want)
	}
	for _, s := range homeStores() {
		if s.Name == "interventions.jsonl" {
			if s.Purgeable {
				t.Error("provenance of a change Replay made is not subject to a retention window")
			}
			return
		}
	}
	t.Error("interventions.jsonl is not a registered store, so replay purge and the privacy page do not know it exists")
}

// The production path points the plan's record at this home's log, and
// reads the value the settings file holds today as the prior value.
func TestSettingsPlanRecordsUnderThisHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeSettings(t, filepath.Join(home, ".claude"), map[string]any{"promptCacheTtl": "1h"})
	settings, plan := settingsPlan(nil, home)
	if settings != filepath.Join(home, ".claude", "settings.json") {
		t.Errorf("settings path = %s", settings)
	}
	if plan.Log != interventionLogPath(home) {
		t.Errorf("the production plan must record under this home: Log = %q", plan.Log)
	}
	if plan.Have != "1h" {
		t.Errorf("the prior value is what the file holds today, got %q", plan.Have)
	}
}

// The margin is a defined quantity, not decoration.
//
// Over the sessions scored, chooseTTLWithCoverage sums each TTL's simulated
// effective tokens. The winner is the smaller sum. margin = (loser - winner)
// / loser: the share of the losing TTL's total that the winning TTL avoids.
// It is never negative, because the winner is defined as the smaller sum; a
// tie is refused before any margin exists; and the sign of a preference is
// carried by Want, not by the number. Checked here on figures a person can
// add up by hand: 70 against 100 on every one of twelve sessions.
func TestChooseTTLMarginIsTheShareOfTheLoserAvoided(t *testing.T) {
	obs := make([]ttlObservation, 12)
	for i := range obs {
		obs[i] = ttlObservation{Short: 70, Long: 100}
	}
	plan := chooseTTLWithCoverage(obs, "", 1)
	if plan.Want != "5m" || !plan.Trustworthy {
		t.Fatalf("5m sums to 840 against 1200, so 5m wins: %+v", plan)
	}
	if plan.PredictedShare != 0.3 {
		t.Errorf("margin = (1200-840)/1200 = 0.30, got %v", plan.PredictedShare)
	}
	// The mirror image prefers 1h with the same margin: the number is the
	// size of the lead, the TTL is the direction.
	for i := range obs {
		obs[i] = ttlObservation{Short: 100, Long: 70}
	}
	plan = chooseTTLWithCoverage(obs, "", 1)
	if plan.Want != "1h" || plan.PredictedShare != 0.3 {
		t.Errorf("mirror: want 1h at 0.30, got %s at %v", plan.Want, plan.PredictedShare)
	}
	// Token-weighted, not session-weighted: one session of 1000 against 700
	// outweighs eleven sessions that prefer the other way by 1.
	obs = make([]ttlObservation, 12)
	for i := range obs {
		obs[i] = ttlObservation{Short: 101, Long: 100}
	}
	obs[0] = ttlObservation{Short: 700, Long: 1000}
	plan = chooseTTLWithCoverage(obs, "", 1)
	// short = 700 + 11*101 = 1811; long = 1000 + 11*100 = 2100; margin = 289/2100.
	if plan.Want != "5m" || math.Abs(plan.PredictedShare-289.0/2100.0) > 1e-9 {
		t.Errorf("token-weighted: want 5m at %.4f, got %s at %v", 289.0/2100.0, plan.Want, plan.PredictedShare)
	}
	// A tie is refused before any margin exists, and carries no prediction.
	for i := range obs {
		obs[i] = ttlObservation{Short: 100, Long: 100}
	}
	plan = chooseTTLWithCoverage(obs, "", 1)
	if plan.Trustworthy || plan.PredictedShare != 0 {
		t.Errorf("a tie has no margin: %+v", plan)
	}
}

// A sub-agent lane is not an input to the TTL plan, whatever it reproduced.
//
// `promptCacheTtl` governs the main-thread query sources only; sub-agents
// are governed by `subagentPromptCacheTtl`. A report whose lane is a
// sidechain is refused before any aggregation, ahead of the calibration
// gate, so a well-calibrated sub-agent can never move the decision.
func TestTTLInputRefusesASubAgentLaneBeforeAggregation(t *testing.T) {
	side := &analysis.LaneReport{Lane: &transcript.Lane{ID: "sub", Sidechain: true}}
	if ttlEligible(side) {
		t.Error("a sidechain lane is outside the setting's control and must not be an input")
	}
	if ttlEligible(nil) {
		t.Error("nil is not an input")
	}
	if in := ttlInputOf(side); in != (ttlInput{}) {
		t.Errorf("a lane ttlEligible refuses contributes nothing, got %+v", in)
	}
	uncalibrated := &analysis.LaneReport{Lane: &transcript.Lane{ID: "main"}}
	if ttlEligible(uncalibrated) {
		t.Error("a report with no calibration is not an input")
	}
	poor := &analysis.LaneReport{Lane: &transcript.Lane{ID: "main"}, Calibration: &analysis.Calibration{Reproduced: 18, Broken: 2}}
	if ttlEligible(poor) {
		t.Error("90% reproduced is under the 95% gate")
	}
	main := &analysis.LaneReport{Lane: &transcript.Lane{ID: "main"}, Calibration: &analysis.Calibration{Reproduced: 19, Broken: 1}}
	if !ttlEligible(main) {
		t.Error("a main-thread lane at 95% reproduced is an input, even before its policies are priced")
	}
	// The same calibration on a sidechain is still refused: the lane, not
	// the calibration, decides eligibility.
	main.Lane.Sidechain = true
	if ttlEligible(main) {
		t.Error("eligibility is decided by the lane before the calibration is consulted")
	}
}

// The decile veto behaves exactly as specified: when the costliest tenth
// of sessions prefers the other TTL from the token-weighted total, the
// plan refuses and says so, even though the total clears the margin.
func TestChooseTTLDecileVetoRefusesASplitVerdict(t *testing.T) {
	obs := make([]ttlObservation, 12)
	for i := range obs {
		obs[i] = ttlObservation{Short: 80, Long: 100}
	}
	// The costliest session prefers 1h; it alone is the top tenth of twelve.
	obs[0] = ttlObservation{Short: 2000, Long: 1950}
	// Totals: short 2880, long 3050: 5m leads by 5.6%, above the 1% margin.
	plan := chooseTTLWithCoverage(obs, "", 1)
	if plan.Trustworthy {
		t.Fatalf("the top decile prefers 1h while the total prefers 5m; the plan must refuse, got %+v", plan)
	}
	if !strings.Contains(plan.Reason, "no single setting is right") || !strings.Contains(plan.Reason, "1h is cheaper on the largest sessions") {
		t.Errorf("the refusal must name the split: %q", plan.Reason)
	}
	// Under ten sessions the decile is too small to veto, and the total decides.
	plan = chooseTTLWithCoverage(obs[:9], "", 1)
	if !plan.Trustworthy || plan.Want != "5m" {
		t.Errorf("with nine sessions there is no decile to consult: %+v", plan)
	}
}

// Reports that are not inputs leave the plan with nothing to decide on.
func TestTTLPlanIgnoresReportsThatAreNotInputs(t *testing.T) {
	reports := []*analysis.LaneReport{nil, {Lane: &transcript.Lane{ID: "sub", Sidechain: true}}}
	plan := ttlPlan(reports, "")
	if plan.Trustworthy || !strings.Contains(plan.Reason, "no session in this corpus reproduced well enough") {
		t.Errorf("nil and sidechain reports are not inputs: %+v", plan)
	}
}

// An eligible lane whose TTLs were not priced counts toward the spend the
// plan is answerable for, and not toward what it scored: coverage falls,
// and a plan over a thin slice of the spend refuses.
func TestTTLPlanFromInputsCountsUnpricedSpendAgainstCoverage(t *testing.T) {
	var inputs []ttlInput
	for i := 0; i < 11; i++ {
		inputs = append(inputs, ttlInput{asRun: 100, obs: ttlObservation{Short: 70, Long: 100}})
	}
	plan := ttlPlanFromInputs(inputs, "")
	if !plan.Trustworthy || plan.Want != "5m" || plan.PredictedShare != 0.3 {
		t.Fatalf("eleven priced lanes decide: %+v", plan)
	}
	// One eligible lane worth more than all of them together, unpriced.
	inputs = append(inputs, ttlInput{asRun: 5000})
	plan = ttlPlanFromInputs(inputs, "")
	if plan.Trustworthy || !strings.Contains(plan.Reason, "only 18%") {
		t.Errorf("1100 of 6100 scored is 18%% coverage and must refuse: %+v", plan)
	}
}

// The --json document says "applied" only for a change read back from the
// file, and carries the transition as it ended.
//
// The guide offers replay.apply.v1 "for an agent to act on". It said
// "applied": true from the --yes flag, before the write ran and whatever the
// read-back found: one boolean carrying intent, attempt and verification at
// once. The record on disk distinguishes them; the document an agent reads
// must say the same thing.
//
// PASS: applied is true only with state_change VERIFIED; an unconfirmed
// write says applied false, APPLY_ATTEMPTED, UNVERIFIED, and the value the
// file holds; a dry run says applied false and that nothing was requested.
// FAIL: applied true on an unconfirmed write, which is what shipped.
func TestApplyDocumentSaysAppliedOnlyForAVerifiedChange(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "1h"})
	log := filepath.Join(dir, "interventions.jsonl")
	plan := applyPlan{Setting: "promptCacheTtl", Want: "5m", Have: "1h", Trustworthy: true, Log: log,
		verify: func(_, _ string) (string, bool) { return "1h", true }}
	doc, err := applyDocument(plan, p, true)
	if err != nil {
		t.Fatal(err)
	}
	entry := doc["applicable"].([]any)[0].(map[string]any)
	for k, want := range map[string]any{"applied": false, "event": "APPLY_ATTEMPTED", "state_change": "UNVERIFIED", "actual": "1h"} {
		if entry[k] != want {
			t.Errorf("unconfirmed write: %s = %v, want %v", k, entry[k], want)
		}
	}

	plan.verify = nil
	doc, err = applyDocument(plan, p, true)
	if err != nil {
		t.Fatal(err)
	}
	entry = doc["applicable"].([]any)[0].(map[string]any)
	for k, want := range map[string]any{"applied": true, "event": "INTERVENTION_APPLIED", "state_change": "VERIFIED", "actual": "5m"} {
		if entry[k] != want {
			t.Errorf("verified write: %s = %v, want %v", k, entry[k], want)
		}
	}

	doc, err = applyDocument(plan, p, false)
	if err != nil {
		t.Fatal(err)
	}
	entry = doc["applicable"].([]any)[0].(map[string]any)
	if entry["applied"] != false || entry["state_change"] != "NOT_REQUESTED" {
		t.Errorf("dry run: applied = %v, state_change = %v", entry["applied"], entry["state_change"])
	}
	if len(readInterventions(t, log)) != 2 {
		t.Error("two writes were requested, so two records exist; the dry run wrote none")
	}
}

// The document's other two endings: a refusal is reported as one with its
// reason and no applicable entry, and a write that fails is an error, not a
// document that says anything about the setting.
func TestApplyDocumentReportsRefusalsAndWriteFailures(t *testing.T) {
	dir := t.TempDir()
	p := writeSettings(t, dir, map[string]any{"promptCacheTtl": "1h"})
	refused := applyPlan{Setting: "promptCacheTtl", Have: "1h", Reason: "no single setting is right for this corpus"}
	doc, err := applyDocument(refused, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc["refused"].(map[string]any)["reason"]; got != refused.Reason {
		t.Errorf("refused reason = %v", got)
	}
	if entries := doc["applicable"].([]any); len(entries) != 0 {
		t.Errorf("a refused plan has no applicable entry: %v", entries)
	}

	// The record cannot be written: the log path sits under a file.
	unwritable := filepath.Join(p, "interventions.jsonl")
	plan := applyPlan{Setting: "promptCacheTtl", Want: "5m", Have: "1h", Trustworthy: true, Log: unwritable}
	if _, err := applyDocument(plan, p, true); err == nil || !strings.Contains(err.Error(), "its record was not") {
		t.Errorf("a write whose record failed is an error, got %v", err)
	}
	// The same for a refusal under --yes whose record cannot be written.
	refused.Log = unwritable
	var out strings.Builder
	if err := refused.write(p, &out, true); err == nil || !strings.Contains(err.Error(), "refusal was not recorded") {
		t.Errorf("a refusal whose record failed says so, got %v", err)
	}
}
