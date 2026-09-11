package tui

import (
	"strings"

	"github.com/RedRobotKK/Replay/internal/version"
)

// versionTag is what the header shows on the right.
//
// It reads the value injected at link time. Every screen used to spell "v0.4.0"
// as a literal, so a 0.5.0 binary would have gone on reporting 0.4.0 on every
// screen it drew — a figure on screen that did not come from the thing it named,
// which is the defect this tool exists to find in other people's bills.
func versionTag() string {
	v := version.Version
	if v == "" || v == "dev" || v == "unknown" {
		return "dev"
	}
	return "v" + strings.TrimPrefix(v, "v")
}

// screenNamed finds one of the ten questions by the name the -screen flag uses.
//
// The label is the join between the two: it is what a reader types, what the
// key strip prints, and what every screen builder already passes to header.
// Looking it up here means a screen cannot be titled by a question it does not
// answer, because there is only one place the pairing is written down.
func screenNamed(label string) (Shortcut, bool) {
	for _, s := range Shortcuts() {
		if s.Label == label {
			return s, true
		}
	}
	return Shortcut{}, false
}

// header renders a screen's title row.
//
// The title is the QUESTION the screen answers, not the command that answered
// it. Shortcut.Question has said "what the user actually wants to know, in
// their words" since the surface was designed and nothing rendered it: every
// screen was headed `replay cost`, `replay why`, `replay guards`. Naming a
// panel by its query is legible to whoever wrote the query and to nobody else,
// and this surface is explicitly for people who will never type a flag.
//
// The command stays on the row, set right beside the version. It is this
// design's stated cheapest honesty — a surface that runs commands on your
// behalf and does not show them is asking for trust it has not earned — and
// readers copy it. Putting it here rather than on a line of its own is what
// makes the change cost nothing: the body budget is unchanged and the doctor
// screen's worst case, which already fills its body exactly, keeps every note.
//
// It is also the first time that command has been true. `replay why`, `replay
// guards`, `replay safe`, `replay model` and `replay live` are not commands and
// never were; the header printed the screen's label and called it an
// invocation. It now prints Shortcut.Command, which is the subcommand actually
// run. The flags stay on the `ran` line, which has the room to be exact.
//
// serve and settings are storyboard illustrations rather than questions, so
// they fall through to the old form: the command is still the best name a
// screen has when it has no question.
func header(label string) string {
	if s, ok := screenNamed(label); ok {
		return titleRow(s.Question, "replay "+s.Command)
	}
	return titleRow("replay "+label, "")
}

// screenHead is the two rows every one of the ten screens opens with: the
// question it answers, and the blank that separates it from the body.
//
// Two rows, which is exactly what the title and its blank cost before, so
// titling ten screens by their question costs nothing from any body budget.
// That is the constraint the layout was chosen against: pad(), padCost(),
// padWhy() and padShare() truncate from the bottom in silence, and the doctor
// screen's worst case already fills its body exactly, so a header block one row
// taller would have deleted a staleness warning and said nothing.
func screenHead(label string) []string {
	return []string{header(label), ""}
}

// titleRow lays out the title, the command that produced the screen, and the
// version, flush to the measured width.
//
// It degrades rather than shears, in the order storyboard.go scene 25 sets for
// a narrow terminal: the least of it goes first. The command goes when there is
// no room for it, because it is recoverable — the `ran` line has it, the footer
// names the screen, and the reader typed something to get here. The title is
// cut last and only when the version tag alone will not share the row with it.
//
// The padding is measured from the strings rather than assumed. The literals
// this replaced subtracted a hardcoded 6 for the width of "v0.4.0", so any
// other version would have sheared the right edge of every header at once.
func titleRow(title, cmd string) string {
	tag := versionTag()
	left := "  " + title
	if cmd != "" {
		// Two spaces each side of the command, so it reads as its own field
		// rather than as the tail of the question.
		if pad := Cols() - len(left) - len(cmd) - 2 - len(tag); pad >= 2 {
			return left + spaces(pad) + cmd + "  " + tag
		}
	}
	pad := Cols() - len(left) - len(tag)
	if pad < 1 {
		// Nothing left to drop. Cut the title rather than let the row wrap,
		// because a wrapped title row moves every line of the screen under it.
		left = truncate(left, Cols()-len(tag)-1)
		pad = 1
	}
	return left + spaces(pad) + tag
}
