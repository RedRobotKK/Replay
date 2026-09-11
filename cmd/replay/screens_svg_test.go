package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The screens as committed images, generated from the screens themselves.
//
// This repository had no screenshots at all, on any surface: no image in the
// README, nothing rendered anywhere, and the only walkthrough of any part of
// the flow covered install.sh and stopped at the shell prompt. That is a real
// gap for a terminal tool whose whole argument is what it puts on screen.
//
// The reason not to fix it by taking screenshots is that a screenshot goes
// stale silently. It is a claim about what the software prints, made once, and
// then never checked again while the software moves underneath it. That is the
// defect class this project exists to catch, so shipping an unguarded PNG of a
// figure would be an odd thing to do.
//
// So the images are generated from the running screens and diffed against what
// is committed. A screen that changes fails this test until the image is
// regenerated, and the diff is readable because SVG is text: a reviewer sees
// which line changed rather than a binary blob swapping places.
//
//	go test ./cmd/replay -run TestScreenSVGs -update
//
// Determinism comes from the fixture, not from the machine. These render
// against the repository's own redacted session rather than whatever corpus
// the developer happens to have, which is the same lesson TC1 taught: a check
// whose inputs vary by machine is a check that passes at home and fails in CI,
// or worse, the other way round.

var updateScreens = flag.Bool("update", false, "rewrite the committed screen SVGs from the current output")

// screenDir is where the images live, beside the docs that embed them.
const screenDir = "../../docs/screens"

// unpinnable names screens whose content cannot be fixed, so no image of them
// is committed.
//
// doctor is the whole list, and the reason is the point of the list existing.
// Its job is to report this machine, today. Two of its rows move with HOME and
// are handled by pinning that. The third cannot be: "price table dated
// 2026-09-07, 1 days old" counts from the compiled table to the current date,
// so an image of it is correct for one day and wrong every day after.
//
// safe joined it for the same reason by a different route. It lists what
// Replay has written to this machine — store names, byte sizes, file counts —
// and the only store present under the pinned HOME is the cost index the test
// run creates for itself. Its size is a property of the run, so the committed
// image was correct on the machine that generated it and wrong on the next
// one: it passed here and failed on windows-latest, which is the good outcome
// of a bad image.
//
// Pinning it harder was available and is the wrong trade. The screen's whole
// job is to answer "what does this thing know about me", and an image of
// somebody else's disk is not an answer to that question — it is a picture of
// a screen nobody sees, which is the same objection that keeps doctor off this
// list.
//
// The options were to inject a clock into production code so a screenshot
// could be stable, or to normalise the line in the image and publish a
// rendering of a screen nobody sees. Both are worse than not shipping this
// one image. A test that fails every morning is a test somebody switches off,
// and it would take the nine that do work with it.
var unpinnable = map[string]bool{"doctor": true, "safe": true}

// sgrColour maps the palette to what the SVG paints.
//
// One place, and it is the same set internal/tui/color.go emits: TestCL2 pins
// that a screen can produce no other code, so a sequence arriving here that is
// not in this map is a palette change that skipped its own guard.
var sgrColour = map[string]string{
	"1":  "#e6e8ec", // Strong: bold reads as brighter here, since SVG has no weight axis to spare
	"2":  "#767d87", // Faint
	"36": "#3fc7c9", // Accent
	"32": "#7dc47d", // Good
	"33": "#d6a854", // Warn
	"31": "#e08585", // Alarm
}

func TestScreenSVGs(t *testing.T) {
	corpus(t)
	// A locale with no conversion, so the images do not carry one machine's
	// currency. The currency layer has its own tests; these are about layout.
	t.Setenv("LC_ALL", "en_US.UTF-8")

	// Point the live screen at a port nothing listens on.
	//
	// It is the one screen that reads a socket rather than a file, and the
	// first version of this test did not account for that. It captured the
	// maintainer's actual proxy, "proxy up 35h49m", whose uptime changes every
	// minute: the committed image would have failed its own drift check on the
	// next run, and rendered something different again on a CI runner with no
	// proxy at all.
	//
	// A screenshot generator whose output depends on what happens to be
	// running is the same defect as a test whose fixture depends on the
	// machine, which this file already avoids for transcripts and then walked
	// into for the network.
	//
	// The unreachable state is also the right one to publish. A reader looking
	// at these images has not started a proxy, so the screen that says how to
	// is the screen they will actually meet.
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:1")

	// A clean HOME, so nothing about the developer's own machine reaches an
	// image. Without it the doctor screen reported "~/.replay/ledger, writable"
	// and "4 probe readings, across 4 model(s)" here, against "NOT writable"
	// and "none" on a runner with no ~/.replay at all.
	home := t.TempDir()
	t.Setenv("HOME", home)
	// USERPROFILE as well, because os.UserHomeDir reads that on Windows and
	// HOME is ignored there. Pinning only HOME would leave the Windows job
	// rendering against the runner's real profile, which is the same
	// machine-dependence this whole block exists to remove, hidden on the one
	// platform nobody here develops on.
	t.Setenv("USERPROFILE", home)

	colourEnv(t)
	for _, name := range tuiScreens {
		if unpinnable[name] {
			continue
		}
		painted := render(t, "tui", "-once", "-screen", name, "-color", "always")
		got := svgFor(name, painted)
		path := filepath.Join(screenDir, name+".svg")

		if *updateScreens {
			if err := os.MkdirAll(screenDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("no committed image for the %s screen: %v\n"+
				"Run: go test ./cmd/replay -run TestScreenSVGs -update", name, err)
			continue
		}
		if string(want) != got {
			t.Errorf("the %s screen no longer matches its committed image.\n"+
				"A screenshot that drifts from the screen is a claim nobody is checking, "+
				"which is what this test exists to prevent.\n"+
				"Run: go test ./cmd/replay -run TestScreenSVGs -update", name)
		}
	}
}

// svgFor renders one captured frame as an SVG.
//
// Text, not pixels, for three reasons that all matter here. A diff shows which
// line changed. Nothing needs a browser or a headless renderer to produce it,
// so it runs in the same CI job as everything else. And it stays legible when
// somebody views the raw file, which a base64 PNG does not.
func svgFor(name, frame string) string {
	const (
		cw, lh  = 8.4, 19.0 // a monospace cell at 14px
		padX    = 16.0
		padY    = 26.0
		bg      = "#14161a"
		fgPlain = "#d9dce1"
	)
	lines := strings.Split(strings.TrimRight(frame, "\n"), "\n")
	w := padX*2 + cw*80
	h := padY + lh*float64(len(lines)) + padX

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img" aria-label="replay %s">`,
		w, h, w, h, html.EscapeString(name))
	b.WriteString("\n")
	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" rx="8" fill="%s"/>`, w, h, bg)
	b.WriteString("\n")
	b.WriteString(`<g font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" font-size="14">` + "\n")

	for i, line := range lines {
		y := padY + lh*float64(i)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" xml:space="preserve" fill="%s">`, padX, y, fgPlain)
		for _, run := range splitSGR(line) {
			fill := sgrColour[run.code]
			if run.code == "" || fill == "" {
				b.WriteString(html.EscapeString(run.text))
				continue
			}
			fmt.Fprintf(&b, `<tspan fill="%s">%s</tspan>`, fill, html.EscapeString(run.text))
		}
		b.WriteString("</text>\n")
	}
	b.WriteString("</g>\n</svg>\n")
	return b.String()
}

// coloured is a run of text under one SGR code, empty for unstyled.
type coloured struct{ code, text string }

// splitSGR breaks a line into runs by the colour in force.
//
// It handles only what internal/tui/color.go emits: one code, then a reset.
// SGR does not nest and TestCL6 asserts no screen tries to, so a stack here
// would be machinery for a case that is separately guaranteed not to arise.
func splitSGR(line string) []coloured {
	var out []coloured
	cur := coloured{}
	for i := 0; i < len(line); {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && (line[j] == ';' || (line[j] >= '0' && line[j] <= '9')) {
				j++
			}
			if j < len(line) && line[j] == 'm' {
				if cur.text != "" {
					out = append(out, cur)
				}
				code := line[i+2 : j]
				if code == "0" {
					code = ""
				}
				cur = coloured{code: code}
				i = j + 1
				continue
			}
		}
		cur.text += string(line[i])
		i++
	}
	if cur.text != "" {
		out = append(out, cur)
	}
	return out
}

// TestScreenSVGsAreNotEmpty stops the guard above passing vacuously.
//
// If tuiScreens were emptied, or the renderer produced nothing, every
// comparison would be over zero screens and the test would report success.
// This is the same shape of check the colour and ed25519 guards each needed,
// and each needed it because the first version did not have it.
func TestScreenSVGsAreNotEmpty(t *testing.T) {
	if len(tuiScreens) < 9 {
		t.Fatalf("only %d screens listed; this surface has ten questions and the image "+
			"guard asserts nothing about the ones missing from the list", len(tuiScreens))
	}
	for _, name := range tuiScreens {
		if unpinnable[name] {
			continue
		}
		path := filepath.Join(screenDir, name+".svg")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s has no committed image: %v", name, err)
			continue
		}
		if !bytes.Contains(b, []byte("<text")) {
			t.Errorf("%s.svg carries no text, so it is an image of nothing", name)
		}
	}
}
