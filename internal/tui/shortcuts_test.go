package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
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

// The comment above Shortcuts() named a count that the literal below it did not
// have. Same shape as F2: a comment denying what the code does.
//
// It said "nine questions, nine keys" and "A TENTH DOES NOT FIT" sitting
// directly above a ten-element composite. The tenth is real: live ('l') and
// share ('p') are both wired screens. #240's scan cannot see this file — it
// is scoped to internal/proxy — so the disagreement lived next to tests that
// already knew there were ten.
//
// PASS: the doc names the length of the literal, and does not deny a key the
// slice already holds.
// FAIL: the comment claims nine (or any other count) while Shortcuts() returns
// a different number, or says a tenth does not fit while the tenth is there.
func TestShortcutsCommentAgreesWithTheLiteral(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "shortcuts.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("cannot parse shortcuts.go: %v", err)
	}

	var doc string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "Shortcuts" {
			continue
		}
		if fn.Doc == nil {
			t.Fatal("Shortcuts has no doc comment, so this test has nothing to compare to the literal")
		}
		doc = fn.Doc.Text()
		break
	}
	if doc == "" {
		t.Fatal("no Shortcuts declaration found; this test asserts nothing")
	}

	n := len(Shortcuts())
	if n == 0 {
		t.Fatal("Shortcuts() returned nothing, so a count agreement would pass over an empty surface")
	}

	// The first sentence is the claim. A pattern edited into not matching the
	// defect it was written for would pass while checking nothing — the hole
	// F2's scan carries a positive to close.
	claim := regexp.MustCompile(`(?i)Shortcuts is the whole surface:\s+(\w+) questions?,\s+(\w+) keys?`)
	const defect = "Shortcuts is the whole surface: nine questions, nine keys."
	if !claim.MatchString(defect) {
		t.Fatal("the claim pattern no longer matches its own defect; this test asserts nothing")
	}

	m := claim.FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("Shortcuts' doc no longer states the surface size in the form the defect used:\n%s\n"+
			"      Name the length of the literal (`ten questions, ten keys` today) rather than "+
			"dropping the sentence so a stale count can come back unnoticed.", doc)
	}
	for i, word := range []string{m[1], m[2]} {
		kind := []string{"questions", "keys"}[i]
		got, ok := spelledCount(word)
		if !ok {
			t.Errorf("Shortcuts' comment names %q %s, which is not a count this test can check", word, kind)
			continue
		}
		if got != n {
			t.Errorf("Shortcuts' comment claims %s %s; the literal below it has %d. "+
				"A comment that denies what the code does is F2, one package over.",
				word, kind, n)
		}
	}

	// The other half of the same lie: not just the wrong count, the sentence
	// that a tenth does not exist.
	if n >= 10 && tenthDenial.MatchString(doc) {
		t.Errorf("Shortcuts' comment says a tenth does not fit; the literal has %d entries. "+
			"The tenth is a wired screen, not a layout overflow.", n)
	}
}

var tenthDenial = regexp.MustCompile(`(?i)a tenth does not fit`)

func spelledCount(s string) (int, bool) {
	words := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
		"eleven": 11, "twelve": 12,
	}
	if n, ok := words[strings.ToLower(s)]; ok {
		return n, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}
