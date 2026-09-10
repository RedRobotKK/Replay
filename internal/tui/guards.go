package tui

import (
	"fmt"
	"sort"
	"strings"
)

// GuardState is what the proxy's guards are doing, as distinct from what a cap
// drawn from history would suggest.
//
// Populated by the caller from proxy.Status, because internal/tui does no I/O.
// Reachable is the field the screen turns on: a proxy that did not answer is
// not a proxy with nothing to report, and this is the one screen where reading
// silence as safety is the expensive mistake.
type GuardState struct {
	Reachable  bool
	Addr       string
	Caps       Caps
	Refused    int
	Refusals   map[string]int
	CostUSD    float64
	DayCostUSD float64
	// SpendCapNotEnforced is a dollar cap that is set and is not being applied
	// to some traffic, because that traffic could not be priced.
	SpendCapNotEnforced bool
}

// Caps is which limits are configured, not their values. Whether a limit
// exists is what a diagnostic needs; the numbers are the operator's business
// and the status endpoint deliberately does not carry them.
type Caps struct {
	SessionUSD    bool
	DayUSD        bool
	SessionTokens bool
	DayTokens     bool
}

// set names the caps that are on, in the order an operator thinks of them.
func (c Caps) set() []string {
	var out []string
	for _, p := range []struct {
		on   bool
		name string
	}{
		{c.SessionUSD, "session $"},
		{c.DayUSD, "day $"},
		{c.SessionTokens, "session tokens"},
		{c.DayTokens, "day tokens"},
	} {
		if p.on {
			out = append(out, p.name)
		}
	}
	return out
}

// armedCols is this screen's own table, narrower than the doctor screen's.
//
// docCols lays a row out at 65 cells, which overruns a 60-column terminal and
// cannot be wrapped without breaking the grid — the rule line is an unbroken
// run of dashes. The doctor screen's overrun is a known, frozen one; adding a
// second table at the same width would have made the narrow layout worse, and
// this table has short labels and short values, so it does not need the width.
var armedCols = []Column{{"check", 20}, {"result", 34}}

// GuardsScreen answers "Am I about to blow a budget?"
//
// Two halves, in this order and never merged. The top is what the proxy is
// enforcing right now, read from its status endpoint. The bottom is what a cap
// COULD be, drawn from the reader's own spread and written nowhere.
//
// They used to be one thing: the screen rendered only the suggestion, under a
// key that promised "every guard, whether it is armed, and whether it can
// fire". So the single screen named for the budget question could not report
// the one state that costs money — a dollar cap configured against traffic
// that cannot be priced, where the operator believes they have a limit they do
// not have. `replay doctor` reported it; this screen did not, and this screen
// is the one the installer opens.
//
// The halves have different prerequisites on purpose. Live state needs a proxy
// and no transcripts; the suggestion needs transcripts and no proxy. Either can
// be present without the other, and neither absence is allowed to delete the
// other's answer.
func GuardsScreen(g GuardState, advice []string, sessions int) Screen {
	// Unavailable only when BOTH halves are empty. A proxy that answered is a
	// measurement even with no transcripts behind it, and a corpus is a
	// measurement even with no proxy running; the screen is unmeasured only
	// when it was able to ask neither question.
	if !g.Reachable && sessions == 0 {
		return unavailable("guards", "no proxy answered and no sessions were read",
			"Start one with "+paint(Accent, "replay serve")+", or read a corpus with "+
				paint(Accent, "replay cost")+".")
	}
	sc := Screen{Key: 'g', Title: "guards", From: Measured}
	lines := []string{header("guards"), ""}
	lines = append(lines,
		"  What the proxy is enforcing right now", "",
		Row(armedCols, "check", "result"),
		Row(armedCols, strings.Repeat("-", 20), strings.Repeat("-", 34)),
		Row(armedCols, "proxy", proxyRow(g)),
		Row(armedCols, "caps set", capsRow(g)),
		Row(armedCols, "refused", refusedRow(g)),
		Row(armedCols, "spent", spentRow(g)),
		"", "  notes")

	if g.SpendCapNotEnforced {
		// Urgent, and the mechanism named. "Not enforced" on its own is a
		// status a reader cannot act on; the reason they cannot act on it is
		// that the fix is a price, not a number.
		lines = append(lines,
			note(true, "a dollar cap is set and is NOT enforced on some"),
			"      traffic, because it could not be priced. A loop on",
			"      an unpriced model bills with no limit reached.")
		lines = append(lines, nextFor(g)...)
	}
	if !g.Reachable {
		lines = append(lines,
			note(false, "no proxy answered: nothing here is enforcing"),
			"      anything. That is not the same as nothing",
			"      having been refused.",
			"      next: replay serve")
	}

	lines = append(lines, "", "  Suggested from your own spread, not applied")
	switch {
	case sessions == 0:
		lines = append(lines,
			paint(Faint, "  No sessions were read, so there is no spread to draw a cap from."))
	case len(advice) == 0:
		lines = append(lines,
			paint(Faint, fmt.Sprintf("  Too few sessions, or a spread too flat to fence, over %d.", sessions)))
	default:
		for _, l := range advice {
			// The command side writes these for a printed report, where the
			// left margin is the terminal. Here every other line is indented
			// two, and a block that starts at column zero reads as a different
			// screen element rather than as the answer under its own heading.
			if !strings.HasPrefix(l, " ") {
				l = "  " + l
			}
			lines = append(lines, fitTo(l, Cols()))
		}
	}
	lines = append(lines, paint(Faint, "  Nothing here is written to your configuration."))
	sc.Lines = padGuards(lines)
	return sc
}

// nextFor names the cheaper of the two fixes for an unenforced dollar cap.
//
// A token cap counts whether or not a model can be priced, so an operator who
// already runs one is covered and telling them to add one is noise. Noise in a
// warning is how a warning stops being read.
func nextFor(g GuardState) []string {
	if g.Caps.DayTokens || g.Caps.SessionTokens {
		return []string{"      Your token cap already covers the loop."}
	}
	return []string{
		"      next: replay serve --max-day-tokens N",
		"      then replay rules --update <file|URL>",
	}
}

func proxyRow(g GuardState) string {
	if !g.Reachable {
		return "no answer; nothing is enforced"
	}
	if g.Addr == "" {
		return "answering"
	}
	return "answering at " + g.Addr
}

func capsRow(g GuardState) string {
	if !g.Reachable {
		return "unknown, nothing to ask"
	}
	on := g.Caps.set()
	if len(on) == 0 {
		return "none: nothing will be refused"
	}
	return strings.Join(on, ", ")
}

func refusedRow(g GuardState) string {
	if !g.Reachable {
		return "unknown, nothing to ask"
	}
	if g.Refused == 0 {
		return "none so far"
	}
	kinds := make([]string, 0, len(g.Refusals))
	for k := range g.Refusals {
		kinds = append(kinds, k)
	}
	// Sorted, so two looks at the same state read the same.
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s %d", k, g.Refusals[k]))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d", g.Refused)
	}
	return fmt.Sprintf("%d: %s", g.Refused, strings.Join(parts, ", "))
}

func spentRow(g GuardState) string {
	if !g.Reachable {
		return "unknown, nothing to ask"
	}
	if g.SpendCapNotEnforced {
		return fmt.Sprintf("$%.2f today, cap NOT enforced", g.DayCostUSD)
	}
	return fmt.Sprintf("$%.2f since start, $%.2f today", g.CostUSD, g.DayCostUSD)
}

// padGuards trims to the body budget from the bottom, because the top half is
// the half that answers the key's question.
func padGuards(lines []string) []string {
	for len(lines) < bodyRows {
		lines = append(lines, "")
	}
	if len(lines) > bodyRows {
		lines = lines[:bodyRows]
	}
	return lines
}

// Urgent reports whether any line is an urgent note, which is what the "!"
// marker means. Exported so a test can assert that a warning was rendered as a
// warning rather than as information.
func Urgent(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, "  ! ") {
			return true
		}
	}
	return false
}
