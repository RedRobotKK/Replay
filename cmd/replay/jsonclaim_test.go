package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// The help overlay is drawn on every screen, and it advertised `--json` as
// "the same answer for a machine, no screen at all" with nothing qualifying
// it. Three of the commands behind those screens have no such flag, and
// that is four of the ten SCREENS, not three: guards and live are both
// `replay serve`. Four keys, three commands. The line was a promise on ten
// screens that four of them break:
//
//	replay doctor --json   flag provided but not defined: -json
//	replay blame  --json   flag provided but not defined: -json
//	replay serve  --json   flag provided but not defined: -json
//
// A user who trusts the overlay and copies the flag gets a usage dump. That
// is the one place a beginner looks when they are already lost.
//
// The repair was not to write "four of the ten" into the overlay. A count in
// prose is a measurement with the arithmetic hidden, and this file exists
// because one of those rotted: the audit that found it said TWO screens, and
// it is four. Shortcut.JSON now records the fact per screen, the overlay
// derives its sentence from that field, and this test pins the field to the
// binary.
//
// So the check is: does the DECLARATION agree with what the command actually
// accepts. Not "is the flag there" — the answer may legitimately be no for a
// streaming screen. The defect is a screen whose declaration and whose
// command disagree, because that is what the user is shown.
func TestJC1_EveryScreenDeclaresItsJSONFlagCorrectly(t *testing.T) {
	var checked int
	for _, s := range tui.Shortcuts() {
		t.Run(s.Label, func(t *testing.T) {
			checked++
			var out, errb bytes.Buffer
			// --json with no other argument. A command that HAS the flag
			// fails later, on a missing path or a missing value; a command
			// that does not have it fails at parse time with this exact
			// sentence from the flag package.
			_ = run([]string{s.Command, "--json"}, &out, &errb)
			undefined := strings.Contains(errb.String()+out.String(),
				"flag provided but not defined: -json")

			if s.JSON && undefined {
				t.Errorf("screen %q (replay %s) declares JSON: true and the overlay "+
					"therefore offers --json, but the command rejects it at parse "+
					"time. A user copying the flag off the help screen gets a usage "+
					"dump.", s.Label, s.Command)
			}
			if !s.JSON && !undefined {
				t.Errorf("screen %q (replay %s) declares JSON: false, so the overlay "+
					"tells the user this screen has no machine-readable form — but "+
					"the command accepts --json. The declaration is understating the "+
					"tool, which is the same defect pointing the other way.",
					s.Label, s.Command)
			}
		})
	}
	// A loop that ran zero times passes. Shortcuts() is the only source of
	// screens, so if it ever returns empty this test would report a clean
	// bill on a surface it never looked at.
	if checked == 0 {
		t.Fatal("no screens were checked: Shortcuts() returned nothing, so this " +
			"test proved nothing about the overlay it exists to guard")
	}
}

// TestJC2 is the other half, and it is the half that actually rots. JC1 pins
// the field to the binary; this pins the OVERLAY to the field, so the
// sentence a user reads cannot drift from the data that backs it.
func TestJC2_TheOverlayDoesNotPromiseJSONEverywhere(t *testing.T) {
	var line string
	for _, b := range tui.Bindings() {
		if b.Keys == "--json" {
			line = b.Does
		}
	}
	if line == "" {
		t.Fatal("the overlay no longer lists --json at all. If that was deliberate " +
			"this test should go with it; if it was not, the flag has become " +
			"undiscoverable on every screen that does have it")
	}

	var missing []string
	for _, s := range tui.Shortcuts() {
		if !s.JSON {
			missing = append(missing, string(s.Key))
		}
	}
	if len(missing) == 0 {
		t.Skip("every screen's command now takes --json, so an unqualified " +
			"sentence is true and there is nothing to qualify")
	}
	for _, k := range missing {
		if !strings.Contains(line, k) {
			t.Errorf("screen key %q has no --json and the overlay line does not "+
				"name it: %q\nThe user is told the flag works here and it does not.", k, line)
		}
	}
}
