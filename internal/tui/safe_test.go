package tui

import (
	"strings"
	"testing"
)

// The 's' key asks "Is my setup safe?" and must answer that question.
//
// It rendered a trim byte-cap summary — what capping tool output would have
// saved — which is a token-savings question wearing a safety key. Meanwhile
// `replay privacy`, which lists everything Replay has written to this machine
// and hands off to `purge`, had no screen at all: the best-built journey in the
// product, absent from the surface the installer opens.

func stores() []Store {
	return []Store{
		{Name: "vault", Bytes: 32, Files: 1, Sensitive: true, Purgeable: false},
		{Name: "ledger", Bytes: 187807, Files: 8, Purgeable: true},
		{Name: "cost-index.json", Bytes: 1879352, Files: 1, Purgeable: true},
		{Name: "policy.json", Bytes: 3615, Files: 1, Purgeable: false},
	}
}

// SF1: it lists what is on the machine, largest first.
func TestSafeListsWhatIsHeldHere(t *testing.T) {
	body := SafeScreen(Privacy{Root: "~/.replay", Stores: stores()}).String()
	for _, want := range []string{"vault", "ledger", "cost-index.json", "~/.replay"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not name %q:\n%s", want, body)
		}
	}
	iBig := strings.Index(body, "cost-index.json")
	iSmall := strings.Index(body, "policy.json")
	if iBig < 0 || iSmall < 0 || iBig > iSmall {
		t.Errorf("stores are not ordered by size, so the largest thing on the disk "+
			"is not the first thing read:\n%s", body)
	}
	// And the vault, which is 32 bytes, still comes first.
	if i := strings.Index(body, "vault"); i < 0 || i > iBig {
		t.Errorf("the store holding secrets sorts below the store holding an index; "+
			"size is deciding what a safety screen shows:\n%s", body)
	}
}

// SF2: a store holding secrets is marked, and marked urgently.
//
// The vault holds the real values behind the placeholders sent to a provider —
// the only store that holds secrets rather than counts about them. A list that
// renders it like a log file has answered "is my setup safe" with a size.
func TestSafeMarksTheStoreThatHoldsSecrets(t *testing.T) {
	sc := SafeScreen(Privacy{Root: "~/.replay", Stores: stores()})
	if !Urgent(sc.Lines) {
		t.Errorf("no urgent mark on a machine holding a secrets vault:\n%s", sc.String())
	}
	if !strings.Contains(sc.String(), "vault") {
		t.Error("the sensitive store is not named")
	}
}

// SF3: it says which stores a retention window will not reach.
//
// This is the difference between "purge cleans up" and "purge cleans up
// everything", and a reader who believes the second one is wrong about what
// leaving looks like.
func TestSafeSaysWhatPurgeWillNotRemove(t *testing.T) {
	body := SafeScreen(Privacy{Root: "~/.replay", Stores: stores()}).String()
	if !strings.Contains(body, "retention window") {
		t.Errorf("the screen does not distinguish what a window reaches:\n%s", body)
	}
	if !strings.Contains(body, "replay purge") {
		t.Errorf("the screen does not hand off to the command that removes things:\n%s", body)
	}
}

// SF4: nothing written is a finding, not an absence.
func TestSafeSaysWhenNothingIsHeld(t *testing.T) {
	sc := SafeScreen(Privacy{Root: "~/.replay"})
	body := sc.String()
	if strings.Contains(body, "0 B") {
		t.Errorf("an empty machine is reported as a zero-sized store:\n%s", body)
	}
	if !strings.Contains(body, "nothing") {
		t.Errorf("the screen does not say plainly that nothing is held:\n%s", body)
	}
}

// SF5: a machine that could not be read is not a machine holding nothing.
//
// resolveStores can fail — an unreadable ~/.replay, a permission change — and
// "Replay has written nothing to this machine" is the most reassuring possible
// rendering of "I could not look".
func TestSafeDoesNotReadAnErrorAsAnEmptyDisk(t *testing.T) {
	sc := SafeScreen(Privacy{Root: "~/.replay", Err: "permission denied"})
	body := sc.String()
	if strings.Contains(body, "nothing") {
		t.Errorf("an unreadable directory was reported as empty:\n%s", body)
	}
	if sc.From != Unavailable {
		t.Errorf("an unreadable machine declares itself %v, want Unavailable", sc.From)
	}
}

// SF6: a long inventory is capped rather than truncated silently, and says so.
func TestSafeCapsALongInventoryAndSaysSo(t *testing.T) {
	var many []Store
	for i := 0; i < 12; i++ {
		many = append(many, Store{Name: "store-" + string(rune('a'+i)), Bytes: int64(1000 - i), Files: 1, Purgeable: true})
	}
	sc := SafeScreen(Privacy{Root: "~/.replay", Stores: many})
	if len(sc.Lines) > bodyRows() {
		t.Errorf("%d rows, body budget %d", len(sc.Lines), bodyRows())
	}
	if !strings.Contains(sc.String(), "more") {
		t.Errorf("rows were dropped without saying so:\n%s", sc.String())
	}
}

// SF7: a sensitive store is never the one the row cap drops.
//
// The vault is 32 bytes. Sorted by size on a real machine it fell past the cap,
// so the screen said "1 store(s) hold your secrets" and did not show which —
// a warning the reader cannot act on, under the key that asks whether the setup
// is safe.
func TestSafeNeverHidesASensitiveStoreBehindTheCap(t *testing.T) {
	all := []Store{{Name: "vault", Bytes: 32, Files: 1, Sensitive: true}}
	for i := 0; i < 11; i++ {
		all = append(all, Store{Name: "bulk-" + string(rune('a'+i)), Bytes: int64(1 << 20), Files: 1, Purgeable: true})
	}
	body := SafeScreen(Privacy{Root: "~/.replay", Stores: all}).String()
	if !strings.Contains(body, "vault") {
		t.Errorf("the only store holding secrets was cut by the row cap:\n%s", body)
	}
}
