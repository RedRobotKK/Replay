package guardcheck

// The reviewer's outcome: which survivors a change owns, and whether the run
// fails.
//
// These two decisions are the reviewer's fail-closed contract, and they live
// here rather than in scripts/guard-reachability/main.go for one reason: that
// file carries //go:build ignore, so `go test ./...` cannot reach a line of it.
// The contract was correct by reading and observed by nothing — which is the
// exact condition the reviewer exists to report in everyone else's code.
//
// The script keeps the parts that touch the world (the detached worktree, the
// test runs, the base parse) and feeds their result here. What the script is
// left holding is plumbing; what is decided is decided in this file.

// ClassifySurvivors splits survivors into the ones the base tree already had
// and the ones this change introduced.
//
// baseSurvivors says, per identity, how many conditions with that identity were
// MEASURED surviving neutralisation in the base tree. baseErr is the base
// tree's failure to answer at all: the worktree could not be checked out, its
// own suite was red, or its sources would not parse.
//
// When baseErr is non-nil nothing is exempted and every survivor is introduced.
// The error dominates whatever counts arrive with it, deliberately: a future
// caller that returned partial counts alongside a late failure — five
// identities measured, the sixth unparseable — must not leak those five into an
// exemption. A base either answered or it did not.
//
// The defect this exists to prevent is a silent turn from fail-closed to
// fail-open. A reviewer that read an unanswerable base as "all of this
// pre-dates the change" would keep printing its summary and stop failing
// builds, which is worse than having no reviewer at all, because the summary is
// then read as evidence and there is none behind it. The exemption is earned
// per guard, measured in the base tree, or not at all.
func ClassifySurvivors(survivors []Guard, baseSurvivors map[Identity]int, baseErr error) (pre, introduced []Guard) {
	if baseErr != nil {
		return nil, survivors
	}
	return PairSurvivors(survivors, baseSurvivors)
}

// ExitCode is the process status a finished review exits with.
//
// The three inputs are kept apart rather than summed because they are three
// different absences, and only one of them is about this change's guards.
// introduced is a hole the change owns. unchecked is a mutant the compiler
// rejected, so the suite never judged it. unbuilt is a file this host does not
// compile, so no build existed to neutralise against. Neither of the last two
// is a pass; a run that exited 0 on them would print a clean line over guards
// nothing looked at, which is the false green the reviewer exists to prevent,
// occurring inside the reviewer.
func ExitCode(introduced, unchecked, unbuilt []Guard) int {
	if len(introduced) == 0 && len(unchecked) == 0 && len(unbuilt) == 0 {
		return 0
	}
	return 1
}
