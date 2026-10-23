package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/outcome"
)

// The contract these tests hold is docs/design/outcome-compare.md.

type comparisonReport struct {
	Schema            string   `json:"schema"`
	TaskID            string   `json:"task_id"`
	ManifestSHA256    string   `json:"manifest_sha256"`
	MinEligiblePerArm int      `json:"min_eligible_per_arm"`
	ControlArm        string   `json:"control_arm"`
	TreatmentArm      string   `json:"treatment_arm"`
	Status            string   `json:"status"`
	Note              string   `json:"note"`
	Claim             string   `json:"claim"`
	NotMeasured       []string `json:"not_measured"`
	Control           armCount `json:"control"`
	Treatment         armCount `json:"treatment"`
	Runs              []struct {
		RunDir    string   `json:"run_dir"`
		Arm       string   `json:"arm"`
		Verdict   string   `json:"verdict"`
		Reasons   []string `json:"reasons"`
		Role      string   `json:"role"`
		Exclusion string   `json:"exclusion"`
	} `json:"runs"`
}

type armCount struct {
	Pass        int `json:"pass"`
	Eligible    int `json:"eligible"`
	Unavailable int `json:"unavailable"`
}

// armRuns writes one run directory per reply, in arm, on the synthetic
// substrate. A reply of "3" PASSes there and anything else FAILs.
func armRuns(t *testing.T, m outcome.Manifest, raw []byte, tree, arm string, replies ...string) []string {
	t.Helper()
	var dirs []string
	for _, reply := range replies {
		f := runFields(m, raw, reply)
		f["arm"] = arm
		dirs = append(dirs, outcomeRunDir(t, f, tree))
	}
	return dirs
}

func compareJSON(t *testing.T, args ...string) comparisonReport {
	t.Helper()
	var out, errb bytes.Buffer
	if err := dispatch(append([]string{"outcome-compare", "--json"}, args...), &out, &errb); err != nil {
		t.Fatalf("outcome-compare: %v\n%s", err, errb.String())
	}
	var r comparisonReport
	dec := json.NewDecoder(strings.NewReader(out.String()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("outcome-compare --json: %v\n%s", err, out.String())
	}
	return r
}

func TestE2E_OutcomeCompare(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	control := armRuns(t, m, raw, tree, "control", "3", "3", "3", "2", "2")
	treatment := armRuns(t, m, raw, tree, "terse", "3", "3", "3", "3", "2")
	other := armRuns(t, m, raw, tree, "verbose", "3")
	missing := filepath.Join(t.TempDir(), "never-written")
	args := []string{"outcome-compare", "--control", "control", "--treatment", "terse"}
	args = append(append(append(append(args, control...), treatment...), other...), missing)

	var out, errb bytes.Buffer
	if err := dispatch(args, &out, &errb); err != nil {
		t.Fatalf("outcome-compare: %v\n%s", err, errb.String())
	}
	human := out.String()
	for _, want := range []string{
		"COMPARABLE", "control  3 of 5 PASS", "terse  4 of 5 PASS",
		"at least 5 eligible runs an arm", "no effect is estimated",
		"cost: NOT MEASURED", "time: NOT MEASURED", "tokens: NOT MEASURED",
		"arm_not_compared", "arm_unknown",
	} {
		if !strings.Contains(human, want) {
			t.Errorf("output lacks %q:\n%s", want, human)
		}
	}

	r := compareJSON(t, append(append(append(append([]string{"--control", "control", "--treatment", "terse"}, control...), treatment...), other...), missing)...)
	if r.Status != "COMPARABLE" || r.MinEligiblePerArm != 5 || r.Schema != "replay.outcome.comparison.v1" {
		t.Fatalf("status %s, minimum %d, schema %s", r.Status, r.MinEligiblePerArm, r.Schema)
	}
	if r.Control != (armCount{Pass: 3, Eligible: 5}) || r.Treatment != (armCount{Pass: 4, Eligible: 5}) {
		t.Errorf("counts control %+v treatment %+v", r.Control, r.Treatment)
	}
	if len(r.Runs) != 12 {
		t.Fatalf("%d runs listed, want all 12 supplied", len(r.Runs))
	}
	roles := map[string]int{}
	for _, run := range r.Runs {
		roles[run.Role+"/"+run.Exclusion]++
	}
	want := map[string]int{"control/": 5, "treatment/": 5, "excluded/arm_not_compared": 1, "excluded/arm_unknown": 1}
	for k, n := range want {
		if roles[k] != n {
			t.Errorf("%s: %d runs, want %d (all: %v)", k, roles[k], n, roles)
		}
	}
	for _, nm := range []string{"cost", "time", "tokens"} {
		if !containsString(r.NotMeasured, nm) {
			t.Errorf("not_measured %v lacks %s", r.NotMeasured, nm)
		}
	}
	if !strings.Contains(r.Claim, "not a statistical test") {
		t.Errorf("claim %q", r.Claim)
	}

	// Below the minimum nothing is compared, whatever the counts.
	short := compareJSON(t, append(append([]string{"--control", "control", "--treatment", "terse"}, control[:4]...), treatment...)...)
	if short.Status != "INSUFFICIENT" {
		t.Errorf("4 control runs: %s, want INSUFFICIENT", short.Status)
	}

	// A run of another arm is never counted in either arm.
	if r.Control.Eligible+r.Treatment.Eligible != 10 {
		t.Errorf("eligible %d + %d: a run of another arm was counted", r.Control.Eligible, r.Treatment.Eligible)
	}
}

func TestOutcomeCompareUnavailableNeverMakesUpTheMinimum(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	control := armRuns(t, m, raw, tree, "control", "3", "2", "3", "2")
	broken := runFields(m, raw, "3")
	delete(broken, "exit_code")
	for i := 0; i < 3; i++ {
		control = append(control, outcomeRunDir(t, broken, tree))
	}
	treatment := armRuns(t, m, raw, tree, "terse", "3", "3", "2", "3", "2")
	r := compareJSON(t, append(append([]string{"--control", "control", "--treatment", "terse"}, control...), treatment...)...)
	if r.Status != "INSUFFICIENT" || r.Control != (armCount{Pass: 2, Eligible: 4, Unavailable: 3}) {
		t.Fatalf("status %s control %+v: UNAVAILABLE runs were counted toward the minimum", r.Status, r.Control)
	}
	if !strings.Contains(r.Note, "5 eligible") {
		t.Errorf("note %q does not name the minimum", r.Note)
	}
	var out, errb bytes.Buffer
	args := append(append([]string{"outcome-compare", "--control", "control", "--treatment", "terse"}, control...), treatment...)
	if err := dispatch(args, &out, &errb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "control  2 of 4 PASS, 3 UNAVAILABLE not counted") || !strings.Contains(out.String(), "INSUFFICIENT") {
		t.Errorf("the human report does not keep UNAVAILABLE apart:\n%s", out.String())
	}
}

func TestOutcomeCompareRefusesACeiling(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	control := armRuns(t, m, raw, tree, "control", "2", "2", "2", "2", "2")
	treatment := armRuns(t, m, raw, tree, "terse", "3", "3", "2", "3", "2")
	r := compareJSON(t, append(append([]string{"--control", "control", "--treatment", "terse"}, control...), treatment...)...)
	if r.Status != "REFUSED_CEILING" || !strings.Contains(r.Note, "not evidence of no effect") {
		t.Fatalf("status %s note %q", r.Status, r.Note)
	}
}

func TestOutcomeCompareRefusesBadInvocations(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	run := armRuns(t, m, raw, tree, "control", "3")[0]
	for name, args := range map[string][]string{
		"no control":        {"--treatment", "terse", run},
		"no treatment":      {"--control", "control", run},
		"the same label":    {"--control", "control", "--treatment", "control", run},
		"no run directory":  {"--control", "control", "--treatment", "terse"},
		"a directory twice": {"--control", "control", "--treatment", "terse", run, run},
		"an empty label":    {"--control", "", "--treatment", "terse", run},
		"an unknown flag":   {"--control", "control", "--treatment", "terse", "--metric", "cost", run},
	} {
		t.Run(name, func(t *testing.T) {
			var out, errb bytes.Buffer
			err := dispatch(append([]string{"outcome-compare"}, args...), &out, &errb)
			if !errors.Is(err, errUsage) {
				t.Fatalf("%v: %v, want a usage refusal", args, err)
			}
			if out.Len() != 0 {
				t.Errorf("something was reported before the refusal:\n%s", out.String())
			}
		})
	}
}

func TestOutcomeCompareRefusesAnInvalidManifest(t *testing.T) {
	_, _, tree := withSyntheticManifest(t)
	prev := outcomeManifest
	outcomeManifest = func() (outcome.Manifest, []byte, error) {
		_, err := outcome.ParseManifest([]byte(`{"schema":"replay.outcome.manifest.v0"}`))
		return outcome.Manifest{}, nil, err
	}
	t.Cleanup(func() { outcomeManifest = prev })
	var out, errb bytes.Buffer
	err := dispatch([]string{"outcome-compare", "--control", "a", "--treatment", "b", tree}, &out, &errb)
	if err == nil || !strings.Contains(err.Error(), "manifest") || out.Len() != 0 {
		t.Fatalf("an invalid embedded manifest: %v, stdout %q", err, out.String())
	}
}

func TestOutcomeCompareWritesNothing(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	runs := armRuns(t, m, raw, tree, "control", "3", "2")
	before := []string{snapshotDir(t, runs[0]), snapshotDir(t, runs[1])}
	_ = compareJSON(t, "--control", "control", "--treatment", "terse", runs[0], runs[1])
	if snapshotDir(t, runs[0]) != before[0] || snapshotDir(t, runs[1]) != before[1] {
		t.Error("a run directory changed while being compared")
	}
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
