package tui

import (
	"strings"
	"testing"
	"unicode"
)

// Every line of every scene must fit the frame budget and contain nothing that
// could occupy two cells.
//
// This is the check the original storyboard asserted in prose and did not have.
// It claimed its borders could never shear while carrying 98 ambiguous-width
// characters and data rows one cell short of their header.
func TestStoryboard_EveryLineIsASCIIAndInsideTheFrame(t *testing.T) {
	for _, sc := range Storyboard() {
		for i, line := range sc.Lines {
			for _, r := range line {
				if r > unicode.MaxASCII {
					t.Errorf("state %d (%s) line %d has non-ASCII %q. A rune outside ASCII "+
						"risks two cells in a CJK terminal, and the frame is built on one",
						sc.N, sc.Name, i, r)
				}
			}
			if len(line) > Width {
				t.Errorf("state %d (%s) line %d is %d cells, budget is %d:\n%s",
					sc.N, sc.Name, i, len(line), Width, line)
			}
		}
	}
}

// Inside one scene, every traffic row must align with its header.
//
// A row that is one space short shears the column for every row after it, and
// it is invisible in review. This is the defect the previous storyboard shipped
// with, so it is the one the storyboard itself is checked for.
func TestStoryboard_TrafficRowsAlignWithTheirHeader(t *testing.T) {
	head := Header()[0]
	want := columnStarts(head)
	for _, sc := range Storyboard() {
		for i, line := range sc.Lines {
			if !looksLikeTraffic(line) {
				continue
			}
			if got := columnStarts(line); !equal(got, want) {
				t.Errorf("state %d (%s) line %d does not line up with the header.\n"+
					"header cols at %v\nrow    cols at %v\n%s\n%s",
					sc.N, sc.Name, i, want, got, head, line)
			}
		}
	}
}

// The storyboard must actually cover the states the design says exist, or it is
// a demo of the pleasant ones.
func TestStoryboard_CoversTheStatesThatEarnTheSurface(t *testing.T) {
	have := map[int]bool{}
	for _, sc := range Storyboard() {
		have[sc.N] = true
	}
	// 6 is a path forwarded blind; 15 is a cap that cannot be enforced. Both
	// are states a dashboard would normally omit, and both are the reason this
	// surface is worth building.
	for _, n := range []int{1, 2, 6, 15, 16, 22, 24, 25} {
		if !have[n] {
			t.Errorf("state %d is not storyboarded", n)
		}
	}
}

// looksLikeTraffic identifies a row of the main traffic table.
//
// It deliberately excludes the two scenes that do not use that table: the
// non-TTY scene, whose whole point is that it emits plain lines with no
// columns, and the narrow-terminal scene, which drops columns by design. A
// check that demanded the same alignment from those would be asserting the
// opposite of what they exist to show.
func looksLikeTraffic(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) < 8 {
		return false
	}
	if s[2] != ':' || s[5] != ':' || !unicode.IsDigit(rune(s[0])) {
		return false
	}
	return len(columnStarts(line)) == 5
}

// columnStarts reports where each cell begins, which is the property that has
// to hold. Line length varies with the last cell's content once trailing space
// is stripped, and that is not shear.
func columnStarts(line string) []int {
	var out []int
	for i := 2; i < len(line); i++ {
		if line[i] != ' ' && line[i-1] == ' ' && line[i-2] == ' ' {
			out = append(out, i)
		}
	}
	// No prepend for the first column. The loop above already reports index 2
	// for an indented row — line[2] is non-space with two spaces before it,
	// which is exactly its condition — so adding it again returned 2 twice.
	// Every traffic row then scored 6 starts against a looksLikeTraffic that
	// requires 5, no row was ever examined, and the alignment assertion this
	// file is named for never ran. See SB1.
	return out
}

func equal(a, b []int) bool {
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

// The header block must be the same table in every state it appears in.
//
// It was not. State 1 put its third column at 54 and state 2 at 48, because
// both were assembled from hand-counted spaces inside a value string. The
// traffic grid below them has been aligned since it was written, so the screen
// held one table a reader could scan and one they had to re-find, and the
// second was the one carrying the spend figure.
func TestStoryboard_TheHeaderBlockHasOneGeometryEverywhere(t *testing.T) {
	seen := map[string][]int{}
	for _, sc := range Storyboard() {
		for _, line := range sc.Lines {
			if !looksLikeHeaderBlock(line) {
				continue
			}
			cols := columnStarts(line)
			if len(cols) < 2 {
				continue
			}
			key := strings.TrimSpace(line[:cols[1]])
			_ = key
			if prev, ok := seen["block"]; ok {
				if !prefixEqual(prev, cols) {
					t.Errorf("state %d's header block starts columns at %v; an earlier state "+
						"used %v. A reader who moves between screens should not have to "+
						"re-find the column carrying the spend figure.\n%s",
						sc.N, cols, prev, line)
				}
				continue
			}
			seen["block"] = cols
		}
	}
}

// looksLikeHeaderBlock matches the label/value lines above the traffic table:
// two leading spaces, a word, and a wide gap before the value.
func looksLikeHeaderBlock(line string) bool {
	for _, k := range []string{"listening", "upstream", "ledger"} {
		if strings.HasPrefix(line, "  "+k+" ") {
			return true
		}
	}
	return false
}

// prefixEqual compares the columns both lines actually have, so a state with a
// stat column is still checked against one without it for the columns they share.
func prefixEqual(a, b []int) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SB1: the alignment check examines rows.
//
// TestStoryboard_TrafficRowsAlignWithTheirHeader was vacuous for its whole
// life. Instrumented, 166 lines were considered and 0 reached the assertion,
// so the check whose comment says it exists to catch "the defect the previous
// storyboard shipped with" would have passed a storyboard with every row one
// cell off its header. Verified: giving Traffic() the widths
// {"time",9},{"surface",9},{"endpoint",22} — every row misaligned, total width
// unchanged — left `go test ./internal/tui/` green.
//
// The cause is one line in columnStarts. The loop already reports index 2 for
// an indented row, and the prepend below it adds a second 2, so every traffic
// row scored 6 column starts against a looksLikeTraffic that requires 5. The
// filter was correct and the function it called was not.
//
// This test is the one that could not have been written after the fact and
// still be trusted, because it asserts on the count rather than the outcome: a
// filter that matches nothing produces a green suite either way.
func TestSB1_TheAlignmentCheckActuallyExaminesRows(t *testing.T) {
	var timeShaped, examined int
	for _, sc := range Storyboard() {
		for _, line := range sc.Lines {
			s := strings.TrimSpace(line)
			if len(s) >= 8 && s[2] == ':' && s[5] == ':' && unicode.IsDigit(rune(s[0])) {
				timeShaped++
			}
			if looksLikeTraffic(line) {
				examined++
			}
		}
	}
	if timeShaped == 0 {
		t.Fatal("the storyboard contains no rows shaped like traffic at all, so this " +
			"guard is measuring the wrong thing")
	}
	if examined == 0 {
		t.Fatalf("%d rows in the storyboard are shaped like traffic and looksLikeTraffic "+
			"admits none of them, so the alignment assertion never runs", timeShaped)
	}
	// Some rows are excluded on purpose, and the exclusion is pinned by scene
	// rather than by count. looksLikeTraffic's comment names two: scene 24
	// emits plain lines with no columns, and scene 25 drops columns to fit a
	// narrow terminal. Demanding the wide table's alignment from either would
	// assert the opposite of what they exist to show. A bare count would let a
	// future filter quietly stop admitting a third scene.
	allowed := map[int]bool{24: true, 25: true}
	for _, sc := range Storyboard() {
		if allowed[sc.N] {
			continue
		}
		for i, line := range sc.Lines {
			s := strings.TrimSpace(line)
			if len(s) >= 8 && s[2] == ':' && s[5] == ':' && unicode.IsDigit(rune(s[0])) &&
				!looksLikeTraffic(line) {
				t.Errorf("scene %d (%s) line %d is a traffic row the alignment check skips, "+
					"and it is not one of the two scenes documented as columnless:\n  %q -> %v",
					sc.N, sc.Name, i, line, columnStarts(line))
			}
		}
	}
	if examined != 6 {
		t.Errorf("the alignment check examines %d rows; it examined 6 when this guard was "+
			"written, so either a scene gained traffic or the filter narrowed", examined)
	}
}

// SB3: the narrow-terminal scene is internally aligned.
//
// Scene 25 is excluded from the wide-table check because it drops columns by
// design, which is correct and also left it unchecked entirely — its rows could
// shear against each other and nothing would notice. They share a header of
// their own, so they can be held to it.
func TestSB3_TheNarrowSceneAlignsWithItself(t *testing.T) {
	var rows [][]int
	var text []string
	for _, sc := range Storyboard() {
		if sc.N != 25 {
			continue
		}
		for _, line := range sc.Lines {
			s := strings.TrimSpace(line)
			if len(s) >= 8 && s[2] == ':' && s[5] == ':' && unicode.IsDigit(rune(s[0])) {
				rows = append(rows, columnStarts(line))
				text = append(text, line)
			}
		}
	}
	if len(rows) < 2 {
		t.Fatalf("scene 25 has %d traffic rows; with fewer than two there is nothing to "+
			"align and this guard is measuring nothing", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if !equal(rows[i], rows[0]) {
			t.Errorf("scene 25 row %d does not line up with row 0.\n  %s -> %v\n  %s -> %v",
				i, text[0], rows[0], text[i], rows[i])
		}
	}
}

// SB2: a column start is reported once.
//
// columnStarts is the measurement every alignment assertion here is built on.
// It reported index 2 twice for any indented row — the loop finds it, then the
// prepend adds it again — which is what pushed every traffic row out of
// looksLikeTraffic's range and made SB1's parent test vacuous.
func TestSB2_ColumnStartsAreNotDuplicated(t *testing.T) {
	for _, line := range append(Header(), "  15:06:44  anthropic  api.anthropic.com        messages          parsed   ") {
		seen := map[int]bool{}
		for _, c := range columnStarts(line) {
			if seen[c] {
				t.Errorf("column start %d reported twice for %q -> %v", c, line, columnStarts(line))
			}
			seen[c] = true
		}
	}
}

// SB4: the traffic table's geometry is pinned to literals.
//
// The alignment check above compares each row to Header(), and Header() is
// built from trafficCols by the same Row() that builds the rows. Both sides of
// the comparison come from one source, so they agree by construction: no edit
// to trafficCols can ever make them disagree.
//
// That is why fixing looksLikeTraffic was necessary and not sufficient. With
// rows finally being examined, the mutation the audit reported escaping —
// {"time",9},{"surface",9},{"endpoint",22}, total width preserved — still left
// the package green, because it moved the header and the rows together.
//
// A check needs an oracle it cannot rewrite. These are the column starts the
// storyboard was designed around, written out. If a width changes, this fails
// and someone decides whether the new geometry is intended; the alignment test
// keeps proving the rows agree with the header, which is the separate property
// it is good at.
func TestSB4_TheTrafficGeometryIsPinned(t *testing.T) {
	const layout = "time 8, surface 9, endpoint 23, wire 16, status 9, two-space gutters"
	want := []int{2, 12, 23, 48, 66}
	head := Header()[0]
	if got := columnStarts(head); !equal(got, want) {
		t.Errorf("the traffic table's columns start at %v, not %v (%s).\n  %q\n"+
			"If the new geometry is intended, change the literal here and say why.",
			got, want, layout, head)
	}
	if got := len(head); got != 75 {
		t.Errorf("the header is %d cells wide, was 75: %q", got, head)
	}
}
