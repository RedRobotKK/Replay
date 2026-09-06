package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
