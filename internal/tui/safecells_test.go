package tui

import (
	"strings"
	"testing"
)

// The safe screen's cells, one state at a time, for the same reason the guards
// cells got their own file: a whole-screen Contains check passes whichever of
// two places produced the string.

func TestStoreSizeNamesFilesOnlyWhenThereAreSeveral(t *testing.T) {
	if got := storeSize(Store{Bytes: 32, Files: 1}); strings.Contains(got, "file") {
		t.Errorf("a single-file store reports a file count: %q", got)
	}
	got := storeSize(Store{Bytes: 187807, Files: 8})
	if !strings.Contains(got, "8 files") {
		t.Errorf("a directory of eight files does not say so: %q", got)
	}
}

func TestHumanBytesUsesTheUnitTheReaderExpects(t *testing.T) {
	for _, c := range []struct {
		in   int64
		want string
	}{
		{32, "32 B"},
		{2048, "2.0 kB"},
		{5 << 20, "5.0 MB"},
		{3 << 30, "3.0 GB"},
	} {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A machine holding nothing, a machine that could not be read, and a machine
// holding something are three different screens.
func TestSafeHasThreeDistinctStates(t *testing.T) {
	empty := SafeScreen(Privacy{Root: "~/.replay"})
	unreadable := SafeScreen(Privacy{Root: "~/.replay", Err: "permission denied"})
	held := SafeScreen(Privacy{Root: "~/.replay", Stores: stores()})

	if empty.From != Measured {
		t.Errorf("an empty machine declares itself %v; nothing found is a "+
			"measurement", empty.From)
	}
	if unreadable.From != Unavailable {
		t.Errorf("an unreadable machine declares itself %v", unreadable.From)
	}
	if held.From != Measured {
		t.Errorf("a machine with stores declares itself %v", held.From)
	}
	if empty.String() == unreadable.String() {
		t.Error("nothing written and could not look render identically")
	}
}

// The sensitive mark is on the store that holds secrets and on no other.
func TestOnlyTheSensitiveStoreIsMarked(t *testing.T) {
	body := SafeScreen(Privacy{Root: "~/.replay", Stores: stores()}).String()
	marked := 0
	for _, l := range strings.Split(body, "\n") {
		if strings.Contains(l, "! ") && !strings.HasPrefix(strings.TrimSpace(l), "!") {
			continue
		}
		if strings.Contains(l, "! vault") {
			marked++
		}
		if strings.Contains(l, "! ledger") || strings.Contains(l, "! policy.json") {
			t.Errorf("a store that holds counts is marked as holding secrets: %q", l)
		}
	}
	if marked == 0 {
		t.Errorf("the vault is not marked:\n%s", body)
	}
}

// A machine with no sensitive store gets no sensitive note, so the note is not
// one every reader learns to skip.
func TestNoSensitiveNoteWhenNothingIsSensitive(t *testing.T) {
	body := SafeScreen(Privacy{Root: "~/.replay", Stores: []Store{
		{Name: "ledger", Bytes: 100, Files: 2, Purgeable: true},
	}}).String()
	if strings.Contains(body, "hold your secrets") {
		t.Errorf("a machine with no vault is told it holds secrets:\n%s", body)
	}
	if Urgent(SafeScreen(Privacy{Root: "~/.replay", Stores: []Store{
		{Name: "ledger", Bytes: 100, Files: 2, Purgeable: true},
	}}).Lines) {
		t.Error("an urgent mark on a machine with nothing urgent on it")
	}
}

// padSafe trims an over-long body rather than letting the frame grow.
func TestPadSafeTrimsAndFills(t *testing.T) {
	long := make([]string, bodyRows*2)
	for i := range long {
		long[i] = "  row"
	}
	if got := len(padSafe(long)); got != bodyRows {
		t.Errorf("an over-long safe body is %d rows, budget %d", got, bodyRows)
	}
	if got := len(padSafe([]string{"  one"})); got != bodyRows {
		t.Errorf("a short safe body is %d rows, budget %d", got, bodyRows)
	}
}

// countLines is how many rendered rows are exactly this one, so an assertion
// about a note is an assertion about the note rather than about a substring
// that some other row also happens to carry.
func countLines(lines []string, want string) int {
	n := 0
	for _, l := range lines {
		if strings.TrimRight(l, " ") == want {
			n++
		}
	}
	return n
}

// The sensitive note is a count, and the count is of the stores that hold
// secrets.
//
// Its ABSENCE was asserted and its presence was not. That left both halves of
// it disabled with the suite green: the loop that counts and the gate that
// prints. A screen that stopped warning about a vault of live credentials
// passed the test named for the warning, because that test only checks that a
// clean machine stays quiet.
func TestSafeNotesCountTheStoresThatHoldSecrets(t *testing.T) {
	two := SafeScreen(Privacy{Root: "~/.replay", Stores: []Store{
		{Name: "vault", Bytes: 32, Files: 1, Sensitive: true},
		{Name: "keys.json", Bytes: 64, Files: 1, Sensitive: true},
		{Name: "ledger", Bytes: 187807, Files: 8, Purgeable: true},
	}}).Lines
	want := note(true, "2 store(s) hold your secrets rather than counts")
	if n := countLines(two, want); n != 1 {
		t.Errorf("two stores holding secrets produce the row %q %d times:\n%s",
			want, n, strings.Join(two, "\n"))
	}

	// And the number is the number, not "some". One sensitive store among
	// three says one.
	one := SafeScreen(Privacy{Root: "~/.replay", Stores: []Store{
		{Name: "vault", Bytes: 32, Files: 1, Sensitive: true},
		{Name: "keys.json", Bytes: 64, Files: 1, Purgeable: true},
		{Name: "ledger", Bytes: 187807, Files: 8, Purgeable: true},
	}}).Lines
	want1 := note(true, "1 store(s) hold your secrets rather than counts")
	if n := countLines(one, want1); n != 1 {
		t.Errorf("one store holding secrets produces the row %q %d times:\n%s",
			want1, n, strings.Join(one, "\n"))
	}
	if countLines(one, want) != 0 {
		t.Errorf("one sensitive store was counted as two:\n%s", strings.Join(one, "\n"))
	}
}
