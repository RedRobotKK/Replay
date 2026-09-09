package tui

import (
	"strings"
	"testing"
)

// Triage: pick a finding and do something about it.
//
// The advise screen printed a ranked list and stopped. Ten screens did the
// same, which is why the TUI reads as a report renderer with keyboard
// shortcuts rather than a tool: nothing on it can be selected, nothing can be
// acted on, and nothing remembers what the reader already dealt with.
//
// The machinery was already there. Screen.Rows tells the loop how many
// selectable lines exist, Loop.Cursor tracks which one, and the cost screen has
// used enter-to-open since it was built. Advise never set Rows, so on that
// screen the movement keys did nothing at all.
//
// The order matters and it is deliberate: selection first, then status. A
// reader who cannot point at a finding cannot be asked what to do about it.

// TR1: the screen declares its selectable rows.
func TestTR1_TheScreenHasSelectableRows(t *testing.T) {
	sc := AdviseScreen(rows(), 12)
	if sc.Rows != len(rows()) {
		t.Errorf("the screen shows %d findings and declares %d selectable rows, so the "+
			"movement keys do nothing on it", len(rows()), sc.Rows)
	}
}

// TR2: the selected finding is visibly the selected one.
//
// A cursor the reader cannot see is not a cursor. This asserts the marker
// moves with the selection rather than that any particular glyph is used.
func TestTR2_TheSelectionIsVisible(t *testing.T) {
	first := strings.Join(AdviseScreenAt(rows(), 12, 0).Lines, "\n")
	second := strings.Join(AdviseScreenAt(rows(), 12, 1).Lines, "\n")
	if first == second {
		t.Fatal("moving the selection changed nothing on screen, so the reader cannot " +
			"tell which finding they are about to act on")
	}
	// And the marker must sit on the row it names, not float.
	marked := 0
	for _, l := range strings.Split(first, "\n") {
		if strings.Contains(l, selMarker) {
			marked++
		}
	}
	if marked != 1 {
		t.Errorf("%d lines carry the selection marker; exactly one row is selected", marked)
	}
}

// TR3: the screen says what can be done to the selected finding.
//
// Discoverability, not decoration. A key that exists and is never mentioned is
// a key nobody presses — the same defect as `replay tui --screen live`, which
// worked while the help text denied it.
func TestTR3_TheActionsAreOnScreen(t *testing.T) {
	body := strings.ToLower(strings.Join(AdviseScreenAt(rows(), 12, 0).Lines, "\n"))
	for _, want := range []string{"applied", "dismiss"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen offers no way to mark a finding %q, so triage is "+
				"looking at a list and closing it", want)
		}
	}
}

// TR4: an out-of-range selection cannot crash or select nothing.
//
// The loop clamps, but the screen is exported and callers get this wrong. A
// panic here takes the whole TUI down mid-keystroke.
func TestTR4_AnOutOfRangeSelectionIsClamped(t *testing.T) {
	for _, at := range []int{-5, 0, 99} {
		sc := AdviseScreenAt(rows(), 12, at)
		if len(sc.Lines) == 0 {
			t.Fatalf("selection %d produced an empty screen", at)
		}
		marked := 0
		for _, l := range sc.Lines {
			if strings.Contains(l, selMarker) {
				marked++
			}
		}
		if marked != 1 {
			t.Errorf("selection %d marked %d rows, want exactly 1", at, marked)
		}
	}
}

// TR5: enter shows the evidence behind the selected finding.
//
// The screen advertised "enter evidence" with nothing behind it. That is the
// overpromise pattern docs/design/unwired-3-branches-and-docs.md catalogues —
// the CLI printing a command it then rejects — and it was written into a help
// line during the same session that catalogued it.
//
// A finding says "Bash results are 30% of prompt tokens". The reader's next
// question is which sessions, how much, and how it was measured. Ranking
// without that is asking somebody to act on a number whose provenance they
// cannot see, and this repository refuses that everywhere else.
func TestTR5_EnterShowsTheEvidence(t *testing.T) {
	r := rows()[0]
	sc := AdviceDetail(r)
	body := strings.Join(sc.Lines, "\n")

	if !strings.Contains(body, r.Title) {
		t.Errorf("the detail does not name the finding it is about:\n%s", body)
	}
	// Every figure on the list must be reachable here, or the detail is a
	// second summary rather than the evidence.
	for _, want := range []string{"336,060", "3 transcript"} {
		if !strings.Contains(body, want) {
			t.Errorf("the detail does not carry %q from the finding:\n%s", want, body)
		}
	}
	if !strings.Contains(body, r.Action) {
		t.Errorf("the detail omits the action, which is the reason to open it:\n%s", body)
	}
	// And it must say how to get back, or the reader is stranded.
	if !strings.Contains(strings.ToLower(body), "esc") {
		t.Errorf("the detail offers no way back:\n%s", body)
	}
}

// TR6: a detail for a finding with nothing predicted says so.
//
// "not predicted on this corpus" is a result. Rendering an empty saving line,
// or a zero, would dress an absence as a modest win — the distinction
// measured.go exists to hold.
func TestTR6_ADetailWithNoPredictionSaysSo(t *testing.T) {
	r := rows()[1] // PredictedShare 0, Estimated true
	body := strings.Join(AdviceDetail(r).Lines, "\n")
	if strings.Contains(body, "0.0%") {
		t.Errorf("an absent prediction is rendered as 0.0%%:\n%s", body)
	}
	if !strings.Contains(body, "not predicted") {
		t.Errorf("the detail does not say the saving was not predicted:\n%s", body)
	}
}
