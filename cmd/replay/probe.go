package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/RedRobotKK/Replay/internal/probe"
)

// `replay probe` measures a model's caching floor on purpose.
//
// This is the only command that originates a billable request. Everything else
// reads files or forwards what an agent already sent; this creates traffic with
// the operator's credential and spends their money. So it plans by default and
// sends nothing until `--execute` is passed, and it never takes the key as a
// flag: a credential on a command line lands in shell history and in the
// process table, where every other user on the box can read it.
func runProbe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	model := fs.String("model", "", "model id to measure, for example claude-opus-5")
	minTok := fs.Int("min", 0, "smallest prefix size the floor could be")
	maxTok := fs.Int("max", 65536, "largest prefix size the floor could be")
	resolution := fs.Int("resolution", 512, "how narrow a bracket is narrow enough, in tokens")
	relative := fs.Float64("relative", 0, "stop within this fraction of the answer instead of a fixed token width")
	// 16 is not arbitrary: the defaults below need 7 bisection decisions to
	// resolve 65,536 tokens to 512, and 2 confirmations each makes 14. The
	// default budget must clear its own default resolution, or the first thing
	// every user sees is the tool warning about a configuration it chose.
	maxProbes := fs.Int("max-probes", 16, "how many billable requests this run may make")
	confirm := fs.Int("confirm", 2, "agreeing answers required before a boundary is believed")
	execute := fs.Bool("execute", false, "actually send the probes; without this, only the plan is printed")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if *model == "" {
		return fmt.Errorf("a model is required: replay probe --model claude-opus-5: %w", errUsage)
	}

	cfg := probe.Config{
		Min:                *minTok,
		Max:                *maxTok,
		Resolution:         *resolution,
		RelativeResolution: *relative,
		MaxProbes:          *maxProbes,
		Confirm:            *confirm,
	}

	base := os.Getenv("ANTHROPIC_BASE_URL")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	r := &probe.Runner{BaseURL: base, APIKey: os.Getenv("ANTHROPIC_API_KEY"), Out: stdout}

	if !*execute {
		r.Plan(cfg, *model)
		fmt.Fprintf(stdout, "\nNothing was sent. Add --execute to run it.\n")
		return nil
	}
	if r.APIKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is not set in this shell. It is read from the environment " +
			"and never taken as a flag, because a key on a command line is recorded in shell history " +
			"and visible in the process table")
	}

	fmt.Fprintf(stdout, "probing %s at %s\n\n", *model, base)
	s, err := r.Run(cfg, *model)
	if s != nil {
		reportProbe(stdout, s)
	}
	return err
}

// reportProbe states what was established, and refuses to state more.
func reportProbe(out io.Writer, s *probe.Search) {
	lo, hi := s.Bracket()
	fmt.Fprintf(out, "\n%d billable decision(s)\n", s.Probes())

	switch {
	case s.NonDeterministic():
		fmt.Fprintf(out, "\nThe same prefix cached on one request and not the next. There is no single\n"+
			"floor to report, and that is the finding: something else is moving the\n"+
			"boundary — block granularity, a per-account difference, or a change during\n"+
			"the run. Averaging it would hide the only interesting thing here.\n")
		return
	case s.Contradicted():
		fmt.Fprintf(out, "\nA prefix cached at a size another prefix failed to cache at. No single floor\n"+
			"explains that, so none is reported.\n")
		return
	}

	fmt.Fprintf(out, "floor        above %d, at most %d tokens\n", lo, hi)
	if g := s.Granularity(); g > 0 {
		fmt.Fprintf(out, "granularity  writes land on %d-token blocks (inferred from a GCD, not measured)\n", g)
	}
	if s.StoppedEarly() {
		fmt.Fprintf(out, "\nThe budget ran out before the bracket reached the resolution asked for, so it\n"+
			"is wider than requested. Raise --max-probes to narrow it.\n")
	}
	fmt.Fprintf(out, "\nThis is a bracket, not a value. The exact floor inside it was never tested,\n"+
		"and reporting a point would claim precision the probes did not buy.\n")
}
