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

// header renders a screen's title row: the command on the left, the version on
// the right, flush to the column budget.
//
// The padding is measured from the strings rather than assumed. The literals
// this replaced subtracted a hardcoded 6 for the width of "v0.4.0", so any
// other version would have sheared the right edge of every header at once.
func header(cmd string) string {
	left := "  replay " + cmd
	tag := versionTag()
	pad := Cols() - len(left) - len(tag)
	if pad < 1 {
		pad = 1
	}
	return left + spaces(pad) + tag
}
