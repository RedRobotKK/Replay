package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// The countdown in the ask.
//
// A countdown is the strongest lever available here and the easiest one to
// cheat with, so this file is mostly about what it is NOT allowed to say. The
// project's whole claim is that its figures carry their population and their
// date; an invented deadline on the one surface that asks for money would be
// the single most expensive sentence in the binary.
//
// So the countdown is arithmetic on a constant that is already compiled in and
// already printed elsewhere: the price table's 60-day check window. It ticks
// without anyone deciding it does, what it counts down to is real, and the
// money being asked for is literally what resets it - re-verification means
// live API calls that cost money. Nothing stops when it reaches zero. The
// figures just get older, which is what the line says.

func onDay(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

// TD-1: inside the window it states the days remaining, and the number is the
// one cachemodel computes rather than a second opinion.
func TestTD1_ItNamesTheDaysLeft(t *testing.T) {
	now := onDay("2026-09-12")
	want, ok := cachemodel.RemainingCheckDays(now, cachemodel.PriceTableCheckedAt)
	if !ok {
		t.Skip("the compiled table is already past its window; TD-3 covers that state")
	}
	got := tipCountdown(now)
	if !strings.Contains(got, strconv.Itoa(want)) {
		t.Errorf("the countdown does not name the %d days cachemodel computes: %q", want, got)
	}
}

// TD-2: it counts DOWN. Asserted as a direction over successive days, because a
// line that printed the table's AGE would satisfy a single-day check and would
// be the opposite sentence.
func TestTD2_TheNumberFalls(t *testing.T) {
	num := regexp.MustCompile(`(\d+) more days`)
	prev := -1
	for d := 0; d < 20; d++ {
		line := tipCountdown(onDay("2026-09-07").AddDate(0, 0, d))
		m := num.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("day %d: no countdown in %q", d, line)
		}
		n, _ := strconv.Atoi(m[1])
		if prev >= 0 && n >= prev {
			t.Fatalf("day %d: %d did not fall below %d; this is not a countdown", d, n, prev)
		}
		prev = n
	}
}

// TD-3: past the window it does not keep counting. A countdown that runs
// negative is a lie told in a new format; the true statement there is that the
// window has closed, and it has to read as older figures rather than as peril.
func TestTD3_PastTheWindowItStopsCounting(t *testing.T) {
	line := tipCountdown(onDay("2026-09-07").AddDate(0, 0, cachemodel.PriceTableStaleDays+12))
	if strings.Contains(line, "more days") {
		t.Errorf("the countdown ran past its own end: %q", line)
	}
	if !strings.Contains(line, "12") {
		t.Errorf("past the window the line should say how far past, got: %q", line)
	}
}

// TD-4: an unreadable date prints nothing at all.
//
// This is the failure that matters most. If a malformed constant fell through
// to "0 days left", a typo would silently become maximum urgency on the one
// line in the binary that asks for money.
func TestTD4_NoDateMeansNoCountdown(t *testing.T) {
	if got := tipCountdownAt(onDay("2026-09-12"), "not a date"); got != "" {
		t.Errorf("an unreadable check date produced a countdown anyway: %q", got)
	}
}

// TD-5: the countdown may not manufacture peril.
//
// Every banned word here describes something that is not true: nothing is lost,
// nothing closes, nothing is the reader's last chance, and the project does not
// stop if nobody pays. The lever is that the figures age, and that is all it is
// allowed to claim.
func TestTD5_NothingIsAtStake(t *testing.T) {
	lines := []string{
		tipCountdown(onDay("2026-09-12")),
		tipCountdown(onDay("2026-12-01")),
	}
	banned := []string{
		"last chance", "act now", "hurry", "expires", "expire", "running out",
		"before it", "don't miss", "only ", "left to", "urgent", "final",
		"shut", "stop", "lose", "lost", "risk", "danger",
	}
	for _, line := range lines {
		low := strings.ToLower(line)
		for _, b := range banned {
			if strings.Contains(low, b) {
				t.Errorf("the countdown manufactures peril (%q): %q", b, line)
			}
		}
	}
}

// TD-6: both arms carry it.
//
// The countdown is not an experimental variable. It says what the money buys,
// which is true in both framings, so withholding it from half the machines
// would be withholding a true statement to make a cleaner chart.
func TestTD6_BothArmsCarryTheCountdown(t *testing.T) {
	for _, arm := range []string{"A", "B"} {
		line := tipLineAt(arm, 149.44, false, onDay("2026-09-12"))
		if !strings.Contains(line, "check window") && !strings.Contains(line, "more days") {
			t.Errorf("arm %s has no countdown:\n%s", arm, line)
		}
	}
}

// TD-7: the countdown names what the money actually does.
//
// A countdown with no stated connection to the ask is pressure for its own
// sake. The connection here is real - re-verifying the table means live API
// calls that cost money - and stating it is what separates this from a timer.
func TestTD7_ItSaysWhatResetsIt(t *testing.T) {
	line := tipCountdown(onDay("2026-09-12"))
	if !strings.Contains(strings.ToLower(line), "api calls") {
		t.Errorf("the countdown does not say what re-checking costs: %q", line)
	}
}
