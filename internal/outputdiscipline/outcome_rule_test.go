package outputdiscipline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The n=10 set of 2026-09-25 read by the answer rule `replay outcome` ships.
//
// outcome.Score reads a reply with Run.Answer, the function tested here, so
// this is the shipped rule and not a second one. It is only the answer half
// of Score: these historical runs left no run.json or tree/, so nothing else
// Score checks can be checked, and no verdict below is a full Score.
//
// Three measures, kept apart:
//   - rule: Answer() reads 60. What `replay outcome` would count as a correct
//     answer.
//   - numeric: the reply states 60 as its answer, whether or not Answer() can
//     read it. Decided per run below from the reply text, by hand, and pinned
//     by run name because the fixture is pinned by hash.
//   - format: Answer() can read a leading integer at all.
//
// A reply that is numerically right and unreadable is a format failure, not a
// reasoning failure, and is never counted as either a PASS or a wrong answer.

// The four replies that answer 61.
var n10WrongAnswers = map[string]int{
	"n10/verbose_1.json": 61, "n10/verbose_6.json": 61, "n10/verbose_7.json": 61, "n10/verbose_10.json": 61,
}

// The five replies that state 60 in a form Answer() does not read: bold, or a
// per-file breakdown ending in bold. Each one contains "**60**".
var n10FormatOnly = map[string]bool{
	"n10/verbose_5.json": true, "n10/sonnet_verbose_1.json": true, "n10/sonnet_verbose_3.json": true,
	"n10/sonnet_verbose_8.json": true, "n10/sonnet_verbose_9.json": true,
}

type ruleTally struct{ n, rule, numeric, format int }

func TestN10UnderTheShippedAnswerRule(t *testing.T) {
	runs, err := Load("testdata/n10.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 40 {
		t.Fatalf("n10 holds %d runs, want 40", len(runs))
	}
	seen := map[string]bool{}
	got := map[string]ruleTally{}
	for _, r := range runs {
		if seen[r.Run] {
			t.Fatalf("%s appears twice; a duplicate would inflate its cell", r.Run)
		}
		seen[r.Run] = true
		cell := r.Model + "/" + r.Arm
		tl := got[cell]
		tl.n++
		a, ok := r.Answer()
		switch {
		case ok && a == 60:
			tl.rule++
			tl.numeric++
			tl.format++
		case ok:
			if n10WrongAnswers[r.Run] != a {
				t.Errorf("%s reads %d, which is not a classified wrong answer", r.Run, a)
			}
			tl.format++
		default:
			if !n10FormatOnly[r.Run] || !strings.Contains(r.Result, "**60**") {
				t.Errorf("%s has no leading integer and is not a classified format-only reply: %.60q", r.Run, r.Result)
			}
			tl.numeric++
		}
		got[cell] = tl
	}
	want := map[string]ruleTally{
		"haiku/bounded":  {n: 10, rule: 10, numeric: 10, format: 10},
		"haiku/verbose":  {n: 10, rule: 5, numeric: 6, format: 9},
		"sonnet/bounded": {n: 10, rule: 10, numeric: 10, format: 10},
		"sonnet/verbose": {n: 10, rule: 6, numeric: 10, format: 6},
	}
	for cell, w := range want {
		if got[cell] != w {
			t.Errorf("%s: %+v, want %+v", cell, got[cell], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("cells %v, want exactly %d", got, len(want))
	}
}

// The rule's edges, as the implementation reads them today. These pin
// behaviour; they do not propose a new contract.
func TestTheAnswerRuleAtItsEdges(t *testing.T) {
	for _, c := range []struct {
		reply string
		want  int
		ok    bool
	}{
		{"60", 60, true},
		{"  60", 60, true},
		{"\n\t60\n", 60, true},
		{"60.", 60, true},
		{"60 test functions", 60, true},
		{"59", 59, true},
		{"061", 61, true},
		{"**60**", 0, false},
		{"The answer is 60", 0, false},
		{"-60", 0, false},
		{"sixty", 0, false},
		{"", 0, false},
		{"   ", 0, false},
	} {
		a, ok := Run{Result: c.reply}.Answer()
		if ok != c.ok || (ok && a != c.want) {
			t.Errorf("Answer(%q) = %d, %v; want %d, %v", c.reply, a, ok, c.want, c.ok)
		}
	}
}

// The published headroom figure must say which measure it is. Before
// 2026-10-23 OC-4 and UNWIRED-LOG #19 named "haiku, verbose, 6 of 10" as the
// one cell below ceiling: true of numeric correctness, false under the rule
// `replay outcome` applies, where haiku verbose is 5 of 10 and sonnet verbose,
// 10 of 10 numerically, is 6 of 10.
func TestThePublishedHeadroomFigureNamesItsMeasure(t *testing.T) {
	for _, doc := range []string{"docs/requirements.md", "docs/design/UNWIRED-LOG.md"} {
		b, err := os.ReadFile(filepath.Join("..", "..", doc))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(b)), " ")
		for _, want := range []string{"haiku, verbose, 5 of 10", "sonnet, verbose, 6 of 10", "numerically"} {
			if !strings.Contains(strings.ToLower(text), want) {
				t.Errorf("%s does not state %q; the headroom figure must be the rule's, with the numeric reading named as such", doc, want)
			}
		}
		// The old wording may survive only where it is quoted as corrected,
		// so the correction stays auditable.
		for _, para := range strings.Split(string(b), "\n\n") {
			p := strings.Join(strings.Fields(para), " ")
			for _, line := range strings.Split(p, " | OC-") {
				old := strings.Contains(line, "the one sub-ceiling cell") || strings.Contains(line, "the one cell below ceiling")
				if old && !strings.Contains(line, "Corrected 2026-10-23") {
					t.Errorf("%s still calls haiku verbose the one cell below ceiling, outside a dated correction", doc)
				}
			}
		}
	}
}
