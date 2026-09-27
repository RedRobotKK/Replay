package outputdiscipline

import "testing"

// Table 1 of docs/evidence/output-discipline-2026-09-25.md, re-derived.
//
// The published figures came from the `bench/` run set. Nothing in the
// document said so, and the corpus that holds those runs holds eight other
// experiment directories beside it. A reader who swept the whole corpus got
// 54,418 where the table says 53,700, and would reasonably have concluded the
// evidence was wrong. This test names the fixture set and proves the numbers
// follow from it.
func TestTable1ReDerivesFromBench(t *testing.T) {
	runs, err := Load("testdata/bench.json")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Cell{
		"haiku/verbose":  {N: 3, PromptTokens: 144256, CacheWrite: 53700, CostUSD: 0.1352, Turns: 17, Answers: []int{61, 61, 61}},
		"haiku/bounded":  {N: 3, PromptTokens: 72307, CacheWrite: 18545, CostUSD: 0.0467, Turns: 2, Answers: []int{60, 60, 60}},
		"sonnet/verbose": {N: 2, PromptTokens: 218399, CacheWrite: 81282, CostUSD: 0.3896, Turns: 18, Answers: []int{60, 60}},
		"sonnet/bounded": {N: 2, PromptTokens: 143135, CacheWrite: 32818, CostUSD: 0.1570, Turns: 3, Answers: []int{60, 60}},
		"opus/verbose":   {N: 2, PromptTokens: 433625, CacheWrite: 83301, CostUSD: 1.0942, Turns: 19, Answers: []int{60, 60}},
		"opus/bounded":   {N: 2, PromptTokens: 143752, CacheWrite: 35289, CostUSD: 0.4241, Turns: 4, Answers: []int{60, 60}},
		"fable/verbose":  {N: 2, PromptTokens: 185642, CacheWrite: 79139, CostUSD: 1.7195, Turns: 17, Answers: []int{60, 60}},
		"fable/bounded":  {N: 2, PromptTokens: 92685, CacheWrite: 32048, CostUSD: 0.6748, Turns: 2, Answers: []int{60, 60}},
	}
	got := Medians(runs)
	for k, w := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("%s missing from the derivation", k)
			continue
		}
		if g.N != w.N || g.PromptTokens != w.PromptTokens || g.CacheWrite != w.CacheWrite ||
			g.Turns != w.Turns || !sameInts(g.Answers, w.Answers) || !sameCost(g.CostUSD, w.CostUSD) {
			t.Errorf("%s\n  got  %+v\n  want %+v", k, g, w)
		}
	}
}

// Table 2, the n=10 replication, comes from a DIFFERENT fixture set.
//
// One document, two run sets. Conflating them is how the first table's n=3
// medians get compared against the second table's n=10 means.
func TestTable2ReDerivesFromN10(t *testing.T) {
	runs, err := Load("testdata/n10.json")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Spread{
		"haiku/verbose":  {N: 10, Mean: 54413, SD: 445, CostUSD: 0.1344, Turns: 17},
		"haiku/bounded":  {N: 10, Mean: 18116, SD: 505, CostUSD: 0.0438, Turns: 2},
		"sonnet/verbose": {N: 10, Mean: 77901, SD: 1786, CostUSD: 0.3699, Turns: 17},
		"sonnet/bounded": {N: 10, Mean: 33878, SD: 1856, CostUSD: 0.1734, Turns: 4},
	}
	got := Spreads(runs)
	for k, w := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("%s missing", k)
			continue
		}
		if g.N != w.N || g.Mean != w.Mean || g.SD != w.SD || g.Turns != w.Turns || !sameCost(g.CostUSD, w.CostUSD) {
			t.Errorf("%s\n  got  %+v\n  want %+v", k, g, w)
		}
	}
}
