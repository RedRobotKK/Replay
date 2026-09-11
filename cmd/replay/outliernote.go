package main

import (
	"fmt"
	"math"

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
// "everything replay priced", not "everything you spent". The total excludes
// sessions whose model is not in the price table, covers only the directories
// this walk was given, and is the estimated tier rather than an invoice. The
// note used to say "everything you spent", which is false in all three
// directions at once and is the only clause here a reader could act on wrongly.
//
// What the instruction promises is bounded by what the command delivers.
// `replay blame <transcript>` reports the session's MAIN LANE and discloses the
// rest ("Scope: 1 of 17 lanes"). On a fanned-out session that is a small share
// of the cost the finding just quoted: the corpus figure for one session here
// was $1,056.14, of which the main transcript accounted for $286.59 and 1,013
// sub-agent transcripts for $769.55. Saying "ranks what filled it" over that
// would be the same defect as naming a command that does not exist — an
// instruction the reader takes at face value and a result that answers a
// narrower question than the one they asked.
//
// The instruction names the transcript rather than the session id. It used to
// print `replay why <id>`, and `replay why` was never a command: it is a TUI
// screen label that reached CLI output and shipped, firing on the one line a
// first-time reader is most likely to act on. A session id prefix is enough to
// recognise a row and not enough to open one, so the note now carries the path
// the row was priced from and names the command that reads it.
//
// Silence is the intended common case, and on a fanned-out corpus it is not
// the actual one: Notable() clears on four of 116 rows here. The threshold is
// a judgement rather than a measurement, and ADR-0009 says so of every
// threshold in this tool. What that buys is one line, so the cost of the
// number being wrong is bounded — but it is not zero, and the note says
// "your largest" rather than "one", because on this corpus four rows cleared
// the same bar and only one of them is being shown.
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
	// The noun the report is already using. Under --per-lane a row is a lane,
	// and calling it a session five lines under a header that said "lanes" is
	// how one word came to mean two things in this file in the first place.
	noun := s.Unit
	if noun == "" {
		noun = unitSession
	}
	// Floored, not rounded. %.0f turns 99.5% into "100%", which asserts every
	// other row cost nothing — and the %.2f totals beside it then render
	// "$199.00 of $199.00", so the arithmetic appears to confirm the false
	// claim. Flooring can understate by less than a point and cannot state
	// something untrue.
	pct := math.Floor(o.Share * 100)
	finding := fmt.Sprintf("\n  Your largest %s was %.0f%% of everything replay priced: %s, $%.2f of $%.2f across %d %ss.\n",
		noun, pct, prefixID(peak.ID), o.Cost, o.Total, o.N, noun)
	if peak.path == "" {
		// No transcript to name, so no instruction. The finding is still true
		// and still worth printing; an instruction with nothing runnable in it
		// is not, and printing one is the defect this line was rewritten to
		// remove.
		return finding
	}
	return finding + fmt.Sprintf("  replay blame %s   ranks what filled its main lane.\n", peak.path)
}
