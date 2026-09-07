package tui

import (
	"strings"
	"testing"
	"unicode"
)

// Every error screen must say what still works.
//
// This is the part usually missing and the part that decides what the reader
// does next. A proxy refusing one path is still proxying every other path, and
// somebody who cannot tell the difference kills the whole thing over a warning
// about one endpoint.
func TestErrorsSayWhatIsUnaffected(t *testing.T) {
	for _, p := range Problems() {
		if strings.TrimSpace(p.Unaffected) == "" {
			t.Errorf("%q does not say what is still working. The reader has to assume, "+
				"and they will assume the worst", p.Code)
		}
		if strings.Contains(strings.ToLower(p.Unaffected), "nothing") &&
			p.Code != "port-taken" {
			t.Errorf("%q claims nothing is unaffected. That is true for a refusal to "+
				"start and almost never otherwise", p.Code)
		}
	}
}

// Every error screen must offer something to run or decide.
//
// A message that describes a problem and stops has moved the work to the
// reader without giving them anything to work with.
func TestErrorsOfferAWayForward(t *testing.T) {
	for _, p := range Problems() {
		if len(p.Do) == 0 {
			t.Errorf("%q offers no next step", p.Code)
			continue
		}
		joined := strings.Join(p.Do, "\n")
		hasCommand := strings.Contains(joined, "replay ") ||
			strings.Contains(joined, "chmod ") ||
			strings.Contains(joined, "curl ") ||
			strings.Contains(joined, "lsof ")
		hasChoice := strings.Contains(strings.ToLower(joined), "or ")
		if !hasCommand && !hasChoice {
			t.Errorf("%q gives neither a command to paste nor a decision to make:\n%s",
				p.Code, joined)
		}
	}
}

// The consequence comes before the mechanism.
//
// "Replay is forwarding this traffic and reading none of it" is what happened.
// "This path is not one of the two request shapes Replay parses" is why. A
// reader who only reads the first line should still know where they stand.
func TestErrorsLeadWithTheConsequence(t *testing.T) {
	for _, p := range Problems() {
		if !strings.HasSuffix(p.Happened, ".") {
			t.Errorf("%q does not open with a sentence: %q", p.Code, p.Happened)
		}
		if len(p.Happened) > 72 {
			t.Errorf("%q opens with %d characters. The first line is the one that gets "+
				"read; anything past a line is the second line", p.Code, len(p.Happened))
		}
		low := strings.ToLower(p.Happened)
		for _, jargon := range []string{"nil", "err", "panic", "0x", "goroutine", "eof"} {
			if strings.Contains(low, jargon) {
				t.Errorf("%q leads with %q, which is the exception rather than the "+
					"consequence", p.Code, jargon)
			}
		}
	}
}

// A code is stable; a message is not.
//
// Messages get rewritten. A user searching for one, or an agent branching on
// one, needs something that does not move.
func TestErrorsCarryAStableCode(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Problems() {
		if p.Code == "" {
			t.Error("a problem with no code cannot be searched for or branched on")
		}
		if seen[p.Code] {
			t.Errorf("%q is used twice", p.Code)
		}
		seen[p.Code] = true
		for _, r := range p.Code {
			if !unicode.IsLower(r) && r != '-' {
				t.Errorf("code %q is not lowercase-and-hyphen, so it is not greppable "+
					"without quoting", p.Code)
				break
			}
		}
	}
}

// The screens fit, and stay ASCII like everything else.
func TestErrorScreensFitTheBudget(t *testing.T) {
	for _, p := range Problems() {
		lines := p.Render()
		if len(lines) > BudgetRows {
			t.Errorf("%q is %d rows, budget %d", p.Code, len(lines), BudgetRows)
		}
		for i, l := range lines {
			if len(l) > BudgetCols {
				t.Errorf("%q line %d is %d columns:\n%s", p.Code, i, len(l), l)
			}
			for _, r := range l {
				if r > unicode.MaxASCII {
					t.Errorf("%q line %d has non-ASCII %q", p.Code, i, r)
				}
			}
		}
	}
}

// Most refusals are not failures, and the screen has to know the difference.
func TestMostRefusalsDoNotStopTheRun(t *testing.T) {
	blocking := 0
	for _, p := range Problems() {
		if p.Blocking() {
			blocking++
		}
	}
	if blocking == 0 {
		t.Error("no problem is blocking, so the distinction carries no information")
	}
	if blocking == len(Problems()) {
		t.Error("every problem is blocking. A refusal to read one path is not a " +
			"refusal to run, and treating them the same teaches the reader to " +
			"ignore both")
	}
}
