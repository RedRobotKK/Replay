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
	if len(line) > 0 && line[0] == ' ' && len(line) > 2 && line[2] != ' ' {
		out = append([]int{2}, out...)
	}
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
