package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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
	model := fs.String("model", "", "model id to measure; repeat with commas to measure several at once")
	minTok := fs.Int("min", 0, "smallest prefix size the floor could be")
	maxTok := fs.Int("max", 65536, "largest prefix size the floor could be")
	resolution := fs.Int("resolution", 512, "how narrow a bracket is narrow enough, in tokens")
	relative := fs.Float64("relative", 0, "stop within this fraction of the answer instead of a fixed token width")
	// 16 is not arbitrary: the defaults below need 7 bisection decisions to
	// resolve 65,536 tokens to 512, and 2 confirmations each makes 14. The
	// default budget must clear its own default resolution, or the first thing
	// every user sees is the tool warning about a configuration it chose.
	maxProbes := fs.Int("max-probes", 16, "how many billable requests this run may make")
	candidates := fs.String("candidates", "512,1024,2048,4096", "plausible floors to test before searching between them; empty to disable")
	prior := fs.Int("prior", 0, "a documented floor to test first; 0 uses the compiled table's figure for the model, and -1 disables it")
	confirm := fs.Int("confirm", 2, "agreeing answers required before a boundary is believed")
	trend := fs.Bool("trend", false, "read the recorded series and report what has changed; sends nothing")
	maxAge := fs.Duration("max-age", 0, "skip probing when a reading for this model is younger than this, and print it instead")
	record := fs.String("record", "", "append the reading to a measurement series (default ~/.replay/measurements.jsonl; \"-\" for none)")
	execute := fs.Bool("execute", false, "actually send the probes; without this, only the plan is printed")
	yes := fs.Bool("yes", false, "with --execute, skip the confirmation. For scripts that meant it")
	contributeTo := fs.String("contribute", "", "build a submission file for this campaign from the recorded reading; writes a file, sends nothing")
	contributeDir := fs.String("contribute-dir", ".", "where --contribute writes its file")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if *trend {
		return reportTrend(seriesPath(*record), stdout)
	}
	if *contributeTo != "" {
		if *model == "" {
			return fmt.Errorf("a model is required: replay probe --model claude-opus-5 --contribute <campaign>: %w", errUsage)
		}
		return contribute(*contributeTo, *contributeDir, *model, seriesPath(*record), stdout)
	}
	if *model == "" {
		return fmt.Errorf("a model is required: replay probe --model claude-opus-5: %w", errUsage)
	}
	// A reading already in the series answers the question a probe would, and
	// a probe costs real money at the provider.
	if *maxAge > 0 {
		if r, ok := probe.RecentReading(seriesPath(*record), *model, *maxAge); ok {
			_, _ = fmt.Fprintf(stdout, "%s was measured %s: floor above %d, at most %d tokens.\n"+
				"Nothing was sent. Drop --max-age to measure it again.\n",
				r.Model, r.TakenAt, r.Above, r.AtMost)
			return nil
		}
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

	var cands []int
	for _, part := range strings.Split(*candidates, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 {
			return fmt.Errorf("--candidates takes positive whole numbers: %q: %w", part, errUsage)
		}
		cands = append(cands, n)
	}

	cfg := probe.Config{
		Candidates:         cands,
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
		_, _ = fmt.Fprintf(stdout, "\nNothing was sent. Add --execute to run it.\n")
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

	_, _ = fmt.Fprintf(stdout, "\nprobing %s at %s\n\n", *model, base)
	s, err := r.Run(cfg, *model)
	if s != nil {
		reportProbe(stdout, s)
		reportProvenance(stdout, *model, r.Provenance())

		// Store it. A reading that exists only in a terminal is not a series,
		// and the series is the part that cannot be reconstructed later: a
		// floor is a fact anyone can copy the day it is published, while "it
		// changed on this date" needs someone to have been measuring before
		// the change.
		if path := seriesPath(*record); path != "" {
			reading := probe.ReadingFrom(*model, cachemodel.DocumentedMinPrefix(*model), s, r.Provenance(), *confirm)
			if werr := probe.AppendReading(path, reading); werr != nil {
				_, _ = fmt.Fprintf(stderr, "the reading could not be recorded: %v\n", werr)
			} else {
				_, _ = fmt.Fprintf(stdout, "\nrecorded to %s\n", path)
			}
		}
	}
	return err
}

// reportProbe states what was established, and refuses to state more.
func reportProbe(out io.Writer, s *probe.Search) {
	lo, hi := s.Bracket()
	_, _ = fmt.Fprintf(out, "\n%d billable decision(s)\n", s.Probes())

	if n := s.Inconclusive(); n > 0 {
		_, _ = fmt.Fprintf(out, "%d probe(s) read an existing cache entry and decided nothing. They were\n"+
			"billed anyway, and that many of them says more about the cache than the floor.\n", n)
	}
	for _, a := range s.Anomalies() {
		_, _ = fmt.Fprintf(out, "\nanomaly      %s at %d tokens: cached %d time(s), did not cache %d\n",
			a.Kind, a.Size, a.Wrote, a.DidNotWrite)
	}

	switch {
	case s.NonDeterministic():
		_, _ = fmt.Fprintf(out, "\nThe same prefix cached on one request and not the next. There is no single\n"+
			"floor to report, and that is the finding: something else is moving the\n"+
			"boundary — block granularity, a per-account difference, or a change during\n"+
			"the run. Averaging it would hide the only interesting thing here.\n")
		return
	case s.Contradicted():
		_, _ = fmt.Fprintf(out, "\nA prefix cached at a size another prefix failed to cache at. No single floor\n"+
			"explains that, so none is reported.\n")
		return
	}

	_, _ = fmt.Fprintf(out, "floor        above %d, at most %d tokens\n", lo, hi)
	if g := s.Granularity(); g > 1 {
		_, _ = fmt.Fprintf(out, "granularity  writes land on %d-token blocks (inferred from a GCD, not measured)\n", g)
	} else if g == 1 {
		// A GCD of one is the absence of a finding, not a finding of one-token
		// blocks. Printing it as though a block size had been established
		// would be a claim the evidence does not support.
		_, _ = fmt.Fprintf(out, "granularity  no common block size in what was cached; none inferred\n")
	}
	if s.Stalled() {
		_, _ = fmt.Fprintf(out, "\nThe bracket stopped narrowing before reaching the resolution asked for,\n"+
			"and further probes would buy nothing. Two things can cause that and this run\n"+
			"cannot tell them apart: the provider rounding a prefix up to a block, or the\n"+
			"gap between the prefix size asked for and the size it actually became. Either\n"+
			"way the bracket above is as tight as this method reaches.\n")
	}
	if s.StoppedEarly() {
		_, _ = fmt.Fprintf(out, "\nThe budget ran out before the bracket reached the resolution asked for, so it\n"+
			"is wider than requested. Raise --max-probes to narrow it.\n")
	}
	_, _ = fmt.Fprintf(out, "\nThis is a bracket, not a value. The exact floor inside it was never tested,\n"+
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
	_, _ = fmt.Fprintf(out, "\nThis will send %s. Type yes to continue: ", what)

	buf := make([]byte, 64)
	n, _ := in.Read(buf)
	answer := strings.ToLower(strings.TrimSpace(string(buf[:n])))
	if answer == "yes" {
		return true
	}
	if answer == "" {
		_, _ = fmt.Fprintf(out, "\nNo answer, so nothing was sent. Pass --yes to run unattended.\n")
		return false
	}
	_, _ = fmt.Fprintf(out, "\nNot confirmed, so nothing was sent. Only \"yes\" proceeds; pass --yes to run unattended.\n")
	return false
}

// reportProvenance says what actually answered, which is not what was asked.
//
// A floor measured against `claude-opus-5` is a floor measured against
// whatever snapshot that alias resolved to, on whatever tier, routed wherever.
// Two readings taken a month apart are only comparable if each says which. A
// dated series with no provenance is a list of numbers.
func reportProvenance(out io.Writer, asked string, p probe.Provenance) {
	_, _ = fmt.Fprintf(out, "\nmeasured\n")
	_, _ = fmt.Fprintf(out, "  asked for    %s\n", asked)
	if p.ResolvedModel != "" {
		_, _ = fmt.Fprintf(out, "  answered by  %s\n", p.ResolvedModel)
	} else {
		_, _ = fmt.Fprintf(out, "  answered by  (the provider did not name a snapshot)\n")
	}
	if p.ServiceTier != "" {
		_, _ = fmt.Fprintf(out, "  tier         %s\n", p.ServiceTier)
	}
	if p.Geo != "" {
		_, _ = fmt.Fprintf(out, "  geography    %s\n", p.Geo)
	}
	_, _ = fmt.Fprintf(out, "  taken        %s\n", time.Now().UTC().Format(time.RFC3339))
	if p.Mixed {
		_, _ = fmt.Fprintf(out, "\n  More than one snapshot, tier or geography answered this run, so the\n"+
			"  bracket above has more than one subject in it. Treat it as two partial\n"+
			"  readings rather than one measurement, and repeat with a pinned model id.\n")
	}
	if p.ResolvedModel != "" && p.ResolvedModel != asked {
		_, _ = fmt.Fprintf(out, "\n  %s is an alias. Pin %s to make this reading reproducible.\n", asked, p.ResolvedModel)
	}
}

// seriesPath resolves where a reading is stored, or "" when recording is off.
func seriesPath(flagValue string) string {
	if flagValue == "-" {
		return ""
	}
	if flagValue != "" {
		return flagValue
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".replay", "measurements.jsonl")
}

// reportTrend reads the series and says what has provably changed.
//
// The pressure on a dated measurement is always toward finding news in it, so
// this declines to find any it cannot support. A change is reported only when
// two brackets cannot both describe one floor; brackets that merely differ are
// both consistent with nothing having happened.
func reportTrend(path string, out io.Writer) error {
	readings, err := probe.LoadSeries(path)
	if err != nil {
		return err
	}
	if len(readings) == 0 {
		_, _ = fmt.Fprintf(out, "No readings yet at %s.\n\n"+
			"A series is worth exactly what has accumulated in it, and nothing can be\n"+
			"backfilled: a floor is a fact anyone can copy today, while the date it\n"+
			"changed needs someone to have been measuring beforehand. Take the first\n"+
			"reading with:\n\n  replay probe --model claude-opus-5 --execute\n", path)
		return nil
	}

	latest := map[string]probe.Reading{}
	count := map[string]int{}
	for _, r := range readings {
		count[r.Model]++
		if cur, ok := latest[r.Model]; !ok || r.TakenAt > cur.TakenAt {
			latest[r.Model] = r
		}
	}
	models := make([]string, 0, len(latest))
	for m := range latest {
		models = append(models, m)
	}
	sort.Strings(models)

	_, _ = fmt.Fprintf(out, "%d reading(s) at %s\n\n", len(readings), path)
	for _, m := range models {
		r := latest[m]
		switch {
		case r.Outcome != "":
			_, _ = fmt.Fprintf(out, "  %-28s %-16s  %d reading(s), last %s\n", m, r.Outcome, count[m], r.TakenAt[:10])
		default:
			bracket := fmt.Sprintf("(%d, %d]", r.Above, r.AtMost)
			doc := ""
			if r.Documented > 0 {
				doc = fmt.Sprintf("  documented %d", r.Documented)
			}
			_, _ = fmt.Fprintf(out, "  %-28s %-16s%s  %d reading(s), last %s\n", m, bracket, doc, count[m], r.TakenAt[:10])
		}
	}

	changes := probe.Changes(readings)
	_, _ = fmt.Fprintf(out, "\n")
	if len(changes) == 0 {
		_, _ = fmt.Fprintf(out, "No floor has provably moved. A change is only reported when two brackets\n"+
			"cannot both be true; brackets that merely differ are both consistent with\n"+
			"nothing having happened.\n")
	} else {
		_, _ = fmt.Fprintf(out, "Changed:\n")
		for _, c := range changes {
			_, _ = fmt.Fprintf(out, "  %s  (%d, %d] -> (%d, %d]  by %s\n",
				c.Model, c.FromAbove, c.FromAtMost, c.ToAbove, c.ToAtMost, c.At[:10])
		}
		_, _ = fmt.Fprintf(out, "\nThe date is when the change was first observed, not when it happened.\n")
	}

	if breaks := probe.MethodBreaks(readings); len(breaks) > 0 {
		_, _ = fmt.Fprintf(out, "\nThe instrument changed between readings, so numbers either side are not\n"+
			"comparable and no change is drawn across them:\n")
		for _, b := range breaks {
			_, _ = fmt.Fprintf(out, "  %s  %s -> %s  at %s\n", b.Model, b.From, b.To, b.At[:10])
		}
	}
	return nil
}
