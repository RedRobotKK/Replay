package tui

import "fmt"

// Row selection.
//
// j/k, g/G, enter and the arrows were removed because nothing had rows to move
// between, and advertising a key that does nothing is lying in the one place a
// beginner looks. This is the change that earns them back.
//
// It also unblocks two screens. `why` and `context` answer questions about ONE
// session, not the corpus: `replay blame` and `replay context` both refuse
// without a transcript argument. They could not be measured because there was
// no way to say which session, and the answer to "why was it expensive" is
// meaningless averaged over 1,599 tasks anyway. Somebody wants to know about
// the one that cost forty dollars.

// Line is one selectable row: what it says, and what it stands for.
//
// Named Line rather than Row because Row is already the column formatter, and
// two meanings for one word in one package is how a reader ends up debugging
// the wrong function.
type Line struct {
	// Text is the rendered line, already fitted to its columns.
	Text string
	// Path is the transcript this row is about, passed to the command a
	// deeper screen runs. Empty for rows that are not sessions.
	Path string
	// Label names the row in a status line, so a deeper screen can say which
	// session it is answering about without the reader re-reading the list.
	Label string
}

// Selection is where the cursor is in a list, and how it moves.
//
// Held separately from the rows so the list can be replaced while the reader
// stays where they were: a repaint that resets the cursor to the top is a
// repaint that loses their place, which on a screen that redraws four times a
// second happens before they can read anything.
type Selection struct {
	At int
	// Window is how many rows are visible at once, so paging moves by a
	// screenful rather than by an arbitrary number.
	Window int
}

// Move applies a keystroke to a selection over n rows.
//
// Returns whether the key was consumed, so the caller can tell a movement key
// from one that means something else on this screen. A selection that silently
// swallows every key would make the shortcut letters stop working the moment a
// list appeared.
func (s *Selection) Move(k rune, n int) bool {
	if n <= 0 {
		return false
	}
	switch k {
	case 'j':
		s.At++
	case 'k':
		s.At--
	case 'H':
		s.At = 0
	case 'G':
		s.At = n - 1
	default:
		return false
	}
	// Clamp rather than wrap. Wrapping means holding j past the end jumps the
	// reader back to the top without their asking, and on a long list they
	// will not notice they have gone round.
	if s.At < 0 {
		s.At = 0
	}
	if s.At >= n {
		s.At = n - 1
	}
	return true
}

// Visible returns the slice of rows on screen and the cursor's index within it.
//
// The window follows the cursor rather than the cursor following the window: a
// list that scrolls only when the cursor hits the edge keeps the reader's eye
// still for as long as possible, which is the anchoring rule applied to a list.
func (s Selection) Visible(rows []Line) ([]Line, int) {
	if len(rows) == 0 {
		return nil, 0
	}
	w := s.Window
	if w <= 0 || w > len(rows) {
		w = len(rows)
	}
	start := s.At - w/2
	if start < 0 {
		start = 0
	}
	if start+w > len(rows) {
		start = len(rows) - w
	}
	return rows[start : start+w], s.At - start
}

// RenderRows draws a list with the cursor marked.
//
// The marker is two characters wide and present on every row, selected or not,
// so the text starts at the same column throughout. A cursor that indents the
// line it is on moves every other line as it travels, which is the shear the
// whole surface is built to avoid.
func RenderRows(rows []Line, cursor int) []string {
	out := make([]string, 0, len(rows))
	for i, r := range rows {
		mark := "  "
		if i == cursor {
			mark = "> "
		}
		line := mark + r.Text
		if len(line) > BudgetCols {
			line = line[:BudgetCols-1] + string(truncationMark)
		}
		out = append(out, line)
	}
	return out
}

// SelectedLine names what the cursor is on, for a status row.
func SelectedLine(rows []Line, cursor int) string {
	if cursor < 0 || cursor >= len(rows) {
		return ""
	}
	return fmt.Sprintf("  %d of %d   %s", cursor+1, len(rows), rows[cursor].Label)
}
