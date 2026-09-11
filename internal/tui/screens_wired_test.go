package tui

import (
	"os"
	"strings"
	"testing"
)

// The last four screens that described nobody.
//
// context, guards, model and safe all fell through to Outcome(), the canned
// illustration the dispatch renders for any key with no case. advise was wired
// first and is the worked example; these follow the same three states, and the
// same rule from measured.go: a screen is Measured only when every figure came
// from somewhere a reader could go and check.
//
// The tests below are the ones that caught real defects while advise was being
// wired, applied to all four before rather than after. Each of these existed as
// a bug once:
//
//   - a screen that painted only its populated branch, so a CI runner with no
//     transcripts rendered a colourless frame and TestCL5 failed on all four
//     platforms at once
//   - a fixture with short strings, so the width test passed while real output
//     ran three lines past the terminal
//   - "no data" and "nothing found" collapsed into one message, which throws
//     away the only thing a clean run established

// paintOn forces colour on for a test, whatever the ambient environment is.
//
// It clears TERM as well as NO_COLOR, and that second line is the whole point.
// NewPainter answers TERM=dumb with NoPaint BEFORE it looks at mode, so
// NewPainter("always", true) returns a painter that emits nothing when the
// suite runs under a dumb terminal — and the assertion "every branch paints"
// then fails on a product that is behaving exactly as designed.
//
// It bit in a container: NO_COLOR=1 and TERM=dumb are ordinary CI and
// container defaults, and internal/tui went red there while passing on a
// developer's terminal. WS3 exists because a screen "failed CI on all four
// platforms while passing here" — it had the same disease it was written to
// cure, one environment variable over.
//
// The t.Setenv call is what registers the restore: t.Setenv records the old
// value and reinstates it at cleanup, and Unsetenv immediately afterwards is
// how a variable gets cleared FOR the test without leaking the clear into the
// rest of the package.
//
// This does not decide whether `--color=always` ought to beat TERM=dumb in the
// product. That is a precedence question about explicit flags versus
// environment heuristics, it changes shipped behaviour, and it is not a test
// helper's to answer.
func paintOn(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "x")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	// TERM is SET to a real terminal, not unset.
	//
	// NewPainter matches "dumb" exactly, so an empty TERM would do here. Other
	// code in this repository does not: cmd/replay/tipline.go:59 treats
	// TERM=="" the same as "dumb". A helper that leaves TERM empty is a trap
	// for whoever reuses it near that code, and it would spring quietly —
	// a suppressed tip line is not a failure, it is a missing sentence.
	//
	// Naming a real terminal satisfies both rules and asserts what the test
	// actually means: a terminal that can paint.
	t.Setenv("TERM", "xterm-256color")
	old := active
	active = NewPainter("always", true)
	t.Cleanup(func() { active = old })
}

func ctxRows() []ContextRow {
	return []ContextRow{
		{Label: "Bash", Tokens: 209000, Share: 0.432, Occurrences: 106},
		{Label: "Read", Tokens: 88000, Share: 0.182, Occurrences: 41},
	}
}

// WS1: every wired screen is Measured when it has data, and says so by silence.
func TestWS1_WithDataThereIsNoNotice(t *testing.T) {
	for name, lines := range map[string][]string{
		"context": ContextScreen(ctxRows(), 1).Lines,
		"guards":  GuardsScreen(liveGuards(), []string{"  session cap  $4.10", "  daily cap    $18.00"}, 12).Lines,
		"safe":    SafeScreen(Privacy{Root: "~/.replay", Stores: stores()}).Lines,
		"model":   ModelScreen("claude-haiku-4-5", []ModelRow{{Model: "claude-opus-5", Turns: 40, Share: 0.9}}, 1).Lines,
	} {
		body := strings.Join(lines, "\n")
		if strings.Contains(body, "example data") {
			t.Errorf("%s still claims to describe nobody", name)
		}
		if strings.Contains(body, "not measured here") {
			t.Errorf("%s has data and says it was not measured", name)
		}
	}
}

// WS2: no corpus is Unavailable, never Example.
//
// "Example data" says these numbers describe nobody. "Not measured here" says
// there were no numbers. Only one is about the reader.
func TestWS2_NoCorpusIsUnavailable(t *testing.T) {
	for name, lines := range map[string][]string{
		"context": ContextScreen(nil, 0).Lines,
		"guards":  GuardsScreen(GuardState{}, nil, 0).Lines,
		// The safe screen no longer depends on a corpus: it reads the
		// filesystem, and an empty ~/.replay is a measured finding rather than
		// an absence. Its Unavailable state is a directory it could not read.
		"safe":  SafeScreen(Privacy{Root: "~/.replay", Err: "permission denied"}).Lines,
		"model": ModelScreen("", nil, 0).Lines,
	} {
		body := strings.Join(lines, "\n")
		if strings.Contains(body, "example data") {
			t.Errorf("%s calls an absence an illustration", name)
		}
		if !strings.Contains(body, "not measured here") {
			t.Errorf("%s does not say it was not measured:\n%s", name, body)
		}
	}
}

// WS3: every branch paints.
//
// The defect that failed CI on all four platforms while passing here: a runner
// has no transcripts, takes the empty branch, and a screen that colours only its
// populated path emits nothing at all.
func TestWS3_EveryBranchPaints(t *testing.T) {
	paintOn(t)
	cases := map[string][][]string{
		"context": {ContextScreen(nil, 0).Lines, ContextScreen(nil, 9).Lines, ContextScreen(ctxRows(), 1).Lines},
		"guards":  {GuardsScreen(GuardState{}, nil, 0).Lines, GuardsScreen(GuardState{}, nil, 9).Lines, GuardsScreen(liveGuards(), []string{"  cap  $4"}, 12).Lines},
		"safe":    {SafeScreen(Privacy{Root: "~/.replay"}).Lines, SafeScreen(Privacy{Root: "~/.replay", Err: "permission denied"}).Lines, SafeScreen(Privacy{Root: "~/.replay", Stores: stores()}).Lines},
		"model":   {ModelScreen("", nil, 0).Lines, ModelScreen("claude-haiku-4-5", nil, 9).Lines},
	}
	for name, branches := range cases {
		for i, lines := range branches {
			if !strings.Contains(strings.Join(lines, "\n"), "\x1b[") {
				t.Errorf("%s branch %d emits no colour", name, i)
			}
		}
	}
}

// WS4: realistic strings do not run past the terminal.
//
// The fixtures in a test are short by construction. A real tool label, model id
// or advice line is whatever the analysis produced, and that is what shipped
// over-width the first time.
func TestWS4_RealisticStringsFit(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	long := []ContextRow{{Label: "mcp__some_connector__a_tool_with_a_genuinely_long_name",
		Tokens: 209000123, Share: 0.432, Occurrences: 1061}}
	all := map[string][]string{
		"context": ContextScreen(long, 3).Lines,
		"guards": GuardsScreen(liveGuards(), []string{
			"  a guard line long enough to run past an eighty column terminal if nothing cuts it"}, 12).Lines,
		"safe":  SafeScreen(Privacy{Root: "~/.replay", Stores: stores()}).Lines,
		"model": ModelScreen("claude-a-model-identifier-that-is-unusually-long-indeed", []ModelRow{{Model: "claude-another-very-long-model-identifier", Turns: 4000, Share: 0.9}}, 2).Lines,
	}
	for name, lines := range all {
		for i, l := range lines {
			if VisibleLen(l) > 80 {
				t.Errorf("%s line %d is %d cells in an 80-cell terminal: %q", name, i, VisibleLen(l), l)
			}
		}
	}
}

// WS5: a corpus that found nothing is a result, not an absence.
func TestWS5_NothingFoundIsMeasured(t *testing.T) {
	for name, lines := range map[string][]string{
		"context": ContextScreen(nil, 6).Lines,
		"guards":  GuardsScreen(GuardState{}, nil, 6).Lines,
	} {
		body := strings.Join(lines, "\n")
		if strings.Contains(body, "not measured here") {
			t.Errorf("%s read 6 sessions and calls that unmeasured", name)
		}
		if !strings.Contains(body, "6") {
			t.Errorf("%s does not say how many sessions found nothing:\n%s", name, body)
		}
	}
}
