package main

import (
	"fmt"
	"time"
)

// The subscription half of the status line.
//
// Claude Code hands a statusline script a `rate_limits` object on stdin,
// carrying used_percentage and resets_at for a five hour window, a seven day
// window, and a spend limit where the account has one. It has been shipped for
// a while and Replay was not reading it, which meant the one population the
// dollar figures do not apply to had nothing at all.
//
// Two currencies, one payload. A metered account is billed per token and gets
// `cost`. A subscription seat is not, and gets `rate_limits`. Neither is a
// substitute for the other and this file does not try to convert between them:
// the titration that attempted to price a window in tokens returned a null
// result, and inventing a rate here would be exactly the figure this project
// refuses to state.
type window struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type rateLimits struct {
	FiveHour   *window `json:"five_hour"`
	SevenDay   *window `json:"seven_day"`
	SpendLimit *window `json:"spend_limit"`
}

// quotaLine reports the window that will stop you first.
//
// Three windows can be live at once and they do not agree. Reporting the
// five-hour figure because it is listed first tells a reader they have room
// while the seven-day window is the one about to bind. So this picks the
// highest consumed and names which it is, because "83%" without the window is
// not actionable.
//
// A window whose reset has already passed is not reported at all. Claude Code
// shipped a fix for a bug where the pre-reset percentage kept displaying after
// the window reset on an idle session, and a stale reading rendered identically
// to a live one. Reading the field back after that reset is the same defect
// moved downstream, so an expired window is treated as no reading rather than
// as a full one.
func quotaLine(s statusInput, now time.Time) string {
	if s.RateLimits == nil {
		return ""
	}
	type named struct {
		label string
		w     *window
	}
	var best named
	for _, c := range []named{
		{"5h", s.RateLimits.FiveHour},
		{"7d", s.RateLimits.SevenDay},
		{"spend", s.RateLimits.SpendLimit},
	} {
		if c.w == nil {
			continue
		}
		// Expired is not a reading. Anything at or before now describes a
		// window that has already turned over.
		if c.w.ResetsAt > 0 && !time.Unix(c.w.ResetsAt, 0).After(now) {
			continue
		}
		if best.w == nil || c.w.UsedPercentage > best.w.UsedPercentage {
			best = c
		}
	}
	if best.w == nil {
		return ""
	}
	if best.w.ResetsAt <= 0 {
		return fmt.Sprintf("%s %.0f%%", best.label, best.w.UsedPercentage)
	}
	return fmt.Sprintf("%s %.0f%% resets in %s",
		best.label, best.w.UsedPercentage, shortUntil(time.Unix(best.w.ResetsAt, 0), now))
}

// shortUntil renders a duration the way somebody deciding whether to start a
// long run would want it: coarse, and never in seconds.
func shortUntil(t, now time.Time) string {
	d := t.Sub(now)
	if d < 0 {
		d = 0
	}
	if h := int(d.Hours()); h >= 1 {
		if m := int(d.Minutes()) % 60; m > 0 {
			return fmt.Sprintf("%dh%02dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// timeNow is a seam so the rendered line is testable without a clock.
var timeNow = time.Now

// bindingPercent is the consumed fraction of whichever window binds, or 0 when
// none is live. Used only to decide whether to colour, never to state a figure.
func bindingPercent(s statusInput) float64 {
	if s.RateLimits == nil {
		return 0
	}
	now := timeNow()
	var best float64
	for _, w := range []*window{s.RateLimits.FiveHour, s.RateLimits.SevenDay, s.RateLimits.SpendLimit} {
		if w == nil {
			continue
		}
		if w.ResetsAt > 0 && !time.Unix(w.ResetsAt, 0).After(now) {
			continue
		}
		if w.UsedPercentage > best {
			best = w.UsedPercentage
		}
	}
	return best
}
