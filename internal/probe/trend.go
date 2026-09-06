package probe

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"time"
)

// Reading the series back.
//
// The series is only worth keeping if what it says can be trusted, and the
// pressure on a dated measurement is always toward finding news in it. Three
// rules hold that back, and every one of them costs a headline:
//
//   - A change is claimed only when two brackets cannot both be true. Brackets
//     that overlap are both consistent with one unchanged floor, and calling
//     that movement is how a series manufactures news.
//   - Readings taken with different methods are never compared. The method
//     moved the numbers four times on 2026-09-05; a difference across that
//     boundary says nothing about the provider.
//   - A run that established no bracket is not a data point.

// Change is a floor that provably moved between two readings.
type Change struct {
	Model string
	// At is the timestamp of the first reading that could not agree with what
	// came before — the earliest moment the change is known to have happened
	// by, not the moment it happened.
	At                    string
	FromAbove, FromAtMost int
	ToAbove, ToAtMost     int
}

// MethodBreak is a point where the instrument changed, so readings either side
// are not comparable.
type MethodBreak struct {
	Model    string
	At       string
	From, To string
}

// comparable reports a reading that can be measured against another.
func (r Reading) comparable() bool {
	return r.Outcome == "" && r.AtMost > 0
}

// disjoint reports two brackets that cannot both describe one floor.
//
// Brackets are (above, atMost]. They agree if they share any value; only a
// gap between them proves the floor moved.
func disjoint(a, b Reading) bool {
	return a.AtMost <= b.Above || b.AtMost <= a.Above
}

// Changes returns every provable movement, in time order.
func Changes(rs []Reading) []Change {
	var out []Change
	for _, group := range byModel(rs) {
		var last *Reading
		for i := range group {
			r := group[i]
			if !r.comparable() {
				continue
			}
			if last == nil || last.Method != r.Method {
				// A method break resets the comparison rather than producing
				// one. See MethodBreaks.
				cur := r
				last = &cur
				continue
			}
			if disjoint(*last, r) {
				out = append(out, Change{
					Model: r.Model, At: r.TakenAt,
					FromAbove: last.Above, FromAtMost: last.AtMost,
					ToAbove: r.Above, ToAtMost: r.AtMost,
				})
			}
			cur := r
			last = &cur
		}
	}
	return out
}

// MethodBreaks reports where the instrument changed between readings.
//
// Surfaced rather than hidden: a reader comparing two numbers across a break
// is comparing two instruments, and the series should say so instead of
// silently declining to draw a line.
func MethodBreaks(rs []Reading) []MethodBreak {
	var out []MethodBreak
	for _, group := range byModel(rs) {
		var prev *Reading
		for i := range group {
			r := group[i]
			if !r.comparable() {
				continue
			}
			if prev != nil && prev.Method != r.Method {
				out = append(out, MethodBreak{Model: r.Model, At: r.TakenAt, From: prev.Method, To: r.Method})
			}
			cur := r
			prev = &cur
		}
	}
	return out
}

// byModel groups readings by model, each in time order.
func byModel(rs []Reading) map[string][]Reading {
	g := map[string][]Reading{}
	for _, r := range rs {
		g[r.Model] = append(g[r.Model], r)
	}
	for k := range g {
		sort.SliceStable(g[k], func(i, j int) bool { return g[k][i].TakenAt < g[k][j].TakenAt })
	}
	return g
}

// LoadSeries reads every reading from a series file. A missing file is not an
// error: an empty series is the normal state before the first measurement.
func LoadSeries(path string) ([]Reading, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []Reading
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Reading
		// A line that will not parse is skipped rather than failing the read.
		// The series is append-only and a truncated final write must not make
		// the whole history unreadable.
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

// RecentReading returns a usable reading for a model taken within maxAge.
//
// Probing costs real money at the provider, and measuring again shortly after
// an identical reading buys nothing. The conditions are deliberately narrow:
// same model, same method, a clean bracket, inside the window. A reading that
// does not meet all four would silently freeze the series at whatever it last
// saw, which is worse than paying for the probe.
func RecentReading(path, model string, maxAge time.Duration) (Reading, bool) {
	rs, err := LoadSeries(path)
	if err != nil {
		return Reading{}, false
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	var best Reading
	var found bool
	for _, r := range rs {
		if r.Model != model || r.Method != MethodVersion || !r.comparable() {
			continue
		}
		t, perr := time.Parse(time.RFC3339, r.TakenAt)
		if perr != nil || t.Before(cutoff) {
			continue
		}
		if !found || r.TakenAt > best.TakenAt {
			best, found = r, true
		}
	}
	return best, found
}
