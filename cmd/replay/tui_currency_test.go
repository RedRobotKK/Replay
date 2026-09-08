package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The TUI shows local currency the way the cost report does, with one
// difference forced on it by the surface.
//
// The report puts the local figure last on its line, so a currency symbol is
// safe there: two cells where one was budgeted shifts nothing after it. A
// screen is a grid. Every symbol worth printing is East Asian width class
// Ambiguous, so on this surface the currency is named by its ISO code, which
// is ASCII and one cell in every locale.
//
// That is not a detail of formatting, it is the reason TestTW1 exists, and
// these tests exist because wiring currency in is the most likely way for
// somebody to break it.

// corpus points the tool at a transcript of this test's own, and returns.
//
// Without it these tests read whatever corpus the machine happens to have.
// That is how the first version of this file passed here and failed on CI in
// three jobs: a runner has no transcripts, so the cost screen took its
// nothing-found branch and returned before reaching any of the code under
// test, while the author's machine has 1,681 and took the measured one.
//
// A test that only exercises the interesting path on the author's machine is
// the same defect this project has spent the day finding, wearing a different
// hat.
func corpus(t *testing.T) {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	dir := t.TempDir()
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPLAY_TRANSCRIPTS", dir)
}

// TC-1: a converted screen is still ASCII and still fits the budget.
//
// The whole hazard in one test. TW1 and TW2 already assert this, but they run
// with whatever locale the developer happens to have, and every developer on
// this project has a Latin one. On the operator's own ja_JP machine they would
// have been exercising a different code path than the one they check.
func TestTC1_AConvertedScreenIsStillASCIIAndFits(t *testing.T) {
	corpus(t)
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	for _, s := range tuiScreens {
		out := render(t, "tui", "-once", "-screen", s)
		for ln, line := range strings.Split(out, "\n") {
			for _, r := range line {
				if r >= 0x80 && !safeNonASCII(r) {
					t.Errorf("screen %s line %d: non-ASCII %q (U+%04X) under a ja_JP locale:\n  %s",
						s, ln+1, r, r, line)
				}
			}
			if n := len([]rune(line)); n > 80 {
				t.Errorf("screen %s line %d is %d cells under a ja_JP locale:\n  %s", s, ln+1, n, line)
			}
		}
	}
}

// TC-2: the local figure actually appears, named by its code.
func TestTC2_TheLocalFigureIsShownAndNamed(t *testing.T) {
	corpus(t)
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	out := render(t, "tui", "-once", "-screen", "cost")
	if !strings.Contains(out, "JPY") {
		t.Errorf("a ja_JP reader sees no local figure on the cost screen:\n%s", out)
	}
	if strings.Contains(out, "¥") {
		t.Errorf("the yen sign is Ambiguous width and shears this grid; the code is what belongs here:\n%s", out)
	}
}

// TC-3: a dollar reader's screen is byte-identical to before.
//
// Stated as an equality against the never case rather than by eye, so the
// feature cannot cost anything to the people it is not for.
func TestTC3_ADollarReaderSeesNoChange(t *testing.T) {
	corpus(t)
	t.Setenv("LC_ALL", "en_US.UTF-8")
	us := render(t, "tui", "-once", "-screen", "cost")
	t.Setenv("LC_ALL", "C")
	none := render(t, "tui", "-once", "-screen", "cost")
	if us != none {
		t.Errorf("a US locale and no locale render differently:\n%s\n---\n%s", us, none)
	}
	if strings.Contains(us, "JPY") || strings.Contains(us, "EUR") {
		t.Errorf("a dollar reader was shown a conversion:\n%s", us)
	}
}

// TC-4: the rate and its date are on the screen, once.
//
// A converted figure with no rate beside it is a number the reader cannot
// check, and this surface has no note under the block to carry it the way the
// report does.
func TestTC4_TheRateIsStatedOnScreen(t *testing.T) {
	corpus(t)
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	out := render(t, "tui", "-once", "-screen", "cost")
	if !strings.Contains(out, "/USD") {
		t.Errorf("no rate on screen:\n%s", out)
	}
	if n := strings.Count(out, "/USD"); n != 1 {
		t.Errorf("the rate appears %d times; once is the whole point of putting it in provenance", n)
	}
}
