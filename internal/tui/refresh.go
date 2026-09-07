package tui

import (
	"fmt"
	"strings"
	"time"
)

// A screen that lives on a second monitor is not a screen anybody reads.
//
// It is glanced at, from two metres, between other things, and the question
// being asked is never "what are the numbers". It is "do I need to do
// something". Everything below follows from that.
//
// Three consequences, and each one contradicts an instinct:
//
//	Refresh is not one rate. Different things go stale at different speeds, and
//	repainting a spend total sixty times a second costs a laptop battery to tell
//	somebody a number that changed in the third decimal place.
//
//	New data must not move old data. On a screen being read, a row arriving at
//	the top is fine. On a screen being glanced at, it means the thing your eye
//	was trained on is now one row lower, and you re-read instead of recognising.
//
//	The most valuable line is not the newest. It is the one that answers the
//	question, and it belongs where the eye lands first and never moves.

// Refresh cadences. Named rather than sprinkled, because a rate chosen once and
// forgotten is how a monitoring screen becomes a space heater.
const (
	// TickLiveness is how often the cue advances. Fast enough to read as alive,
	// slow enough that a screen nobody is watching costs nothing. Four frames a
	// second is the floor at which motion still reads as motion.
	TickLiveness = 250 * time.Millisecond

	// TickTraffic is how often the request log redraws. Requests arrive in
	// bursts and a human cannot follow rows faster than this anyway; anything
	// quicker is animation rather than information.
	TickTraffic = 1 * time.Second

	// TickTotals is how often accumulated figures redraw. A spend total that
	// updates every frame is a number nobody can read, and reading it is the
	// entire point.
	TickTotals = 5 * time.Second

	// TickAmbient is how often the screen redraws when nothing has happened.
	// It is not zero: the liveness rule says a still frame is indistinguishable
	// from a dead process. It is slow, because proving you are alive is the
	// only work being done.
	TickAmbient = 2 * time.Second
)

// Attention is how loudly a screen is asking for someone.
type Attention int

const (
	// Calm means the numbers moved and nothing needs a person. The overwhelming
	// majority of the time a monitor is on, this is the state.
	Calm Attention = iota
	// Notice means something a person should know about, at their convenience.
	Notice
	// Act means the screen is asking for someone now.
	Act
)

// Glance is the one line that has to be readable without focusing.
//
// It sits on the same row on every screen and never moves, because a signal
// that changes position is a signal the eye has to search for, and searching is
// the thing a glance cannot afford.
//
// The marker is text rather than colour alone. At two metres on a badly
// calibrated monitor, colour is a hint; the word is the message. That is also
// the sixteen-colour rule holding at a distance instead of over SSH.
func Glance(a Attention, what string) string {
	// Bracketed, so the marker has a SHAPE and not only a word.
	//
	// The first version was "  ok  " / " note " / " ACT  ": fixed width, which
	// solved the reflow problem, and interchangeable silhouettes, which did not
	// solve the reading one. Brackets give each state a distinct outline, so it
	// is legible squinting, off-axis, on a badly calibrated monitor, and to a
	// reader who cannot separate the colours at all. Colour is the third signal
	// after shape and word, not the first.
	mark := map[Attention]string{
		Calm:   "[ OK ]",
		Notice: "[NOTE]",
		Act:    "[CRIT]",
	}[a]
	line := mark + " " + what
	if len(line) > BudgetCols {
		line = line[:BudgetCols-1] + string(truncationMark)
	}
	return line
}

// Since answers the question somebody actually has when they come back after
// three hours: what did I miss.
//
// A live monitor that only shows now is a monitor that punishes you for looking
// away, which is the one thing its owner will certainly do. The window is
// deliberately what happened, not what is happening.
func Since(d time.Duration, requests int, breaks int, fired []string) []string {
	out := []string{fmt.Sprintf("  while you were away, %s", humanFor(d))}
	out = append(out, fmt.Sprintf("  %-24s%d", "requests", requests))
	out = append(out, fmt.Sprintf("  %-24s%d", "cache breaks", breaks))
	if len(fired) == 0 {
		out = append(out, "  "+cell("guards fired", 24)+"none")
		return out
	}
	out = append(out, "  "+cell("guards fired", 24)+strings.Join(fired, ", "))
	return out
}

// humanFor writes a duration the way somebody says it out loud.
func humanFor(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "under a minute"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}

// Anchored reports whether a body of rows keeps the reader's eye still.
//
// New rows go at the bottom of a fixed window and the window scrolls its
// contents, so the header, the glance line and the footer are on the same rows
// forever. The alternative, unshifting the list from the top, moves every row
// the reader had already located.
func Anchored(prev, next []string) bool {
	if len(prev) != len(next) {
		return false
	}
	// Row count alone is not the anchor. A window can keep its height while
	// every row inside it changes width, and a row that widens pushes nothing
	// vertically but reflows the column beside it, which moves the figure the
	// reader had located. The first version of this function checked only the
	// count, and a mutation replacing its whole body with "return true" passed:
	// it was a check that could not fail.
	for i := range prev {
		if len(prev[i]) != len(next[i]) {
			return false
		}
	}
	return true
}
