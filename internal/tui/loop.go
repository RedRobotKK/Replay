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
	// Start is the screen to open on. Zero falls back to the first shortcut.
	//
	// `replay tui --screen advise` is documented as "which question to open
	// on" and was honoured only by --once: StartWith built this struct with no
	// initial key, so the interactive path fell to Shortcuts()[0] and the flag
	// was silently dropped. Found by recording a demo, after two renders were
	// blamed on the wrong binary before the flag itself was suspected.
	Start rune
	// Keys carries input. Closing it ends the loop, which is what q does.
	Keys <-chan rune
	// Addressable says the destination is a terminal that took raw mode, so
	// the frame may move the cursor and clear rows in place.
	//
	// False is the safe default and means a pipe, a file, or an agent reading
	// on someone's behalf. enter and leave already refused to write the
	// alternate-screen sequences in that case; paint did not share the gate,
	// so every frame still carried cursor addressing and line clears into the
	// pipe. The reader got the text wrapped in control codes.
	Addressable bool

	mu      sync.Mutex
	cur     rune
	tick    int
	painted []string
	help    bool

	// sel is where the cursor sits in the current screen's list, and rows is
	// how many there are. Held here rather than in the source so a repaint
	// does not reset it: a cursor that jumps to the top four times a second
	// loses the reader's place before they can read anything.
	sel  Selection
	rows int
	// opened records that enter was pressed, so a source can act on it and
	// clear it. A flag rather than a callback, because a callback would run on
	// the key goroutine and the source is already called on the render path.
	opened bool

	// local is the current screen's own key handler, set by the source as it
	// renders and offered every keystroke first.
	//
	// A callback here and a flag for enter, which looks inconsistent and is
	// not: enter means the same thing on every screen that has rows, so the
	// loop can record it and let the source decide later. A screen's own keys
	// mean something only while that screen is on, and deciding them a repaint
	// later would apply them to whatever screen the reader had moved to.
	local func(rune) bool
}

// SetLocal gives the current screen first refusal on a keystroke.
//
// Called by the source as it renders, and cleared by every screen that has no
// keys of its own. The handler reports whether it consumed the key, so a
// screen that declines leaves the shortcut letters working: a surface where a
// question becomes unreachable because some screen swallowed its letter is a
// surface people get stuck in.
func (l *Loop) SetLocal(f func(rune) bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.local = f
}

// TakeOpened reports whether enter was pressed since the last call, and clears
// it. Read-and-clear so one keystroke opens one row.
func (l *Loop) TakeOpened() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	was := l.opened
	l.opened = false
	return was
}

// SetRows tells the loop how many selectable rows the current screen has.
//
// Called by the source as it renders. A screen with none reports zero, and the
// movement keys then decline the keystroke rather than swallowing it, which is
// what keeps the shortcut letters working everywhere.
func (l *Loop) SetRows(n int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rows = n
	if l.sel.At >= n {
		l.sel.At = n - 1
	}
	if l.sel.At < 0 {
		l.sel.At = 0
	}
}

// Cursor is where the selection sits, for a source that renders a list.
func (l *Loop) Cursor() Selection {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sel
}

// Run draws until Keys closes or ctx-like stop arrives via a closed channel.
//
// The ticker is the liveness cadence, the fastest of the four, because a loop
// that woke on the slowest could not advance the cue. Everything slower is
// derived from the tick count rather than from its own timer, which keeps one
// clock in the program and makes the relationship between the rates something
// a test can assert instead of something four tickers agree on by luck.
// first settles the opening screen, and paints nothing.
//
// Split out so the choice can be tested without a terminal, which is the one
// thing run.go says it cannot give.
func (l *Loop) first() {
	if l.cur == 0 {
		l.cur = l.Start
	}
	if l.cur == 0 {
		l.cur = Shortcuts()[0].Key
	}
	if l.Source != nil {
		l.Source(l.cur, 0)
	}
}

func (l *Loop) Run(stop <-chan struct{}) {
	if l.Now == nil {
		l.Now = time.Now
	}
	l.first()
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
	// The current screen first, and outside the lock: a handler that called
	// back into the loop while it was held would deadlock the keyboard, which
	// is the one thing this design says never to do.
	l.mu.Lock()
	local, helping := l.local, l.help
	l.mu.Unlock()
	if !helping && local != nil && local(k) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	// Movement first, and only when the screen has rows. Move reports whether
	// it consumed the key, so a screen with no list leaves j and k to fall
	// through rather than swallowing them silently.
	if !l.help && l.sel.Move(k, l.rows) {
		return
	}
	switch k {
	case '?':
		l.help = !l.help
	case 27: // escape
		l.help = false
	case '\r', '\n':
		// Enter opens the selected row. The source reads Cursor() and decides
		// what opening means; the loop does not know what a row stands for.
		l.opened = true
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
	if !l.Addressable {
		// Nothing to address. Write the frame as plain lines and skip the
		// diff: a pipe has no previous frame to leave stale, and repeating
		// an unchanged line costs a reader nothing while omitting it would
		// hand them a frame with holes in it.
		for _, line := range lines {
			b.WriteString(strings.TrimRight(line, " "))
			b.WriteByte('\n')
		}
		_, _ = io.WriteString(l.Out, b.String())
		l.painted = append(l.painted[:0], lines...)
		return
	}
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
