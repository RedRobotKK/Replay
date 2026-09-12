package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// The countdown in the ask.
//
// A countdown is the strongest lever on this line and the easiest one to cheat
// with, so it is worth writing down which countdown this is and which ones were
// available and refused.
//
// REFUSED: a deadline on the appeal ("support by the 30th"). There is no such
// date. REFUSED: a goal thermometer. There is no goal, and a bar that fills is
// a claim about other people's behaviour that nothing here measures. REFUSED: a
// projection of the reader's own future waste ("at this rate, $X a month"). The
// arithmetic is easy and the figure would be estimated while sitting one line
// under a measured one, which is the exact confusion ADR-0002 exists to
// prevent. REFUSED: "only N people have supported this". True, small, and
// it would be shaming a reader with a number that is only low because nobody
// has heard of the project.
//
// TAKEN: the price table's own check window. It is arithmetic on a constant
// that is already compiled in and already printed elsewhere by
// PriceTableAgeNote. It ticks without anyone deciding that it does. What it
// counts down to is real and is stated in the same sentence. And the money
// being asked for is literally what resets it, because re-verifying the table
// means live API calls that cost money - which makes this the one countdown
// here where the ask and the clock are the same fact seen twice.
//
// What it must never imply is that something is lost at zero. Nothing is. The
// figures get older, the tool says so louder, and that is the whole stake.
// tipcountdown_test.go holds the line to that with a ban list.

// tipCountdown is the countdown sentence for the compiled price table.
func tipCountdown(now time.Time) string {
	return tipCountdownAt(now, cachemodel.PriceTableCheckedAt)
}

// tipCountdownAt takes the check date as an argument so the two ends of the
// window can be exercised without waiting for the calendar.
//
// An unreadable date returns the empty string rather than a countdown of zero.
// Absence and zero are different values (ADR-0018), and collapsing them here
// would turn a typo'd constant into maximum urgency on the one line in the
// binary that asks for money.
func tipCountdownAt(now time.Time, checkedAt string) string {
	if days, ok := cachemodel.RemainingCheckDays(now, checkedAt); ok {
		return fmt.Sprintf(
			"The price table behind that number stays current for %d more days;\n"+
				"re-checking it means live API calls that cost real money.", days)
	}
	checked, err := time.Parse("2006-01-02", strings.TrimSpace(checkedAt))
	if err != nil {
		return ""
	}
	over := int(now.UTC().Sub(checked)/(24*time.Hour)) - cachemodel.PriceTableStaleDays
	return fmt.Sprintf(
		"The price table behind that number is %d days past its %d-day check\n"+
			"window; re-checking it means live API calls that cost real money.",
		over, cachemodel.PriceTableStaleDays)
}
