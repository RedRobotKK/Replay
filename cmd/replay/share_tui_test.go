package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/card"
	"github.com/RedRobotKK/Replay/internal/tui"
)

func aPreviewable() tui.ShareState {
	return tui.ShareState{
		Data: card.Data{
			AvoidableShare:      0.03,
			Tasks:               1384,
			Breaks:              748,
			MedianUSD:           0.77,
			P90USD:              2.30,
			PeakAvoidableTokens: 32_635_820,
		},
		Ready: true, Tasks: 1384,
		Variant: card.VariantB, Tone: card.ToneRekt,
	}
}

// The preview and the file are one card.
//
// A preview that can disagree with the thing it writes is worse than no
// preview: the reader posts the file, not the screen. So the words on screen
// and the bytes on disk have to come from one card.Data through one renderer,
// and this asserts both halves of that against the same state.
//
// PASS: every line the card says is on the screen, and the file the screen
// wrote is byte-identical to encoding that same Data in that same design and
// register.
// FAIL: a preview showing something the file does not carry, or a file built
// from anything but the previewed state.
func TestSharePreviewAndFileAreOneCard(t *testing.T) {
	st := aPreviewable()
	frame := strings.Join(tui.ShareScreen(st).Lines, "\n")
	said := card.Say(st.Variant, st.Tone, st.Data).Lines
	if len(said) < 5 {
		t.Fatalf("the card says %d lines; this check would pass over nothing", len(said))
	}
	for _, l := range said {
		if !strings.Contains(frame, l) {
			t.Errorf("the card says %q and the preview does not show it:\n%s", l, frame)
		}
	}

	// A relative directory, deliberately. Handed an absolute one the write
	// returns an absolute path whether or not it resolves anything, so the
	// check below would be satisfied by its argument rather than by the code.
	t.Chdir(t.TempDir())
	path, err := shareWrite(".", st)
	if err != nil {
		t.Fatalf("writing the previewed card: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("the write returned %q, which is not an absolute path. A key with a "+
			"filesystem side effect has to say where", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := card.Encode(&want, st.Variant, st.Tone, st.Data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want.Bytes()) {
		t.Errorf("the file is not the card that was previewed (%d bytes on disk against "+
			"%d for the previewed state)", len(got), want.Len())
	}
}

// t and d change what is previewed, and therefore what w writes.
func TestShareKeysToggleToneAndDesign(t *testing.T) {
	dir := t.TempDir()
	u := shareUI{variant: card.VariantC, tone: card.ToneMeasured}
	// The state the reader is looking at, which in the running surface is
	// always built from u. Rebuilt on every press here for the same reason:
	// a toggle answers the frame that was on screen.
	st := func() tui.ShareState {
		s := aPreviewable()
		s.Variant, s.Tone = u.variant, u.tone
		return s
	}

	if !u.press('t', st(), dir) || u.tone != card.ToneRekt {
		t.Errorf("t did not switch the register: %v", u.tone)
	}
	if !u.press('t', st(), dir) || u.tone != card.ToneMeasured {
		t.Errorf("t does not toggle back: %v", u.tone)
	}
	if !u.press('d', st(), dir) || u.variant != card.VariantB {
		t.Errorf("d did not switch the design: %v", u.variant)
	}
	if !u.press('d', st(), dir) || u.variant != card.VariantC {
		t.Errorf("d does not toggle back: %v", u.variant)
	}
	// Anything else belongs to the surface, or the questions become
	// unreachable from this screen.
	for _, k := range []rune{'c', 'x', 'a', 'g', 'm', 's', 'q', '?'} {
		if u.press(k, st(), dir) {
			t.Errorf("the share screen swallowed %q, which is somebody else's key", k)
		}
	}
}

// w writes, and the screen then states the absolute path.
func TestShareWriteSaysWhatItDid(t *testing.T) {
	t.Chdir(t.TempDir())
	u := shareUI{variant: card.VariantB, tone: card.ToneRekt}
	if !u.press('w', aPreviewable(), ".") {
		t.Fatal("w was not handled by the screen that advertises it")
	}
	if u.failed != "" {
		t.Fatalf("the write failed: %s", u.failed)
	}
	if !filepath.IsAbs(u.wrote) {
		t.Errorf("wrote %q, which is not an absolute path", u.wrote)
	}
	if _, err := os.Stat(u.wrote); err != nil {
		t.Errorf("the screen says it wrote %s and there is no file there: %v", u.wrote, err)
	}
	body := strings.Join(tui.ShareScreen(tui.ShareState{
		Data: aPreviewable().Data, Ready: true, Tasks: 1384,
		Variant: u.variant, Tone: u.tone, Wrote: u.wrote,
	}).Lines, "\n")
	if !strings.Contains(body, filepath.Base(u.wrote)) {
		t.Errorf("the screen does not name the file it wrote:\n%s", body)
	}
}

// w refuses on exactly the condition `replay cost --share` refuses on, and
// leaves nothing behind.
func TestShareWriteRefusesWhenNothingIsMeasured(t *testing.T) {
	dir := t.TempDir()
	u := shareUI{variant: card.VariantC, tone: card.ToneMeasured}
	if !u.press('w', tui.ShareState{Variant: card.VariantC, Tone: card.ToneMeasured}, dir) {
		t.Fatal("w was not handled at all")
	}
	if u.wrote != "" {
		t.Errorf("a card was written over a corpus with nothing in it: %s", u.wrote)
	}
	if !strings.Contains(u.failed, "nothing measured") {
		t.Errorf("the refusal does not say why: %q", u.failed)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the refusal left %d file(s) behind; a file that exists reads as a "+
			"card that worked", len(entries))
	}
}

// The share screen reads real cost data, so it is Measured and carries no
// example-data banner.
//
// The corpus is a real one built for the test rather than a hand-made state,
// so what is asserted is the whole path: the walk, the report, the guard, the
// card, and the screen.
func TestShareScreenOverARealCorpusIsMeasured(t *testing.T) {
	m := tui.Machine{}
	dir := corpusAt(t, "proj", 3)
	t.Setenv("REPLAY_TRANSCRIPTS", dir)
	m.Found = true
	m = costState(m)
	if !m.ShareOK {
		t.Fatalf("a priced corpus produced nothing worth sharing: %+v", m.Card)
	}
	sc := tui.ShareScreen(shareState(m, shareUI{variant: card.VariantC, tone: card.ToneMeasured}))
	if sc.From != tui.Measured {
		t.Errorf("the share screen over a real corpus declares itself %v", sc.From)
	}
	if tui.Marked(sc.Lines) {
		t.Error("the share screen stamped measured figures as example data")
	}

	// And the guard must be able to answer no, or the check above is satisfied
	// by a screen that never asks it. A transcript that exists and prices to
	// nothing is the condition the guard is actually about; a directory with no
	// transcripts is caught earlier by a different check.
	// The corpus path can only ever exercise half the guard. A report with no
	// rows returns before the guard is reached at all, so the seam is asked
	// directly below for the other half.
	nothing := t.TempDir()
	if err := os.WriteFile(filepath.Join(nothing, "nothing.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPLAY_TRANSCRIPTS", nothing)
	empty := costState(tui.Machine{Found: true})
	if empty.ShareOK {
		t.Error("a corpus that priced to nothing was reported as worth posting. The " +
			"screen must ask the command's own guard, not decide for itself")
	}
	blank := tui.ShareScreen(shareState(empty, shareUI{variant: card.VariantC, tone: card.ToneMeasured}))
	if blank.From != tui.Unavailable {
		t.Errorf("a screen with nothing behind it declares itself %v", blank.From)
	}
}

// The screen asks the command's own guard, both halves of it.
//
// A corpus-shaped test can only reach one: a report with no rows never gets as
// far as the guard, so `--share` refusing and the screen refusing agree there
// for the wrong reason. The condition that separates them is rows that priced
// to nothing, which is the half that has to be asked directly.
//
// PASS: priced rows are postable, rows with no priced spend are not, and an
// empty summary is not.
// FAIL: any of the three answered the other way, which is a screen deciding
// for itself what the command already decides.
func TestTheShareScreenAsksTheCommandsOwnGuard(t *testing.T) {
	priced := costSummary{
		Tasks: 1384, Unit: unitSession, MedianUSD: 0.77, P90USD: 2.30,
		AvoidableShare: 0.03,
	}
	d, ok := shareFrom(priced, 748, 32_635_820)
	if !ok {
		t.Error("a priced corpus was reported as having nothing worth posting")
	}
	if d.PeakAvoidableTokens != 32_635_820 || d.Tasks != 1384 || d.Breaks != 748 {
		t.Errorf("the figures were re-derived rather than taken from the report: %+v", d)
	}

	unpriced := priced
	unpriced.MedianUSD = 0
	if _, ok := shareFrom(unpriced, 748, 32_635_820); ok {
		t.Error("rows that priced to nothing were reported as worth posting. " +
			"`replay cost --share` refuses on exactly this condition, and a screen " +
			"that offered the card anyway would be offering one nobody stands behind")
	}
	if _, ok := shareFrom(costSummary{Unit: unitSession}, 0, 0); ok {
		t.Error("an empty summary was reported as worth posting")
	}
}
