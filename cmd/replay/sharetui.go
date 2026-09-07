package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/RedRobotKK/Replay/internal/card"
	"github.com/RedRobotKK/Replay/internal/tui"
)

// The share screen's own state and its one side effect.
//
// internal/tui does no I/O, which is what keeps every frame testable without a
// disk. The write therefore lives here, and so does the toggling, and the
// screen is handed the result rather than performing it.

// shareUI is which card is being previewed and what the last write did.
type shareUI struct {
	variant card.Variant
	tone    card.Tone
	wrote   string
	failed  string
}

// press applies the share screen's own keys, and reports whether it took one.
//
// Three keys, and two of them are borrowed: d and w are the doctor and why
// questions everywhere else on this surface. They belong to this screen while
// it is open and the screen says so, which is the trade — a key that quietly
// means something different is worse than one that does nothing. Everything
// else is declined, so the other seven questions stay one keystroke away.
//
// The state is the one that was previewed, passed in rather than rebuilt here.
// w writes the card that was on the screen when it was pressed, and t and d
// toggle away from what was on the screen rather than away from a field this
// struct holds. The two are the same value in the running surface, and reading
// the previewed one means a keystroke always answers the frame the reader was
// looking at.
func (u *shareUI) press(k rune, st tui.ShareState, dir string) bool {
	switch k {
	case 't':
		u.tone = card.ToneRekt
		if st.Tone == card.ToneRekt {
			u.tone = card.ToneMeasured
		}
	case 'd':
		u.variant = card.VariantB
		if st.Variant == card.VariantB {
			u.variant = card.VariantC
		}
	case 'w':
		path, err := shareWrite(dir, st)
		u.wrote, u.failed = path, ""
		if err != nil {
			u.wrote, u.failed = "", err.Error()
		}
	default:
		return false
	}
	return true
}

// shareWrite writes the previewed card and returns the absolute path.
//
// The refusal is the command's, asked rather than restated: a screen that
// offered a card `replay cost --share` would have declined to produce is
// offering the reader something nobody stands behind. It writes nothing when
// it refuses, for the same reason the command does — a file that exists reads
// as a card that worked, and the next thing that happens to it is being
// posted.
func shareWrite(dir string, st tui.ShareState) (string, error) {
	if !st.Ready {
		return "", fmt.Errorf("nothing measured enough to share: %d priced sessions", st.Tasks)
	}
	// The design and the register are in the name. Two cards written from one
	// session then sit side by side instead of one silently replacing the
	// other, which is the whole reason for previewing both.
	path, err := filepath.Abs(filepath.Join(dir,
		fmt.Sprintf("replay-card-%s-%s.png", st.Variant, st.Tone)))
	if err != nil {
		return "", err
	}
	// Discarded, not printed: writeCard's note goes to stderr, and stderr is
	// underneath a full-screen surface where nobody would ever see it. The
	// screen says the path instead.
	if err := writeCard(path, st.Variant, st.Tone, st.Data, io.Discard); err != nil {
		return "", err
	}
	return path, nil
}

// shareFrom builds the share card's figures and asks whether there is anything
// worth posting.
//
// Both halves are the command's own: cardData is the one place a report becomes
// a picture, and shareCard is the one place the decision is made about whether
// a corpus has produced anything to stand behind. Asked here rather than
// restated, because a screen offering a card that `replay cost --share` would
// have refused to produce is offering something nobody stands behind, and two
// copies of that judgement would drift.
func shareFrom(s costSummary, breaks, peak int) (card.Data, bool) {
	return cardData(s, breaks, peak), shareCard(s, breaks) != ""
}

// shareState assembles what the screen draws from.
func shareState(m tui.Machine, u shareUI) tui.ShareState {
	return tui.ShareState{
		Data: m.Card, Ready: m.ShareOK, Tasks: m.Card.Tasks,
		Variant: u.variant, Tone: u.tone,
		Wrote: u.wrote, Failed: u.failed,
	}
}

// screenKey reads a screen's key off the surface rather than spelling it twice.
//
// The label is the name `-screen` takes and the key is what the loop
// dispatches on. Looking one up from the other means the binding is stated in
// exactly one place, which is tui.Shortcuts.
func screenKey(label string) rune {
	for _, s := range tui.Shortcuts() {
		if s.Label == label {
			return s.Key
		}
	}
	return 0
}
