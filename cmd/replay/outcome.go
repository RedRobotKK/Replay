package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/outcome"
)

// `replay outcome` scores run directories against the one task Replay pins:
// the count of `func Test` lines in cmd/replay/b*_test.go at commit 6871022
// (internal/outcome, UNWIRED-LOG #19).
//
// It reads and never writes. It runs no agent, calls no provider and touches
// no ledger: a run directory is evidence someone else produced, and scoring it
// must not change it. The manifest is the one embedded in the binary. There is
// no flag to supply another, because a manifest the caller hands in is a truth
// the caller chose.
//
// Three verdicts and only three. PASS and FAIL need admissible evidence;
// anything missing, malformed or unverifiable is UNAVAILABLE, reported with
// every reason and never counted as a failure. A truth is printed only when it
// was counted from a tree proven identical to the pinned substrate.
//
// The exit status says whether every run directory was scored, not what the
// verdicts were: a run that FAILs or is UNAVAILABLE still exits 0, so read the
// verdicts from the output.

// outcomeManifest is the manifest scored against. Tests replace it with a
// small synthetic substrate; nothing in the binary does.
var outcomeManifest = outcome.Pinned

const outcomeReportSchema = "replay.outcome.report.v1"

type outcomeRun struct {
	RunDir  string   `json:"run_dir"`
	Verdict string   `json:"verdict"`
	Reasons []string `json:"reasons"`
	Arm     string   `json:"arm"`
	Answer  *int     `json:"answer"`
	Truth   *int     `json:"truth"`
	TreeID  string   `json:"tree_id"`
}

type outcomeSummary struct {
	Pass        int `json:"pass"`
	Fail        int `json:"fail"`
	Unavailable int `json:"unavailable"`
}

type outcomeOutput struct {
	Schema          string         `json:"schema"`
	TaskID          string         `json:"task_id"`
	ManifestSHA256  string         `json:"manifest_sha256"`
	SubstrateCommit string         `json:"substrate_commit"`
	SubstrateTree   string         `json:"substrate_tree"`
	Runs            []outcomeRun   `json:"runs"`
	Summary         outcomeSummary `json:"summary"`
}

func runOutcome(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("outcome", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print the report as JSON")
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), "Usage: replay outcome <run-dir...> [--json]\n\n"+
			"Scores each run directory against the task pinned in this binary\n"+
			"(count-b-tests-v1). A run directory holds run.json, the run's record,\n"+
			"and tree/, the working copy the run left. Each run is PASS, FAIL or\n"+
			"UNAVAILABLE; UNAVAILABLE means evidence was missing or could not be\n"+
			"verified, and is not a failed run. Reads only; writes nothing.\n\n")
		fs.PrintDefaults()
	}
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("at least one run directory is required: %w", errUsage)
	}
	if err := refuseRepeatedRunDirs(fs.Args()); err != nil {
		return err
	}
	m, raw, err := outcomeManifest()
	if err != nil {
		return fmt.Errorf("the embedded outcome manifest is invalid: %w", err)
	}
	rep := outcomeOutput{
		Schema: outcomeReportSchema, TaskID: m.TaskID, ManifestSHA256: outcome.ManifestDigest(raw),
		SubstrateCommit: m.SubstrateCommit, SubstrateTree: m.SubstrateTree,
	}
	for _, dir := range fs.Args() {
		r := outcome.Score(m, raw, dir)
		rep.Runs = append(rep.Runs, outcomeRun{
			RunDir: dir, Verdict: string(r.Verdict), Reasons: append([]string{}, r.Reasons...),
			Arm: r.Arm, Answer: r.Answer, Truth: r.Truth, TreeID: r.TreeID,
		})
		switch r.Verdict {
		case outcome.Pass:
			rep.Summary.Pass++
		case outcome.Fail:
			rep.Summary.Fail++
		default:
			rep.Summary.Unavailable++
		}
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	return writeOutcome(stdout, rep)
}

func writeOutcome(w io.Writer, rep outcomeOutput) error {
	p := analysis.NewPrinter(w)
	p.Printf("replay outcome: task %s, manifest sha256 %s\n", rep.TaskID, rep.ManifestSHA256[:12])
	p.Printf("substrate %s, tree %s\n\n", rep.SubstrateCommit[:7], rep.SubstrateTree[:7])
	for _, r := range rep.Runs {
		p.Printf("  %-11s  %s  %s\n", r.Verdict, r.RunDir, outcomeDetail(r))
	}
	p.Printf("\n%d PASS, %d FAIL, %d UNAVAILABLE\n", rep.Summary.Pass, rep.Summary.Fail, rep.Summary.Unavailable)
	p.Printf("UNAVAILABLE is missing or unverifiable evidence, not a failed run.\n")
	return p.Err()
}

// outcomeDetail names why, and states a truth only when one was derived.
func outcomeDetail(r outcomeRun) string {
	var parts []string
	if len(r.Reasons) > 0 {
		parts = append(parts, strings.Join(r.Reasons, ", "))
	}
	switch {
	case r.Truth != nil && r.Answer != nil:
		parts = append(parts, fmt.Sprintf("answer %d, truth %d derived from the verified tree", *r.Answer, *r.Truth))
	case r.Truth != nil:
		parts = append(parts, fmt.Sprintf("no answer read, truth %d derived from the verified tree", *r.Truth))
	default:
		parts = append(parts, "no truth derived")
	}
	return strings.Join(parts, ": ")
}

// refuseRepeatedRunDirs refuses a list that names one run directory twice,
// which would score it twice and count it twice.
func refuseRepeatedRunDirs(dirs []string) error {
	seen := map[string]string{}
	for _, dir := range dirs {
		key := outcomeRunKey(dir)
		if first, ok := seen[key]; ok {
			return fmt.Errorf("%s and %s name the same run directory, which would be scored and counted twice: %w", first, dir, errUsage)
		}
		seen[key] = dir
	}
	return nil
}

// outcomeRunKey is where dir points: its absolute path with symbolic links
// resolved, or its cleaned path where it cannot be made absolute, and its
// absolute path where it does not exist. Two arguments with one key are one
// run directory. Two directories holding copies of one run have two keys;
// this is identity by path, not by content.
func outcomeRunKey(dir string) string {
	key := filepath.Clean(dir)
	if abs, err := filepath.Abs(key); err == nil {
		key = abs
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			key = resolved
		}
	}
	return key
}
