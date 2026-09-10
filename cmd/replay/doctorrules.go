package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// rulesStaleAfter is when a table stops being worth trusting silently.
//
// Thirty days, matching the binary's own StaleNotice, and chosen for the same
// reason: it is long enough that a working operator is not nagged and short
// enough that a provider change published in between is still news. It is a
// round number and is not derived from evidence, which ADR-0009 says every
// threshold in this tool currently is. The difference here is that crossing it
// prints a sentence rather than refusing anything, so the cost of the number
// being wrong is one line of text.
const rulesStaleAfter = 30 * 24 * time.Hour

// rulesNotice is what `replay doctor` says about the table every dollar figure
// was computed against.
//
// The binary tells you when IT is old. Nothing told you when the TABLE was, so
// an operator running a six-week-old document was told their install was
// current and never told their prices were not.
//
// What staleness costs is not mainly the price column. It is the cache-read
// multiple, which enters EffectiveTokens and decides which policy the
// comparison calls cheaper. internal/cachemodel/anthropic.go states the
// direction: falling back to the older 0.10 tier while current models read at
// 0.025 "overstated the cost of every cached token fourfold, and the bias has a
// direction: it inflates the apparent value of cache-preserving policies". A
// tool that advises keeping a cache, on a table that overstates what reading
// one costs, is making a claim on its own behalf. That is the sentence this
// notice exists to prevent being true silently.
//
// Local arithmetic only. It reads the version and fetch date already in hand
// and reaches nothing; the operator types the command or does not.
func rulesNotice(version, fetchedAt string, now time.Time) string {
	var b strings.Builder
	fetchedAt = strings.TrimSpace(fetchedAt)

	if fetchedAt == "" {
		// No fetch date is the compiled-in table, which is a floor rather than
		// a stale document. Ageing a constant would print a number with nothing
		// behind it.
		fmt.Fprintf(&b, "rules         %s (compiled in)\n", version)
		fmt.Fprintf(&b, "              the table this build shipped with. Nothing has been installed over it\n")
		return b.String()
	}

	at, ok := parseFetchedAt(fetchedAt)
	if !ok {
		// Absence, zero and unknown are three values (ADR-0018). A date this
		// build cannot read must not pass as current, because the reading that
		// suppresses the warning is the one that hides the problem.
		fmt.Fprintf(&b, "rules         %s, fetch date unreadable (%q)\n", version, fetchedAt)
		fmt.Fprintf(&b, "              this build cannot age the table, so treat its figures as undated\n")
		return b.String()
	}

	age := now.Sub(at)
	days := int(age.Hours() / 24)
	fmt.Fprintf(&b, "rules         %s, fetched %d days ago\n", version, days)
	if age < rulesStaleAfter {
		return b.String()
	}

	fmt.Fprintf(&b, "              stale: prices and cache floors move, and the cache-read multiple is\n")
	fmt.Fprintf(&b, "              the one that changes what replay recommends rather than only what it\n")
	fmt.Fprintf(&b, "              reports. Reading at 0.10 when a model now reads at 0.025 overstates\n")
	fmt.Fprintf(&b, "              every cached token fourfold, and biases the comparison toward keeping\n")
	fmt.Fprintf(&b, "              a cache — which is advice this tool would be giving on its own behalf\n")
	fmt.Fprintf(&b, "              next: replay rules --check-prices\n")
	return b.String()
}

// parseFetchedAt reads a rules document's fetch date, in the two shapes one is
// written in: RFC3339 as the fetcher stamps it, and a bare date as a human
// editing the file by hand writes it.
//
// It exists so the two surfaces that report this cannot disagree. `replay
// doctor` and the TUI doctor screen answer the same question, and a date one
// of them can read and the other cannot would tell one operator their table is
// current and the other that it is undated — from the same file, on the same
// machine, in the same minute.
func parseFetchedAt(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// rulesAge classifies the rules document for the TUI, which renders rather than
// prints and so needs the three states as values instead of as sentences.
//
// The classification is the same one rulesNotice makes; only the output shape
// differs. Both go through parseFetchedAt, so the line between "dated" and
// "undated" is drawn once.
func rulesAge(fetchedAt string, now time.Time) (tui.RulesAge, int) {
	if strings.TrimSpace(fetchedAt) == "" {
		return tui.RulesBuiltIn, 0
	}
	at, ok := parseFetchedAt(fetchedAt)
	if !ok {
		return tui.RulesUndated, 0
	}
	return tui.RulesFetched, int(now.Sub(at).Hours() / 24)
}
