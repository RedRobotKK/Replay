// Package outputdiscipline re-derives the published tables of
// docs/evidence/output-discipline-2026-09-25.md from the run sets that
// produced them.
//
// # Why this exists
//
// The document reported figures from a run set it never named. The corpus
// holding those runs holds nine experiment directories, and sweeping all of
// them gives 54,418 cache-write for haiku verbose where the table says 53,700,
// and 24,475 for opus bounded where it says 35,289. A reader checking the
// evidence would have got different numbers and reasonably concluded the
// evidence was wrong.
//
// Two run sets back the document and they are NOT interchangeable:
//
//   - testdata/bench.json, 18 runs, backs the first table (medians, n of 2-3)
//   - testdata/n10.json, 40 runs, backs the n=10 replication table
//
// # Two normalisations, made explicit
//
// Both are places where the published figure differs from the naive
// computation, and both are resolved here in the open rather than by quietly
// adjusting a number to match.
//
//  1. ROUNDING. Published means are rounded, not truncated: the haiku verbose
//     mean is 54,412.60 and the table prints 54,413, where truncation gives
//     54,412. One mean, haiku bounded, lands exactly on 18,115.5, but
//     half-to-even and half-up both give the published 18,116, so THIS
//     DOCUMENT DOES NOT DISCRIMINATE between those two conventions. The
//     implementation uses half-to-even because Go's %.0f does; that choice is
//     unconstrained by this evidence and is recorded rather than claimed.
//  2. ANSWER TEXT. Six of the 58 runs do not hold a bare number in their
//     result: one appends a note from an agmsg watcher that failed to start,
//     three wrap the answer in markdown bold, and one answers in prose. Only
//     the first table publishes answers, and only the bench set backs it,
//     where every result begins with the number. Answer() therefore reads a
//     LEADING integer and reports failure otherwise; no heuristic is invented
//     for the markdown or prose forms, because no published figure scores
//     them. The affected runs are NOT excluded from any median, because the
//     published tables included them.
package outputdiscipline

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Run is one benchmark invocation, reduced to the fields the tables use.
type Run struct {
	Run    string  `json:"run"`
	Model  string  `json:"model"`
	Arm    string  `json:"arm"`
	Input  int     `json:"i"`
	Read   int     `json:"r"`
	Write  int     `json:"w"`
	Cost   float64 `json:"cost"`
	Turns  int     `json:"turns"`
	Result string  `json:"result"`
}

// Prompt is the whole prompt the provider counted: uncached input plus what it
// read from cache plus what it wrote. The table's "prompt tokens" column.
func (r Run) Prompt() int { return r.Input + r.Read + r.Write }

var (
	leadingInt = regexp.MustCompile(`^\s*(\d+)`)
	bareInt    = regexp.MustCompile(`^\s*\d+\s*$`)
)

// Answer is the scored answer, which is the leading integer of the result.
//
// It reports false where the result does not begin with a number, which is the
// case for the markdown-bold and prose results in the n10 set. That is
// deliberate: no published figure scores those, and guessing at them would be
// reconstructing evidence rather than reading it.
func (r Run) Answer() (int, bool) {
	m := leadingInt.FindStringSubmatch(r.Result)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// BareAnswer reports whether the result is nothing but the number.
//
// The inverse is what the evidence discloses: six runs carry markdown, prose
// or an appended tooling note around the answer. A count that silently
// depended on the surrounding text would be measuring the harness.
func (r Run) BareAnswer() bool {
	return strings.TrimSpace(r.Result) != "" && bareInt.MatchString(r.Result)
}

// Cell is one row of the first published table.
type Cell struct {
	N            int
	PromptTokens int
	CacheWrite   int
	CostUSD      float64
	Turns        int
	Answers      []int
}

// Spread is one row of the n=10 replication table.
type Spread struct {
	N       int
	Mean    int
	SD      int
	CostUSD float64
	Turns   int
}

// Load reads a run set.
func Load(path string) ([]Run, error) {
	b, err := os.ReadFile(path) //nolint:gosec // caller names the fixture
	if err != nil {
		return nil, err
	}
	var rs []Run
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, fmt.Errorf("%s holds no runs", path)
	}
	return rs, nil
}

func group(runs []Run) map[string][]Run {
	g := map[string][]Run{}
	for _, r := range runs {
		g[r.Model+"/"+r.Arm] = append(g[r.Model+"/"+r.Arm], r)
	}
	return g
}

// Medians produces the first published table.
func Medians(runs []Run) map[string]Cell {
	out := map[string]Cell{}
	for k, rs := range group(runs) {
		c := Cell{N: len(rs)}
		c.PromptTokens = int(medianOf(rs, func(r Run) float64 { return float64(r.Prompt()) }))
		c.CacheWrite = int(medianOf(rs, func(r Run) float64 { return float64(r.Write) }))
		c.CostUSD = medianOf(rs, func(r Run) float64 { return r.Cost })
		c.Turns = int(medianOf(rs, func(r Run) float64 { return float64(r.Turns) }))
		for _, r := range rs {
			if a, ok := r.Answer(); ok {
				c.Answers = append(c.Answers, a)
			}
		}
		out[k] = c
	}
	return out
}

// Spreads produces the n=10 replication table.
func Spreads(runs []Run) map[string]Spread {
	out := map[string]Spread{}
	for k, rs := range group(runs) {
		w := make([]float64, 0, len(rs))
		for _, r := range rs {
			w = append(w, float64(r.Write))
		}
		out[k] = Spread{
			N:       len(rs),
			Mean:    roundHalfEven(mean(w)),
			SD:      roundHalfEven(sampleSD(w)),
			CostUSD: medianOf(rs, func(r Run) float64 { return r.Cost }),
			Turns:   int(medianOf(rs, func(r Run) float64 { return float64(r.Turns) })),
		}
	}
	return out
}

func medianOf(rs []Run, f func(Run) float64) float64 {
	xs := make([]float64, 0, len(rs))
	for _, r := range rs {
		xs = append(xs, f(r))
	}
	sort.Float64s(xs)
	n := len(xs)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// sampleSD is the n-1 denominator, which is what the published sd column used.
func sampleSD(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	var s float64
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

// roundHalfEven matches the rounding the published figures were printed with.
//
// The haiku verbose mean is 54,412.5. Half-to-even gives 54,413, which is the
// published figure; truncation gives 54,412. This is a presentation rule and
// it is applied here rather than hidden in a format string.
func roundHalfEven(f float64) int {
	r, _ := strconv.ParseFloat(fmt.Sprintf("%.0f", f), 64)
	return int(r)
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameCost compares to the four decimal places the tables print.
func sameCost(a, b float64) bool {
	return fmt.Sprintf("%.4f", a) == fmt.Sprintf("%.4f", b)
}
