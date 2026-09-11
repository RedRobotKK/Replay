package tui

import (
	"strings"
	"testing"
	"time"
)

// A screen is named by the question it answers.
//
// Shortcut.Question has carried "what the user actually wants to know, in
// their words" since the surface was designed, and nothing ever rendered it.
// Every screen was headed `replay cost`, `replay why`, `replay guards` — the
// query, not the finding — so the one thing that makes a dashboard legible to
// somebody who did not build it was sitting in the struct, unread.
//
// Five of those headings were not even commands. There is no `replay why`, no
// `replay guards`, no `replay safe`, no `replay model` and no `replay live`;
// cmd/replay/outliernote_test.go already refuses `replay why` in prose for
// exactly that reason, while the header printed it on a screen.

// shortcutFor finds a screen by the name the -screen flag uses.
func shortcutFor(t *testing.T, label string) Shortcut {
	t.Helper()
	for _, s := range Shortcuts() {
		if s.Label == label {
			return s
		}
	}
	t.Fatalf("no screen is labelled %q", label)
	return Shortcut{}
}

// tenScreens builds every screen from a fixture, keyed by its label.
//
// All ten, because the defects this file covers were each present on a subset
// and invisible from any other: the live screen had no header at all, and the
// question was missing from every one of them.
func tenScreens(t *testing.T) map[string]Screen {
	t.Helper()
	m := aMachine()
	m.CostReady = true
	m.Tasks, m.TotalUSD, m.MedianUSD, m.P90USD = 116, 3739.80, 0.77, 3.41
	m.TaskRows = []Task{{Session: "abcd1234", Model: "opus-5", CostUSD: 9.0, Path: "/c/a.jsonl"}}
	task := m.TaskRows[0]
	return map[string]Screen{
		"cost":   CostScreen(m, 0, Selection{Window: 6}),
		"doctor": DoctorScreen(m),
		"why": WhyScreen(&task, func(string) (string, error) {
			return "  a line of blame output", nil
		}),
		"context": ContextScreen(ctxRows(), 1),
		"advise":  AdviseScreen(nil, 0),
		"guards":  GuardsScreen(liveGuards(), someAdvice(), 20),
		"model":   ModelScreen("claude-haiku-4-5", []ModelRow{{Model: "claude-opus-5", Turns: 40, Share: 0.9}}, 1),
		"safe":    SafeScreen(Privacy{Root: "~/.replay", Stores: stores()}),
		"live":    LiveScreen(Live{Reachable: false, Addr: "127.0.0.1:4000"}, time.Now()),
		"share":   ShareScreen(ShareState{}),
	}
}

// QT1: the title row of every screen is the question that screen answers.
func TestQT1_TheTitleIsTheQuestion(t *testing.T) {
	for name, sc := range tenScreens(t) {
		want := shortcutFor(t, name).Question
		if len(sc.Lines) == 0 {
			t.Errorf("the %s screen renders nothing", name)
			continue
		}
		got := StripSGR(sc.Lines[0])
		if !strings.Contains(got, want) {
			t.Errorf("the %s screen opens with %q. It should open with the question it "+
				"answers, %q: the command is the query, and a reader who did not build "+
				"this tool cannot read a dashboard titled by its query", name, got, want)
		}
	}
}

// QT2: and it still names the command that produced it, which must be a
// command that exists.
//
// `replay why`, `replay guards`, `replay safe`, `replay model` and `replay
// live` are not commands. Printing one on the screen it produced is the same
// defect the outlier note was corrected for, one surface over.
func TestQT2_TheTitleNamesTheRealCommand(t *testing.T) {
	for name, sc := range tenScreens(t) {
		s := shortcutFor(t, name)
		got := StripSGR(sc.Lines[0])
		if !strings.Contains(got, "replay "+s.Command) {
			t.Errorf("the %s screen does not say what it ran; the title is %q and the "+
				"command was `replay %s`. Showing it is this design's stated cheapest "+
				"honesty, and readers copy it", name, got, s.Command)
		}
		if s.Label != s.Command && strings.Contains(got, "replay "+s.Label) {
			t.Errorf("the %s screen prints `replay %s`, which is not a command:\n%q",
				name, s.Label, got)
		}
	}
}

// QT3: the title row fits the terminal it is measured against.
//
// Three things now share one row where two shared it before, so this is the
// row most likely to shear. It has to degrade rather than overflow: the
// command goes when there is no room for it, and the question is the last
// thing cut, because the question is the title.
func TestQT3_TheTitleRowFitsEveryWidth(t *testing.T) {
	for _, w := range []string{"40", "60", "80", "120"} {
		t.Setenv("COLUMNS", w)
		for _, s := range Shortcuts() {
			h := header(s.Label)
			if VisibleLen(h) > Cols() {
				t.Errorf("the %s title is %d cells in a %s-cell terminal:\n%q",
					s.Label, VisibleLen(h), w, h)
			}
			// The question survives every width this surface supports.
			if Cols() >= 80 && !strings.Contains(StripSGR(h), s.Question) {
				t.Errorf("the %s title dropped its question at %s columns:\n%q",
					s.Label, w, h)
			}
		}
	}
}

// QT4: the live screen says what it is before it says what is wrong.
//
// Nine screens opened with a banner. `replay tui --screen live` opened straight
// into "no proxy answered at 127.0.0.1:4000", which is the answer to a question
// the screen never asked, and it was the only screen that did not say what it
// was.
func TestQT4_TheLiveScreenHasATitle(t *testing.T) {
	for _, l := range []Live{
		{Reachable: false, Addr: "127.0.0.1:4000"},
		{Reachable: true, Addr: "127.0.0.1:4000", UptimeSeconds: 89236},
		{Reachable: true, Addr: "127.0.0.1:4000", UptimeSeconds: 300,
			Sessions: []LiveSession{{ID: "facfd32e", Model: "opus-5", Requests: 3}}},
	} {
		sc := LiveScreen(l, time.Now())
		if got := StripSGR(sc.Lines[0]); !strings.Contains(got, "What is flowing right now?") {
			t.Errorf("the live screen (reachable=%v, sessions=%d) opens with %q and "+
				"never says what it is", l.Reachable, len(l.Sessions), got)
		}
	}
}

// QT5: the cost screen leads with the shape of the distribution.
//
// It led with "$3739.80 across 116 tasks" and buried "Median $0.77, p90 $3.41"
// in a subtitle. On this corpus the mean is $32 and the median is $0.77, so the
// total describes no task anybody ran; the distribution is the finding. The
// total stays, because the question "what did this cost me" has a total in its
// answer, but it stops being the only figure the eye is sent to.
func TestQT5_TheCostScreenLeadsWithTheDistribution(t *testing.T) {
	paintOn(t)
	m := aMachine()
	m.CostReady = true
	m.Tasks, m.TotalUSD, m.MedianUSD, m.P90USD, m.AvoidableUSD = 116, 3739.80, 0.77, 3.41, 177.22
	m.TaskRows = []Task{{Session: "abcd1234", Model: "opus-5", CostUSD: 9.0, Path: "/c/a.jsonl"}}
	sc := CostScreen(m, 0, Selection{Window: 6})

	shape, total := -1, -1
	for i, l := range sc.Lines {
		if strings.Contains(l, paint(Strong, money(m.MedianUSD))) &&
			strings.Contains(l, paint(Strong, money(m.P90USD))) {
			shape = i
		}
		if strings.Contains(l, paint(Strong, money(m.TotalUSD))) {
			total = i
		}
	}
	if shape < 0 {
		t.Errorf("the median and p90 are not raised to the level of the total; both "+
			"must carry the same emphasis, because the distribution is the finding "+
			"and the mean here is $32 against a median of $0.77:\n%s", sc.String())
	}
	if total < 0 {
		t.Errorf("the total is gone. It describes nobody and it is still the answer "+
			"to a question about a bill:\n%s", sc.String())
	}
	if shape >= 0 && total >= 0 && shape > total {
		t.Errorf("the total is still on line %d and the distribution on line %d. The "+
			"screen should open on the shape:\n%s", total, shape, sc.String())
	}
}

// QT6: none of this cost a row.
//
// pad(), padCost(), padWhy() and padShare() truncate silently, from the bottom,
// and the doctor screen's worst case already fills its body exactly. A title
// block that grew by one row would delete the last note on that screen without
// saying so, which is how main went red in #169.
func TestQT6_NoScreenGrewPastItsBudget(t *testing.T) {
	for name, sc := range tenScreens(t) {
		if len(sc.Lines) > bodyRows {
			t.Errorf("the %s screen is %d rows against a body budget of %d; the loop "+
				"trims from the bottom and the reader is not told", name, len(sc.Lines), bodyRows)
		}
	}
}

// QT7: the title row drops the least of itself first.
//
// Three things share a row where two shared it before, so it is the row most
// likely to shear on a narrow terminal. The order is the one scene 25 sets:
// the command goes before the question does, because the command is recoverable
// — the `ran` line has it and the footer names the screen — and the question is
// the title. It is cut only when the version tag alone will not share the row.
func TestQT7_TheTitleRowDropsTheLeastOfItselfFirst(t *testing.T) {
	t.Setenv("COLUMNS", "40")

	if got := titleRow("Short?", "replay cost"); !strings.Contains(got, "replay cost") {
		t.Errorf("a title with room for the command dropped it: %q", got)
	}

	long := titleRow(strings.Repeat("a", 30)+"?", "replay cost")
	if strings.Contains(long, "replay cost") {
		t.Errorf("the command was kept on a row with no room for it, so the row is "+
			"%d cells in a 40-cell terminal: %q", VisibleLen(long), long)
	}
	if !strings.Contains(long, strings.Repeat("a", 30)) {
		t.Errorf("the question was cut while the command was still on the row: %q", long)
	}
	if VisibleLen(long) > 40 {
		t.Errorf("the title row is %d cells in a 40-cell terminal: %q", VisibleLen(long), long)
	}

	cut := titleRow(strings.Repeat("b", 60), "replay cost")
	if VisibleLen(cut) > 40 {
		t.Errorf("a title too long for the row was not cut: %d cells, %q", VisibleLen(cut), cut)
	}
	if !strings.ContainsRune(cut, truncationMark) {
		t.Errorf("the title was cut without saying so; a silent truncation is the "+
			"defect this surface keeps finding: %q", cut)
	}
}
