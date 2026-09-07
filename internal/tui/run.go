package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// Session wires the loop to a real terminal.
//
// Everything the loop needs is injected, so this file is the only place that
// touches a terminal at all and the only place that cannot be tested without
// one. That boundary is deliberate: the parts that decide whether the surface
// feels right are all on the other side of it.

// Start runs the surface until the user quits.
//
// It returns nil on a clean exit, including the exit somebody gets by pressing
// q, because quitting is not an error.
func Start(out io.Writer, src Source) error {
	if out == nil {
		out = os.Stdout
	}

	st, rawErr := rawMode()
	defer st.restore()

	// Alternate screen, cursor hidden. Both are undone on every exit path
	// including a signal, because a program that leaves the terminal in raw
	// mode with no cursor has broken the shell its user came back to.
	enter(out, rawErr == nil)
	restore := func() {
		st.restore()
		leave(out, rawErr == nil)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	stop := make(chan struct{})
	go func() {
		<-sig
		close(stop)
	}()

	keys := make(chan rune)
	go readKeys(os.Stdin, keys, rawErr == nil)

	l := &Loop{Out: out, Source: src, Keys: keys}
	l.Run(stop)
	restore()

	if rawErr != nil {
		// Said after the fact rather than before, so it does not delay the
		// first frame, and said at all because a user pressing keys that do
		// nothing deserves to know why.
		fmt.Fprintf(os.Stderr,
			"replay: this terminal would not take per-key input (%v), so keys needed "+
				"Enter. Everything else worked normally.\n", rawErr)
	}
	return nil
}

// readKeys turns bytes into keystrokes.
//
// In raw mode one byte is one key. In line mode a whole line arrives and only
// its first character is taken, which is what makes the fallback usable rather
// than merely present.
func readKeys(in io.Reader, out chan<- rune, raw bool) {
	defer close(out)
	if raw {
		buf := make([]byte, 1)
		for {
			n, err := in.Read(buf)
			if err != nil || n == 0 {
				return
			}
			out <- rune(buf[0])
		}
	}
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		out <- rune(line[0])
	}
}

// enter and leave bracket the alternate screen.
//
// Only when the terminal took raw mode. Writing escape sequences into a pipe
// produces a file full of control codes, which is the non-TTY failure the
// design already refuses.
func enter(w io.Writer, raw bool) {
	if !raw {
		return
	}
	_, _ = io.WriteString(w, "\x1b[?1049h\x1b[?25l\x1b[2J")
}

func leave(w io.Writer, raw bool) {
	if !raw {
		return
	}
	_, _ = io.WriteString(w, "\x1b[?25h\x1b[?1049l")
}
