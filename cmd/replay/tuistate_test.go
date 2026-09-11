package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/advisor"
	"github.com/RedRobotKK/Replay/internal/tui"
)

// The screens' own keys and the state behind them, one decision at a time.
//
// Each of these sat inside runTUI — in a closure handed to SetLocal, in a
// closure handed to the corpus walk, or behind an environment lookup nothing
// asked twice — where the only way to reach it was to drive the whole surface.
// guard-reachability reported them as branches no test enters, which is what a
// decision written where it cannot be called looks like from outside.

// The advice detail closes on escape, and only when there is one open.
//
// The detail's last row says "esc back to the list". Until the handler existed
// escape did nothing there at all, which a reader cannot tell apart from a
// terminal that dropped the key. The second half of the condition is just as
// load-bearing as the first: on the list there is nothing to close, so escape
// has to fall through to the loop, where it means back.
func TestAdviseEscapeClosesTheDetailAndNothingElse(t *testing.T) {
	row := tui.AdviceRow{}
	open := &row
	if !adviseKeys(27, &open, []string{"tip-1"}, 0, refuseToMark(t)) {
		t.Fatal("escape on an open detail was not consumed, so the loop treated it " +
			"as back and the reader left the screen instead of the detail")
	}
	if open != nil {
		t.Error("escape was consumed and the detail is still open")
	}

	var list *tui.AdviceRow
	if adviseKeys(27, &list, []string{"tip-1"}, 0, refuseToMark(t)) {
		t.Error("escape on the list was consumed, so \"esc back\" does nothing on " +
			"the one screen the footer promises it on")
	}
}

// a and x record the reader's verdict on the row the cursor is on, and report
// a write that failed as a key that did not take.
func TestAdviseKeysMarkTheRowUnderTheCursor(t *testing.T) {
	var gotID string
	var gotStatus advisor.Status
	mark := func(id string, s advisor.Status) error {
		gotID, gotStatus = id, s
		return nil
	}
	ids := []string{"tip-1", "tip-2", "tip-3"}

	var open *tui.AdviceRow
	if !adviseKeys('x', &open, ids, 1, mark) {
		t.Fatal("x on a valid row was not consumed")
	}
	if gotID != "tip-2" || gotStatus != advisor.Dismissed {
		t.Errorf("x on row 1 marked %q as %v, want tip-2 dismissed", gotID, gotStatus)
	}
	if !adviseKeys('a', &open, ids, 0, mark) || gotID != "tip-1" || gotStatus != advisor.Applied {
		t.Errorf("a on row 0 marked %q as %v, want tip-1 applied", gotID, gotStatus)
	}

	// A write that fails must not look like one that worked: the next render
	// re-reads the file and would show the old status, so the key has to report
	// that it did not take.
	failing := func(string, advisor.Status) error { return errors.New("read-only") }
	if adviseKeys('a', &open, ids, 0, failing) {
		t.Error("a failed write reported itself as a keystroke that landed")
	}

	// And a cursor outside the list marks nothing at all.
	for _, at := range []int{-1, 3} {
		if adviseKeys('a', &open, ids, at, refuseToMark(t)) {
			t.Errorf("a with the cursor at %d over %d rows was consumed", at, len(ids))
		}
	}
	// A key the screen does not own is left for the loop.
	if adviseKeys('c', &open, ids, 0, refuseToMark(t)) {
		t.Error("the advice screen swallowed c, so the cost key stops working on it")
	}
}

// refuseToMark fails the test if the advice file is touched at all.
func refuseToMark(t *testing.T) func(string, advisor.Status) error {
	t.Helper()
	return func(id string, s advisor.Status) error {
		t.Errorf("the advice file was written: %q marked %v", id, s)
		return nil
	}
}

// A home directory that cannot be resolved is reported, not rendered as a
// clean machine.
//
// safeState answers "is my setup safe?" by listing what Replay has written
// here. With no home there is nothing to walk, and returning the zero Privacy
// would put the screen into its "Replay has written nothing to this machine"
// state — the most reassuring possible rendering of "I could not look".
func TestSafeStateReportsAHomeItCouldNotResolve(t *testing.T) {
	t.Setenv("HOME", "")
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone leaves the
	// lookup succeeding there.
	t.Setenv("USERPROFILE", "")

	p := safeState()
	if p.Err == "" {
		t.Fatalf("no home directory resolved to a readable machine: %+v", p)
	}
	if len(p.Stores) != 0 {
		t.Errorf("%d store(s) were measured under a home that does not exist", len(p.Stores))
	}
	// And the screen says so rather than saying the disk is clean.
	sc := tui.SafeScreen(p)
	if sc.From != tui.Unavailable {
		t.Errorf("the screen declares itself %v on a machine it could not read", sc.From)
	}
	if strings.Contains(sc.String(), "written nothing") {
		t.Errorf("could not look was rendered as nothing held:\n%s", sc.String())
	}
}

// With no ANTHROPIC_BASE_URL set, the guards screen looks where `replay serve`
// listens.
//
// The screen's own advice is "next: replay serve", and an empty base would
// leave it reporting no answer from an address it declines to name — while
// dialling nothing, so the answer is the same whether or not a proxy is up.
func TestGuardBaseFallsBackToTheAddressServeBindsTo(t *testing.T) {
	t.Setenv(envBaseURL, "")
	if got, want := guardBase(), "http://"+defaultListen; got != want {
		t.Errorf("with no base URL set the guards screen looks at %q, want %q", got, want)
	}
	// An environment that names a proxy is not overridden by the default.
	t.Setenv(envBaseURL, "http://127.0.0.1:9999")
	if got := guardBase(); got != "http://127.0.0.1:9999" {
		t.Errorf("a base URL from the environment was replaced by %q", got)
	}
}
