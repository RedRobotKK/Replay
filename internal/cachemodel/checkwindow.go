package cachemodel

import (
	"strings"
	"time"
)

// RemainingCheckDays is how many days the price table stays trustworthy, given
// when it was last verified.
//
// PriceTableAgeNoteAt answers "how old is this?", which is a number that grows
// and which a reader has to interpret. This answers "how long is it good for?",
// which is the same two dates subtracted the other way round and is the form a
// reader can act on.
//
// The window is anchored to the CHECK rather than to PriceTableVersion,
// because those are different facts and only one of them is a heartbeat: an
// old table verified last week is current, and a new table nobody has looked
// at since is the one running out.
//
// ok is false when there is no countdown to state — the window has closed, or
// the date could not be read. Absence and zero are different values (ADR-0018)
// and a caller that could not tell them apart would turn an unparseable
// constant into maximum urgency, which is exactly the failure this function
// exists not to commit.
func RemainingCheckDays(now time.Time, checkedAt string) (int, bool) {
	checked, err := time.Parse("2006-01-02", strings.TrimSpace(checkedAt))
	if err != nil {
		return 0, false
	}
	elapsed := int(now.UTC().Sub(checked) / (24 * time.Hour))
	// A check dated in the future buys no extra days. The cause is a skewed
	// clock or a typo'd constant, and neither is a reason to tell a reader the
	// figures are good for longer than the rule allows.
	if elapsed < 0 {
		elapsed = 0
	}
	remaining := PriceTableStaleDays - elapsed
	if remaining < 0 {
		return 0, false
	}
	return remaining, true
}
