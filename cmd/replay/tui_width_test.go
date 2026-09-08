package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// tuiScreens are the nine questions `replay tui -screen` accepts.
//
// Every screen goes through all four checks below, so a new one is covered by
// adding it here and nowhere else. A screen missing from this list is a screen
// none of the width, encoding or whitespace invariants apply to, and the gap
// would be invisible: the tests read as passing either way.
var tuiScreens = []string{"cost", "why", "context", "advise", "guards", "model", "safe", "doctor", "share", "live"}

// safeNonASCII is the allowlist of non-ASCII runes a screen may contain.
//
// Braille (U+2800..U+28FF) is East Asian width class NEUTRAL, so it occupies
// one cell in every locale and is safe for a spinner. Box drawing, block
// elements and bullets are class AMBIGUOUS: one cell under a Latin locale and
// TWO under ja_JP, zh_CN and ko_KR. A layout drawn with them is not
// "structurally frozen", it is frozen only on the machine it was drawn on.
func safeNonASCII(r rune) bool { return r >= 0x2800 && r <= 0x28FF }

// TestTW1: every TUI screen is ASCII, apart from an explicit safe allowlist.
//
// This is deliberately a stricter rule than "no ambiguous characters", because
// the strict version needs no width table and cannot rot. The project carries
// no third-party dependency and will not add one for a Unicode table, so the
// invariant is stated in the form that is checkable without one.
//
// The rule exists because the operator works in Tokyo in a ja_JP terminal. A
// box drawn from U+2500 characters measures 58 cells where it was written and
// 76 cells where he reads it, and every "rigid" column in it shears.
func TestTW1(t *testing.T) {
	for _, s := range tuiScreens {
		var out, errb bytes.Buffer
		if err := run([]string{"tui", "-once", "-screen", s}, &out, &errb); err != nil {
			t.Fatalf("%s: %v (stderr %q)", s, err, errb.String())
		}
		for ln, line := range strings.Split(out.String(), "\n") {
			for _, r := range line {
				if r < 0x80 || safeNonASCII(r) {
					continue
				}
				t.Errorf("screen %s line %d: non-ASCII %q (U+%04X) shifts width by locale:\n  %s",
					s, ln+1, r, r, line)
			}
		}
	}
}

// TestTW2: a frame written to a pipe carries no escape bytes, and no line
// overruns the 80-column budget the screens are drawn to.
//
// Both halves matter for the same reason. These frames are piped into README
// screenshots and read by agents; an escape byte or an over-long line is
// invisible to whoever generated it and wrong for everyone downstream.
func TestTW2(t *testing.T) {
	for _, s := range tuiScreens {
		var out, errb bytes.Buffer
		if err := run([]string{"tui", "-once", "-screen", s}, &out, &errb); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		body := out.String()
		if i := strings.IndexRune(body, 0x1b); i >= 0 {
			t.Errorf("screen %s: escape byte at offset %d in piped output", s, i)
		}
		for ln, line := range strings.Split(body, "\n") {
			// ASCII is one cell each, which TestTW1 has already established.
			if len(line) > tui.BudgetCols {
				t.Errorf("screen %s line %d is %d cells, over the %d budget:\n  %s",
					s, ln+1, len(line), tui.BudgetCols, line)
			}
		}
	}
}

// TestTW3: the piped frame carries no trailing whitespace.
//
// Read what this does and does not guard, because the first version of this
// comment claimed the wrong one.
//
// It guards the PIPE CONTRACT. --once trims every line before printing, and
// removing that trim turns this red, because the screens genuinely do pad.
//
// It does NOT guard "no screen emits trailing whitespace", and it cannot: the
// assertion is made downstream of the trim, so a screen padded with anything
// at all still passes. That is the right way round. A live terminal pads a row
// to the column width so the row underneath disappears, and forbidding that
// would be forbidding the mechanism that makes the surface repaint correctly.
// The padding is deliberate; suppressing it in the pipe is also deliberate;
// this checks the second.
//
// A mutation that adds trailing spaces inside a screen will NOT fail this
// test, and should not. A mutation that removes the trim will.
func TestTW3(t *testing.T) {
	for _, s := range tuiScreens {
		var out, errb bytes.Buffer
		if err := run([]string{"tui", "-once", "-screen", s}, &out, &errb); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		for ln, line := range strings.Split(out.String(), "\n") {
			if line != strings.TrimRight(line, " \t") {
				t.Errorf("screen %s line %d has %d trailing space(s):\n  %q",
					s, ln+1, len(line)-len(strings.TrimRight(line, " \t")), line)
			}
		}
	}
}

// TestTW4: `replay tui --help` is help, not an error.
//
// Every other command routes --help through parseArgs and exits 0 on stdout.
// tui called fs.Parse directly, so it printed usage to STDERR and exited 1,
// with a trailing "flag: help requested" line. That is the exact defect
// help.go was written to fix, surviving in the newest command.
func TestTW4(t *testing.T) {
	var out, errb bytes.Buffer
	err := run([]string{"tui", "--help"}, &out, &errb)
	if err != nil && err != errHelpShown {
		t.Errorf("tui --help returned %v, want nil or errHelpShown", err)
	}
	if out.Len() == 0 {
		t.Error("tui --help wrote nothing to stdout")
	}
	if strings.Contains(errb.String(), "help requested") {
		t.Errorf("tui --help leaked flag-package noise to stderr: %q", errb.String())
	}
}
