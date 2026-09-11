package tui

import (
	"fmt"
	"time"
)

// LiveSession is one lane of the live proxy, rendered.
//
// `replay serve` holds per-lane state in memory and publishes it, and nothing
// ever drew it. The proxy on the maintainer's machine ran 24.8 hours with no
// sessions recorded because nothing was pointed at it, and that was discovered
// by curling a JSON endpoint on an unrelated errand. A day of instrument time
// spent for nothing, for want of a window.
//
// The state that matters most here is not the busy one. It is UP AND RECORDING
// NOTHING, which an empty table renders as a quiet day and which is in fact a
// misconfiguration with exactly one cause.
type LiveSession struct {
	ID           string
	Model        string
	Requests     int
	PromptTokens int
	CachedShare  float64
	Breaks       int
	CostUSD      float64
	LastSeen     time.Time
}

// Live is the whole proxy: its address, whether it answers, and what it has seen.
type Live struct {
	Addr          string
	Reachable     bool
	UptimeSeconds int64
	Sessions      []LiveSession
	PriceTable    string
}

// LiveScreen draws what is flowing through the proxy right now.
func LiveScreen(l Live, now time.Time) Screen {
	// The only screen that never said what it was.
	//
	// Nine screens opened with a banner; this one opened straight into "no
	// proxy answered at 127.0.0.1:4000", which is an answer with the question
	// missing. A reader who opened it without meaning to had nothing on screen
	// telling them what they were looking at or how to leave.
	s := Screen{Key: 'l', Title: "live", From: Measured, Lines: screenHead("live")}
	add := func(f string, a ...any) { s.Lines = append(s.Lines, fmt.Sprintf(f, a...)) }

	if !l.Reachable {
		add("  no proxy answered at %s", l.Addr)
		add("")
		add("  %s", Dim("This screen shows traffic as it happens, which needs `replay serve`"))
		add("  %s", Dim("running. Nothing is wrong with your transcripts; there is simply no"))
		add("  %s", Dim("live proxy to look into."))
		add("")
		add("  start one    %s", paint(Accent, "replay serve"))
		return s
	}

	up := time.Duration(l.UptimeSeconds) * time.Second
	if len(l.Sessions) == 0 {
		// The whole reason this screen exists.
		// The one state this screen was built for, and the only Alarm on it.
		// A proxy up for hours with nothing recorded is not a quiet day.
		add("  proxy up %s at %s, and it has %s", coarse(up), l.Addr, paint(Alarm, "recorded nothing"))
		add("")
		add("  A proxy with no traffic is not a quiet day, it is an agent that was")
		add("  never pointed at it. The ledger stays empty and every figure Replay")
		add("  reports keeps coming from transcripts instead of the wire.")
		add("")
		// The fix, and the reason a reader opened this screen. Accented so the
		// eye lands on the line it has to type.
		add("  %s", paint(Accent, fmt.Sprintf("export ANTHROPIC_BASE_URL=http://%s", l.Addr)))
		add("  %s", Dim("then start your agent in that shell"))
		return s
	}

	add("  proxy up %s at %s", coarse(up), l.Addr)
	add("")
	// Headings and rule are structure. Dimming them is what lets the figures
	// rise without anything shouting.
	add("  %s", Dim("session    model                 reqs    cached  breaks     cost   last"))
	add("  %s", Dim("---------  --------------------  ------  ------  ------  -------  -----"))
	for _, x := range l.Sessions {
		// The break count is painted after it is padded, never before: an
		// escape occupies no cells, so colouring the value first would take
		// width out of its own column and shift the two after it.
		add("  %-9s  %-20s  %6d  %5.0f%%  %s  %7.2f  %s",
			trunc(x.ID, 9), trunc(x.Model, 20), x.Requests, x.CachedShare*100,
			paint(Severity(x.Breaks), fmt.Sprintf("%6d", x.Breaks)),
			x.CostUSD, Dim(fmt.Sprintf("%5s", ago(now.Sub(x.LastSeen)))))
	}
	return s
}

// coarse renders a duration the way somebody glancing at a screen wants it.
func coarse(d time.Duration) string {
	if h := int(d.Hours()); h >= 1 {
		return fmt.Sprintf("%dh%02dm", h, int(d.Minutes())%60)
	}
	if m := int(d.Minutes()); m >= 1 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// ago is the same, shortened for a column.
func ago(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "~"
}
