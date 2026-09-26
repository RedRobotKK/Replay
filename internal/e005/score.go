// Package e005 is the recovered E005 scorer, preserved so the frozen result
// can be re-derived rather than trusted.
//
// E005's blinder and scorer were run as inline shell heredocs and were never
// written to disk. They were recovered verbatim from the session transcript on
// 2026-09-26 and are archived beside the corpus at
// replay-e005-corpus-2026-09-26/recovered-procedure/. This package is that
// procedure's analysis arm, made durable and executable.
//
// # What is preserved and what is not
//
// Behaviour is preserved exactly, including two definitions that a reader may
// find surprising and that are NOT corrected here:
//
//   - `Economic` counts oracle events whose (file, second) collided with a
//     Replay break boundary. It is agreement between two instruments, one of
//     which is the subject of the study. It is NOT a provider-usage measure.
//     `MedianDWrite` is the provider-usage observable.
//   - Alignment is by whole second, which is why 375 boundaries are ambiguous
//     and why Bounds exists.
//
// Recovering a procedure is not permission to revise it. Anything that looked
// wrong was left alone and written down instead, here and in
// docs/evidence/detector-observables-2026-09-26.md.
package e005

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Structural and indeterminate class names, exactly as the recovered scorer
// partitioned them.
var (
	Structural    = []string{"tools_changed", "messages_changed", "system_changed", "model_changed"}
	Indeterminate = []string{"previous_message_not_found", "unavailable"}
)

func isStructural(c string) bool {
	for _, s := range Structural {
		if s == c {
			return true
		}
	}
	return false
}

func isIndeterminate(c string) bool {
	for _, s := range Indeterminate {
		if s == c {
			return true
		}
	}
	return false
}

// Event is one provider oracle event with the provider-reported counters of
// the request it landed on and the request before it.
//
// The field set is the MINIMUM that reproduces the frozen result, and the
// short names are a reminder that this is a sanitized fixture rather than the
// original record. The original carried ten further session-derived
// identifiers (message and request UUIDs, per-request timestamps, input-token
// counts, the model name); the scorer read none of them and they are not
// published. testdata/README.md records the derivation.
//
// Two fields are irreducible and are kept for stated reasons:
//
//   - File is an opaque pseudonym (f0001...). Its only use is grouping events
//     and breaks into the same boundary; the real transcript names are not
//     needed for that and are not published.
//   - Second is a bare HH:MM:SS with no date. Alignment is by whole second and
//     the recovered procedure already discarded the date, so this loses
//     nothing and publishes no calendar.
type Event struct {
	File   string `json:"f"`
	Second string `json:"s"`
	Oracle string `json:"c"`
	Missed *int   `json:"m"`
	CurW   int    `json:"cw"`
	CurR   int    `json:"cr"`
	PrevW  int    `json:"pw"`
	PrevR  int    `json:"pr"`
}

// ClassStat is one divergence class's row in the frozen table.
type ClassStat struct {
	N int
	// Economic is instrument agreement, not provider usage. See the package
	// comment; the name is the frozen one and is kept for that reason.
	Economic     int
	MedianMissed float64
	MedianDWrite float64
	MedianDRead  float64
}

// Result is the frozen table plus the accounting that has to balance.
type Result struct {
	ClassCounts map[string]int
	Total       int
	A, B, D     int // structural+agreeing, structural only, indeterminate
	PerClass    map[string]ClassStat
}

// Bound is the conservative alignment interval for one class.
type Bound struct {
	N, Lower, Upper int
}

// key joins the two halves of a boundary identity.
//
// The recovered procedure extracted the second with `T(\d\d:\d\d:\d\d)` at
// scoring time. The fixture applies that same regex once, when it is built, so
// Event.Second is already the extracted value and the regex does not appear
// here. An event the regex could not key carries "" and never collides with a
// break, which is the original behaviour.
func key(file, sec string) string { return file + "\t" + sec }

// LoadEvents reads the preserved oracle, 2,064 events with per-event identity.
func LoadEvents(path string) ([]Event, error) {
	b, err := os.ReadFile(path) //nolint:gosec // caller names the artifact
	if err != nil {
		return nil, err
	}
	var ev []Event
	if err := json.Unmarshal(b, &ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// LoadBreaks reads the break index: boundary to the NUMBER of breaks Replay
// reported there.
//
// A count, not the causes. Score asks only whether a boundary carries any
// break and Bounds asks only how many, so the cause strings are not needed and
// are not published. The full `replay diff` output is not stored at all: its
// `where:` lines quote message and tool-result content.
func LoadBreaks(path string) (map[string]int, error) {
	b, err := os.ReadFile(path) //nolint:gosec // caller names the artifact
	if err != nil {
		return nil, err
	}
	var m map[string]int
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// median matches Python's statistics.median, which averages the two middle
// values on an even-length input rather than taking the lower.
//
// The difference is not cosmetic: the frozen medians were produced by that
// definition, and taking the lower middle would change published numbers.
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// Score reproduces the frozen E005 table from the preserved artifacts.
func Score(ev []Event, breaks map[string]int) Result {
	r := Result{ClassCounts: map[string]int{}, PerClass: map[string]ClassStat{}}
	missed := map[string][]float64{}
	dw := map[string][]float64{}
	dr := map[string][]float64{}
	eco := map[string]int{}
	n := map[string]int{}

	for _, e := range ev {
		r.ClassCounts[e.Oracle]++
		r.Total++
		if isIndeterminate(e.Oracle) {
			r.D++
			continue
		}
		if !isStructural(e.Oracle) {
			continue
		}
		hit := breaks[key(e.File, e.Second)] > 0
		n[e.Oracle]++
		var m float64
		if e.Missed != nil {
			m = float64(*e.Missed)
		}
		missed[e.Oracle] = append(missed[e.Oracle], m)
		// Delta against the previous request. Every event in the frozen corpus
		// has one; the fixture flattens it rather than carrying a null.
		dw[e.Oracle] = append(dw[e.Oracle], float64(e.CurW-e.PrevW))
		dr[e.Oracle] = append(dr[e.Oracle], float64(e.CurR-e.PrevR))
		if hit {
			r.A++
			eco[e.Oracle]++
		} else {
			r.B++
		}
	}
	for _, c := range Structural {
		r.PerClass[c] = ClassStat{
			N: n[c], Economic: eco[c],
			MedianMissed: median(missed[c]),
			MedianDWrite: median(dw[c]),
			MedianDRead:  median(dr[c]),
		}
	}
	return r
}

// Bounds reproduces the conservative alignment interval for one class.
//
// Lower credits an ambiguous second with at most the breaks left after every
// other structural class in that second has taken one; upper credits every
// oracle event in a second containing any break.
func Bounds(ev []Event, breaks map[string]int, class string) Bound {
	bySec := map[string][]Event{}
	for _, e := range ev {
		if e.Second != "" {
			bySec[key(e.File, e.Second)] = append(bySec[key(e.File, e.Second)], e)
		}
	}
	var b Bound
	for k, es := range bySec {
		var mine, others int
		for _, e := range es {
			switch {
			case e.Oracle == class:
				mine++
			case isStructural(e.Oracle):
				others++
			}
		}
		b.N += mine
		nb := breaks[k]
		if nb == 0 || mine == 0 {
			continue
		}
		b.Upper += mine
		if v := nb - others; v > 0 {
			if v > mine {
				v = mine
			}
			b.Lower += v
		}
	}
	return b
}

// ClassOrder is the frozen reporting order.
func ClassOrder() []string { return append([]string(nil), Structural...) }

// Name normalises a class label for display without changing it.
func Name(c string) string { return strings.TrimSpace(c) }

// Pct renders a proportion the way the frozen table renders it.
//
// The frozen percentages came from Python's "%.0f", which rounds half to even.
// Go's %.0f uses the same rule, so the rendering matches, but integer
// truncation does NOT: 100*587/1006 is 58.35 and truncates to 58 either way,
// while 100*932/1006 is 92.64 and truncates to 92 where the frozen table shows
// 93. Truncating would silently move four of the eight published bounds.
func Pct(part, whole int) string {
	if whole == 0 {
		return "0%"
	}
	return strconvFormat(100*float64(part)/float64(whole)) + "%"
}

func strconvFormat(f float64) string { return fmt.Sprintf("%.0f", f) }
