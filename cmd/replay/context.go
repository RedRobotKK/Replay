package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/reference"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// `replay context` answers what a session's context is made of.
//
// Claude Code tells you how full the window is. It does not tell you what is
// filling it, and that is the question a person acts on: a quarter to a third
// of a typical prompt is tool output being resent on every turn, and knowing
// which tool is the difference between a vague intention to "use less context"
// and a specific one.
//
// What it does NOT claim is in the name of the type it prints. See
// analysis.ContextEntry: the underlying attribution never subtracts, so this is
// content that entered the context, not content still in it.
// systemPromptLine places this corpus's system-prompt share against a
// published population, and returns "" when there is nothing to place.
//
// Split out of the walk so the empty cases can be reached from a test. They
// cannot be reached from a Claude Code transcript: every one observed produces
// a system row, because a request carrying a cache read implies a prefix the
// transcript cannot see. The ledger and the Codex reader are not bound by that,
// and a zero share of a system prompt is not the same claim as a session that
// carried none — absence, zero and unknown are three values (ADR-0018).
func systemPromptLine(sysTokens, allTokens int) string {
	return referenceLine("system prompt", "systemPromptShare", sysTokens, allTokens)
}

// referenceLine places a part of the corpus against a published figure for the
// same metric, and returns "" when it cannot.
//
// The metric is a parameter rather than a literal so the not-found branch can
// be entered from a test. It is unreachable today — Compiled() carries
// systemPromptShare and CR6 pins that it does — and a branch no test can enter
// is one guard-reachability reports and this repository does not keep.
func referenceLine(name, metric string, part, whole int) string {
	ref, ok := reference.For(metric)
	if !ok {
		return ""
	}
	// Three values, three renderings. This used to be
	// `if whole <= 0 || part <= 0 { return "" }`, which collapsed the last
	// two into the first and printed nothing for all three — in a function
	// whose caller's doc comment cites ADR-0018 by name.
	//
	// No denominator is the UNKNOWN case: nothing counted the tokens, so
	// there is no local figure and CompareMissing renders the published half
	// alone. A part of zero against a real whole is a MEASUREMENT, and 0% is
	// an answer a reader is entitled to see.
	if whole <= 0 {
		return name + ": " + ref.CompareMissing().Line()
	}
	return name + ": " + ref.Compare(float64(part)/float64(whole)).Line()
}

func runContext(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the attribution as JSON")
	top := fs.Int("top", 12, "how many rows to print")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("a transcript file or directory is required: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}

	printed := 0
	// sysTokens and allTokens accumulate across the walk so the population
	// comparison below is printed once per run rather than once per transcript.
	sysTokens, allTokens := 0, 0
	err = forEachSession(files, func(path string, session *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || rep == nil {
			return nil
		}
		rows := analysis.EnteredContext(rep.Blame)
		if len(rows) == 0 {
			return nil
		}
		if printed > 0 {
			_, _ = fmt.Fprintln(stdout)
		}
		printed++

		if *asJSON {
			b, err := json.MarshalIndent(map[string]any{
				"schema":  "replay.context.v1",
				"session": prefixID(session.ID),
				"measures": "content that entered this context; the attribution does not " +
					"subtract cleared or compacted content",
				"entries": rows,
				"gap":     analysis.MeasureGap(session, rep.Lane, sumTokens(rows)),
			}, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "%s\n", b)
			return err
		}

		total := 0
		for _, r := range rows {
			total += r.Tokens
		}
		_, _ = fmt.Fprintf(stdout, "%s\nSession %s  %s tokens of content entered this context\n\n",
			path, prefixID(session.ID), formatCount(total))
		for i, r := range rows {
			if i >= *top {
				_, _ = fmt.Fprintf(stdout, "  ... and %d more\n", len(rows)-*top)
				break
			}
			mark := ""
			if r.Estimated {
				mark = " *"
			}
			_, _ = fmt.Fprintf(stdout, "  %-*s %5.1f%%  %10s  x%-5d%s\n",
				analysis.MaxContextLabel, r.Label, r.Share*100, formatCount(r.Tokens), r.Occurrences, mark)
		}
		// Accumulated across the walk, not printed per session. `replay context
		// <dir>` reads every transcript, and a comparison under each of 1,800
		// tables is one the reader learns to skip — the objection
		// internal/analysis/outlier.go raises about printing a comparison on
		// every run, at a smaller scale.
		for _, r := range rows {
			if r.Label == transcript.RoleSystem {
				sysTokens += r.Tokens
			}
		}
		allTokens += total
		gap := analysis.MeasureGap(session, rep.Lane, total)
		_, _ = fmt.Fprintf(stdout, "\n  %s\n", analysis.FitNote(rep.Fit))
		_, _ = fmt.Fprintf(stdout, "\n  %s\n", gap.Note())
		return nil
	})
	if err != nil {
		return err
	}
	// Not on --json: a funding line inside machine-readable output is
	// corruption, and a caller whose parser breaks strips the tool.
	//
	// Gated on `printed`, which counts both formats, rather than on a counter
	// incremented only in the human branch. With the latter the !*asJSON test
	// was shadowed -- removing it changed nothing, because the JSON path had
	// already returned without counting -- which is a dead guard by ADR-0014's
	// standard and would have come back to life the day somebody moved the
	// increment.
	// Where this reader's system prompt sits against a published population.
	//
	// The pooled corpus has one member, so this tool cannot supply a reference
	// from its own contributors. It can carry somebody else's, with their
	// citation and their population attached — see
	// docs/design/reference-distribution.md. The population travels with the
	// figure on the same line, every time, because "28.8% here, 14.0% across
	// 13.5M sessions" is a sentence a reader can weigh and "roughly double the
	// average" is not.
	//
	// Not on --json: a citation inside machine-readable output is corruption,
	// for the same reason the funding line below is gated.
	//
	// A corpus with no system block prints nothing rather than a zero share.
	// Absence, zero and unknown are three values, and a transcript that carried
	// no system prompt has not measured a 0% share of one.
	if printed > 0 && !*asJSON {
		if line := systemPromptLine(sysTokens, allTokens); line != "" {
			_, _ = fmt.Fprintf(stdout, "\n  %s\n", line)
		}
	}
	if printed > 0 && !*asJSON {
		_, _ = io.WriteString(stdout, supportLine(describeResult("context"), stdout))
	}
	return nil
}

func sumTokens(rows []analysis.ContextEntry) int {
	n := 0
	for _, r := range rows {
		n += r.Tokens
	}
	return n
}
