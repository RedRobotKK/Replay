package main

import (
	"path/filepath"
	"testing"
	"time"
)

// The ask converts on reciprocity, and repetition destroys it.
//
// Reciprocity is the strongest driver available here and it is also the most
// fragile: it works because the tool gave something concrete first, and it
// stops working the moment the ask reads as a toll. Replay asked on EVERY
// qualifying run. Somebody running `replay cost` daily saw the same request
// thirty times a month, and by the fifth it is no longer a thank-you, it is
// nagging - which converts worse AND costs the goodwill that made the first
// one work.
//
// So the highest-leverage change for conversion is restraint, not persuasion.
// One ask, then silence, then one more only if the tool has since found
// materially more than it had when it last asked.

// TF-1: the first qualifying run asks.
func TestTF1_TheFirstAskIsShown(t *testing.T) {
	dir := t.TempDir()
	if !shouldAsk(dir, 100, t0Tip) {
		t.Error("a first qualifying run must ask")
	}
}

// TF-2: a second run the same day does not.
//
// PASS: silence.
// FAIL: the nag that made the first ask worthless.
func TestTF2_TheSecondRunIsSilent(t *testing.T) {
	dir := t.TempDir()
	if !shouldAsk(dir, 100, t0Tip) {
		t.Fatal("setup: the first ask must be shown")
	}
	noteAsked(dir, 100, t0Tip)
	if shouldAsk(dir, 100, t0Tip.Add(time.Minute)) {
		t.Error("asked twice in one minute")
	}
	if shouldAsk(dir, 100, t0Tip.Add(29*24*time.Hour)) {
		t.Error("asked again inside the cooldown")
	}
}

// TF-3: after the cooldown, one more ask.
//
// Not never. Someone who has used the tool for a month and been shown the
// finding again is a reasonable person to thank a second time.
func TestTF3_AfterTheCooldownItAsksAgain(t *testing.T) {
	dir := t.TempDir()
	noteAsked(dir, 100, t0Tip)
	if !shouldAsk(dir, 100, t0Tip.Add(31*24*time.Hour)) {
		t.Error("the ask must return after the cooldown")
	}
}

// TF-4: materially more found re-opens the ask early.
//
// The cooldown is about repetition, not about a ceiling on gratitude. If the
// tool has since found several times more than it had when it last asked, that
// is new value delivered and the reciprocity is fresh.
//
// PASS: a large increase asks; a small one does not.
// FAIL: any increase re-opening it, which is the nag with a loophole.
func TestTF4_MateriallyMoreReopensTheAsk(t *testing.T) {
	dir := t.TempDir()
	noteAsked(dir, 100, t0Tip)
	soon := t0Tip.Add(2 * 24 * time.Hour)
	if shouldAsk(dir, 140, soon) {
		t.Error("a 40% increase is not new value, it is the same finding growing")
	}
	if !shouldAsk(dir, 400, soon) {
		t.Error("four times the finding is new value and may ask again")
	}
}

// TF-5: an unusable state directory still asks exactly once per process.
//
// PASS: it asks. A reader who cannot be remembered is better served by one ask
// than by silence.
// FAIL: nothing, which would silently disable the only revenue path on any
// machine with a read-only home.
func TestTF5_UnreadableStateStillAsks(t *testing.T) {
	if !shouldAsk(filepath.Join(t.TempDir(), "does", "not", "exist"), 100, t0Tip) {
		t.Error("with no usable state the ask must still be shown")
	}
}

var t0Tip = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
