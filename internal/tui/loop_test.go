package tui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// drive runs a loop against a scripted set of keys and returns what was written.
func drive(t *testing.T, src Source, keys ...rune) (*Loop, string) {
	t.Helper()
	var out bytes.Buffer
	ch := make(chan rune, len(keys))
	for _, k := range keys {
		ch <- k
	}
	close(ch)
	l := &Loop{Out: &out, Source: src, Keys: ch}
	done := make(chan struct{})
	go func() { l.Run(nil); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the loop did not return when its key channel closed, so nothing " +
			"a user does can make it stop")
	}
	return l, out.String()
}

func plain(key rune, tick int) Frame {
	return Frame{Key: key, Lines: []string{"  screen " + string(key), "", "  " + Pinwheel(tick)}}
}

// A quiet screen writes one line, not a screen.
//
// Repainting every row four times a second is how a monitoring surface becomes
// a space heater and how a terminal over SSH becomes unusable. The whole reason
// to diff against the last frame is that the common case, nothing happening, is
// the case that runs for eight hours.
func TestAQuietRedrawWritesOnlyWhatMoved(t *testing.T) {
	var out bytes.Buffer
	l := &Loop{Out: &out, Source: plain, Keys: make(chan rune)}
	l.cur = 'c'
	l.paint()
	first := out.Len()
	if first == 0 {
		t.Fatal("the first paint wrote nothing")
	}
	out.Reset()
	l.paint() // identical frame
	if out.Len() != 0 {
		t.Errorf("an unchanged frame wrote %d bytes; it should write none:\n%q",
			out.Len(), out.String())
	}
	out.Reset()
	l.tick++ // only the cue advances
	l.paint()
	got := out.String()
	if got == "" {
		t.Fatal("the cue advanced and nothing was written, so the screen is frozen")
	}
	if n := strings.Count(got, "\x1b[2K"); n > 2 {
		t.Errorf("advancing one cue rewrote %d rows. The reader's eye is being asked "+
			"to re-read a screen that did not change:\n%q", n, got)
	}
}

// Every row is cleared before it is written.
//
// Without the clear, a shorter line leaves the tail of the previous frame on
// screen. That is not a cosmetic problem: it is how a stale value survives a
// repaint and gets read as current, which is the failure this whole surface
// exists to avoid.
func TestEveryWrittenRowIsClearedFirst(t *testing.T) {
	var out bytes.Buffer
	l := &Loop{Out: &out, Source: func(_ rune, tick int) Frame {
		if tick == 0 {
			return Frame{Lines: []string{"  a much longer line than the next one"}}
		}
		return Frame{Lines: []string{"  short"}}
	}, Keys: make(chan rune)}
	l.cur = 'c'
	l.paint()
	out.Reset()
	l.tick = 1
	l.paint()
	got := out.String()
	if !strings.Contains(got, "\x1b[2K") {
		t.Errorf("a row was rewritten without being cleared, so the tail of the longer "+
			"line stays on screen:\n%q", got)
	}
}

// The loop stops when the user asks.
func TestQEndsTheLoop(t *testing.T) {
	_, _ = drive(t, plain, 'q')
}

// A screen key switches screens; help toggles over them.
func TestKeysSwitchScreensAndEscapeClosesHelp(t *testing.T) {
	l, _ := drive(t, plain, 'g', '?', 27, 'q')
	painted := strings.Join(l.Painted(), "\n")
	if !strings.Contains(painted, "screen g") {
		t.Errorf("g did not switch to the guards screen, or escape did not close the "+
			"overlay over it:\n%s", painted)
	}
	if strings.Contains(painted, "the questions") {
		t.Error("escape left the help overlay open. Escape has to back out of things, " +
			"or the key people press to escape is the one that traps them")
	}
}

// Escape must not quit.
//
// A surface that exits on the key people press to back out of something is a
// surface they lose their place in, and they will press it: escape is what
// every other terminal program has trained them to try first.
func TestEscapeDoesNotQuit(t *testing.T) {
	var out bytes.Buffer
	ch := make(chan rune, 2)
	ch <- 27
	l := &Loop{Out: &out, Source: plain, Keys: ch}
	done := make(chan struct{})
	go func() { l.Run(nil); close(done) }()
	select {
	case <-done:
		t.Fatal("escape ended the loop")
	case <-time.After(150 * time.Millisecond):
	}
	close(ch)
	<-done
}

// The footer is always the last row, whatever the screen did.
func TestTheFooterIsAlwaysTheLastRow(t *testing.T) {
	for _, n := range []int{0, 3, 40} {
		var out bytes.Buffer
		l := &Loop{Out: &out, Keys: make(chan rune), Source: func(_ rune, _ int) Frame {
			lines := make([]string, n)
			for i := range lines {
				lines[i] = "  row"
			}
			return Frame{Lines: lines}
		}}
		l.cur = 'c'
		l.paint()
		got := l.Painted()
		if len(got) != BudgetRows {
			t.Errorf("a %d-row screen painted %d rows, budget is %d", n, len(got), BudgetRows)
		}
		if !strings.Contains(got[len(got)-1], "q quit") {
			t.Errorf("a %d-row screen did not end with the footer, so the way out "+
				"scrolled off: %q", n, got[len(got)-1])
		}
	}
}

// Line mode must be usable, not merely present.
//
// A terminal that will not take raw mode is not an error state: piping, CI, a
// restricted shell, Windows. Taking the first character of the line means the
// same keys work, with a return after them, instead of the surface refusing to
// start.
func TestLineModeTakesTheFirstCharacterOfALine(t *testing.T) {
	in := strings.NewReader("guards\n?\nq\n")
	keys := make(chan rune, 8)
	readKeys(in, keys, false)
	var got []rune
	for k := range keys {
		got = append(got, k)
	}
	if string(got) != "g?q" {
		t.Errorf("line mode read %q, want %q. Typing a word should press its first "+
			"key, or the fallback is a fallback nobody can use", string(got), "g?q")
	}
}

// Raw mode reads one byte as one key.
func TestRawModeReadsSingleKeys(t *testing.T) {
	keys := make(chan rune, 8)
	readKeys(strings.NewReader("g?q"), keys, true)
	var got []rune
	for k := range keys {
		got = append(got, k)
	}
	if string(got) != "g?q" {
		t.Errorf("raw mode read %q, want %q", string(got), "g?q")
	}
}

// Escape sequences must never reach a pipe.
//
// The design's own non-TTY rule: piping produces a readable log, not a file of
// control codes. The alternate-screen switch is the loudest way to break that,
// so it is bracketed on the same condition as raw mode.
func TestAPipeGetsNoEscapeSequences(t *testing.T) {
	var out bytes.Buffer
	enter(&out, false)
	leave(&out, false)
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes of terminal control into something that is not a "+
			"terminal:\n%q", out.Len(), out.String())
	}
	enter(&out, true)
	if !strings.Contains(out.String(), "\x1b[?1049h") {
		t.Error("a real terminal did not get the alternate screen, so the surface " +
			"would scribble over the user's scrollback")
	}
}

// Every key the surface advertises must do something.
//
// The footer said "j k move" and j did nothing. Up, down, enter, g, G and /
// were all in the vocabulary and none reached press(). A surface that lists a
// key it does not handle is lying in the one place a beginner looks, and this
// audience is explicitly people who do not know the tool.
//
// It is the same defect as a status outrunning its evidence, moved to the
// keyboard: the help text is a claim, and nothing checked it against the code.
func TestEveryAdvertisedKeyIsHandled(t *testing.T) {
	l := &Loop{Out: &bytes.Buffer{}, Source: plain, Keys: make(chan rune)}
	l.cur = 'c'

	for _, b := range Bindings() {
		if b.Layer == L3 {
			continue // documentation, not keystrokes
		}
		for _, k := range keysOfBinding(b.Keys) {
			// Start somewhere the key can move away from, and with the overlay
			// open so escape has something to close. Without this the test
			// reports its own starting position as a defect.
			l.cur, l.help = 'd', true
			before := snapshot(l)
			l.press(k)
			after := snapshot(l)
			if before == after && k != 'q' {
				t.Errorf("%q is advertised as %q and changes nothing when pressed. "+
					"Either handle it or stop offering it: a key listed in the footer "+
					"is a promise to somebody who does not know this tool",
					string(k), b.Does)
			}
			l.cur, l.help = 'c', false
		}
	}
}

// No key may mean two things.
//
// g was bound to the guards screen as a shortcut and to "first row" in the vim
// layer at the same time. One of them was never going to happen, and the help
// overlay listed both.
func TestNoKeyIsBoundTwice(t *testing.T) {
	seen := map[rune]string{}
	for _, b := range Bindings() {
		if b.Layer == L3 {
			continue
		}
		for _, k := range keysOfBinding(b.Keys) {
			if prev, dup := seen[k]; dup {
				t.Errorf("%q means both %q and %q. The overlay lists both and only one "+
					"can happen", string(k), prev, b.Does)
			}
			seen[k] = b.Does
		}
	}
}

// keysOfBinding turns a binding label into the runes it stands for.
func keysOfBinding(s string) []rune {
	switch s {
	case "up down", "enter":
		return nil // named keys, not printable runes; covered separately
	case "esc":
		return []rune{27}
	}
	var out []rune
	for _, f := range strings.Fields(s) {
		if len([]rune(f)) == 1 {
			out = append(out, []rune(f)[0])
		}
	}
	return out
}

// snapshot captures the loop state a keystroke could change.
func snapshot(l *Loop) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return string(l.cur) + fmt.Sprint(l.help)
}
