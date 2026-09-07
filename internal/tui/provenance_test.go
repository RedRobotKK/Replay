package tui

import (
	"regexp"
	"strings"
	"testing"
)

// A screen showing figures must say where they came from.
//
// The surface shipped with nine hardcoded answers and forty hardcoded rows and
// nothing on screen said so. Somebody running `replay tui` saw "$2.41 of $5.00,
// NOT ENFORCED" with no way to tell it was not their machine.
//
// That is this project's own defect arriving from the inside. A figure with
// nothing behind it is what the installer was corrected for twice tonight, and
// on a screen it is worse than in a document, because a screen looks like an
// instrument.
func TestUnmeasuredScreensSaySo(t *testing.T) {
	money := regexp.MustCompile(`\$[\d,]+`)
	for _, sc := range Outcomes() {
		body := strings.Join(sc.Lines, "\n")
		hasFigures := money.MatchString(body) || regexp.MustCompile(`\d{2,}`).MatchString(body)
		if !hasFigures {
			continue
		}
		if sc.From == Measured {
			continue
		}
		if !Marked(sc.Lines) {
			t.Errorf("%q shows figures, declares itself %v, and does not say so on "+
				"screen. A reader takes them for their own machine", sc.Title, sc.From)
		}
	}
}

// The notice sits above the numbers, not under them.
//
// A caveat below a table is a caveat read after the number has already been
// believed.
func TestTheNoticeComesBeforeTheFigures(t *testing.T) {
	money := regexp.MustCompile(`\$[\d,]+`)
	for _, sc := range Outcomes() {
		if !Marked(sc.Lines) {
			continue
		}
		notice, first := -1, -1
		for i, l := range sc.Lines {
			if notice < 0 && strings.Contains(l, "[NOTE]") {
				notice = i
			}
			if first < 0 && money.MatchString(l) {
				first = i
			}
		}
		if first >= 0 && notice > first {
			t.Errorf("%q puts its first figure on row %d and the notice on row %d. The "+
				"number is read first and believed before the caveat arrives",
				sc.Title, first, notice)
		}
	}
}

// A measured screen must not be stamped.
//
// Marking real data teaches the reader to skim the mark, and then the mark
// stops working on the screens that need it.
func TestMeasuredScreensCarryNoNotice(t *testing.T) {
	if Banner(Measured, "") != "" {
		t.Errorf("a measured screen was given a provenance notice: %q", Banner(Measured, ""))
	}
}

// The notice fits, like everything else.
func TestNoticesFitTheBudget(t *testing.T) {
	for _, p := range []Provenance{Example, Unavailable} {
		b := Banner(p, "no ledger at ~/.replay/ledger, and no proxy has run today")
		if len(b) > BudgetCols {
			t.Errorf("a %v notice is %d columns, budget %d:\n%s", p, len(b), BudgetCols, b)
		}
	}
}

// No screen may claim to be measured until something measures it.
//
// The test above skips screens declaring themselves Measured, which means a
// screen could lie about its provenance and be believed: flipping the constant
// from Example to Measured made the notice vanish and every check still passed.
//
// Nothing here is wired to a source yet, so the honest state is that none of
// them is measured. Pinning that turns the flip into a decision somebody makes
// deliberately: the day a screen is wired, this test fails and whoever wired it
// says so in the same change.
func TestNoScreenClaimsToBeMeasuredYet(t *testing.T) {
	for _, sc := range Outcomes() {
		if sc.From == Measured {
			t.Errorf("%q declares itself measured. Nothing on these screens reads a "+
				"ledger, a corpus or a running proxy yet. If that changed, this test "+
				"is the place to say which source it now reads", sc.Title)
		}
	}
}
