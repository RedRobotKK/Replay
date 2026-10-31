// Command replay-bench measures what the Replay proxy adds to a request,
// against an in-process mock upstream, with interleaved repeats, spread and
// an environment record. It is a developer tool, not part of the binary.
// ADR-0029 records why it measures the way it does.
//
//	scripts/replay-bench/run.sh                 # full run: 10 repeats
//	go run ./scripts/replay-bench -render FILE  # summarize an earlier result
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// Exit codes.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// execute is Execute, a variable so the command's own handling of flags,
// failures and output can be tested without running the measurement.
var execute = Execute

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("replay-bench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "bench-results", "directory for timestamped result files (never overwritten)")
	repeats := fs.Int("repeats", DefaultConfig().Repeats, "interleaved repeats (the brief asks for at least 10)")
	quick := fs.Bool("quick", false, "the smoke configuration: every scenario, tiny counts; for checking the harness, not for figures")
	gitSHA := fs.String("git-sha", "", "commit being measured (default: from build info)")
	sidecar := fs.String("sidecar", "", "file of shell-captured environment (uptime, top CPU consumers) to embed")
	noisy := fs.String("noisy-label", "", "mark the result noisy with this reason regardless of load")
	render := fs.String("render", "", "render an existing result JSON as Markdown and exit")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *render != "" {
		return renderFile(*render, stdout, stderr)
	}
	cfg := DefaultConfig()
	if *quick {
		cfg = SmokeConfig()
	}
	if *repeats > 0 && !*quick {
		cfg.Repeats = *repeats
	}
	cfg.NoisyLabel = *noisy
	res, err := execute(cfg, *gitSHA, recordedArgs(args, *sidecar))
	if err != nil {
		say(stderr, "replay-bench: %v\n", err)
		return exitError
	}
	if *sidecar != "" {
		side, err := readSidecar(*sidecar)
		if err != nil {
			say(stderr, "replay-bench: sidecar: %v\n", err)
			return exitError
		}
		res.Environment.Sidecar = side
	}
	path, err := WriteResult(*out, time.Now(), res)
	if err != nil {
		say(stderr, "replay-bench: %v\n", err)
		return exitError
	}
	say(stdout, "result: %s\n\n", path)
	if err := Render(stdout, res); err != nil {
		say(stderr, "replay-bench: %v\n", err)
		return exitError
	}
	return exitOK
}

func renderFile(path string, stdout, stderr io.Writer) int {
	b, err := os.ReadFile(path)
	if err != nil {
		say(stderr, "replay-bench: %v\n", err)
		return exitError
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil {
		say(stderr, "replay-bench: %s: %v\n", path, err)
		return exitError
	}
	if err := Render(stdout, r); err != nil {
		say(stderr, "replay-bench: %v\n", err)
		return exitError
	}
	return exitOK
}

// say writes one line of output. A failed write to the terminal has nowhere
// else to be reported, so its error is dropped here, once, for that reason.
func say(w io.Writer, format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }

// sidecarPlaceholder stands in for the -sidecar path in the recorded args:
// run.sh passes a mktemp path, which is local and means nothing elsewhere.
const sidecarPlaceholder = "<sidecar>"

func recordedArgs(args []string, sidecar string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		switch {
		case sidecar != "" && a == sidecar:
			out[i] = sidecarPlaceholder
		case sidecar != "" && a == "-sidecar="+sidecar:
			out[i] = "-sidecar=" + sidecarPlaceholder
		default:
			out[i] = a
		}
	}
	return out
}
