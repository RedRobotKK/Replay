package tui

import (
	"fmt"
	"strings"
)

// What to change, read from this machine.
//
// This screen used to fall through to Outcome(), the canned illustration the
// dispatch renders for any key without a case. Nothing was broken and nothing
// was dishonest — it carried the example-data notice — but `replay advise`
// answers the same question from real transcripts, so the surface the installer
// opens was the one place the analysis did not run.
//
// The rule measured.go states is what shapes the three states below: a screen is
// Measured only when every figure came from somewhere a reader could go and
// check. Two of those states look alike and are not.
//
//	suggestions found    Measured
//	sessions read, none  Measured, and says how many found nothing
//	no sessions at all   Unavailable, "not measured here"
//
// "Example data" says these numbers describe nobody. "Not measured here" says
// there were no numbers. Only one of them is about the reader, and someone who
// has just installed this needs to know which.

// AdviceRow is one suggestion, flattened to what the screen draws.
//
// A local type rather than advisor.Suggestion, for the same reason Machine is
// local: this package renders and does no I/O, and a screen that imports the
// analysis is a screen that can only be tested by running it. The caller maps
// one to the other, which is also where a field that stops being measured has to
// be noticed.
type AdviceRow struct {
	Title  string
	Action string
	// Sessions is how many carried this target above threshold, and Share its
	// mean share of prompt tokens across them.
	Sessions     int
	Share        float64
	PromptTokens int
	// PredictedShare is what the suggestion expects to remove per session.
	// Zero means it was not predicted, which is printed as such rather than
	// as 0.0%: a prediction of nothing and no prediction are different claims.
	PredictedShare float64
	Estimated      bool
	Status         string
}

// AdviseScreen renders the ranked suggestions for a corpus.
//
// sessions is how many were read, and it is the parameter that separates an
// empty result from an absent one. Passing the rows alone would collapse both
// into "no rows", which is the distinction this screen exists to keep.
// selMarker is the cursor. Two cells so it reserves the same width on every
// row, selected or not, and the columns after it cannot shift as the reader
// moves — the defect TestCL1 exists to catch, arriving through the front door.
const selMarker = "\u25b8 "

// AdviseScreen renders the findings with no selection, for callers that only
// want the list.
func AdviseScreen(rows []AdviceRow, sessions int) Screen {
	return adviseScreen(rows, sessions, -1)
}

// AdviseScreenAt renders with one finding selected.
//
// Triage is the point of this screen: a reader picks a finding and marks it
// applied or dismissed. Selection comes first because a reader who cannot
// point at a row cannot be asked what to do about it. The index is clamped
// rather than validated — this is exported, and a panic mid-keystroke takes
// the whole TUI down.
func AdviseScreenAt(rows []AdviceRow, sessions, at int) Screen {
	if len(rows) > 0 {
		if at < 0 {
			at = 0
		}
		if at >= len(rows) {
			at = len(rows) - 1
		}
	}
	return adviseScreen(rows, sessions, at)
}

func adviseScreen(rows []AdviceRow, sessions, at int) Screen {
	sc := Screen{Key: 'a', Title: "advise", From: Measured}
	lines := make([]string, 0, BudgetRows)
	lines = append(lines, header("advise"), "")

	if sessions == 0 {
		sc.From = Unavailable
		// Painted like every other branch. The first version coloured only the
		// populated path, which passed here and failed on all four CI platforms:
		// a runner has no transcripts, so it takes this branch, and TestCL5
		// found a screen that emits no colour at all. Local data made the test
		// kinder than the machine it has to run on.
		lines = append(lines,
			paint(Warn, Banner(Unavailable, "no sessions were read, so nothing was ranked")), "",
			"  Run an agent, then come back. "+paint(Accent, "replay doctor")+" says what is visible.", "")
		sc.Lines = lines
		return sc
	}

	if len(rows) == 0 {
		// A result, not an absence. Sessions were read and none crossed a
		// threshold; reporting that as "no data" would throw away the only
		// thing the run established.
		lines = append(lines,
			paint(Good, fmt.Sprintf("  Nothing worth changing across %d transcript(s).", sessions)), "",
			paint(Faint, "  Every target measured came in under the threshold."), "")
		sc.Lines = lines
		return sc
	}

	// Transcripts, not sessions. The caller counts once per file and a session
	// writes one file per agent lane, so this number is an order of magnitude
	// above the session count doctor reports in the same shell. Naming it
	// "sessions" is the conflation calibration-corpus-2026-09-06.md retracted in
	// public — 1450 transcripts from 78 sessions — corrected in the evidence and
	// left live here.
	//
	// Only the top few are shown. 140 findings rendered 704 lines into a 24-row
	// terminal, so everything past the third scrolled away unread, and a total
	// is not something a reader can act on. The rest stay in `replay advise`
	// and advice.json.
	// Built until it stops fitting, rather than estimated.
	//
	// The first version fixed four findings on screen; the second estimated six
	// lines each. Both were constants pretending to be measurements: a finding
	// costs five lines when its action fits on one and six when it wraps, so
	// six overran on short actions and under-filled on long ones. The recorded
	// demo shows the fourth finding clipped by the footer.
	//
	// The rows are laid out one at a time and the loop stops before the next
	// would overflow. That cannot drift when the action text, the width or the
	// chrome changes, because it is not predicting any of them.
	budget := Body() - 3 // header, its blank, and the key legend
	if len(rows) > 0 {
		budget -= 2 // the "N more" note and the blank above the legend
	}
	// The window starts at the selection so the cursor is always on screen,
	// then walks back to fill the space above it when there is room.
	first := 0
	if at > 0 {
		first = at
	}
	if first >= len(rows) && len(rows) > 0 {
		first = len(rows) - 1
	}

	var body []string
	shownN := 0
	for i := first; i < len(rows); i++ {
		block := adviseBlock(rows[i], i, at)
		if len(body)+len(block) > budget && shownN > 0 {
			break
		}
		body = append(body, block...)
		shownN++
	}
	// Backfill upwards, so a selection near the end does not leave the top of
	// the screen empty.
	for i := first - 1; i >= 0; i-- {
		block := adviseBlock(rows[i], i, at)
		if len(body)+len(block) > budget {
			break
		}
		body = append(block, body...)
		first = i
		shownN++
	}

	head := fmt.Sprintf("  %d change(s) worth making, across %d transcript(s)",
		len(rows), sessions)
	if shownN < len(rows) {
		head = fmt.Sprintf("  Showing %d of %d changes worth making, across %d transcript(s)",
			shownN, len(rows), sessions)
	}
	lines = append(lines, head, "")
	lines = append(lines, body...)

	// Chrome is shed rather than allowed to push a finding off the screen.
	//
	// On a very short terminal one finding plus a header, a count and a key
	// legend does not fit, and something has to go. The finding is what the
	// reader came for, so the legend goes first and the count second — a reader
	// who cannot see a finding cannot use a key that acts on it.
	if shownN < len(rows) {
		if len(lines)+1 <= Body() {
			lines = append(lines,
				paint(Faint, fmt.Sprintf("  %d more in `replay advise`, ranked the same way.",
					len(rows)-shownN)))
		}
	}
	if at >= 0 && len(rows) > 0 && len(lines)+2 <= Body() {
		lines = append(lines, "",
			paint(Faint, "  j/k move   a mark applied   x dismiss   enter evidence"))
	}
	// Last resort, and it drops whole findings rather than cutting one open.
	//
	// The first version sliced to Body() and landed inside a block: the
	// recorded demo showed a fourth finding's title with its evidence, saving
	// and action all missing, which reads as a rendering fault rather than a
	// screen that ran out of room. A partial finding is worse than one fewer,
	// because the reader cannot tell which they are looking at.
	for len(lines) > Body() && Body() > 0 {
		cut := lastBlockStart(lines)
		if cut <= 0 {
			lines = lines[:Body()]
			break
		}
		lines = lines[:cut]
	}
	sc.Rows = len(rows)
	sc.Lines = lines
	return sc
}

// fitTo cuts a string the analysis wrote to the room the terminal has.
//
// Truncated rather than wrapped, and the reason is the wrapper: these rows carry
// a label and a value separated by two spaces, which the output boundary reads
// as a column layout and correctly refuses to fold. So a line too long here is
// broken mid-word by the terminal instead, and cutting it deliberately is the
// only way it stays a table.
func fitTo(s string, room int) string {
	if room < 2 || VisibleLen(s) <= room {
		return s
	}
	return truncateVisible(s, room-1) + string(truncationMark)
}

// AdviceRowsFit reports whether every line sits inside the terminal.
//
// Exported so the caller can assert it on real data: the fixtures a test uses
// are short by construction, and a real suggestion title is whatever the
// analysis produced.
func AdviceRowsFit(s Screen) bool {
	for _, l := range s.Lines {
		if VisibleLen(l) > Cols() {
			return false
		}
	}
	return !strings.Contains(strings.Join(s.Lines, "\n"), "example data")
}

// wrapAction folds an action to the width, breaking on spaces.
//
// Two lines at most. An action that needs a third is an action nobody reads on
// a screen, and the full text is in `replay advise` and advice.json either way;
// the last line is cut so a runaway string cannot push the list off the
// terminal, which is the failure this screen already had once.
func wrapAction(action string, width int) []string {
	if width < 20 {
		return []string{fitTo(action, width)}
	}
	if VisibleLen(action) <= width {
		return []string{action}
	}
	cut := width
	for cut > 0 && action[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		return []string{fitTo(action, width)}
	}
	return []string{action[:cut], fitTo(strings.TrimSpace(action[cut:]), width)}
}

// AdviceDetail is what enter opens on a selected finding.
//
// The list ranks; this says why. A finding reads "Bash results are 30% of
// prompt tokens", and the reader's next question is always which sessions, how
// much, and how it was measured — ranking without that asks somebody to act on
// a number whose provenance they cannot see, which this repository refuses
// everywhere else.
//
// It carries no figure the list does not already have. That is deliberate: a
// detail screen that computes its own numbers is a second answer to the same
// question, and the two drift.
func AdviceDetail(r AdviceRow) Screen {
	sc := Screen{Title: "finding", From: Measured}
	lines := []string{
		"", "  " + paint(Strong, fitTo(r.Title, Cols()-2)), "",
	}

	lines = append(lines, "  "+paint(Faint, cell("seen in", 12))+
		fmt.Sprintf("%d transcript(s)", r.Sessions))
	lines = append(lines, "  "+paint(Faint, cell("cost", 12))+
		fmt.Sprintf("%s prompt tokens, %.0f%% of the corpus", commas(r.PromptTokens), r.Share*100))

	// An absence is a result. Rendering 0.0%% would dress it as a modest win,
	// which is the distinction measured.go exists to hold.
	saving := "not predicted on this corpus"
	style := Faint
	if r.PredictedShare > 0 {
		saving = fmt.Sprintf("%.1f%% of prompt tokens", r.PredictedShare*100)
		style = Good
		if r.Estimated {
			saving += " (estimated from the byte-to-token fit)"
		}
	}
	lines = append(lines, "  "+paint(Faint, cell("if applied", 12))+paint(style, saving))
	lines = append(lines, "  "+paint(Faint, cell("status", 12))+r.Status, "")

	if r.Action != "" {
		lines = append(lines, "  "+paint(Faint, "what to do"), "")
		for _, part := range wrapAction(r.Action, Cols()-6) {
			lines = append(lines, "    "+part)
		}
		lines = append(lines, "")
	}

	lines = append(lines,
		paint(Faint, "  a mark applied   x dismiss   esc back to the list"))
	sc.Lines = lines
	return sc
}

// adviseBlock renders one finding, and is the unit the layout measures.
//
// Separated so the screen can lay rows out until they stop fitting rather than
// predicting how tall each will be. That prediction was wrong twice: a fixed
// four findings, then a fixed six lines each, when the real cost is five when
// the action fits on one line and six when it wraps. Asking the block how tall
// it is cannot drift as the text, the width or the chrome changes.
func adviseBlock(r AdviceRow, i, at int) []string {
	num := fmt.Sprintf("%d", i+1)
	mark := strings.Repeat(" ", VisibleLen(selMarker))
	title := Strong
	if i == at {
		mark = paint(Accent, selMarker)
		title = Accent
	}
	prefix := 4 + len(num) + VisibleLen(selMarker)
	out := []string{fmt.Sprintf("  %s%s  %s",
		mark, paint(Faint, num), paint(title, fitTo(r.Title, Cols()-prefix)))}

	out = append(out, "     "+paint(Faint, cell("evidence", 10))+
		fitTo(fmt.Sprintf("%d transcript(s), %s prompt tokens",
			r.Sessions, commas(r.PromptTokens)), Cols()-15))

	pred, style := "not predicted on this corpus", Faint
	if r.PredictedShare > 0 {
		pred = fmt.Sprintf("%.1f%% of prompt tokens", r.PredictedShare*100)
		style = Good
		if r.Estimated {
			pred += " (estimated)"
		}
	}
	row := "     " + paint(Faint, cell("saving", 10))
	if Cols()-15-VisibleLen(r.Status)-9 >= 12 {
		row += paint(style, cell(pred, Cols()-15-VisibleLen(r.Status)-9)) +
			paint(Faint, cell("status", 8)) + r.Status
	} else {
		row += paint(style, fitTo(pred, Cols()-15))
	}
	out = append(out, row)

	if r.Action != "" {
		for j, part := range wrapAction(r.Action, Cols()-15) {
			label := "do"
			if j > 0 {
				label = ""
			}
			out = append(out, "     "+paint(Faint, cell(label, 10))+part)
		}
	}
	return append(out, "")
}

// lastBlockStart is the index of the final finding's first line.
//
// Findings are separated by a blank, so the line after the last blank before
// the trailing chrome begins the block to drop. Returns 0 when there is no
// block boundary to cut at, which the caller treats as "nothing safe to drop".
func lastBlockStart(lines []string) int {
	// Walk back over the trailing chrome to the last content line.
	end := len(lines) - 1
	for end > 0 && strings.TrimSpace(StripSGR(lines[end])) == "" {
		end--
	}
	for i := end; i > 0; i-- {
		if strings.TrimSpace(StripSGR(lines[i])) == "" {
			return i + 1
		}
	}
	return 0
}
