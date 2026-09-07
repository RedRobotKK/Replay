package tui

import (
	"strings"
	"testing"
	"unicode"

	"github.com/RedRobotKK/Replay/internal/card"
)

func aCard() card.Data {
	return card.Data{
		AvoidableShare:      0.03,
		Tasks:               1384,
		Breaks:              748,
		MedianUSD:           0.77,
		P90USD:              2.30,
		PeakAvoidableTokens: 32_635_820,
	}
}

func aShare() ShareState {
	return ShareState{
		Data: aCard(), Ready: true, Tasks: 1384,
		Variant: card.VariantC, Tone: card.ToneMeasured,
	}
}

// The preview is the card's own words, not a description of them.
//
// A preview that can disagree with the file it writes is worse than no
// preview: it is a second source of truth for something the reader is about to
// post under their own name. So every string the card would carry has to be on
// the screen, and it has to arrive from the same function the renderer draws
// from.
//
// PASS: every line of the card's script appears in the frame, for both designs
// and both tones.
// FAIL: any line missing, which is a preview showing something else.
func TestSharePreviewIsTheCardsOwnWords(t *testing.T) {
	for _, v := range card.Variants() {
		for _, tone := range card.Tones() {
			st := aShare()
			st.Variant, st.Tone = v, tone
			frame := strings.Join(ShareScreen(st).Lines, "\n")
			said := card.Say(v, tone, st.Data).Lines
			if len(said) < 5 {
				t.Fatalf("the card says %d lines; this check would pass over nothing", len(said))
			}
			for _, l := range said {
				if !strings.Contains(frame, l) {
					t.Errorf("card %s/%s says %q and the preview does not:\n%s",
						v, tone, l, frame)
				}
			}
		}
	}
}

// It reads this machine, so it is Measured and carries no example-data notice.
func TestShareScreenIsMeasuredAndUnstamped(t *testing.T) {
	sc := ShareScreen(aShare())
	if sc.From != Measured {
		t.Errorf("the share screen reads real cost data and declares itself %v", sc.From)
	}
	if Marked(sc.Lines) {
		t.Error("the share screen carries a provenance notice over measured figures. " +
			"Stamping real data teaches the reader to skim the stamp")
	}
	if sc.Title != "share" {
		t.Errorf("title is %q, want share", sc.Title)
	}
}

// Nothing to stand behind is said, not drawn as zeros.
//
// `replay cost --share` refuses when nothing was measured well enough to post,
// and a screen that drew a card of zeros over the same corpus would be offering
// the user a picture the command would have declined to produce.
func TestShareScreenRefusesWhenThereIsNothingToStandBehind(t *testing.T) {
	sc := ShareScreen(ShareState{Variant: card.VariantC, Tone: card.ToneMeasured})
	if sc.From != Unavailable {
		t.Errorf("a screen with nothing behind it declares itself %v, want Unavailable", sc.From)
	}
	if !Marked(sc.Lines) {
		t.Error("a screen with nothing behind it does not say so")
	}
	body := strings.Join(sc.Lines, "\n")
	if strings.Contains(body, "billed me twice") || strings.Contains(body, "I paid for") {
		t.Errorf("a card was previewed over a corpus with nothing in it:\n%s", body)
	}
	if !strings.Contains(body, "priced session") {
		t.Errorf("the refusal does not name its own reason:\n%s", body)
	}
	if !strings.Contains(body, "w") {
		t.Errorf("the screen does not say the write key will not work:\n%s", body)
	}
}

// A key with a filesystem side effect says what it did.
func TestShareScreenStatesThePathItWrote(t *testing.T) {
	st := aShare()
	st.Wrote = "/tmp/somewhere/replay-card-c-measured.png"
	body := strings.Join(ShareScreen(st).Lines, "\n")
	if !strings.Contains(body, st.Wrote) {
		t.Errorf("the screen wrote a file and does not say where:\n%s", body)
	}

	st.Wrote, st.Failed = "", "permission denied"
	body = strings.Join(ShareScreen(st).Lines, "\n")
	if !strings.Contains(body, "permission denied") {
		t.Errorf("a write that failed said nothing:\n%s", body)
	}
}

// The screen names its own keys, because two of them are borrowed.
//
// d and w are the doctor and why questions everywhere else on this surface.
// While the share screen is open they belong to it, so it has to say so: a key
// that quietly means something different is worse than one that does nothing.
func TestShareScreenNamesItsOwnKeys(t *testing.T) {
	body := strings.Join(ShareScreen(aShare()).Lines, "\n")
	for _, want := range []string{"t ", "d ", "w "} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not offer %q:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "this screen") {
		t.Errorf("the screen does not say that d and w are its own here:\n%s", body)
	}
}

// It fits, like everything else.
func TestShareScreenFitsTheBudget(t *testing.T) {
	states := []ShareState{ShareState{}, aShare()}
	for _, v := range card.Variants() {
		for _, tone := range card.Tones() {
			st := aShare()
			st.Variant, st.Tone, st.Wrote = v, tone, "/Users/somebody/work/replay-card.png"
			states = append(states, st)
		}
	}
	for _, st := range states {
		sc := ShareScreen(st)
		if len(sc.Lines) > BudgetRows {
			t.Errorf("%s/%s is %d rows, budget %d", st.Variant, st.Tone, len(sc.Lines), BudgetRows)
		}
		for i, l := range sc.Lines {
			if len(l) > BudgetCols {
				t.Errorf("%s/%s line %d is %d columns, budget %d:\n%s",
					st.Variant, st.Tone, i, len(l), BudgetCols, l)
			}
			if l != strings.TrimRight(l, " \t") {
				t.Errorf("%s/%s line %d has trailing whitespace: %q", st.Variant, st.Tone, i, l)
			}
			for _, r := range l {
				if r > unicode.MaxASCII {
					t.Errorf("%s/%s line %d has non-ASCII %q", st.Variant, st.Tone, i, r)
				}
			}
		}
	}
}

// The loop hands the current screen its keystroke first.
//
// Contextual intelligence, and the only way t, d and w can mean what the share
// screen needs while d and w are questions everywhere else. A screen that
// declines the key must leave it to the shortcuts, or a list appearing would
// stop every letter working.
func TestALocalKeyIsOfferedToTheScreenFirst(t *testing.T) {
	l := &Loop{}
	var seen []rune
	l.SetLocal(func(k rune) bool {
		seen = append(seen, k)
		return k == 't'
	})
	l.press('t')
	if l.cur == 't' {
		t.Error("the screen consumed t and the loop switched screens anyway")
	}
	l.press('d')
	if l.cur != 'd' {
		t.Error("the screen declined d and the loop did not fall through to the shortcut")
	}
	if len(seen) != 2 {
		t.Errorf("the screen was offered %d keys, want 2", len(seen))
	}
	// Cleared, or a screen's keys would follow the reader off it.
	l.SetLocal(nil)
	l.press('t')
	if len(seen) != 2 {
		t.Error("a cleared local handler still received a keystroke")
	}
}
