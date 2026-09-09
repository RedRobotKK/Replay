package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The advise screen, reading this machine.
//
// It was one of five that fell through to Outcome(), the canned illustration
// rendered for any key without a case in the dispatch. Nothing was broken and
// nothing was dishonest — each carried the example-data notice — but the
// analysis behind it runs perfectly well from the command line, so the surface
// the installer opens was the one place it did not.
//
// The rule measured.go states is the one that shapes this: a screen is Measured
// only when every figure came from somewhere a reader could go and check. So
// there are three states here rather than two, and the middle one is the point.
//
//	suggestions        Measured, no banner
//	corpus, none found Measured, and says so plainly
//	no corpus at all    Unavailable, "not measured here", never Example
//
// The last distinction is the one worth defending. "Example data" says these
// numbers describe nobody. "Not measured here" says there were no numbers. A
// reader who has just installed the tool needs to know which, because only one
// of them is about them.

func rows() []AdviceRow {
	return []AdviceRow{
		{Title: "Bash inputs are 28% of prompt tokens", Action: "keep tool inputs short",
			Sessions: 3, Share: 0.28, PromptTokens: 336060, PredictedShare: 0.042, Status: "pending"},
		{Title: "the system prompt is re-sent every turn", Action: "cache it explicitly",
			Sessions: 12, Share: 0.11, PromptTokens: 91000, PredictedShare: 0.0, Estimated: true, Status: "advice only"},
	}
}

// AD1: with suggestions, the screen is measured and carries no notice.
func TestAD1_MeasuredCarriesNoBanner(t *testing.T) {
	s := AdviseScreen(rows(), 1)
	body := strings.Join(s.Lines, "\n")
	if strings.Contains(body, "example data") {
		t.Error("a screen built from real suggestions still claims to describe nobody")
	}
	if strings.Contains(body, "not measured here") {
		t.Error("a screen with suggestions says it was not measured")
	}
}

// AD2: the figures come from the input, not from the renderer.
//
// The check that stops this becoming a prettier illustration. If the numbers on
// screen do not move when the input moves, they are decoration.
func TestAD2_FiguresComeFromTheInput(t *testing.T) {
	body := strings.Join(AdviseScreen(rows(), 1).Lines, "\n")
	// "3 transcript" rather than "3 session": the unit was renamed on 2026-09-09
	// because the caller counts files and a session writes one per agent lane.
	// This test is about the figure reaching the screen from its input, and 3
	// still has to appear beside the noun.
	for _, want := range []string{"Bash inputs are 28%", "336,060", "3 transcript"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not carry %q from its input:\n%s", want, body)
		}
	}
	changed := rows()
	changed[0].PromptTokens = 999111
	if !strings.Contains(strings.Join(AdviseScreen(changed, 1).Lines, "\n"), "999,111") {
		t.Error("changing the input did not change the screen; the figures are decoration")
	}
}

// AD3: no corpus is Unavailable, never Example.
//
// "Example data" says these numbers describe nobody. "Not measured here" says
// there were no numbers. Only one of those is about the reader, and someone who
// has just installed the tool needs to know which.
func TestAD3_NoCorpusIsUnavailableNotExample(t *testing.T) {
	body := strings.Join(AdviseScreen(nil, 0).Lines, "\n")
	if strings.Contains(body, "example data") {
		t.Error("with no corpus the screen claims to be an illustration; it is an absence")
	}
	if !strings.Contains(body, "not measured here") {
		t.Errorf("with no corpus the screen does not say it was not measured:\n%s", body)
	}
}

// AD4: a corpus that yielded nothing is measured, and says so.
//
// Distinct from AD3 and easy to conflate. Sessions were read and none crossed a
// threshold: that is a finding, and reporting it as "no data" would throw away
// the one thing the run established.
func TestAD4_ACleanCorpusIsAResult(t *testing.T) {
	body := strings.Join(AdviseScreen(nil, 7).Lines, "\n")
	if strings.Contains(body, "not measured here") {
		t.Error("7 sessions were read; that is measured, not unavailable")
	}
	if !strings.Contains(body, "7") {
		t.Errorf("the screen does not say how many sessions found nothing:\n%s", body)
	}
}

// AD5: the screen fits the terminal.
func TestAD5_FitsTheWidth(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	for i, l := range AdviseScreen(rows(), 1).Lines {
		if VisibleLen(l) > 80 {
			t.Errorf("line %d is %d cells in an 80-cell terminal: %q", i, VisibleLen(l), l)
		}
	}
}

// AD6: a realistic action does not run past the terminal.
//
// AD5 used the short strings in rows(), which is why it passed while the screen
// shipped three over-width lines on real data: the actions the analysis actually
// produces run to a sentence. The row reads as tabular to the output-boundary
// wrapper — two spaces between the label and the text — so it is never folded,
// and the terminal breaks it mid-word instead.
func TestAD6_ALongActionIsTruncatedNotWrapped(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	long := rows()
	long[0].Action = "keep tool inputs short: run scripts from files instead of inline " +
		"heredocs, and pass paths instead of contents, then re-run to confirm"
	for i, l := range AdviseScreen(long, 1).Lines {
		if VisibleLen(l) > 80 {
			t.Errorf("line %d is %d cells in an 80-cell terminal: %q", i, VisibleLen(l), l)
		}
	}
}

// AD7: every branch paints, including the ones with no data.
//
// The populated branch was the only one coloured, which passed locally and
// failed on all four CI platforms at once: a runner has no transcripts, so it
// takes the empty branch, and TestCL5 found a screen emitting no colour at all.
// Having a corpus on the development machine made every other test kinder than
// the machine this has to run on.
func TestAD7_EveryBranchPaints(t *testing.T) {
	// Unset, not set-to-empty. NewPainter uses os.LookupEnv, which asks whether
	// the variable EXISTS, so t.Setenv("NO_COLOR", "") switches colour off
	// rather than on — which is how the first version of this test failed
	// against correct code. t.Setenv is called first purely so the testing
	// package restores whatever was there afterwards.
	t.Setenv("NO_COLOR", "x")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	old := active
	active = NewPainter("always", true)
	defer func() { active = old }()

	for _, c := range []struct {
		name     string
		rows     []AdviceRow
		sessions int
	}{
		{"no corpus", nil, 0},
		{"corpus, nothing found", nil, 7},
		{"suggestions", rows(), 1},
	} {
		body := strings.Join(AdviseScreen(c.rows, c.sessions).Lines, "\n")
		if !strings.Contains(body, "\x1b[") {
			t.Errorf("the %q branch emits no colour; the palette tests pass "+
				"trivially against a branch that paints nothing", c.name)
		}
	}
}

// AD8: the corpus unit is named correctly.
//
// The screen said "140 change(s) worth making, across 1729 session(s)" while
// `replay doctor`, in the same shell, said "123 sessions across 12 projects,
// 1738 transcript files". Both cannot be right, and doctor is.
//
// adviceState increments once per FILE, and a session writes one file per agent
// lane, so the number is transcripts wearing the word sessions. That is the
// exact conflation calibration-corpus-2026-09-06.md retracted in public: "the
// previous file counted transcript files and called them sessions", 1450
// transcripts from 78 sessions. It was corrected in the evidence and left live
// in the product.
//
// The count is not wrong. The noun is.
func TestAD8_TheCorpusUnitIsTranscriptsNotSessions(t *testing.T) {
	body := strings.Join(AdviseScreen(rows(), 1729).Lines, "\n")
	if strings.Contains(body, "session(s)") {
		t.Errorf("the screen calls its corpus \"session(s)\" while counting transcript "+
			"files. doctor reports both separately and disagrees by an order of "+
			"magnitude:\n%s", body)
	}
	if !strings.Contains(body, "transcript") {
		t.Errorf("the screen does not name what it counted:\n%s", body)
	}
}

// AD9: the screen fits a terminal.
//
// 140 findings rendered 705 lines into a 24-row terminal. Everything past the
// third scrolled away before it could be read, and the count itself is not a
// finding a reader can act on — "140 changes worth making" is a number to feel
// bad about, not a list to work through.
//
// A screen is a screen. The full set stays available in `replay advise` and in
// advice.json; what is on the terminal is what someone can act on now.
func TestAD9_TheScreenFitsATerminal(t *testing.T) {
	many := make([]AdviceRow, 140)
	for i := range many {
		many[i] = AdviceRow{
			Title: fmt.Sprintf("finding number %d about prompt tokens", i),
			Action: "truncate outputs before they enter the conversation: head, tail, " +
				"grep with limits, or a summarizing wrapper",
			Sessions: 3, Share: 0.3, PromptTokens: 1000, Status: "pending",
		}
	}
	got := AdviseScreen(many, 1729).Lines
	if len(got) > 40 {
		t.Errorf("140 findings rendered %d lines; a terminal is about 24 rows, so "+
			"everything past the top few scrolls away unread", len(got))
	}
	body := strings.Join(got, "\n")
	// And it must say what it left out, or the screen is quietly lying about
	// how much there is.
	if !strings.Contains(body, "140") {
		t.Errorf("the screen shows a subset and does not say how many exist:\n%s", body)
	}
}

// AD10: the action is never cut off.
//
// Every row on the real screen ended in "~": "or a summarizing wrap~",
// "and pass paths instead of con~". The action is the entire product — the
// title states a fact, the action is what the reader does about it — and it was
// the one field truncated. A finding whose remedy is unreadable has told the
// reader they have a problem and withheld the fix.
func TestAD10_TheActionIsNotTruncated(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	long := rows()
	long[0].Action = "truncate outputs before they enter the conversation: head, tail, " +
		"grep with limits, or a summarizing wrapper"
	for _, l := range AdviseScreen(long, 12).Lines {
		if !strings.Contains(l, "truncate outputs") {
			continue
		}
		if strings.HasSuffix(strings.TrimRight(l, " "), "~") {
			t.Errorf("the action is truncated, so the reader is told what is wrong and "+
				"not what to do:\n  %s", l)
		}
	}
}
