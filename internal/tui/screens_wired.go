package tui

import "fmt"

// The last four screens that described nobody.
//
// context, guards, model and safe all fell through to Outcome(), the canned
// illustration the dispatch renders for any key with no case. advise was wired
// first and is the worked example; these follow it exactly, including the three
// states, because the distinction between them is the whole point:
//
//	data                Measured
//	sessions, none      Measured, and says how many found nothing
//	no sessions         Unavailable, "not measured here", never Example
//
// Every branch paints. A screen that colours only its populated path renders a
// colourless frame on a machine with no transcripts, which is every CI runner,
// and that is how TestCL5 failed on four platforms at once while passing here.

// ContextRow is one tool's contribution to a session's context.
type ContextRow struct {
	Label       string
	Tokens      int
	Share       float64
	Occurrences int
}

// TrimSummary is what a byte cap would have removed.
type TrimSummary struct {
	CapBytes            int
	Blocks              int
	RemovedBytes        int
	RemovedPromptTokens int
}

// ModelRow is one model's share of the corpus.
type ModelRow struct {
	Model string
	Turns int
	Share float64
}

// unavailable builds the no-corpus frame every wired screen shares.
func unavailable(cmd, why, next string) Screen {
	return Screen{Key: 0, Title: cmd, From: Unavailable, Lines: []string{
		header(cmd), "",
		paint(Warn, Banner(Unavailable, why)), "",
		"  " + fitTo(next, Cols()-2), "",
	}}
}

// ContextScreen ranks what entered the context, by tool.
func ContextScreen(rows []ContextRow, sessions int) Screen {
	if sessions == 0 {
		return unavailable("context", "no sessions were read, so nothing was attributed",
			"Run an agent, then come back. "+paint(Accent, "replay doctor")+" says what is visible.")
	}
	sc := Screen{Key: 'x', Title: "context", From: Measured}
	lines := []string{header("context"), ""}
	if len(rows) == 0 {
		lines = append(lines,
			paint(Good, fmt.Sprintf("  Nothing attributable across %d session(s).", sessions)), "",
			paint(Faint, "  No block carried a label this build understands."), "")
		sc.Lines = lines
		return sc
	}
	lines = append(lines, fmt.Sprintf("  What entered the context across %d session(s)", sessions), "")
	lines = append(lines, "  "+paint(Faint, cell("source", 26)+cell("share", 8)+cell("tokens", 12)+"blocks"))
	for _, r := range rows {
		lines = append(lines, "  "+
			paint(Strong, cell(fitTo(r.Label, 24), 26))+
			cell(fmt.Sprintf("%.1f%%", r.Share*100), 8)+
			cell(commas(r.Tokens), 12)+
			fmt.Sprintf("%d", r.Occurrences))
	}
	lines = append(lines, "", paint(Faint, "  Share is of what was attributed, not of the context window."))
	sc.Lines = lines
	return sc
}

// GuardsScreen shows spend caps drawn from this machine's own spread.
//
// The lines arrive already rendered, because the fence arithmetic and the
// wording that explains it live together on the command side and splitting them
// would let the two drift.
func GuardsScreen(advice []string, sessions int) Screen {
	if sessions == 0 {
		return unavailable("guards", "no sessions were read, so no spread to draw a cap from",
			"A cap from one session is a cap from an accident. "+paint(Accent, "replay cost")+" shows what is there.")
	}
	sc := Screen{Key: 'g', Title: "guards", From: Measured}
	lines := []string{header("guards"), ""}
	if len(advice) == 0 {
		lines = append(lines,
			paint(Good, fmt.Sprintf("  No cap suggested from %d session(s).", sessions)), "",
			paint(Faint, "  Too few sessions, or a spread too flat to fence."), "")
		sc.Lines = lines
		return sc
	}
	lines = append(lines, fmt.Sprintf("  Caps drawn from %d session(s) on this machine", sessions), "")
	for _, l := range advice {
		lines = append(lines, fitTo(l, Cols()))
	}
	lines = append(lines, "", paint(Faint, "  Printed only. Nothing here is written to your configuration."))
	sc.Lines = lines
	return sc
}

// SafeScreen shows what a byte cap on tool output would have removed.
func SafeScreen(t TrimSummary, sessions int) Screen {
	if sessions == 0 {
		return unavailable("safe", "no sessions were read, so no cap could be scored",
			"Run an agent, then come back. "+paint(Accent, "replay trim --cap 2000")+" scores a cap.")
	}
	sc := Screen{Key: 's', Title: "safe", From: Measured}
	lines := []string{header("safe"), ""}
	lines = append(lines, fmt.Sprintf("  A %s-byte cap on tool results, over %d session(s)",
		commas(t.CapBytes), sessions), "")
	if t.Blocks == 0 {
		lines = append(lines,
			paint(Good, "  Nothing was over the cap."), "",
			paint(Faint, "  No tool result exceeded it, so this cap would remove nothing."), "")
		sc.Lines = lines
		return sc
	}
	lines = append(lines,
		"  "+paint(Faint, cell("over the cap", 18))+paint(Strong, commas(t.Blocks)+" block(s)"),
		"  "+paint(Faint, cell("removable", 18))+commas(t.RemovedBytes)+" bytes",
		"  "+paint(Faint, cell("prompt tokens", 18))+paint(Good, commas(t.RemovedPromptTokens)+" once resending is counted"),
		"",
		paint(Faint, fitTo("  Removing a result the agent later needs costs a re-read, and that is counted.", Cols())))
	sc.Lines = lines
	return sc
}

// ModelScreen reports what the corpus ran on, and what a switch would compare against.
//
// Without a target the screen is Unavailable rather than Example: naming a model
// the reader did not choose and pricing a switch to it would be inventing the
// question as well as the answer.
func ModelScreen(target string, rows []ModelRow, sessions int) Screen {
	if sessions == 0 {
		return unavailable("model", "no sessions were read, so there is nothing to switch",
			"Run an agent, then come back. "+paint(Accent, "replay route --to <model>")+" compares one.")
	}
	sc := Screen{Key: 'm', Title: "model", From: Measured}
	lines := []string{header("model"), ""}
	if len(rows) == 0 {
		lines = append(lines,
			paint(Good, fmt.Sprintf("  No model named in %d session(s).", sessions)), "",
			paint(Faint, "  The transcripts carry no model id this build recognises."), "")
		sc.Lines = lines
		return sc
	}
	lines = append(lines, fmt.Sprintf("  What %d session(s) actually ran on", sessions), "")
	lines = append(lines, "  "+paint(Faint, cell("model", 34)+cell("turns", 9)+"share"))
	for _, r := range rows {
		lines = append(lines, "  "+
			paint(Strong, cell(fitTo(r.Model, 32), 34))+
			cell(fmt.Sprintf("%d", r.Turns), 9)+
			fmt.Sprintf("%.1f%%", r.Share*100))
	}
	lines = append(lines, "")
	if target == "" {
		lines = append(lines, paint(Faint, fitTo(
			"  No target chosen. `replay route --to <model>` prices a switch against these.", Cols())))
	} else {
		lines = append(lines, paint(Accent, fitTo("  Comparing against "+target, Cols())))
	}
	sc.Lines = lines
	return sc
}
