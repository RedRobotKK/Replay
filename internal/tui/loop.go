package tui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// The render loop.
//
// Everything before this was a design that could be read. This is the part that
// runs, and it is written so the parts that decide whether it feels right are
// testable without a terminal: the clock is injected, the output is an
// io.Writer, and input arrives on a channel. A loop that can only be judged by
// staring at it is a loop nobody can hold to a standard.
//
// Three properties it has to keep, all of them stated earlier as design and
// none of them true until something redraws:
//
//	the cadences are real, so a quiet screen still proves it is alive
//	nothing moves that did not change, so the reader's eye stays where it was
//	the surface never blocks, so a slow ledger read cannot freeze the keys

// Frame is one rendered screen: the lines, and which shortcut produced them.
type Frame struct {
	Key   rune
	Lines []string
}

// Source produces the current frame for a screen. It is called on the loop's
// own schedule and must not block: a source that waits on a disk read hands
// the delay to the keyboard.
type Source func(key rune, tick int) Frame

// Loop drives the surface.
type Loop struct {
	Out io.Writer
	// Now is injected so the cadence can be tested without waiting for it.
	Now func() time.Time
	// Source renders the current screen.
	Source Source
	// Keys carries input. Closing it ends the loop, which is what q does.
	Keys <-chan rune

	mu      sync.Mutex
	cur     rune
	tick    int
	painted []string
	help    bool
}

// Run draws until Keys closes or ctx-like stop arrives via a closed channel.
//
// The ticker is the liveness cadence, the fastest of the four, because a loop
// that woke on the slowest could not advance the cue. Everything slower is
// derived from the tick count rather than from its own timer, which keeps one
// clock in the program and makes the relationship between the rates something
// a test can assert instead of something four tickers agree on by luck.
func (l *Loop) Run(stop <-chan struct{}) {
	if l.Now == nil {
		l.Now = time.Now
	}
	if l.cur == 0 {
		l.cur = Shortcuts()[0].Key
	}
	t := time.NewTicker(TickLiveness)
	defer t.Stop()

	l.paint()
	for {
		select {
		case <-stop:
			return
		case k, ok := <-l.Keys:
			if !ok || k == 'q' {
				return
			}
			l.press(k)
			l.paint()
		case <-t.C:
			l.mu.Lock()
			l.tick++
			l.mu.Unlock()
			l.paint()
		}
	}
}

// press applies a keystroke.
//
// Contextual intelligence: what a key does depends on where you are. Escape
// closes the help overlay if it is open and otherwise does nothing, rather than
// quitting, because a surface that exits on the key people press to back out is
// a surface people lose work in.
func (l *Loop) press(k rune) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch k {
	case '?':
		l.help = !l.help
	case 27: // escape
		l.help = false
	default:
		for _, s := range Shortcuts() {
			if s.Key == k {
				l.cur = k
				l.help = false
			}
		}
	}
}

// paint renders and writes only what changed.
//
// Redrawing every row every quarter second is how a monitoring screen becomes a
// space heater and how a terminal over SSH becomes unusable. Comparing against
// the last frame costs one string compare per row and means a quiet screen
// writes one line: the one carrying the cue that proves it is alive.
func (l *Loop) paint() {
	l.mu.Lock()
	key, tick, help := l.cur, l.tick, l.help
	l.mu.Unlock()

	var lines []string
	if help {
		lines = Help()
	} else if l.Source != nil {
		lines = l.Source(key, tick).Lines
	}
	for len(lines) < BudgetRows-1 {
		lines = append(lines, "")
	}
	if len(lines) > BudgetRows-1 {
		lines = lines[:BudgetRows-1]
	}
	lines = append(lines, Footer(key))

	var b strings.Builder
	for i, line := range lines {
		if i < len(l.painted) && l.painted[i] == line {
			continue
		}
		// Move to the row, clear it, write it. Clearing matters: without it a
		// shorter line leaves the tail of the previous frame on screen, which
		// is how a stale value survives a repaint and gets read as current.
		fmt.Fprintf(&b, "\x1b[%d;1H\x1b[2K%s", i+1, line)
	}
	if b.Len() == 0 {
		return
	}
	// Park the cursor where it cannot sit inside the data.
	fmt.Fprintf(&b, "\x1b[%d;1H", BudgetRows)
	_, _ = io.WriteString(l.Out, b.String())

	l.painted = append(l.painted[:0], lines...)
}

// Painted exposes the last frame for tests, which is the only honest way to
// assert what a reader would have seen.
func (l *Loop) Painted() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.painted...)
}
