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
	"runtime"
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
	// Rules first: every figure any command prints is built on them, so a
	// command must not run under the compiled defaults when a document has
	// been installed. `rules --update` validates through the same loader, so
	// nothing installable can fail here.
	LoadInstalledRules(os.Stderr)
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "replay:", err)
		os.Exit(exitCode(err))
	}
}

// exitCode maps an error to a process exit status.
//
// 2 means "this resource wants paying", which is a decision for whoever holds
// the wallet rather than a fault, and an agent needs to tell it from a broken
// URL without parsing prose. Everything else is 1.
//
// It is a function rather than a few lines inside main so that the mapping can
// be tested: main() calls os.Exit, which no test can observe.
func exitCode(err error) int {
	var pay *paymentRequiredError
	if errors.As(err, &pay) {
		return 2
	}
	return 1
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runDefault(stdout, stderr)
	}
	err := dispatch(args, stdout, stderr)
	if errors.Is(err, errHelpShown) {
		// The usage text is already on stdout. Nothing went wrong.
		return nil
	}
	return err
}

// runDefault is what `replay` on its own does: lead with the finding.
//
// The install line is `curl | sh`, and the next thing a person types is the
// name of the thing they just installed. What that used to print was a list of
// sixteen commands with `cost` — the one that reports money, and the reason any
// of this exists — eleventh. A menu is what a tool prints when it does not know
// what you want, and this one does know: defaultTranscriptRoots finds the
// transcripts unaided, and `replay doctor` has been reporting on them from that
// same path all along.
//
// The menu stays for the case where there is genuinely nothing to say. A report
// over no transcripts is not a finding — an empty table and a $0.00 total read
// as "this tool is broken" rather than "there is nothing here yet" — so an
// undiscoverable corpus falls back to the list, which is the useful answer
// there.
func runDefault(stdout, stderr io.Writer) error {
	// A home directory that cannot be resolved is the same situation as one
	// with no transcripts under it: nothing to lead with, so print the list.
	// defaultTranscriptRoots treats the empty string as "no root" rather than
	// resolving it against the working directory, which is why the error needs
	// no separate branch here.
	home, _ := os.UserHomeDir()
	if len(defaultTranscriptRoots(home)) == 0 {
		// Say what happened before showing the list. The menu is the right
		// answer here (BR-2) and it is not a self-explanatory one: a reader
		// who pasted `replay` and received sixteen commands cannot tell
		// whether the tool needs arguments, failed, or simply found nothing.
		// All three look identical, and only the third is true.
		//
		// It matters more since the installer began pointing every new user
		// at bare `replay` as their first command. On a machine that has
		// never run Claude Code, this branch IS the first impression.
		if home != "" {
			_, _ = fmt.Fprintf(stderr, "No transcripts found in %s\n",
				filepath.Join(claudeConfigDir(home), "projects"))
			_, _ = fmt.Fprint(stderr, "Replay reads Claude Code sessions. If you use a different "+
				"agent, replay serve proxies any of them.\n\n")
		}
		return printUsage(stdout)
	}
	if err := runCost(nil, stdout, stderr); err != nil {
		return err
	}
	return printMoreCommands(stdout)
}

func dispatch(args []string, stdout, stderr io.Writer) error {
	switch args[0] {
	case "version", "--version", "-v":
		if wantsHelp(args[1:]) {
			_, _ = fmt.Fprint(stdout, "Usage of version:\n"+
				"  replay version\n\n"+
				"Prints the version, the commit and the build date. Takes no flags.\n"+
				"A source build reports \"dev (unknown, built unknown)\": the real\n"+
				"values are injected at release time, so an agent that built from\n"+
				"source cannot confirm which release it has.\n")
			return errHelpShown
		}
		_, err := fmt.Fprintln(stdout, "replay", version.String())
		return err
	case "help", "--help", "-h":
		return printUsage(stdout)
	case "replay":
		return runReport("replay", args[1:], stdout, stderr, (*analysis.LaneReport).WriteReplay)
	case "blame":
		return runReport("blame", args[1:], stdout, stderr, func(r *analysis.LaneReport, w io.Writer) error { return r.WriteBlame(w, defaultBlameLimit) })
	case "diff":
		return runReport("diff", args[1:], stdout, stderr, (*analysis.LaneReport).WriteDiff)
	case "corpus":
		return runCorpus(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "probe":
		return runProbe(args[1:], stdout, stderr)
	case "rules":
		return runRules(args[1:], stdout, stderr)
	case "statusline":
		return runStatusline(args[1:], stdout, stderr)
	case "cost":
		return runCost(args[1:], stdout, stderr)
	case "trim":
		return runTrim(args[1:], stdout, stderr)
	case "route":
		return runRoute(args[1:], stdout, stderr)
	case "context":
		return runContext(args[1:], stdout, stderr)
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
			return runReport("replay", args, stdout, stderr, (*analysis.LaneReport).WriteReplay)
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
// A path is anything that is not a flag. Values that belong to a flag are NOT
// separated from it here, so this is only safe for flag sets whose flags are
// all boolean, or whose values are written as `--flag=value`. Do not use it on
// a command with a space-separated string flag: hoisting would move the flag
// and leave its value behind as a path. `replay rules --update <src>` is such a
// command and parses its arguments directly.
// hoistFlagsFor lets flags appear after the path, which the flag package
// will not do on its own, and knows which flags take a value.
//
// The flag package stops parsing at the first argument that is not a flag, so
// `replay route dir --to m2` would leave --to unparsed. Hoisting fixes that,
// but hoisting a flag while leaving its value behind is worse than not
// hoisting at all: the value silently becomes a path, and the command fails
// on a file nobody named. That was a real defect on every value-taking flag
// placed after a path, --to, --top and --compare among them.
//
// Booleans are the exception and must not swallow the next argument: `--json
// dir` means two things, not one flag with the value "dir". The FlagSet knows
// which is which, so ask it rather than keeping a list here that drifts.
func hoistFlagsFor(fs *flag.FlagSet, args []string) []string {
	takesValue := func(name string) bool {
		name = strings.TrimLeft(name, "-")
		f := fs.Lookup(name)
		if f == nil {
			return false
		}
		b, ok := f.Value.(interface{ IsBoolFlag() bool })
		return !ok || !b.IsBoolFlag()
	}

	flags := make([]string, 0, len(args))
	paths := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			// Everything after the terminator is a path by definition,
			// including something that looks like a flag.
			paths = append(paths, args[i:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			paths = append(paths, a)
			continue
		}
		flags = append(flags, a)
		// --name=value carries its own value; --name does not.
		if !strings.Contains(a, "=") && takesValue(a) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, paths...)
}

// name is the command the user actually typed. Three commands share this
// body, and a flagset called "report" printed "Usage of report:" for a command
// nobody can invoke by that name.
func runReport(name string, args []string, stdout, stderr io.Writer, write func(*analysis.LaneReport, io.Writer) error) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dollars := fs.Bool("dollars", false, "add a list-price cost column (first-party rates, dated price table)")
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
	// Once per run, not once per session, and only when something was actually
	// printed. A reader who got nothing is not asked for anything.
	if printed > 0 {
		if _, err := io.WriteString(stdout, supportLine(describeResult(name), stdout)); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
	}
	return nil
}

// describeResult names, in the reader's terms, what the command just handed
// them. A specific sentence is both more useful and easier to believe than a
// generic plea, and it keeps the ask in the same register as the report above
// it.
func describeResult(name string) string {
	switch name {
	case "context":
		return "what is filling your context"
	case "blame":
		return "what your carried content cost"
	case "diff":
		return "where your prompt cache broke, and why"
	case "trim":
		return "what trimming would have cost you"
	case "route":
		return "what another model would have cost"
	case "cost":
		return "what your agent work actually cost"
	case "advise":
		return "what to change, taken from your own history"
	case "learn":
		return "a policy scored against your own sessions"
	default:
		return "your sessions, replayed against the provider's caching rules"
	}
}

// forEachSession loads and analyzes every file's main lane and hands each
// result, or the reason it could not be produced, to visit. It stops at
// the first error visit returns.
func forEachSession(files []string, visit func(path string, session *transcript.Session, rep *analysis.LaneReport, err error) error) error {
	// Parsing is around 89% of the work and files are independent, but the
	// process used 108% of ten cores. Parsing and analysis now run on a pool;
	// visit still runs serially, in file order.
	//
	// The ordering is not a nicety. Corpus rows are sorted with sort.Slice,
	// which is unstable and keyed only on request count, so the order rows are
	// APPENDED decides every tie. Handing results to visit as they finish
	// would quietly change the report; reassembling by file index does not,
	// and the output is byte-identical across corpus, blame, cost, advise,
	// trim and route.
	//
	// Work is launched only a short way ahead of the cursor. Without that
	// bound a fast worker could finish file 1400 while visit is still on file
	// 5, holding a thousand parsed sessions in memory — and the largest single
	// transcript here is 336 MB.
	workers := runtime.GOMAXPROCS(0)
	if workers > 8 {
		workers = 8
	}
	if workers < 1 || len(files) < 2 {
		workers = 1
	}

	type result struct {
		session *transcript.Session
		rep     *analysis.LaneReport
		err     error
	}
	results := make([]result, len(files))
	ready := make([]chan struct{}, len(files))
	for i := range ready {
		ready[i] = make(chan struct{})
	}

	sem := make(chan struct{}, workers)
	launch := func(i int) {
		go func() {
			sem <- struct{}{}
			defer func() { <-sem; close(ready[i]) }()
			session, err := loadSession(files[i])
			if err != nil {
				results[i] = result{err: err}
				return
			}
			lane := analysis.MainLane(session)
			if lane == nil {
				results[i] = result{session: session, err: errors.New("no requests")}
				return
			}
			results[i] = result{session: session, rep: analysis.AnalyzeLane(session, lane)}
		}()
	}

	window := workers * 2
	next := 0
	for cursor := 0; cursor < len(files); cursor++ {
		for next < len(files) && next < cursor+window {
			launch(next)
			next++
		}
		<-ready[cursor]
		r := results[cursor]
		// Release the session before visiting the next file, so the window
		// bounds live memory rather than merely bounding concurrency.
		results[cursor] = result{}
		if err := visit(files[cursor], r.session, r.rep, r.err); err != nil {
			// Drain the launched goroutines so none outlives this call.
			for i := cursor + 1; i < next; i++ {
				<-ready[i]
				results[i] = result{}
			}
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
	// redact reads its argument as a path and parses no flags, so without this
	// `redact --help` was opened as a filename and reported
	// "open --help: no such file or directory".
	if wantsHelp(args) {
		_, _ = fmt.Fprint(stdout, "Usage of redact:\n"+
			"  replay redact <transcript.jsonl> > redacted.jsonl\n\n"+
			"Rewrites a transcript with message text removed, keeping block kinds,\n"+
			"sizes and usage counts, so a session can be shared without its content.\n"+
			"Takes one path and writes to stdout. No flags.\n")
		return errHelpShown
	}
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

// printMoreCommands is the way out of the report a bare `replay` just printed.
//
// Leading with one answer is only an improvement if the others stay findable.
// A reader who wanted `doctor` should not have to already know that `--help`
// exists, and four lines under the result is a cheaper way to tell them than
// twenty lines above it.
func printMoreCommands(w io.Writer) error {
	_, err := fmt.Fprint(w, `
More:
  replay doctor         what replay can see on this machine, and what to do next
  replay blame <dir>    what is eating your prompt tokens
  replay diff  <dir>    where the cache broke, and why
  replay --help         every command
`)
	return err
}

func printUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `replay - see where your coding agent's prompt cache broke and what it cost

Start here:
  replay <no args>                 what your sessions cost, from transcripts already on disk
  replay diff   <transcript|dir>   locate and classify every cache break, with its cause
  replay advise <dir...>           the largest token sources, ranked, with predicted savings
  replay serve  [flags]            local proxy: byte-for-byte passthrough, records a ledger

Look closer:
  replay cost   <dir...>           cost per task, and --compare <date> for before/after
  replay cost   <dir> --share      a card that is safe to post: a rate, no total, no paths
  replay context <transcript|dir>  what entered a session's context, by tool
  replay blame  <transcript|dir>   rank what is eating prompt tokens
  replay <transcript|dir>          reproduce caching, then score alternative layouts
  replay replay <transcript|dir>   the same, named explicitly
  replay route  <dir> --to <model> what switching models would change, structurally
  replay trim   <dir> --cap <n>    what a byte cap on tool output would have saved, and cost
  replay advise <dir> --guards     spend caps from your own session spread, print-only

Corpus and calibration:
  replay corpus <dir...>           calibration across many sessions, as Markdown (no paths or content)
  replay learn  <dir...>           re-score the policy catalog, select one with held-out checks
  replay probe  --model <id>       measure a model's caching floor; plans by default, --execute sends

Setup and maintenance:
  replay doctor                    what replay can see on this machine and what to do next
  replay rules  [--update <src>]   show the provider rules in effect, or install a dated document
  replay statusline                live spend and cache-miss cost, for Claude Code's status line
  replay redact <transcript>       strip content, keep structure and usage (for bug reports)
  replay version                   print build information

With no arguments at all, replay reports cost per task across the transcripts it
finds on this machine, and prints this list only when it finds none.

Transcripts: Claude Code writes them under ~/.claude/projects/<project>/*.jsonl
             ($CLAUDE_CONFIG_DIR/projects if you have relocated it)
Ledger:      replay serve writes ~/.replay/ledger/<session>.jsonl (measured tier)
`)
	return err
}
