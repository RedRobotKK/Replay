package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/money"
	"github.com/RedRobotKK/Replay/internal/usage"
)

// Cost per task from usage records alone.
//
// `replay cost` reads transcripts. Three situations do not have any.
//
// A regulated or enterprise operator frequently cannot let a tool read prompt
// content at all, and equally frequently can export a usage ledger. For that
// segment this path is the difference between an approval and a rejection in
// security review, and no amount of reassurance about what the parser does
// with the bytes substitutes for not having the bytes.
//
// A finance owner wants a reconciliation number. The conversation is not
// evidence for anything they are being asked, and reading it is a cost with no
// benefit on their side of the desk.
//
// And transcripts get rotated away. When they are gone the usage records are
// the only surviving evidence that the spend happened, and reporting from them
// is the honest fallback rather than reporting nothing.
//
// The reason this is possible at all is that Replay's cache arithmetic never
// needed the content. cachemodel.ExpectedRead is the previous request's prompt
// minus its uncached tail; ClassifyRead compares that against the next
// request's reported read; analysis.Calibrate walks a lane touching no message
// body. Content is what turns a deficit into a CAUSE — which block moved,
// which tool definition appeared — and that is exactly the half this path
// gives up.
//
// So the tiers of ADR-0002 split inside a single row here, and the split is
// the point: the same break can be perfectly Measured for what it cost and
// NOT MEASURED for why it happened.

// usageCostSchema is the report's own schema.
//
// It is not replay.cost.v2. The two documents answer different questions from
// different evidence, and a consumer handed this one under the transcript
// report's version string would read `avoidableUsd: null` as a bug in a report
// it thought it understood.
const usageCostSchema = "replay.cost.usage.v1"

// usageRow is one session priced from usage records.
//
// Every figure the export could not support is a pointer. That is deliberately
// wider than a float64 would be: ADR-0018's first rule is that provenance is a
// field rather than a comment, and a renderer that is handed a nil cannot print
// a zero by accident.
type usageRow struct {
	ID       string  `json:"session"`
	Model    string  `json:"model"`
	Requests int     `json:"requests"`
	CostUSD  float64 `json:"costUsd"`
	// Breaks, AvoidableUSD and AvoidableTokens are nil when the export could
	// not support a sequence. See usageReport.NotMeasuredWhy.
	Breaks          *int      `json:"breaks"`
	AvoidableUSD    *float64  `json:"avoidableUsd"`
	AvoidableTokens *int      `json:"avoidableTokens"`
	At              time.Time `json:"at"`
}

// usageReport is the summary of a usage-only run.
type usageReport struct {
	Sessions int `json:"sessions"`
	// Requests counts the requests actually priced. UnpricedRequests counts
	// the rest, and they are kept apart for the reason cost.go keeps unpriced
	// transcripts out of its total: excluded is not free.
	Requests         int `json:"requests"`
	UnpricedRequests int `json:"unpricedRequests"`

	TotalUSD  float64 `json:"totalUsd"`
	MedianUSD float64 `json:"medianUsd"`
	P90USD    float64 `json:"p90Usd"`

	Breaks          *int     `json:"breaks"`
	AvoidableUSD    *float64 `json:"avoidableUsd"`
	AvoidableTokens *int     `json:"avoidableTokens"`
	AvoidableShare  *float64 `json:"avoidableShare"`

	// Causes counts only the causes usage and timing settle on their own.
	// CauseNotMeasured counts the breaks whose cause needs the message
	// history, which this input does not have.
	//
	// The transcript path has a fourth answer for those — "prefix diverged
	// inside the message history at an unknown block", with the position
	// located by the byte-to-token fit. It is not available here and must not
	// be borrowed: there is no fit, because there are no bytes, and a borrowed
	// classification is ADR-0018's "ratio used outside the population it was
	// fitted on" wearing a different hat.
	Causes           map[string]int `json:"causes"`
	CauseNotMeasured *int           `json:"causeNotMeasured"`

	// NotMeasuredWhy is the reason the break figures are absent, empty when
	// they are present.
	NotMeasuredWhy string `json:"notMeasuredWhy,omitempty"`
}

// usageNotMeasured lists what token counts can never report, with the reason.
//
// It is a table rather than prose because it is printed on every run.
// ADR-0018 rule 4: saying "not measured" has to be cheap on every surface, and
// a reader arriving from `replay cost` will notice figures missing and needs to
// be told they are structurally absent rather than broken.
var usageNotMeasured = [][2]string{
	{"repeated tool results", "needs the tool result blocks; a usage record has no content"},
	{"tool errors", "needs the is_error flag on each result block"},
	{"blame, per block", "needs the byte size of every block, and the fit built from them"},
	{"alternative layouts", "needs each request's context to re-run it a different way"},
	{"agent lane fan-out", "needs a lane id on each record; this export carries none"},
}

// priceUsage prices an export and measures what the export can support.
//
// The break arithmetic is deliberately the same functions the transcript path
// uses — cachemodel.ExpectedRead through ClassifyRead, and ClassifyBreak for
// the causes usage settles — rather than a second implementation. A second
// implementation of the same arithmetic is free to disagree with the one on
// the reader's screen, and nothing would notice.
func priceUsage(e *usage.Export) (usageReport, []usageRow) {
	rep := usageReport{Causes: map[string]int{}}
	sequenced, why := e.SequenceEvidence()
	rep.NotMeasuredWhy = why

	var rows []usageRow
	var costs []float64
	breaks, deficit, unexplained := 0, 0, 0
	var avoidable float64

	for _, g := range e.BySession() {
		row := usageRow{ID: g.Session}
		// Per-session break figures accumulate here and are totalled from the
		// same walk. Re-deriving them for the row afterwards was the obvious
		// alternative and it is a second implementation of the same
		// arithmetic, free to disagree with the summary above it.
		sBreaks, sDeficit := 0, 0
		var sAvoidable float64
		// The model column names the model that ran the largest share of the
		// money, which is the one a routing decision is about. A session is not
		// one model and this is a label, not a key.
		dominant := 0.0
		for i, r := range g.Records {
			u := r.ToAnthropic()
			price, priced := cachemodel.PriceFor(r.Model)
			if !priced {
				// Excluded, not free. cost.go draws the same line one level
				// up, over whole transcripts; here it has to be drawn per
				// record, because an unpriced record sits inside a session
				// that is otherwise priced and would contribute a silent zero.
				rep.UnpricedRequests++
			} else {
				c := cachemodel.CostUSD(u, price)
				row.CostUSD += c
				row.Requests++
				rep.Requests++
				if c > dominant {
					dominant, row.Model = c, r.Model
				}
			}
			// A session began when its earliest record began. Records arrive
			// in query order, which is not time order, so this is a minimum
			// rather than a first-seen.
			if row.At.IsZero() || (!r.At.IsZero() && r.At.Before(row.At)) {
				row.At = r.At
			}
			// Breaks need the record before this one, in the order it ran.
			// Unpriced records stay in the sequence: they are real requests,
			// and dropping them would compare two records that were never
			// adjacent and book the difference as a deficit.
			if !sequenced || i == 0 {
				continue
			}
			prev := g.Records[i-1]
			outcome, expected := cachemodel.ClassifyRead(prev.ToAnthropic(), u)
			if outcome != cachemodel.ReadBroken {
				continue
			}
			d := expected - r.CachedRead
			sBreaks++
			sDeficit += d
			if priced {
				sAvoidable += float64(d) / 1_000_000 * price.InputPerMTok
			}
			if cause, ok := cachemodel.ClassifyBreak(prev.ToAnthropic(), u, prev.Model, r.Model, r.At.Sub(prev.At)); ok {
				rep.Causes[string(cause)]++
			} else {
				unexplained++
			}
		}
		if row.Requests == 0 {
			// Nothing in this session could be priced, so it is not a row.
			// Counting it would put a $0.00 session into the median, which is
			// the shape of cost.go's unpriced-transcript rule one level down.
			continue
		}
		if sequenced {
			// Per-row figures only where the report as a whole has them. A row
			// carrying a break count under a summary that says NOT MEASURED
			// would be two answers to one question.
			b, d, a := sBreaks, sDeficit, sAvoidable
			row.Breaks, row.AvoidableTokens, row.AvoidableUSD = &b, &d, &a
		}
		breaks += sBreaks
		deficit += sDeficit
		avoidable += sAvoidable
		rows = append(rows, row)
		costs = append(costs, row.CostUSD)
		rep.TotalUSD += row.CostUSD
	}

	rep.Sessions = len(rows)
	sort.Float64s(costs)
	rep.MedianUSD = percentile(costs, 0.5)
	rep.P90USD = percentile(costs, 0.9)

	if sequenced {
		rep.Breaks, rep.AvoidableTokens, rep.CauseNotMeasured = &breaks, &deficit, &unexplained
		rep.AvoidableUSD = &avoidable
		// A share over a total of zero is a division, not a measurement. The
		// avoidable dollars can be measured at nothing while the share of a
		// nothing total is unknown, and those are two of the three values
		// ADR-0018 keeps apart.
		if rep.TotalUSD > 0 {
			share := avoidable / rep.TotalUSD
			rep.AvoidableShare = &share
		}
	}
	return rep, rows
}

// renderUsageCost writes the report a human reads.
func renderUsageCost(rep usageReport, rows []usageRow, perTask bool) string {
	var b strings.Builder
	if rep.Sessions == 0 {
		fmt.Fprintf(&b, "No usage record could be priced. %s\nThey are excluded rather than counted as free.\n", unpricedClause(rep.UnpricedRequests))
		return b.String()
	}
	fmt.Fprintf(&b, "Cost per task, across %d session(s) and %d request(s) priced from usage records\nalone, at list prices dated %s (caching rules %s).%s\n\n",
		rep.Sessions, rep.Requests, cachemodel.PriceTableVersion, cachemodel.RulesVersionInEffect(),
		cachemodel.PriceTableAgeNote(time.Now()))

	fx := money.Detect(os.LookupEnv, time.Now())
	fmt.Fprintf(&b, "  total          %s\n", fxCol(fx, rep.TotalUSD))
	fmt.Fprintf(&b, "  median task    %s\n", fxCol(fx, rep.MedianUSD))
	fmt.Fprintf(&b, "  p90 task       %s\n", fxCol(fx, rep.P90USD))
	if rep.AvoidableUSD != nil {
		// The share is its own measurement and can be absent while the dollars
		// are present: a corpus that priced at zero has no denominator, and
		// "(0% of the total)" over it would be a ratio nobody took.
		share := "share NOT MEASURED: nothing priced to be a share of"
		if rep.AvoidableShare != nil {
			share = fmt.Sprintf("(%.0f%% of the total)", *rep.AvoidableShare*100)
		}
		fmt.Fprintf(&b, "  avoidable      %s  %s\n", fxCol(fx, *rep.AvoidableUSD), share)
		fmt.Fprintf(&b, "                 %s tokens re-billed across %d cache break(s)\n",
			shortTokens(*rep.AvoidableTokens), *rep.Breaks)
	} else {
		const indent = "                 "
		fmt.Fprintf(&b, "  avoidable      NOT MEASURED\n")
		// wrapAt indents continuation lines only, so the first one is indented
		// here. Without it the reason starts in column zero, under a block
		// where every other value is in a column, and reads as a new section.
		fmt.Fprintf(&b, "%s%s\n", indent, wrapAt(rep.NotMeasuredWhy, 60, indent))
	}
	if n := fx.Note(); n != "" {
		fmt.Fprintf(&b, "\n%s\n", wrapAt(n, 78, ""))
	}

	fmt.Fprintf(&b, "\nThe cost is MEASURED: every token count above is the provider's own, and no\n"+
		"conversation content was read, because this file contains none.\n")

	if rep.Breaks != nil && *rep.Breaks > 0 {
		fmt.Fprintf(&b, "\nWhy the cache broke, where token counts can say:\n")
		causes := make([]string, 0, len(rep.Causes))
		for c := range rep.Causes {
			causes = append(causes, c)
		}
		sort.Strings(causes)
		for _, c := range causes {
			fmt.Fprintf(&b, "  %3d  %s\n", rep.Causes[c], c)
		}
		if *rep.CauseNotMeasured > 0 {
			fmt.Fprintf(&b, "  %3d  NOT MEASURED. The prefix diverged somewhere inside the message history,\n"+
				"       and only the message history says where. The deficit above is still a\n"+
				"       measurement; the cause is not.\n", *rep.CauseNotMeasured)
		}
	}

	fmt.Fprintf(&b, "\nNOT MEASURED from usage alone, and not zero:\n")
	for _, nm := range usageNotMeasured {
		fmt.Fprintf(&b, "  %-23s %s\n", nm[0], nm[1])
	}
	fmt.Fprintf(&b, "\nFor any of those, point `replay cost` at the transcripts instead.\n")

	if rep.UnpricedRequests > 0 {
		fmt.Fprintf(&b, "\n%d further request(s) were read but not priced, because their model is not in\nthe price table. They are excluded rather than counted as free.\n", rep.UnpricedRequests)
	}

	if perTask && len(rows) > 0 {
		sorted := append([]usageRow(nil), rows...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].CostUSD > sorted[j].CostUSD })
		fmt.Fprintf(&b, "\n  %-16s %-24s %8s %10s %10s %9s\n",
			"session", "model", "requests", "cost", "avoidable", "breaks")
		for _, r := range sorted {
			avoid, brk := "NOT MEAS.", "NOT MEAS."
			if r.AvoidableUSD != nil {
				avoid = fmt.Sprintf("$%.2f", *r.AvoidableUSD)
				brk = fmt.Sprintf("%d", *r.Breaks)
			}
			fmt.Fprintf(&b, "  %-16s %-24s %8d %10s %10s %9s\n",
				r.ID, r.Model, r.Requests, fmt.Sprintf("$%.2f", r.CostUSD), avoid, brk)
		}
	}
	return b.String()
}

// checkUsageCeiling enforces --max-avoidable-usd over a usage-only report.
//
// The nothing-measured case is the reason this is not checkAvoidableCeiling
// with a different argument. That one refuses when nothing was PRICED. Here a
// corpus can be fully priced and still have no avoidable figure at all,
// because the export could not support a sequence — and that is the input a
// regulated operator is most likely to hand it. A gate that treated the
// absent figure as zero would report a clean bill of health nobody earned,
// which is precisely what the flag exists to prevent.
func checkUsageCeiling(ceiling float64, rep usageReport, stdout io.Writer) error {
	if ceiling == 0 {
		return nil // not asked for
	}
	if ceiling < 0 {
		return fmt.Errorf("--max-avoidable-usd needs a positive ceiling, got %.2f", ceiling)
	}
	if rep.Sessions == 0 {
		_, _ = fmt.Fprintf(stdout, "\n  GATE: NOT MEASURED. Nothing here priced, so there is no avoidable spend to\n"+
			"  compare against $%.2f.\n", ceiling)
		return fmt.Errorf("refusing to pass a gate over 0 priced sessions: NOT MEASURED")
	}
	if rep.AvoidableUSD == nil {
		_, _ = fmt.Fprintf(stdout, "\n  GATE: NOT MEASURED. This export was priced, but it cannot support a cache\n"+
			"  break figure, so there is no avoidable spend to compare against $%.2f.\n  %s\n",
			ceiling, rep.NotMeasuredWhy)
		return fmt.Errorf("refusing to pass a gate over an avoidable figure that is NOT MEASURED")
	}
	if *rep.AvoidableUSD > ceiling {
		_, _ = fmt.Fprintf(stdout, "\n  GATE: avoidable spend $%.2f is over the $%.2f ceiling, across %d session(s).\n",
			*rep.AvoidableUSD, ceiling, rep.Sessions)
		return fmt.Errorf("%w: $%.2f over $%.2f", errGate, *rep.AvoidableUSD, ceiling)
	}
	_, _ = fmt.Fprintf(stdout, "\n  GATE: avoidable spend $%.2f is within the $%.2f ceiling.\n", *rep.AvoidableUSD, ceiling)
	return nil
}

// runCostUsage is `replay cost --usage <file>`.
func runCostUsage(path string, asJSON, perTask bool, ceiling float64, stdout io.Writer) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("--usage could not read %s: %w", path, err)
	}
	e, err := usage.ParseExport(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	rep, rows := priceUsage(e)

	if asJSON {
		out := map[string]any{
			"schema":  usageCostSchema,
			"summary": rep,
			"notMeasured": func() []map[string]string {
				list := make([]map[string]string, 0, len(usageNotMeasured))
				for _, nm := range usageNotMeasured {
					list = append(list, map[string]string{"figure": nm[0], "why": nm[1]})
				}
				return list
			}(),
		}
		if perTask {
			out["tasks"] = rows
		}
		// Wrapped, not returned bare. A broken pipe or a full disk here
		// leaves a truncated JSON document that a consumer may well parse, so
		// the one thing the operator must not have to infer is that a WRITE is
		// what failed rather than the export being bad.
		if err := writeJSON(stdout, out); err != nil {
			return fmt.Errorf("writing the usage report as JSON: %w", err)
		}
		return checkUsageCeiling(ceiling, rep, stdout)
	}

	// Same reasoning as the JSON branch, and the quieter failure of the two: a
	// report that stops halfway reads as a report with less to say, and the
	// block it stops before is the one listing what was NOT MEASURED.
	if _, err := io.WriteString(stdout, renderUsageCost(rep, rows, perTask)); err != nil {
		return fmt.Errorf("writing the usage report: %w", err)
	}
	return checkUsageCeiling(ceiling, rep, stdout)
}

// usageOnlyRefusals names the flags this path cannot honour.
//
// Each of these would carry a figure the usage-only path did not measure into
// somewhere it cannot be retracted from: a card that gets posted publicly, a
// comparison whose change is the whole claim, somebody else's calibration
// corpus. Refusing is cheap. A card with a blank where the headline goes is
// not, and it is the version that travels.
func usageOnlyRefusals(share bool, png, compare, contribute string, perLane bool, rest []string) error {
	switch {
	case share:
		return fmt.Errorf("--share publishes the avoidable rate as a headline, and a usage export may not " +
			"support one. --usage will not build a card on a figure it might have to mark NOT MEASURED")
	case png != "":
		return fmt.Errorf("--png writes the share card, which --usage refuses to build; see --share")
	case compare != "":
		return fmt.Errorf("--compare reports a change between two dates, and the change is the whole claim. " +
			"It is not built for --usage: splitting an export nobody has declared complete would compare two " +
			"unknown fractions of two periods")
	case contribute != "":
		return fmt.Errorf("--contribute pools these figures into a shared corpus, and a usage-only run has no " +
			"cache-break distribution to pool. Contribute from transcripts")
	case perLane:
		return fmt.Errorf("--per-lane reports agent lanes, and a usage export carries no lane id. The unit " +
			"is the session here, and there is nothing to fan out")
	case len(rest) > 0:
		return fmt.Errorf("--usage reads one usage export; %q looks like a transcript path. The two are "+
			"different evidence and mixing them in one total would hide which figure came from which", rest[0])
	}
	return nil
}
