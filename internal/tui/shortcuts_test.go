package tui

import (
	"strings"
	"testing"
)

// Every question is reachable from every screen.
func TestEveryQuestionIsOneKeystrokeAway(t *testing.T) {
	seen := map[rune]string{}
	for _, s := range Shortcuts() {
		if prev, dup := seen[s.Key]; dup {
			t.Errorf("key %q is bound to both %q and %q", s.Key, prev, s.Question)
		}
		seen[s.Key] = s.Question
		if s.Question == "" || s.Answers == "" {
			t.Errorf("shortcut %q has no question or no answer line", s.Key)
		}
		if !strings.HasSuffix(s.Question, "?") {
			t.Errorf("%q is not phrased as the question a user would ask: %q", s.Key, s.Question)
		}
	}
	// There is no cap on how many questions this surface may have, and there
	// is no longer a layout pretending to impose one.
	//
	// It used to be a hardcoded nine, then a one-line key strip measured
	// against eighty columns. That strip had no callers in its whole life, so
	// the ceiling was being enforced for a line no reader had seen. It is gone;
	// the index is Help(), which TestHelpCarriesEveryQuestion measures against
	// the same twenty-four rows and which a reader reaches with the keystroke
	// the footer advertises.
}

// The provenance line must show the real command, never a paraphrase.
func TestRanShowsACopyableCommand(t *testing.T) {
	for _, s := range Shortcuts() {
		got := Ran(s)[0]
		if !strings.Contains(got, "replay "+s.Command) {
			t.Errorf("the provenance line for %q does not name the command it ran: %q",
				s.Label, got)
		}
		for _, f := range s.Flags {
			if !strings.Contains(got, f) {
				t.Errorf("%q ran with %s and the screen does not say so: %q", s.Label, f, got)
			}
		}
	}
}
