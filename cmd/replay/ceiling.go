package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// replay ceiling — what a cache-blind budget stops you at.
//
// The finding lived in internal/cachemodel as a tested pure function that
// nothing could run. This is the surface. It walks the reader's own
// transcripts, folds every request into a CeilingEffect, and prints the
// sentence for their billing state.
//
// It derives nothing. Both dollar figures come from the same CostUSD and
// BlindCostUSD the library's tests already pin, over the usage the provider
// reported, so the command cannot drift from the arithmetic it presents.
//
// The blind rate defaults to $5/MTok because that is the flat rate the measured
// case used (docs/evidence/qm-budget-2026-09-08.md). It is a knob rather than a
// constant because the point of the finding is that the rate is NOT where the
// error lives — the cache is — and a reader who disbelieves that should be able
// to move the rate and watch the ratio barely change.
func runCeiling(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("ceiling", flag.ContinueOnError)
	fs.SetOutput(stderr)
	metered := fs.Bool("metered", false, "you are billed per token: report the ratio and where a ceiling halts you")
	subscription := fs.Bool("subscription", false, "you are on a Pro/Max/Team/Enterprise seat: report tokens, not dollars")
	ceiling := fs.Float64("day-ceiling", 0, "a daily spend ceiling in dollars; report where cache-blind arithmetic halts you under it (0 = do not compute a halt point)")
	flatRate := fs.Float64("flat-rate", 5.0, "the flat $/MTok a cache-blind budget prices every token at")
	asJSON := fs.Bool("json", false, "emit the figures as JSON")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}

	// The basis is refused rather than guessed. A bare first-party model id is
	// emitted by an API key and a subscription alike, so nothing in a
	// transcript settles which the reader is — and the two get opposite
	// reports. Guessing would hand a bill to whichever half is wrong.
	if *metered && *subscription {
		return fmt.Errorf("pass one of --metered or --subscription, not both: you are billed one way "+
			"or the other and the report differs: %w", errUsage)
	}
	var basis cachemodel.Basis
	switch {
	case *metered:
		basis = cachemodel.BasisMetered
	case *subscription:
		basis = cachemodel.BasisSubscription
	default:
		return fmt.Errorf("say how you are billed: --metered if you pay per token, --subscription "+
			"for a Pro/Max/Team/Enterprise seat. A model id alone cannot tell the two apart, and "+
			"the report is different for each: %w", errUsage)
	}

	paths := fs.Args()
	if len(paths) == 0 {
		// The binary knows where Claude Code writes. Demanding the path is the
		// funnel dying at step one, the same reasoning as runCost.
		home, _ := os.UserHomeDir()
		roots := defaultTranscriptRoots(home)
		if len(roots) == 0 {
			return fmt.Errorf("no transcripts found, so there is nothing to measure. NOT MEASURED: %w", errUsage)
		}
		_, _ = fmt.Fprintf(stderr, "reading %s\n", roots[0])
		paths = roots
	}

	files, err := transcriptFiles(paths)
	if err != nil {
		return err
	}

	var e cachemodel.CeilingEffect
	err = forEachSession(files, func(_ string, session *transcript.Session, _ *analysis.LaneReport, err error) error {
		if err != nil || session == nil {
			return nil
		}
		for _, lane := range session.Lanes {
			for _, r := range lane.Requests {
				// Add prices both sides; AddTokens counts allowance even for
				// unpriced models, because a model missing from the table still
				// spent a subscriber's quota.
				e.Add(r.Model, r.Usage, *flatRate)
				e.AddTokens(r.Usage)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if *asJSON {
		ratio, measured := e.Ratio()
		out := map[string]any{
			"schema":   "replay.ceiling.v1",
			"basis":    basisString(basis),
			"requests": e.Requests,
			"unpriced": e.Unpriced,
			"measured": measured,
			"note":     e.Note(basis, *ceiling),
		}
		if measured {
			out["ratio"] = ratio
			out["correctUsd"] = e.CorrectUSD
			out["blindUsd"] = e.BlindUSD
			if stop, ok := e.StopsAt(*ceiling); ok {
				out["haltsAtUsd"] = stop
			}
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s\n", b)
		return err
	}

	_, err = fmt.Fprintf(stdout, "%s\n", e.Note(basis, *ceiling))
	return err
}

// basisString names a basis for the JSON, so a consumer reads "metered" rather
// than an integer whose meaning lives in another package.
func basisString(b cachemodel.Basis) string {
	switch b {
	case cachemodel.BasisMetered:
		return "metered"
	case cachemodel.BasisSubscription:
		return "subscription"
	default:
		return "unknown"
	}
}
