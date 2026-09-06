package tui

import (
	"strings"
	"testing"
)

// The key strip has to fit the smallest terminal this is designed for.
//
// It is the one line present on every screen, so if it wraps, every screen
// wraps. It went two columns over on the first attempt, which is not something
// reading catches.
func TestHintsFitTheBudget(t *testing.T) {
	for _, s := range Shortcuts() {
		h := Hints(s.Key)
		if len(h) > BudgetCols {
			t.Errorf("the key strip is %d columns with %q selected, budget is %d:\n%s",
				len(h), s.Label, BudgetCols, h)
		}
		if strings.ContainsRune(h, '\n') {
			t.Errorf("the key strip wrapped: %q", h)
		}
	}
}

// The strip must name what each key does.
//
// A row of bare letters is readable only by somebody who already knows the
// tool, which is precisely the audience this surface is not for: the premise
// is that most people will never type a flag and the screen has to carry them.
func TestHintsNameWhatTheKeysDo(t *testing.T) {
	h := Hints('c')
	for _, s := range Shortcuts() {
		if !strings.Contains(h, s.Label) {
			t.Errorf("key %q does not say what it does on the strip: %q", s.Key, h)
		}
	}
}

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
	if len(seen) > 9 {
		t.Errorf("%d shortcuts. More than nine will not fit one strip inside %d columns, "+
			"and a second row of hints is a menu", len(seen), BudgetCols)
	}
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
