package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/outcome"
)

// `replay outcome-compare` compares two arms of scored run directories by
// count (docs/design/outcome-compare.md, requirement OC-3).
//
// Every run is scored by the unchanged scorer, listed, and placed in exactly
// one of control, treatment or excluded, with the reason it was excluded.
// outcome.Compare then decides whether the arms may be compared at all: each
// needs outcome.MinEligiblePerArm PASS or FAIL runs, and the control may not
// sit at a ceiling. It estimates no effect, and cost, time and tokens are not
// compared.

const outcomeComparisonSchema = "replay.outcome.comparison.v1"

const outcomeComparisonClaim = "A description of the runs supplied, by count. It is not a statistical test, " +
	"it does not say a difference would hold on other runs, tasks or machines, and it is not evidence of any " +
	"saving in cost, time or tokens."

// outcomeNotMeasured is what a comparison never compares. run.json carries
// usage, but it is operator-attested, so tokens are listed here too.
var outcomeNotMeasured = []string{"cost", "time", "tokens"}

type comparisonRun struct {
	RunDir    string   `json:"run_dir"`
	Arm       string   `json:"arm"`
	Verdict   string   `json:"verdict"`
	Reasons   []string `json:"reasons"`
	Role      string   `json:"role"`
	Exclusion string   `json:"exclusion"`
}

type comparisonArm struct {
	Pass        int `json:"pass"`
	Eligible    int `json:"eligible"`
	Unavailable int `json:"unavailable"`
}

type comparisonOutput struct {
	Schema            string          `json:"schema"`
	TaskID            string          `json:"task_id"`
	ManifestSHA256    string          `json:"manifest_sha256"`
	MinEligiblePerArm int             `json:"min_eligible_per_arm"`
	ControlArm        string          `json:"control_arm"`
	TreatmentArm      string          `json:"treatment_arm"`
	Status            string          `json:"status"`
	Note              string          `json:"note"`
	Claim             string          `json:"claim"`
	NotMeasured       []string        `json:"not_measured"`
	Control           comparisonArm   `json:"control"`
	Treatment         comparisonArm   `json:"treatment"`
	Runs              []comparisonRun `json:"runs"`
}

func runOutcomeCompare(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("outcome-compare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	control := fs.String("control", "", "the arm label of the baseline runs, as their run.json names it")
	treatment := fs.String("treatment", "", "the arm label of the runs compared with the baseline")
	asJSON := fs.Bool("json", false, "print the comparison as JSON")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "Usage: replay outcome-compare --control <arm> --treatment <arm> [--json] <run-dir...>\n\n"+
			"Scores each run directory as `replay outcome` does and compares the two\n"+
			"arms by PASS count. Each arm needs at least %d eligible (PASS or FAIL)\n"+
			"runs, a predeclared minimum and not a significance threshold, and the\n"+
			"control may not be at 0 of n or n of n. No effect is estimated; cost, time\n"+
			"and tokens are not compared. Reads only; writes nothing.\n\n", outcome.MinEligiblePerArm)
		fs.PrintDefaults()
	}
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	switch {
	case *control == "" || *treatment == "":
		return fmt.Errorf("--control and --treatment are both required: %w", errUsage)
	case *control == *treatment:
		return fmt.Errorf("--control and --treatment both name %q; two arms are compared: %w", *control, errUsage)
	case fs.NArg() == 0:
		return fmt.Errorf("at least one run directory is required: %w", errUsage)
	}
	if err := refuseRepeatedRunDirs(fs.Args()); err != nil {
		return err
	}
	m, raw, err := outcomeManifest()
	if err != nil {
		return fmt.Errorf("the embedded outcome manifest is invalid: %w", err)
	}
	rep := comparisonOutput{
		Schema: outcomeComparisonSchema, TaskID: m.TaskID, ManifestSHA256: outcome.ManifestDigest(raw),
		MinEligiblePerArm: outcome.MinEligiblePerArm, ControlArm: *control, TreatmentArm: *treatment,
		Claim: outcomeComparisonClaim, NotMeasured: outcomeNotMeasured,
	}
	var ctl, trt []outcome.Result
	for _, dir := range fs.Args() {
		r := outcome.Score(m, raw, dir)
		row := comparisonRun{RunDir: dir, Arm: r.Arm, Verdict: string(r.Verdict), Reasons: append([]string{}, r.Reasons...)}
		switch r.Arm {
		case *control:
			row.Role, ctl = "control", append(ctl, r)
		case *treatment:
			row.Role, trt = "treatment", append(trt, r)
		case "":
			row.Role, row.Exclusion = "excluded", "arm_unknown"
		default:
			row.Role, row.Exclusion = "excluded", "arm_not_compared"
		}
		rep.Runs = append(rep.Runs, row)
	}
	c := outcome.Compare(ctl, trt)
	rep.Status, rep.Note = string(c.Status), c.Note
	rep.Control = comparisonArm{Pass: c.Control.Pass, Eligible: c.Control.N, Unavailable: c.Control.Unavailable}
	rep.Treatment = comparisonArm{Pass: c.Treatment.Pass, Eligible: c.Treatment.N, Unavailable: c.Treatment.Unavailable}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	return writeComparison(stdout, rep)
}

func writeComparison(w io.Writer, rep comparisonOutput) error {
	p := analysis.NewPrinter(w)
	p.Printf("replay outcome-compare: task %s, manifest sha256 %s\n", rep.TaskID, rep.ManifestSHA256[:12])
	p.Printf("%s against %s; at least %d eligible runs an arm, predeclared, not a significance threshold\n\n",
		rep.TreatmentArm, rep.ControlArm, rep.MinEligiblePerArm)
	for _, a := range []struct {
		name string
		arm  comparisonArm
	}{{rep.ControlArm, rep.Control}, {rep.TreatmentArm, rep.Treatment}} {
		p.Printf("  %s  %d of %d PASS", a.name, a.arm.Pass, a.arm.Eligible)
		if a.arm.Unavailable > 0 {
			p.Printf(", %d UNAVAILABLE not counted", a.arm.Unavailable)
		}
		p.Printf("\n")
	}
	p.Printf("\n%s: %s\n%s\n", rep.Status, rep.Note, rep.Claim)
	for _, nm := range rep.NotMeasured {
		p.Printf("%s: NOT MEASURED\n", nm)
	}
	p.Printf("\nruns:\n")
	for _, r := range rep.Runs {
		p.Printf("  %-9s  %-11s  %s", r.Role, r.Verdict, r.RunDir)
		if r.Exclusion != "" {
			p.Printf("  %s", r.Exclusion)
		}
		p.Printf("\n")
	}
	return p.Err()
}
