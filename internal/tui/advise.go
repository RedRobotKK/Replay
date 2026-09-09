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
	// Sized to the terminal rather than fixed at four. A finding costs six
	// lines: title, evidence, saving, two of wrapped action, and the blank that
	// separates it from the next. Header, the more-findings note and the key
	// legend take the rest, and Body() has already taken out the frame.
	//
	// Six by measurement, not arithmetic: five rendered 27 lines into a 24-row
	// terminal, because the action wraps and the estimate had counted it once.
	onScreen := (Body() - 4) / 6
	if onScreen < 1 {
		onScreen = 1
	}
	shown, first := rows, 0
	if len(shown) > onScreen {
		// Scroll to keep the selection visible rather than always showing the
		// top: a cursor that moves off the screen is a cursor the reader loses.
		if at >= onScreen {
			first = at - onScreen + 1
			if first+onScreen > len(rows) {
				first = len(rows) - onScreen
			}
		}
		shown = rows[first : first+onScreen]
	}
	head := fmt.Sprintf("  %d change(s) worth making, across %d transcript(s)",
		len(rows), sessions)
	if len(rows) > len(shown) {
		head = fmt.Sprintf("  Top %d of %d changes worth making, across %d transcript(s)",
			len(shown), len(rows), sessions)
	}
	lines = append(lines, head, "")

	for i, r := range shown {
		// Titles are written by the analysis and run as long as the finding
		// needs, so they are cut to the terminal exactly like actions are. The
		// first version truncated only the action, which is why TestTW2 caught
		// three over-width titles on a corpus larger than the fixtures here.
		// The prefix is measured, not assumed. It was Cols()-6, which is right
		// for a one- or two-digit index and one cell short at 100, and a corpus
		// with a hundred suggestions is exactly where nobody is checking.
		num := fmt.Sprintf("%d", first+i+1)
		mark := strings.Repeat(" ", VisibleLen(selMarker))
		title := Strong
		if first+i == at {
			mark = paint(Accent, selMarker)
			title = Accent
		}
		prefix := 4 + len(num) + VisibleLen(selMarker)
		lines = append(lines, fmt.Sprintf("  %s%s  %s",
			mark, paint(Faint, num), paint(title, fitTo(r.Title, Cols()-prefix))))
		lines = append(lines, "     "+paint(Faint, cell("evidence", 10))+
			fitTo(fmt.Sprintf("%d transcript(s), %s prompt tokens", r.Sessions, commas(r.PromptTokens)), Cols()-15))

		pred := "not predicted on this corpus"
		if r.PredictedShare > 0 {
			pred = fmt.Sprintf("%.1f%% of prompt tokens", r.PredictedShare*100)
			if r.Estimated {
				pred += " (estimated)"
			}
		}
		savingStyle := Good
		if r.PredictedShare == 0 {
			// No prediction is not a small prediction. Painting it Good would
			// dress an absence as a modest win.
			savingStyle = Faint
		}
		// Columns dropped in a fixed order when the terminal is narrow, which is
		// what storyboard.go scene 25 specifies and what the audit counts. The
		// first version used a fixed 34-cell column for the prediction and a
		// status column after it, which is 63 cells before the status text and
		// therefore over budget on any terminal under about 75.
		//
		// Status goes first because it is the shortest thing to lose: a reader
		// who can see the saving can get the status from `replay advise`.
		row := "     " + paint(Faint, cell("saving", 10))
		if Cols()-15-VisibleLen(r.Status)-9 >= 12 {
			row += paint(savingStyle, cell(pred, Cols()-15-VisibleLen(r.Status)-9)) +
				paint(Faint, cell("status", 8)) + r.Status
		} else {
			row += paint(savingStyle, fitTo(pred, Cols()-15))
		}
		lines = append(lines, row)
		if r.Action != "" {
			// Wrapped, not truncated. Every action on the real screen ended in
			// "~" — "or a summarizing wrap~", "pass paths instead of con~" —
			// and the action is the entire product: the title states a fact,
			// the action is what the reader does about it. Cutting it tells
			// somebody they have a problem and withholds the fix.
			//
			// Folded here rather than at the output boundary because that
			// wrapper leaves table-shaped rows alone by design, and this row
			// is table-shaped.
			for j, part := range wrapAction(r.Action, Cols()-15) {
				label := "do"
				if j > 0 {
					label = ""
				}
				lines = append(lines, "     "+paint(Faint, cell(label, 10))+part)
			}
		}
		lines = append(lines, "")
	}

	if len(rows) > len(shown) {
		lines = append(lines,
			paint(Faint, fmt.Sprintf("  %d more in `replay advise`, ranked the same way.",
				len(rows)-len(shown))))
	}
	if at >= 0 && len(rows) > 0 {
		// Named on screen rather than left to `?`. A key nobody is told about
		// is a key nobody presses, which is how `--screen live` shipped
		// working and denied by its own help text.
		lines = append(lines, "",
			paint(Faint, "  j/k move   a mark applied   x dismiss   enter evidence"))
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
