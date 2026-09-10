package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/selfupdate"
)

// buildNotice is what `replay doctor` says about the binary printing the
// report.
//
// It is the caller selfupdate.StaleNotice never had. That function is pure
// arithmetic on the RFC 3339 stamp linked in at release time -- it reaches
// nothing, needs no account, and was exported and tested from the day the
// package landed -- and no code path in this tree invoked it. So the only way
// to learn a newer release existed was `replay upgrade --check`, a command an
// operator types when they already suspect the answer. The operator the package
// was written for is the one who did not suspect it: this project ran 0.4.0 for
// three days while 0.5.4 was published, and the upgrade happened because
// somebody re-read the install instructions by chance (upgrade.go:20).
//
// Why doctor and not everywhere. README's Footprint section promises one ask at
// most once every thirty days, and a line printed under every report is one a
// reader learns to skip -- which costs more than the line is worth, because the
// last paragraph is often where the caveat is. doctor is the surface an
// operator types to ask what is wrong here, it already ages the rules document
// two lines down, and doctor.go's own comment claimed "`replay upgrade` tells
// an operator when the BINARY is stale" while nothing did. This makes that
// sentence true where the reader is already asking the question.
//
// Local arithmetic only. Nothing here reaches the network; a version check
// would be an outbound request the user did not type, which is the drift
// cmd/replay/outbound_drift_test.go exists to catch.
//
// Three states, kept apart (ADR-0018):
//
//	current   named and dated, no warning
//	old       named and dated, plus StaleNotice's own sentence
//	unaged    a source build, a stamp this binary cannot read, or one dated
//	          ahead of this clock -- said out loud, never passed off as current
//
// The unaged case is the one that has to be spelled rather than inferred.
// StaleNotice returns "" for all three of those alongside the current case, and
// a caller that printed nothing on "" would render an unreadable stamp and a
// fresh build identically: the reading that suppresses the warning is the one
// that hides the problem.
func buildNotice(ver, commit, buildDate string, now time.Time) string {
	var b strings.Builder
	ver = strings.TrimSpace(ver)
	commit = strings.TrimSpace(commit)
	buildDate = strings.TrimSpace(buildDate)

	name := ver
	if name == "" {
		name = "unknown"
	}
	if commit != "" && commit != "unknown" {
		name += " (" + commit + ")"
	}

	// A source build first, and before the stamp is even looked at. `replay
	// upgrade` refuses to overwrite one (upgrade.go:78), so calling it stale
	// would be advice the binary itself declines to take. It is also the case
	// every contributor sees, and a nag on `go run ./cmd/replay` is how a
	// warning gets trained out of a reader before it ever means anything.
	if ver == "" || ver == "dev" || ver == "unknown" {
		fmt.Fprintf(&b, "build         %s, from source\n", name)
		fmt.Fprintf(&b, "              a source build carries no release date, so nothing here ages it, and\n")
		fmt.Fprintf(&b, "              replay upgrade will not overwrite one\n")
		return b.String()
	}

	built, err := time.Parse(time.RFC3339, buildDate)
	if err != nil {
		fmt.Fprintf(&b, "build         %s, build date unreadable (%q)\n", name, buildDate)
		fmt.Fprintf(&b, "              this build cannot age itself, so treat its age as unknown, not current\n")
		return b.String()
	}

	// A stamp after now is a clock that moved, not a binary from the future.
	// StaleNotice is silent here on purpose -- it will not report a build as
	// -40 days old -- and silence is exactly what must not reach the reader as
	// "current", because the date it would print is one nothing on this machine
	// can vouch for.
	if built.After(now) {
		fmt.Fprintf(&b, "build         %s, dated %s, ahead of this clock\n", name, built.UTC().Format("2006-01-02"))
		fmt.Fprintf(&b, "              a build dated in the future means a clock moved; its age is unknown\n")
		return b.String()
	}

	fmt.Fprintf(&b, "build         %s, built %s\n", name, built.UTC().Format("2006-01-02"))
	// The age and the threshold come from selfupdate, not from a second copy
	// here. rulesStaleAfter and internal/tui.rulesStaleDays already show what
	// duplicating a threshold across surfaces costs: two answers to one
	// question, from the same file, on the same machine, in the same minute.
	if s := selfupdate.StaleNotice(buildDate, now); s != "" {
		fmt.Fprintf(&b, "              %s\n", s)
	}
	return b.String()
}
