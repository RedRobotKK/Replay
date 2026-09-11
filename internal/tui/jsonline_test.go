package tui

import "testing"

// The --json line in the help overlay is derived from Shortcut.JSON rather
// than written by hand, and these cover both arms of that derivation from
// this package. cmd/replay TestJC1 pins the FIELD to the binary; TestJC2
// pins the rendered line to the field. Neither is in this package, and a
// conditional whose only observer lives two packages away is one
// guard-reachability reports as inert — correctly, because a change here
// would not go red here.
func TestJL1_ExceptionsAreNamed(t *testing.T) {
	got := jsonLineFrom([]Shortcut{
		{Key: 'c', JSON: true},
		{Key: 'w', JSON: false},
		{Key: 'd', JSON: false},
	})
	want := "machine-readable, except on w d"
	if got != want {
		t.Errorf("jsonLineFrom() = %q, want %q\nThe overlay names the screens "+
			"whose command has no --json. A screen missing from this list is a "+
			"screen the user is told the flag works on.", got, want)
	}
}

// The other arm, and the reason jsonLineFrom takes a slice. Shortcuts()
// cannot produce it today — four screens lack the flag — so through the real
// table this branch is unreachable, and a branch no test can enter is not a
// guard. It is not dead code: it is the sentence the overlay should print
// once blame, serve and doctor gain --json, and this is what says so.
func TestJL2_NoExceptionsMeansNoQualifier(t *testing.T) {
	got := jsonLineFrom([]Shortcut{{Key: 'c', JSON: true}, {Key: 'x', JSON: true}})
	want := "the same answer for a machine, no screen at all"
	if got != want {
		t.Errorf("jsonLineFrom() = %q, want %q\nWith nothing to except, the line "+
			"must not carry an empty exception list.", got, want)
	}
}

// A screen table with no JSON screens at all still names every key, rather
// than degrading into the unqualified sentence by accident.
func TestJL3_EveryScreenMissingIsStillNamed(t *testing.T) {
	got := jsonLineFrom([]Shortcut{{Key: 'a', JSON: false}, {Key: 'b', JSON: false}})
	want := "machine-readable, except on a b"
	if got != want {
		t.Errorf("jsonLineFrom() = %q, want %q", got, want)
	}
}
