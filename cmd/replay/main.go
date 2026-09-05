// Command replay analyzes coding-agent sessions for prompt-cache behavior.
//
// replay, blame, and diff work offline on transcripts the agent already
// wrote and on the ledger the proxy records. serve is the proxy.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
	"github.com/RedRobotKK/Replay/internal/version"
)

// errUsage is returned for malformed invocations after usage is printed.
var errUsage = errors.New("invalid usage")

// defaultBlameLimit bounds the blame table so it fits a terminal.
const defaultBlameLimit = 20

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "replay:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return printUsage(stdout)
	}
	switch args[0] {
	case "version", "--version", "-v":
		_, err := fmt.Fprintln(stdout, "replay", version.String())
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
		// The tool is named for its own first command, so "replay replay
		// <path>" would be the common invocation. A first argument that
		// names something on disk is taken as the replay analysis's own
		// argument instead. A subcommand name always wins, so a directory
		// called "serve" still needs the explicit "replay replay serve".
		if namesAPath(args[0]) {
			return runReport(args, stdout, stderr, (*analysis.LaneReport).WriteReplay)
		}
		// The usage text is a courtesy on the error path; the error that
		// matters is the unknown command.
		_ = printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// namesAPath reports whether an argument refers to something that exists,
// which is what separates a path the user meant from a command they
// mistyped. A flag is never a path.
func namesAPath(arg string) bool {
	if arg == "" || strings.HasPrefix(arg, "-") {
		return false
	}
	_, err := os.Stat(arg)
	return err == nil
}

// hoistFlags moves flag arguments ahead of paths.
//
// Go's flag package stops parsing at the first non-flag argument, so
// `replay <dir> --dollars` handed "--dollars" to os.Stat and every documented
// use of the flag failed with "stat --dollars: no such file or directory".
// Putting the flag last is the form people reach for, the form README shows,
// and the form the CLI's own usage text shows, so it has to work.
//
// A path is anything that is not a flag. Values that belong to a flag are not
// separated from it here: only boolean flags exist on this path today, and
// `--flag=value` keeps its value attached.
func hoistFlags(args []string) []string {
	flags := make([]string, 0, len(args))
	paths := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			continue
		}
		paths = append(paths, a)
	}
	return append(flags, paths...)
}

func runReport(args []string, stdout, stderr io.Writer, write func(*analysis.LaneReport, io.Writer) error) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dollars := fs.Bool("dollars", false, "add a list-price cost column (first-party rates, dated price table)")
	if err := fs.Parse(hoistFlags(args)); err != nil {
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
		// Claude Code writes ~/.claude/projects/<project>/*.jsonl, so the
		// directory a person naturally points at is the parent of the
		// transcripts, not the one holding them. Walk, so both work.
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			// Skip dot directories: caches and VCS metadata are not sessions,
			// and walking them is a way to be slow and wrong at once.
			if d.IsDir() {
				if path != p && strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".jsonl") {
				return nil
			}
			return add(path)
		})
		if err != nil {
			return nil, err
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
		return fmt.Errorf("usage: replay redact <transcript.jsonl> > redacted.jsonl: %w", errUsage)
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read-only file; a close error carries no information we can act on
	return transcript.Redact(f, stdout)
}

func printUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `replay - see where your coding agent's prompt cache broke and what it cost

Usage:
  replay <transcript|dir>          reproduce caching, then score alternative layouts (--dollars adds list cost)
  replay replay <transcript|dir>   the same, named explicitly
  replay blame  <transcript|dir>   rank what is eating prompt tokens
  replay diff   <transcript|dir>   locate and classify every cache break
  replay corpus <dir...>           calibration summary across many sessions, as Markdown (no paths or content)
  replay advise <dir...>           turn the largest token sources across sessions into suggestions with predicted savings, tracked to closure
  replay learn  <dir...>           re-score the policy catalog over all sessions, select one with held-out checks, write ~/.replay/policy.json
  replay doctor                    what replay can see on this machine and what to do next
  replay redact <transcript>       strip content, keep structure and usage (for bug reports)
  replay serve [flags]             local proxy: byte-for-byte passthrough, records a ledger
  replay version                   print build information

Transcripts: Claude Code writes them under ~/.claude/projects/<project>/*.jsonl
             ($CLAUDE_CONFIG_DIR/projects if you have relocated it)
Ledger:      replay serve writes ~/.replay/ledger/<session>.jsonl (measured tier)
`)
	return err
}
