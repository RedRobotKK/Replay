package tui

import (
	"strings"
	"testing"
	"unicode"
)

// Every outcome must fit the terminal it was designed for.
//
// Eighty by twenty-four is the floor. A screen that overflows it does not
// degrade gracefully: the hint line scrolls off, and the hint line is how the
// user reaches every other question. Losing it strands them on whichever screen
// happened to be too tall.
func TestOutcomesFitTheBudget(t *testing.T) {
	for _, sc := range Outcomes() {
		if len(sc.Lines) > BudgetRows {
			t.Errorf("%q is %d rows, budget is %d. The key strip would scroll off, and "+
				"that strip is the only way back to the other questions",
				sc.Title, len(sc.Lines), BudgetRows)
		}
		for i, l := range sc.Lines {
			if len(l) > BudgetCols {
				t.Errorf("%q line %d is %d columns, budget is %d:\n%s",
					sc.Title, i, len(l), BudgetCols, l)
			}
			for _, r := range l {
				if r > unicode.MaxASCII {
					t.Errorf("%q line %d has non-ASCII %q", sc.Title, i, r)
				}
			}
		}
	}
}

// The answer comes before the evidence for it.
//
// A screen that opens with a table asks the reader to do the interpreting,
// which is the work they pressed the key to avoid. The headline is in the first
// four rows or the screen is built the wrong way round.
func TestOutcomesLeadWithTheAnswer(t *testing.T) {
	for _, sc := range Outcomes() {
		head := strings.Join(sc.Lines[:4], "\n")
		if !strings.ContainsAny(head, "0123456789") {
			t.Errorf("%q does not state a figure in its first four rows, so it opens with "+
				"furniture rather than an answer:\n%s", sc.Title, head)
		}
	}
}

// Every screen says what produced it, and reaches every other question.
func TestOutcomesCarryProvenanceAndAWayOut(t *testing.T) {
	for _, sc := range Outcomes() {
		body := strings.Join(sc.Lines, "\n")
		if !strings.Contains(body, "ran   replay ") {
			t.Errorf("%q does not print the command it ran. A screen that acts on your "+
				"behalf without showing what it did is asking for trust it has not "+
				"earned", sc.Title)
		}
		last := sc.Lines[len(sc.Lines)-1]
		for _, other := range Shortcuts() {
			if !strings.Contains(last, other.Label) {
				t.Errorf("%q cannot reach %q. Every question is one keystroke from every "+
					"screen, or the answer somebody wants is three screens away and they "+
					"stop looking", sc.Title, other.Label)
			}
		}
	}
}

// A screen that reports something the user cannot act on has to say so.
//
// Three of these carry a refusal rather than a number: a cap that cannot fire,
// a projection whose band is the estimator's own floor, and paths that cannot
// be masked. Those are the screens that justify the surface, and a design that
// quietly dropped them would be a demo.
func TestOutcomesKeepTheUncomfortableOnes(t *testing.T) {
	want := map[rune]string{
		'g': "NOT ENFORCED",
		'm': "estimator's own floor",
		's': "cannot be masked",
	}
	for key, phrase := range want {
		body := strings.Join(Outcome(key).Lines, "\n")
		if !strings.Contains(body, phrase) {
			t.Errorf("the %q screen no longer says %q. That line is the reason the screen "+
				"is worth drawing", string(key), phrase)
		}
	}
}
