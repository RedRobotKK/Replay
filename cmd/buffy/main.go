// Command buffy analyzes coding-agent sessions for prompt-cache behavior.
//
// replay, blame, and diff work offline on transcripts the agent already
// wrote and on the ledger the proxy records. serve is the proxy.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/ledger"
	"github.com/RedRobotKK/Buffy/internal/transcript"
	"github.com/RedRobotKK/Buffy/internal/version"
)

// errUsage is returned for malformed invocations after usage is printed.
var errUsage = errors.New("invalid usage")

// defaultBlameLimit bounds the blame table so it fits a terminal.
const defaultBlameLimit = 20

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "buffy:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return printUsage(stdout)
	}
	switch args[0] {
	case "version", "--version", "-v":
		_, err := fmt.Fprintln(stdout, "buffy", version.String())
		return err
	case "help", "--help", "-h":
		return printUsage(stdout)
	case "replay":
		return runReport(args[1:], stdout, stderr, (*analysis.LaneReport).WriteReplay)
	case "blame":
		return runReport(args[1:], stdout, stderr, func(r *analysis.LaneReport, w io.Writer) error { return r.WriteBlame(w, defaultBlameLimit) })
	case "diff":
		return runReport(args[1:], stdout, stderr, (*analysis.LaneReport).WriteDiff)
	case "corpus":
		return runCorpus(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "advise":
		return runAdvise(args[1:], stdout, stderr)
	case "learn":
		return runLearn(args[1:], stdout, stderr)
	case "redact":
		return runRedact(args[1:], stdout)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	default:
		// The usage text is a courtesy on the error path; the error that
		// matters is the unknown command.
		_ = printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runReport(args []string, stdout, stderr io.Writer, write func(*analysis.LaneReport, io.Writer) error) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dollars := fs.Bool("dollars", false, "add a list-price cost column (first-party rates, dated price table)")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("a transcript file or directory is required: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}
	failures, printed := 0, 0
	err = forEachSession(files, func(f string, _ *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil {
			failures++
			// Diagnostics on stderr are best effort; a failure there must
			// not mask the analysis error being reported.
			_, _ = fmt.Fprintf(stderr, "skip %s: %v\n", f, err)
			return nil
		}
		header := f + "\n"
		if printed > 0 {
			header = strings.Repeat("-", 80) + "\n" + header
		}
		printed++
		if _, err := io.WriteString(stdout, header); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		rep.Dollars = *dollars
		if err := write(rep, stdout); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if failures == len(files) {
		return fmt.Errorf("no transcript could be analyzed")
	}
	return nil
}

// forEachSession loads and analyzes every file's main lane and hands each
// result, or the reason it could not be produced, to visit. It stops at
// the first error visit returns.
func forEachSession(files []string, visit func(path string, session *transcript.Session, rep *analysis.LaneReport, err error) error) error {
	for _, f := range files {
		session, err := loadSession(f)
		if err != nil {
			if err := visit(f, nil, nil, err); err != nil {
				return err
			}
			continue
		}
		lane := analysis.MainLane(session)
		if lane == nil {
			if err := visit(f, session, nil, errors.New("no requests")); err != nil {
				return err
			}
			continue
		}
		if err := visit(f, session, analysis.AnalyzeLane(session, lane), nil); err != nil {
			return err
		}
	}
	return nil
}

// loadSession parses a transcript or a ledger file, deciding by content:
// ledger records carry a schema field on every line.
func loadSession(path string) (*transcript.Session, error) {
	if ledger.IsLedgerFile(path) {
		return ledger.ReadFile(path)
	}
	return transcript.ParseClaudeCodeFile(path)
}

// transcriptFiles expands files and directories into transcript paths,
// largest first so the most informative session prints first.
func transcriptFiles(paths []string) ([]string, error) {
	type entry struct {
		path string
		size int64
	}
	var entries []entry
	add := func(p string) error {
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		entries = append(entries, entry{path: p, size: info.Size()})
		return nil
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			entries = append(entries, entry{path: p, size: info.Size()})
			continue
		}
		matches, err := filepath.Glob(filepath.Join(p, "*.jsonl"))
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if err := add(m); err != nil {
				return nil, err
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no .jsonl transcripts found")
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].size != entries[j].size {
			return entries[i].size > entries[j].size
		}
		return entries[i].path < entries[j].path
	})
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		files = append(files, e.path)
	}
	return files, nil
}

func runRedact(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: buffy redact <transcript.jsonl> > redacted.jsonl: %w", errUsage)
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read-only file; a close error carries no information we can act on
	return transcript.Redact(f, stdout)
}

func printUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `buffy - see where your coding agent's prompt cache broke and what it cost

Usage:
  buffy replay <transcript|dir>   reproduce caching, then score alternative layouts (--dollars adds list cost)
  buffy blame  <transcript|dir>   rank what is eating prompt tokens
  buffy diff   <transcript|dir>   locate and classify every cache break
  buffy corpus <dir...>           calibration summary across many sessions, as Markdown (no paths or content)
  buffy advise <dir...>           turn the largest token sources across sessions into suggestions with predicted savings, tracked to closure
  buffy learn  <dir...>           re-score the policy catalog over all sessions, select one with held-out checks, write ~/.buffy/policy.json
  buffy doctor                    what buffy can see on this machine and what to do next
  buffy redact <transcript>       strip content, keep structure and usage (for bug reports)
  buffy serve [flags]             local proxy: byte-for-byte passthrough, records a ledger
  buffy version                   print build information

Transcripts: Claude Code writes them under ~/.claude/projects/<project>/*.jsonl
Ledger:      buffy serve writes ~/.buffy/ledger/<session>.jsonl (measured tier)
`)
	return err
}
