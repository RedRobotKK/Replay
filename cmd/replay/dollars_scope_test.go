package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// --dollars belongs to one of the three commands runReport backs.
//
// The flag adds a list-price column to the policy table, and the policy table
// is built only by WriteReplay. WriteBlame renders attributed token sources and
// WriteDiff renders cache-break events; neither has a dollar-bearing field, and
// no commit in the history has ever read LaneReport.Dollars from either. The
// guide documents the flag under `replay <path>` and says nothing about it
// under blame or diff.
//
// It was accepted by all of them anyway, because runReport declares one flag
// set for four entry points. So `replay blame --dollars` parsed, stored the
// value, printed the same output as without it, and told the reader nothing.
// A flag that is accepted and ignored is worse than one that is refused: the
// reader has no way to learn it did nothing.
//
// Refusing is the whole change. Giving blame or diff a dollar figure would mean
// apportioning a priced usage record across attributed labels, or pricing a
// per-event break deficit, and neither calculation exists here.

// dollarsFixture is the transcript corpus the other CLI tests read.
func dollarsFixture() string {
	return filepath.Join("..", "..", "internal", "transcript", "testdata")
}

func dollarsRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := run(args, &out, &errb)
	return out.String(), err
}

// DollarScope1: replay still takes the flag, and still prints the column.
func TestDollarScope1_ReplayStillAcceptsDollars(t *testing.T) {
	homeWithTranscript(t)
	out, err := dollarsRun(t, "replay", "--dollars", dollarsFixture())
	if err != nil {
		t.Fatalf("replay --dollars: %v", err)
	}
	// Either the priced column or the explicit note that no list price exists
	// for this model. Both are the flag working; silence would not be.
	if !strings.Contains(out, "list cost") && !strings.Contains(out, "no list price is known") {
		t.Errorf("--dollars produced neither a list cost column nor the no-price note:\n%s", out)
	}
}

// DollarScope2: replay without the flag is untouched.
func TestDollarScope2_ReplayWithoutDollarsIsUnchanged(t *testing.T) {
	homeWithTranscript(t)
	out, err := dollarsRun(t, "replay", dollarsFixture())
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if strings.Contains(out, "list cost") {
		t.Errorf("a list cost column appeared without --dollars:\n%s", out)
	}
}

// DollarScope3: blame refuses the flag, and says which flag and which command.
func TestDollarScope3_BlameRefusesDollars(t *testing.T) {
	homeWithTranscript(t)
	out, err := dollarsRun(t, "blame", "--dollars", dollarsFixture())
	if err == nil {
		t.Fatalf("blame --dollars was accepted:\n%s", out)
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("error is not a usage error, so the exit status will not say the "+
			"invocation was wrong: %v", err)
	}
	for _, want := range []string{"--dollars", "blame"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}
	// The refusal must not read as a pricing failure. Dollars are available;
	// they are available on a different command.
	for _, banned := range []string{"price table", "not priced", "no list price"} {
		if strings.Contains(err.Error(), banned) {
			t.Errorf("the error implies a pricing problem rather than a command scope: %v", err)
		}
	}
}

// DollarScope4: diff refuses it the same way.
func TestDollarScope4_DiffRefusesDollars(t *testing.T) {
	homeWithTranscript(t)
	out, err := dollarsRun(t, "diff", "--dollars", dollarsFixture())
	if err == nil {
		t.Fatalf("diff --dollars was accepted:\n%s", out)
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("error is not a usage error: %v", err)
	}
	for _, want := range []string{"--dollars", "diff"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}
}

// DollarScope5: the refusal is about the flag, not about the path.
//
// Without this, an implementation that rejected the invocation for any reason
// at all would pass DollarScope3 and DollarScope4. The same command and the
// same path succeed once the flag is dropped, so the flag is the only
// difference.
func TestDollarScope5_TheRefusalIsTheFlagAndNotTheInvocation(t *testing.T) {
	homeWithTranscript(t)
	for _, cmd := range []string{"blame", "diff"} {
		if _, err := dollarsRun(t, cmd, dollarsFixture()); err != nil {
			t.Fatalf("%s on the same path without the flag failed, so the refusal in "+
				"DollarScope3/DollarScope4 proves nothing about --dollars: %v", cmd, err)
		}
	}
}

// DollarScope6: blame still reports what it reported.
func TestDollarScope6_BlameWithoutDollarsIsUnchanged(t *testing.T) {
	homeWithTranscript(t)
	out, err := dollarsRun(t, "blame", dollarsFixture())
	if err != nil {
		t.Fatalf("blame: %v", err)
	}
	if !strings.Contains(out, "Tier:") {
		t.Errorf("blame no longer prints its header:\n%s", out)
	}
	if strings.Contains(out, "$") {
		t.Errorf("blame printed a dollar figure, which it has no field for:\n%s", out)
	}
}

// DollarScope7: diff still reports what it reported.
func TestDollarScope7_DiffWithoutDollarsIsUnchanged(t *testing.T) {
	homeWithTranscript(t)
	out, err := dollarsRun(t, "diff", dollarsFixture())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(out, "Tier:") {
		t.Errorf("diff no longer prints its header:\n%s", out)
	}
	if strings.Contains(out, "$") {
		t.Errorf("diff printed a dollar figure, which it has no field for:\n%s", out)
	}
}

// DollarScope8: the bare-path form is the replay report, so it keeps the flag.
//
// `replay <dir> --dollars` dispatches through the same runReport as the named
// `replay` command, and the flag is supported there. A scope check keyed on the
// wrong name would refuse the documented form.
func TestDollarScope8_TheBarePathFormStillAcceptsDollars(t *testing.T) {
	homeWithTranscript(t)
	out, err := dollarsRun(t, dollarsFixture(), "--dollars")
	if err != nil {
		t.Fatalf("replay <dir> --dollars: %v", err)
	}
	if !strings.Contains(out, "list cost") && !strings.Contains(out, "no list price is known") {
		t.Errorf("the bare-path form lost the flag:\n%s", out)
	}
}
