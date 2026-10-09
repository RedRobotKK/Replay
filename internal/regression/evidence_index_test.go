package regression

import (
	"strings"
	"testing"
)

// The evidence index is one table under one heading. On 2026-10-09 the
// live copy at replay.doctor/docs/trust/evidence/ rendered twenty-two of
// its rows as raw pipe text, because an earlier edit had joined two rows
// and the file's "# Evidence" heading into a single line with literal
// backslash-n characters (the two bytes, not a newline). Every row added
// at the top of the file after that landed above the heading, outside the
// table, and Markdown lint could not see it: a line that is not inside a
// table has no column count to check.
//
// PASS: the heading is the first line, no line carries a literal "\n",
// and no table row precedes the table's delimiter row.
// FAIL: any of the three, each named with its line number.
func TestEI1_TheEvidenceIndexIsOneTableUnderItsHeading(t *testing.T) {
	body := readDoc(t, "docs/evidence/README.md")
	lines := strings.Split(body, "\n")
	if len(lines) == 0 || lines[0] != "# Evidence" {
		t.Errorf("docs/evidence/README.md does not open with \"# Evidence\"; line 1 is %q", first(lines))
	}
	delimiter := -1
	for i, line := range lines {
		if strings.Contains(line, `\n`) {
			t.Errorf("docs/evidence/README.md:%d carries a literal backslash-n; the line that follows it is glued on and renders as text", i+1)
		}
		if delimiter < 0 && strings.HasPrefix(line, "|---") {
			delimiter = i
		}
		if delimiter < 0 && strings.HasPrefix(line, "| [") {
			t.Errorf("docs/evidence/README.md:%d is a table row above the table's delimiter row, so it renders as pipe text", i+1)
		}
	}
	if delimiter < 0 {
		t.Error("docs/evidence/README.md has no table delimiter row")
	}
}

func first(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}
