package main

import (
	"fmt"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// outlierNote is the one line a first-time reader can act on.
//
// The report already prints a total, a median and a p90, and none of them
// answers the question somebody has thirty seconds after installing: is any of
// this wrong? A total is not interpretable without a reference, and the
// reference this tool cannot honestly supply is a population — the pooled
// corpus has one member, and a population figure derived from one machine is
// the shape of claim this project has retracted twice.
//
// The reference it CAN supply is the reader against themselves: what share of
// their own total one session took. That needs no population, no key and no
// network, and it means the same thing on a metered account, a subscription
// seat, and a local model where there is no money at all.
//
// It compares against the TOTAL rather than the median because the first
// version compared against the median and, on a real corpus, printed "1363.2x
// your median session" — exact, and a category error wearing a number.
//
// The instruction names the transcript rather than the session id. It used to
// print `replay why <id>`, and `replay why` was never a command: it is a TUI
// screen label that reached CLI output and shipped, firing on the one line a
// first-time reader is most likely to act on. A session id prefix is enough to
// recognise a row and not enough to open one, so the note now carries the path
// the row was priced from and names the command that reads it.
//
// Silence is the common case and is deliberate. A comparison printed on every
// run is one the reader learns to skip, so nothing is said unless the peak is
// far enough from the median to be worth acting on.
func outlierNote(units []costUnit, s costSummary) string {
	if len(units) == 0 {
		return ""
	}
	peak := units[0]
	for _, u := range units[1:] {
		if u.CostUSD > peak.CostUSD {
			peak = u
		}
	}
	// Tasks rather than len(units) is the n: it is the count the summary was
	// computed over, and a ratio quoted against a different denominator than
	// its median is the defect this repository names most often.
	o, ok := analysis.CompareToTotal(peak.CostUSD, s.TotalUSD, s.Tasks)
	if !ok || !o.Notable() {
		return ""
	}
	finding := fmt.Sprintf("\n  One session was %.0f%% of everything you spent: %s, $%.2f of $%.2f across %d sessions.\n",
		o.Share*100, prefixID(peak.ID), o.Cost, o.Total, o.N)
	if peak.path == "" {
		// No transcript to name, so no instruction. The finding is still true
		// and still worth printing; an instruction with nothing runnable in it
		// is not, and printing one is the defect this line was rewritten to
		// remove.
		return finding
	}
	return finding + fmt.Sprintf("  replay blame %s   ranks what filled it.\n", peak.path)
}
