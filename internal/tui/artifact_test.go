package tui

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/version"
)

// A frame written to something that is not a terminal must be plain text.
//
// enter and leave already refuse to write the alternate-screen sequences when
// the terminal did not take raw mode, and say so in a comment. Paint did not
// share the gate, so cursor addressing and line clears went into the pipe
// anyway: piping the surface to a file, a pager, or an agent reading it on a
// user's behalf produced a wall of control codes around the text.
func TestPaintWritesNoEscapesWhenNotAddressable(t *testing.T) {
	var b strings.Builder
	l := &Loop{
		Out:    &b,
		Now:    func() time.Time { return time.Unix(0, 0) },
		Source: func(rune, int) Frame { return Frame{Lines: []string{"one", "two"}} },
	}
	l.paint()
	if got := b.String(); strings.ContainsRune(got, 0x1b) {
		t.Fatalf("escape byte in non-addressable output:\n%q", got)
	}
	if !strings.Contains(b.String(), "one") {
		t.Fatalf("content missing entirely:\n%q", b.String())
	}
}

// No screen may carry a version string of its own.
//
// Every header spelled "v0.4.0" as a literal, so the surface would have gone on
// reporting 0.4.0 out of a 0.5.0 binary. The version is injected at link time
// for exactly this reason and the screens must read it.
func TestNoScreenHardcodesAVersion(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") {
				continue // a comment may name the defect it fixed
			}
			if strings.Contains(line, `"v0.`) {
				t.Errorf("%s:%d hardcodes a version; read version.Version instead:\n\t%s",
					f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// And the rendered header must actually carry the injected value.
func TestHeaderShowsTheInjectedVersion(t *testing.T) {
	got := DoctorScreen(Machine{}).String()
	if !strings.Contains(got, version.Version) {
		t.Fatalf("header does not show version %q:\n%s", version.Version, got)
	}
}

// q ends the loop on its own, not because the key channel closed.
//
// The existing coverage drove 'q' through a helper that closes the channel
// after sending, and Run returns on either event, so the assertion could not
// tell the two apart: 'q' could have done nothing at all and the test would
// still have passed. Here the channel stays open, so only the key can end it.
//
// This is the key that gets a reader out of a full-screen surface they did not
// choose to be in — the installer opens it for them — so it is the one key
// whose failure traps somebody.
func TestQEndsTheLoopWithoutClosingTheChannel(t *testing.T) {
	keys := make(chan rune)
	defer close(keys)

	l := &Loop{
		Out:    io.Discard,
		Now:    func() time.Time { return time.Unix(0, 0) },
		Source: func(rune, int) Frame { return Frame{Lines: []string{"  1 thing"}} },
		Keys:   keys,
	}
	done := make(chan struct{})
	go func() { l.Run(nil); close(done) }()

	keys <- 'q'
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("q did not end the loop while the key channel stayed open, so the " +
			"only way out of the surface is to kill the process")
	}
}
