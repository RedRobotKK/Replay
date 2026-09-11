package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// Colour is the semantic layer, and it must not move anything.
//
// layout.go names three tiers and says what TierMeaning is: "status words,
// notes, figures. ASCII, with colour carrying meaning that is also readable
// without it." The tier was specified and never built, so every screen printed
// one undifferentiated block of text in which the number that costs money and
// the number that is fine look identical.
//
// The whole hazard of adding it is that an escape sequence is bytes in a string
// that occupy no cells. Pad a field before colouring it and the layout holds;
// colour it before padding and every column downstream shifts by the width of
// an escape. That failure is invisible on the machine it is written on, because
// the terminal does not show the bytes it acted on.
//
// So the invariant is stated as an equality rather than as a rule to remember:
// strip the escapes from a coloured screen and you must get the uncoloured
// screen back, byte for byte. A regression cannot hide from that.

// sgr matches a Select Graphic Rendition sequence, which is the only escape
// class the colour layer is allowed to emit.
var sgr = regexp.MustCompile("\x1b\\[[0-9;]*m")

// anyEscape matches any escape sequence at all, so a test can tell "coloured"
// from "moved the cursor", which are very different things to find in output
// that gets piped into a README.
var anyEscape = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")

// colourEnv clears the two variables that switch colour off, for the whole test.
//
// internal/tui.NewPainter answers NO_COLOR-set or TERM=dumb with NoPaint
// BEFORE it looks at the mode, so `-color always` renders colourless under
// either. Two consequences here, and the second is the nastier one:
//
//   - TestScreenSVGs renders with `-color always` and compares against
//     committed images that DO carry colour. Under a container's ordinary
//     NO_COLOR=1 TERM=dumb it fails, on a product behaving as designed.
//   - TestCL1 compares `-color never` against `-color always` and requires
//     them to differ in no cell. Under the same environment BOTH are
//     colourless, the comparison is between two identical strings, and the
//     test passes having checked nothing. That is the worse failure: it is
//     silent.
//
// Verified: with NO_COLOR=1 and TERM=dumb, internal/tui and cmd/replay both go
// red on a tree that is green on a developer's terminal.
//
// Whether `-color always` SHOULD beat TERM=dumb in the shipped binary is a
// separate question about explicit flags versus environment heuristics. It
// changes behaviour a user can see and is not a test helper's to decide.
func colourEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"NO_COLOR", "TERM"} {
		// Setenv first so the testing package restores the original at
		// cleanup; Unsetenv is what actually clears it, because NewPainter
		// asks whether the variable EXISTS, not what it holds.
		t.Setenv(k, "x")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
}

func render(t *testing.T, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if err := run(args, &out, &errb); err != nil {
		t.Fatalf("%v: %v (stderr %q)", args, err, errb.String())
	}
	return out.String()
}

// CL-1: colour changes no cell on any screen.
//
// This is the one that matters. It compares the two renders directly rather
// than counting columns, so it catches a shifted field, a lost space, a
// truncation that moved, and an escape emitted in the middle of a padded
// column, without needing to know what any screen looks like.
func TestCL1_ColourChangesNoCell(t *testing.T) {
	// Pin the corpus, or this compares two renders of moving data.
	//
	// Every screen here is rendered twice and the results compared byte for
	// byte. Without a pinned corpus those two renders read whatever transcripts
	// the machine happens to hold *at that moment* — and on a machine with a
	// coding agent running, that changes between the two calls. The `context`
	// screen went red on main on 2026-09-09 for exactly this reason: rendering
	// it by hand, twice, produced 924 identical bytes each time.
	//
	// So the failure was real and the defect was not where it pointed. A test
	// that reads live state and asserts equality across two reads of it is
	// measuring the clock, and it fails for the people most likely to be
	// running it.
	corpus(t)
	colourEnv(t)
	for _, s := range tuiScreens {
		plain := render(t, "tui", "-once", "-screen", s, "-color", "never")
		painted := render(t, "tui", "-once", "-screen", s, "-color", "always")
		stripped := sgr.ReplaceAllString(painted, "")
		if stripped != plain {
			t.Errorf("screen %s: stripping colour does not reproduce the plain render.\n"+
				"An escape sequence occupies no cells, so a field coloured before it was "+
				"padded shifts every column after it.\nplain:\n%s\nstripped:\n%s",
				s, plain, stripped)
		}
	}
}

// CL-2: the colour layer emits only SGR, and only from the allowlist.
//
// Cursor movement, screen clears and mode switches belong to the loop, not to
// a screen. A screen that emits one has escaped its own layer and will corrupt
// output that is captured rather than displayed.
func TestCL2_OnlySGRAndOnlyFromThePalette(t *testing.T) {
	allowed := map[string]bool{}
	for _, code := range tui.PaletteCodes() {
		allowed[code] = true
	}
	allowed["0"] = true // reset

	for _, s := range tuiScreens {
		painted := render(t, "tui", "-once", "-screen", s, "-color", "always")
		for _, esc := range anyEscape.FindAllString(painted, -1) {
			if !sgr.MatchString(esc) {
				t.Errorf("screen %s emits a non-SGR escape %q; only the loop may move the cursor", s, esc)
				continue
			}
			body := strings.TrimSuffix(strings.TrimPrefix(esc, "\x1b["), "m")
			if !allowed[body] {
				t.Errorf("screen %s emits SGR %q, which is not in the palette. "+
					"Colours are chosen in one place so they can be checked in one place", s, body)
			}
		}
	}
}

// CL-3: piped output carries no colour, whatever the flag says.
//
// -color=always is a test and screenshot affordance. The default path must
// still produce clean bytes, because TestTW2 pipes these screens into README
// captures and agents read them. This asserts the default rather than trusting
// that nothing changed it.
func TestCL3_TheDefaultOnAPipeIsColourless(t *testing.T) {
	for _, s := range tuiScreens {
		out := render(t, "tui", "-once", "-screen", s)
		if esc := anyEscape.FindString(out); esc != "" {
			t.Errorf("screen %s emitted %q with no terminal attached", s, esc)
		}
	}
}

// CL-4: NO_COLOR wins over -color=always.
//
// no-color.org's rule is that the variable's presence disables colour
// regardless of any other setting, and a tool that honours it only when it
// feels like it has not honoured it. The user asked for colour on the command
// line and the environment says no; the environment is the accessibility
// setting and it wins.
func TestCL4_NoColorBeatsTheFlag(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, s := range tuiScreens {
		out := render(t, "tui", "-once", "-screen", s, "-color", "always")
		if esc := anyEscape.FindString(out); esc != "" {
			t.Errorf("screen %s emitted %q with NO_COLOR set", s, esc)
		}
	}
}

// CL-5: colour actually appears when it is asked for.
//
// Without this the four tests above are all satisfied by a colour layer that
// does nothing, which is the shape of check this project has now found seven
// times: every assertion passes and nothing is being tested.
func TestCL5_ColourIsActuallyEmitted(t *testing.T) {
	// The test written to catch vacuous colour assertions was itself reading
	// the ambient environment, which is how it stayed green on a terminal and
	// went red in a container.
	colourEnv(t)
	for _, s := range tuiScreens {
		painted := render(t, "tui", "-once", "-screen", s, "-color", "always")
		if !sgr.MatchString(painted) {
			t.Errorf("screen %s emitted no colour with -color=always; the other colour tests "+
				"pass trivially against a layer that paints nothing", s)
		}
	}
}

// CL-6: no screen nests one colour inside another.
//
// SGR does not nest. "\x1b[0m" is a full reset, not a pop, so an inner colour
// that closes itself also closes the one around it: the text after it renders
// plain and a stray reset trails the line. The cursor row did exactly this,
// wrapping a row that already coloured its own break count by severity.
//
// TestCL1 cannot catch it, and that is the point of writing this one down.
// Stripping the escapes gives back identical text whether they nest or not, so
// the invariant that guards the layout is blind to the one that guards the
// rendering. Two different failures need two different tests, and assuming the
// first covered the second is how the first six defects of the day survived.
func TestCL6_ColoursDoNotNest(t *testing.T) {
	for _, s := range tuiScreens {
		painted := render(t, "tui", "-once", "-screen", s, "-color", "always")
		for ln, line := range strings.Split(painted, "\n") {
			open := ""
			for _, esc := range sgr.FindAllString(line, -1) {
				body := strings.TrimSuffix(strings.TrimPrefix(esc, "\x1b["), "m")
				if body == "0" {
					open = ""
					continue
				}
				if open != "" {
					t.Errorf("screen %s line %d opens SGR %q while %q is still open. "+
						"SGR does not nest: the inner reset closes both, so the rest of "+
						"the line loses the outer colour.\n  %q", s, ln+1, body, open, line)
				}
				open = body
			}
			if open != "" {
				t.Errorf("screen %s line %d ends with SGR %q unclosed, which bleeds into "+
					"every line the terminal draws after it", s, ln+1, open)
			}
		}
	}
}
