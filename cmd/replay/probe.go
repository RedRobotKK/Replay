package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
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
	prior := fs.Int("prior", 0, "a documented floor to test first; 0 uses the compiled table's figure for the model, and -1 disables it")
	confirm := fs.Int("confirm", 2, "agreeing answers required before a boundary is believed")
	execute := fs.Bool("execute", false, "actually send the probes; without this, only the plan is printed")
	yes := fs.Bool("yes", false, "with --execute, skip the confirmation. For scripts that meant it")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if *model == "" {
		return fmt.Errorf("a model is required: replay probe --model claude-opus-5: %w", errUsage)
	}

	// A published figure is a hypothesis worth testing before searching the
	// space that contains it. Taken from the compiled table unless overridden,
	// and refuted by its own probe if wrong.
	seed := *prior
	if seed == 0 {
		seed = cachemodel.DocumentedMinPrefix(*model)
	}
	if seed < 0 {
		seed = 0
	}

	cfg := probe.Config{
		Prior:              seed,
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

	// Confirm before spending. The plan is printed first so the answer is
	// informed rather than reflexive.
	r.Plan(cfg, *model)
	if !confirmSpend(os.Stdin, stdout, fmt.Sprintf("%d billable requests to %s", cfg.MaxProbes, base), *yes) {
		return fmt.Errorf("not confirmed; nothing was sent")
	}

	fmt.Fprintf(stdout, "\nprobing %s at %s\n\n", *model, base)
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
	if g := s.Granularity(); g > 1 {
		fmt.Fprintf(out, "granularity  writes land on %d-token blocks (inferred from a GCD, not measured)\n", g)
	} else if g == 1 {
		// A GCD of one is the absence of a finding, not a finding of one-token
		// blocks. Printing it as though a block size had been established
		// would be a claim the evidence does not support.
		fmt.Fprintf(out, "granularity  no common block size in what was cached; none inferred\n")
	}
	if s.Stalled() {
		fmt.Fprintf(out, "\nThe bracket stopped narrowing before reaching the resolution asked for,\n"+
			"and further probes would buy nothing. Two things can cause that and this run\n"+
			"cannot tell them apart: the provider rounding a prefix up to a block, or the\n"+
			"gap between the prefix size asked for and the size it actually became. Either\n"+
			"way the bracket above is as tight as this method reaches.\n")
	}
	if s.StoppedEarly() {
		fmt.Fprintf(out, "\nThe budget ran out before the bracket reached the resolution asked for, so it\n"+
			"is wider than requested. Raise --max-probes to narrow it.\n")
	}
	fmt.Fprintf(out, "\nThis is a bracket, not a value. The exact floor inside it was never tested,\n"+
		"and reporting a point would claim precision the probes did not buy.\n")
}

// confirmSpend asks before creating billable traffic.
//
// Only the whole word "yes" proceeds. A bare "y" is excluded on purpose: it is
// one keystroke from a reflex, and this spends the operator's money at their
// provider. Typing the word is the point of asking.
//
// End of input is a refusal, never consent. A pipe, a cron job or a CI step
// closes stdin immediately, and reading that as agreement would make every
// unattended invocation spend money. `--yes` is how a script says it meant it,
// and with it nothing is read and nothing is printed — a script that passed
// --yes precisely so it would not be asked must not then hang on a read.
func confirmSpend(in io.Reader, out io.Writer, what string, yes bool) bool {
	if yes {
		return true
	}
	fmt.Fprintf(out, "\nThis will send %s. Type yes to continue: ", what)

	buf := make([]byte, 64)
	n, _ := in.Read(buf)
	answer := strings.ToLower(strings.TrimSpace(string(buf[:n])))
	if answer == "yes" {
		return true
	}
	if answer == "" {
		fmt.Fprintf(out, "\nNo answer, so nothing was sent. Pass --yes to run unattended.\n")
		return false
	}
	fmt.Fprintf(out, "\nNot confirmed, so nothing was sent. Only \"yes\" proceeds; pass --yes to run unattended.\n")
	return false
}
