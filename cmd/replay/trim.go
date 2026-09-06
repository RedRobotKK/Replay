package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// defaultTrimCap is a starting point, not a recommendation. It is large enough
// that only genuinely enormous tool results cross it, which is the only shape
// where trimming beats the provider's own context editing.
const defaultTrimCap = 16384

// `replay trim` scores a per-block byte cap on tool output against real
// sessions, and asks whether the agent later needed what the cap removed.
//
// It is offline and it stays offline. The live trimmer does not ship, for
// reasons that are worth stating because they are not obvious: Go's
// json.Marshal HTML-escapes < > and &, so decode-cut-re-marshal returns a block
// up to six times the cap on HTML, JSX, XML, git conflict markers and shell
// redirects, which destroys the idempotence the whole design rested on.
// Un-trimming a block previously sent trimmed is itself a history edit, so a
// restart without the flag, a changed cap or a second serve on the same ledger
// would corrupt a live session. And trimming before masking splits a secret
// into a prefix that matches no pattern, forwarded in clear, under an operator
// reading "0 secrets masked".
//
// So this command answers the only question worth answering first: on your own
// sessions, in dollars, would a cap have been worth any of that.
func runTrim(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("trim", flag.ContinueOnError)
	fs.SetOutput(stderr)
	capBytes := fs.Int("cap", defaultTrimCap, "per-block byte cap to score")
	asJSON := fs.Bool("json", false, "emit the plan as JSON")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("a transcript file or directory is required: %w", errUsage)
	}
	if *capBytes <= 0 {
		return fmt.Errorf("--cap must be positive: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}

	var total analysis.TrimPlan
	total.CapBytes = *capBytes
	sessions, scored := 0, 0
	_ = forEachSession(files, func(_ string, _ *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || rep == nil || rep.Lane == nil {
			return nil
		}
		sessions++
		plan := analysis.ScoreTrim(rep.Lane, rep.Fit, *capBytes)
		if plan.Blocks == 0 {
			return nil
		}
		scored++
		total.Blocks += plan.Blocks
		total.RemovedBytes += plan.RemovedBytes
		total.RemovedPromptTokens += plan.RemovedPromptTokens
		total.SavedUSD += plan.SavedUSD
		total.SavedInputUSD += plan.SavedInputUSD
		total.Estimated = true
		total.Harms = append(total.Harms, plan.Harms...)
		return nil
	})
	total.Splits = analysis.DeriveSplits(total.Harms)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"schema":   "replay.trim.v1",
			"sessions": sessions,
			"plan":     total,
			"notes":    analysis.ProbeBlindSpots(),
		})
	}

	p := analysis.NewPrinter(stdout)
	p.Printf("Scoring a %d-byte cap on tool results over %d session(s).\n\n", *capBytes, sessions)
	if total.Blocks == 0 {
		p.Printf("No tool result exceeded the cap. Nothing to trim, and nothing to weigh.\n")
		p.Printf("%s", supportLine(describeResult("trim"), stdout))
		return p.Err()
	}
	p.Printf("%d block(s) over the cap, %s bytes removable, %s prompt tokens once resending is counted.\n",
		total.Blocks, formatCount(total.RemovedBytes), formatCount(total.RemovedPromptTokens))
	p.Printf("%s\n\n", trimSummaryLine(total.SavedUSD, total.SavedInputUSD, total.Overstatement()))

	if len(total.Harms) > 0 {
		p.Printf("Harm probe: %d case(s) where the agent later needed removed content.\n", len(total.Harms))
		byKind := map[string]int{}
		for _, h := range total.Harms {
			byKind[h.Kind]++
		}
		for _, k := range []string{analysis.HarmLaterEdit, analysis.HarmReRead, analysis.HarmQuote} {
			if byKind[k] > 0 {
				p.Printf("  %-12s %d\n", k, byKind[k])
			}
		}
	} else {
		p.Printf("Harm probe found nothing, which is a lower bound and not an all-clear.\n")
	}
	for _, s := range total.Splits {
		p.Printf("  %-10s dependencies landed head %.0f%% / middle %.0f%% / tail %.0f%% over %d sample(s)\n",
			s.Tool, s.HeadShare*100, s.MiddleShare*100, s.TailShare*100, s.Samples)
	}
	p.Printf("\n")
	writeTrimNotes(stdout)
	_, _ = io.WriteString(stdout, supportLine(describeResult("trim"), stdout))
	return p.Err()
}

// trimSummaryLine states the saving twice: as what it is, and as what a
// token-share report would have implied. The gap is the reason this command
// took the shape it did.
func trimSummaryLine(savedUSD, savedInputUSD, ratio float64) string {
	return fmt.Sprintf(
		"Worth $%.2f at cache-read prices, which is what a resent byte costs.\n"+
			"Priced as fresh input it would read $%.2f, %.1fx larger and wrong.",
		savedUSD, savedInputUSD, ratio)
}

// writeTrimNotes prints what the report does not and cannot tell you. Every
// line is a way the numbers above would otherwise mislead.
func writeTrimNotes(w io.Writer) {
	for _, l := range analysis.ProbeBlindSpots() {
		_, _ = fmt.Fprintf(w, "%s\n", l)
	}
	_, _ = fmt.Fprintf(w, "\nThis does NOT delay auto-compaction. /v1/messages/count_tokens is not\n"+
		"trimmed and the client's own accounting keeps using untrimmed sizes, so a\n"+
		"trimmed session hits the compaction threshold at the same point. If you want\n"+
		"a longer session, this is not the lever.\n")
	_, _ = fmt.Fprintf(w, "\nTry --context-edit-trigger first. It is provider-sanctioned, excluded from\n"+
		"the history-binding check, invalidates the cache only from the earliest\n"+
		"cleared block, and the provider reports what it did. Trimming beats it only\n"+
		"for a few enormous results, which is exactly what the figures above are for.\n")
}
