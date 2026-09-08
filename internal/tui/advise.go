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
func AdviseScreen(rows []AdviceRow, sessions int) Screen {
	sc := Screen{Key: 'a', Title: "advise", From: Measured}
	lines := make([]string, 0, BudgetRows)
	lines = append(lines, header("advise"), "")

	if sessions == 0 {
		sc.From = Unavailable
		lines = append(lines,
			Banner(Unavailable, "no sessions were read, so nothing was ranked"), "",
			"  Run an agent, then come back. `replay doctor` says what is visible.", "")
		sc.Lines = lines
		return sc
	}

	if len(rows) == 0 {
		// A result, not an absence. Sessions were read and none crossed a
		// threshold; reporting that as "no data" would throw away the only
		// thing the run established.
		lines = append(lines,
			fmt.Sprintf("  Nothing worth changing across %d session(s).", sessions), "",
			"  Every target measured came in under the threshold.", "")
		sc.Lines = lines
		return sc
	}

	lines = append(lines, fmt.Sprintf("  %d change(s) worth making, across %d session(s)",
		len(rows), sessions), "")

	for i, r := range rows {
		// Titles are written by the analysis and run as long as the finding
		// needs, so they are cut to the terminal exactly like actions are. The
		// first version truncated only the action, which is why TestTW2 caught
		// three over-width titles on a corpus larger than the fixtures here.
		// The prefix is measured, not assumed. It was Cols()-6, which is right
		// for a one- or two-digit index and one cell short at 100, and a corpus
		// with a hundred suggestions is exactly where nobody is checking.
		num := fmt.Sprintf("%d", i+1)
		prefix := 4 + len(num)
		lines = append(lines, fmt.Sprintf("  %s  %s",
			paint(Faint, num), paint(Strong, fitTo(r.Title, Cols()-prefix))))
		lines = append(lines, "     "+paint(Faint, cell("evidence", 10))+
			fitTo(fmt.Sprintf("%d session(s), %s prompt tokens", r.Sessions, commas(r.PromptTokens)), Cols()-15))

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
			// Truncated to the terminal rather than left to wrap. A real action
			// runs to a sentence and the fixtures in the test are short, which
			// is how this shipped over-width the first time: the row reads as a
			// table to the output-boundary wrapper, so it is never folded and
			// the terminal breaks it mid-word instead.
			lines = append(lines, "     "+paint(Faint, cell("do", 10))+fitTo(r.Action, Cols()-15))
		}
		lines = append(lines, "")
	}

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
