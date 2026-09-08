package main

import (
	"os"
	"path/filepath"
	"testing"
)

// /dev/null is not a terminal.
//
// canHyperlink tested os.ModeCharDevice, and /dev/null is a character device.
// So `replay cost > /dev/null` satisfied "is a terminal" and emitted OSC 8
// sequences into a sink. It is the same shape as every other defect this
// project has found: a property that is nearly the question being asked, read
// as though it were the question.
//
// The names are TT rather than TF because tipfreq_test.go already owns TF, and
// two files numbering the same prefix is how a reader ends up reading the
// wrong test.

// TT-1: /dev/null is not a terminal.
//
// The exact case on record. A character device satisfied the old test, so a
// redirect to /dev/null was treated as an interactive terminal and got escape
// sequences written into it.
func TestTT1_DevNullIsNotATerminal(t *testing.T) {
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no %s here: %v", os.DevNull, err)
	}
	defer func() { _ = f.Close() }()
	t.Setenv("TERM", "xterm-256color")
	if canHyperlink(f) {
		t.Error("os.DevNull was treated as a terminal, so `replay cost > /dev/null` " +
			"writes OSC 8 sequences into a sink")
	}
}

// TT-2: a regular file is not a terminal either.
func TestTT2_AFileIsNotATerminal(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	t.Setenv("TERM", "xterm-256color")
	if canHyperlink(f) {
		t.Error("a regular file was treated as a terminal; an escape sequence in " +
			"`replay cost > out.txt` is corruption, not a convenience")
	}
}
