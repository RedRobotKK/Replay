package main

import (
	"strings"
	"testing"
	"time"
)

func at(mins int) int64 { return time.Now().Add(time.Duration(mins) * time.Minute).Unix() }

// TestQL1: the BINDING window is the one reported, not the first one listed.
//
// Three windows arrive at once. Reporting five_hour because it came first would
// tell a reader they have room while the seven-day window is what stops them.
func TestQL1(t *testing.T) {
	var s statusInput
	s.RateLimits = &rateLimits{
		FiveHour: &window{UsedPercentage: 22, ResetsAt: at(40)},
		SevenDay: &window{UsedPercentage: 91, ResetsAt: at(3000)},
	}
	got := quotaLine(s, time.Now())
	if !strings.Contains(got, "91") {
		t.Errorf("must report the binding window (91%%), got %q", got)
	}
	if strings.Contains(got, "22") {
		t.Errorf("must not lead with the slack window, got %q", got)
	}
}

// TestQL2: a reset is reported as time remaining, not as an epoch.
//
// "resets_at 1738425600" is a number. "resets in 40m" is a decision.
func TestQL2(t *testing.T) {
	var s statusInput
	s.RateLimits = &rateLimits{FiveHour: &window{UsedPercentage: 60, ResetsAt: at(40)}}
	got := quotaLine(s, time.Now())
	if strings.Contains(got, "17") && strings.Contains(got, "0000") {
		t.Errorf("epoch leaked into the line: %q", got)
	}
	if !strings.Contains(got, "40m") && !strings.Contains(got, "39m") {
		t.Errorf("must say how long until it resets, got %q", got)
	}
}

// TestQL3: absent rate_limits is silence, not a zero.
//
// A metered API user gets no rate_limits at all. Printing "0%" would tell them
// they have a full window when they have no window.
func TestQL3(t *testing.T) {
	var s statusInput
	if got := quotaLine(s, time.Now()); got != "" {
		t.Errorf("no rate_limits should render nothing, got %q", got)
	}
}

// TestQL4: a window already past its reset is not reported as consumed.
//
// Claude Code shipped a fix for exactly this: the pre-reset percentage kept
// showing after the window reset while the session was idle. A stale reading
// rendered identically to a live one.
func TestQL4(t *testing.T) {
	var s statusInput
	s.RateLimits = &rateLimits{FiveHour: &window{UsedPercentage: 95, ResetsAt: at(-5)}}
	got := quotaLine(s, time.Now())
	if strings.Contains(got, "95") {
		t.Errorf("a window past its reset must not be reported as 95%% consumed: %q", got)
	}
}
