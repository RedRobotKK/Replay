package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/RedRobotKK/Replay/internal/analysis"
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
