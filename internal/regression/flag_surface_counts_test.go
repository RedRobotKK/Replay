package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// docs/TUI-FLAG-SURFACE.md states each archetype's size three times — once in
// the summary table, once in the section heading, and once implicitly as the
// number of rows in that section's table — and nothing compared them. Five of
// the six had drifted apart:
//
//	archetype                     summary  heading  rows
//	Replaces the surface                7       13    13
//	Plumbing, shown once               16       20    20
//	Threshold that can fire            20       21    21
//	Posture, on or not covered         14       15    15
//	Scope of the question              12       12    12
//	Action with a consequence          12       12    16
//
// TestFlagSurface_EveryFlagIsClassified already fails when a flag is defined
// and appears nowhere in the document, which is why the rows stayed correct
// while the stated totals rotted: adding a row satisfied that guard and left
// both numbers describing the table as it used to be. A reader deciding how
// much of this surface is one repeated component reads the summary, which is
// the number nothing was checking.
//
// The rows are the authority here, not either stated figure. They are the part
// a change to cmd/replay is forced to touch.

const flagSurfaceDoc = "docs/TUI-FLAG-SURFACE.md"

var (
	// The summary table is the only one whose first cell is prose and second
	// cell a bare number; the per-archetype tables open every row with a
	// backticked flag.
	flagSurfaceSummaryRow = regexp.MustCompile("^\\| ([^|`]+?) \\| (\\d+) \\|")
	flagSurfaceSection    = regexp.MustCompile(`^### (.+?) \((\d+)\)\s*$`)
	flagSurfaceFlagRow    = regexp.MustCompile("^\\| `")
)

// archetypeCounts holds the three figures for one archetype. summaryStated and
// headingStated are separate from their values so that "absent" is not read as
// zero — an archetype that lost its summary row would otherwise look like a
// disagreement about the number 0 rather than a missing claim.
type archetypeCounts struct {
	summary       int
	summaryStated bool
	heading       int
	headingStated bool
	rows          []string
}

// parseFlagSurface reads the document into one record per archetype.
//
// Section membership ends at the next heading of any level, so the rows of the
// last archetype cannot leak into the trailing prose and inflate its count.
func parseFlagSurface(t *testing.T) map[string]*archetypeCounts {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(repoRoot(t), flagSurfaceDoc))
	if err != nil {
		t.Fatalf("reading %s: %v", flagSurfaceDoc, err)
	}

	counts := map[string]*archetypeCounts{}
	at := func(name string) *archetypeCounts {
		if counts[name] == nil {
			counts[name] = &archetypeCounts{}
		}
		return counts[name]
	}

	var inSummary bool
	var section string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "#") {
			if m := flagSurfaceSection.FindStringSubmatch(line); m != nil {
				section = m[1]
				c := at(section)
				c.heading, c.headingStated = atoiOrFail(t, m[2], line), true
			} else {
				section = ""
			}
			// The summary table lives under one H2 and stops at the next.
			inSummary = strings.TrimSpace(line) == "## The six archetypes"
			continue
		}
		if inSummary {
			if m := flagSurfaceSummaryRow.FindStringSubmatch(line); m != nil {
				name := strings.TrimSpace(m[1])
				c := at(name)
				c.summary, c.summaryStated = atoiOrFail(t, m[2], line), true
			}
			continue
		}
		if section != "" && flagSurfaceFlagRow.MatchString(line) {
			c := at(section)
			c.rows = append(c.rows, strings.TrimSpace(line))
		}
	}
	return counts
}

// atoiOrFail refuses to guess. A figure this test cannot read is a figure it
// would otherwise compare as zero, which is the failure this file exists to
// prevent.
func atoiOrFail(t *testing.T, s, line string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("reading the stated count %q in %q: %v", s, line, err)
	}
	return n
}

// FS-C1: the two stated figures and the table they describe must agree.
func TestFlagSurface_TheStatedArchetypeSizesMatchTheRowsTheyCount(t *testing.T) {
	counts := parseFlagSurface(t)

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		c := counts[name]
		rows := len(c.rows)

		if !c.summaryStated {
			t.Errorf("%q has a section in %s but no row in the summary table, so its "+
				"size is stated in one place and unchecked in the other",
				name, flagSurfaceDoc)
			continue
		}
		if !c.headingStated {
			t.Errorf("%q is listed in the summary table of %s but has no "+
				"\"### %s (n)\" section, so the summary counts rows that are not there",
				name, flagSurfaceDoc, name)
			continue
		}
		if c.summary != rows {
			t.Errorf("%s: %q says %d in the summary table and its table holds %d rows. "+
				"The rows are the flags; correct the summary to %d.",
				flagSurfaceDoc, name, c.summary, rows, rows)
		}
		if c.heading != rows {
			t.Errorf("%s: %q says %d in its section heading and its table holds %d rows. "+
				"The rows are the flags; correct the heading to %d.",
				flagSurfaceDoc, name, c.heading, rows, rows)
		}
	}
}

// flagSurfacePositive is one archetype the document really carried on
// 2026-09-11, with one row it really held.
//
// Without this, FS-C1 has a shape in which it cannot turn red. It reports
// disagreements between figures it parsed, so an empty parse has nothing to
// disagree about and it passes. Measured, on 2026-09-11, by breaking each
// pattern in turn and running it:
//
//	row pattern broken only      FS-C1 RED  (stated 21 vs 0 rows)
//	section pattern broken only  FS-C1 RED  (summary names a section not found)
//	summary + section broken     FS-C1 GREEN on a document it never read
//
// The last row is the one that matters. A single broken pattern is caught by
// FS-C1 itself; a parse that returns nothing at all is not, and it fails in
// the reassuring direction. FS-C2 is what turns that case red.
type flagSurfacePositive struct {
	archetype string
	row       string
}

var flagSurfacePositives = []flagSurfacePositive{
	{"Threshold that can fire", "| `--max-day-usd` | `serve` | float64 |"},
	{"Action with a consequence", "| `--flat-rate` | `ceiling` | float64 |"},
	{"Replaces the surface", "| `--json` | `advise` | bool |"},
}

// FS-C2: the parser still sees the sections and rows it was written against.
func TestFlagSurface_TheCountParserStillFindsARealSectionAndARealRow(t *testing.T) {
	counts := parseFlagSurface(t)

	if len(counts) == 0 {
		t.Fatalf("%s parsed to no archetypes at all. FS-C1 is comparing nothing and "+
			"passing", flagSurfaceDoc)
	}
	var rows int
	for _, c := range counts {
		rows += len(c.rows)
	}
	if rows == 0 {
		t.Fatalf("%s parsed to %d archetypes and zero flag rows. Every stated figure "+
			"would then be compared against 0 and FS-C1 would report a document it "+
			"cannot read as consistent", flagSurfaceDoc, len(counts))
	}

	for _, p := range flagSurfacePositives {
		c := counts[p.archetype]
		if c == nil {
			t.Errorf("the parser no longer finds the %q section, which this document "+
				"held when the check was written. The section pattern was broken or "+
				"the archetype was renamed without updating this row", p.archetype)
			continue
		}
		if !c.summaryStated || !c.headingStated {
			t.Errorf("%q parsed with summary=%v heading=%v: the parser stopped reading "+
				"one of the two figures it exists to compare",
				p.archetype, c.summaryStated, c.headingStated)
		}
		var found bool
		for _, row := range c.rows {
			if row == p.row {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the parser counted %d rows under %q and none of them is:\n  %s\n\n"+
				"That row was in the table when this check was written. Either the row "+
				"moved — update this positive — or the row pattern stopped matching, in "+
				"which case FS-C1 is counting fewer flags than the document lists.",
				len(c.rows), p.archetype, p.row)
		}
	}
}
