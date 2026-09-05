package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Cost per task.
//
// Providers report spend. Dashboards report spend. What nobody reports is the
// share of that spend nobody chose: the tokens re-billed because a cache broke,
// and the reads repeated because the same content was sent twice. A business
// that cannot see its cost per unit cannot tell growth from a subsidy, and
// agentic work is currently being bought on exactly that blindness.
//
// The unit is one session, because a session is the closest thing a transcript
// has to a task.

type costUnit struct {
	ID           string    `json:"session"`
	Model        string    `json:"model"`
	Requests     int       `json:"requests"`
	CostUSD      float64   `json:"costUsd"`
	AvoidableUSD float64   `json:"avoidableUsd"`
	Breaks       int       `json:"breaks"`
	At           time.Time `json:"at"`
}

type costSummary struct {
	Tasks          int     `json:"tasks"`
	TotalUSD       float64 `json:"totalUsd"`
	MedianUSD      float64 `json:"medianUsd"`
	P90USD         float64 `json:"p90Usd"`
	AvoidableUSD   float64 `json:"avoidableUsd"`
	AvoidableShare float64 `json:"avoidableShare"`
}

// summarise reduces priced sessions to the figures a unit-economics
// conversation needs.
//
// Deliberately no mean: one very long session drags it somewhere no real task
// lives, and a median with a p90 beside it describes the actual distribution of
// work. The avoidable share is a share of what was priced, never of what was
// merely walked past.
func summarise(units []costUnit) costSummary {
	var s costSummary
	if len(units) == 0 {
		return s
	}
	costs := make([]float64, 0, len(units))
	for _, u := range units {
		s.TotalUSD += u.CostUSD
		s.AvoidableUSD += u.AvoidableUSD
		costs = append(costs, u.CostUSD)
	}
	sort.Float64s(costs)
	s.Tasks = len(units)
	s.MedianUSD = percentile(costs, 0.5)
	s.P90USD = percentile(costs, 0.9)
	if s.TotalUSD > 0 {
		s.AvoidableShare = s.AvoidableUSD / s.TotalUSD
	}
	return s
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	// Median of an even count is the midpoint of the two middle values; other
	// percentiles take the nearest rank, which is the convention a reader of a
	// p90 expects.
	if p == 0.5 && len(sorted)%2 == 0 {
		m := len(sorted) / 2
		return (sorted[m-1] + sorted[m]) / 2
	}
	i := int(p * float64(len(sorted)-1))
	return sorted[i]
}

func renderCost(s costSummary, unpriced int) string {
	var b strings.Builder
	if s.Tasks == 0 {
		fmt.Fprintf(&b, "No session could be priced. %d were read but their model is not in the price table.\n", unpriced)
		return b.String()
	}
	fmt.Fprintf(&b, "%s\n\n", costHeaderLine(s.Tasks))
	fmt.Fprintf(&b, "  total          $%.2f\n", s.TotalUSD)
	fmt.Fprintf(&b, "  median task    $%.2f\n", s.MedianUSD)
	fmt.Fprintf(&b, "  p90 task       $%.2f\n", s.P90USD)
	fmt.Fprintf(&b, "  avoidable      $%.2f  (%.0f%% of the total)\n", s.AvoidableUSD, s.AvoidableShare*100)
	fmt.Fprintf(&b, "\nAvoidable is the part nobody chose: tokens re-billed because a prompt cache\nbroke. It is not a forecast of savings, it is what was already spent twice.\n")
	if unpriced > 0 {
		fmt.Fprintf(&b, "\n%d further sessions were read but not priced, because their model is not in\nthe price table. They are excluded rather than counted as free.\n", unpriced)
	}
	return b.String()
}

func runCost(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("cost", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the figures as JSON")
	perTask := fs.Bool("per-task", false, "list every priced session, most expensive first")
	since := fs.String("compare", "", "split at this date (YYYY-MM-DD) and report cost per task before and after")
	predicted := fs.Float64("predicted", 0, "with --compare, the fractional change you predicted (e.g. -0.2 for a 20% saving)")
	if err := fs.Parse(hoistFlagsFor(fs, args)); err != nil {
		return errUsage
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("one or more transcript directories are required: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}

	var units []costUnit
	unpriced := 0
	_ = forEachSession(files, func(_ string, session *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || rep == nil || session == nil {
			return nil
		}
		// The model is a property of the requests, not the session: a session
		// can in principle carry more than one, and the first request is what
		// the report header names.
		model := ""
		if rep.Lane != nil && len(rep.Lane.Requests) > 0 {
			model = rep.Lane.Requests[0].Model
		}
		var asRun analysis.PolicyResult
		for _, p := range rep.Policies() {
			if p.Name == "as-run" {
				asRun = p
			}
		}
		if asRun.CostUSD <= 0 {
			unpriced++
			return nil
		}
		u := costUnit{
			At:       sessionTime(rep),
			ID:       prefixID(session.ID),
			Model:    model,
			Requests: asRun.Requests,
			CostUSD:  asRun.CostUSD,
			Breaks:   len(rep.Breaks),
		}
		// Price only what was demonstrably spent twice. A cache break's deficit
		// is tokens the provider re-billed, which is spend that already
		// happened, not a projection of what a different layout might save.
		if price, ok := cachemodel.PriceFor(model); ok {
			var deficit int
			for _, br := range rep.Breaks {
				deficit += br.Deficit
			}
			u.AvoidableUSD = float64(deficit) / 1_000_000 * price.InputPerMTok
		}
		units = append(units, u)
		return nil
	})

	// The before/after comparison is the only test here that could be
	// contradicted by a provider invoice, which makes it the only one worth
	// much. Everything else measures the engine against its own model.
	if *since != "" {
		cut, err := time.Parse("2006-01-02", *since)
		if err != nil {
			return fmt.Errorf("--compare wants a date like 2026-09-01: %w", err)
		}
		before, after := splitAt(units, cut.UTC())
		_, err = io.WriteString(stdout, renderCompare(compare(before, after), *predicted))
		return err
	}

	s := summarise(units)
	if *asJSON {
		sort.Slice(units, func(i, j int) bool { return units[i].CostUSD > units[j].CostUSD })
		out := map[string]any{"schema": "replay.cost.v1", "summary": s, "unpriced": unpriced}
		if *perTask {
			out["tasks"] = units
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s\n", b)
		return err
	}

	if _, err := io.WriteString(stdout, renderCost(s, unpriced)); err != nil {
		return err
	}
	if *perTask && len(units) > 0 {
		sort.Slice(units, func(i, j int) bool { return units[i].CostUSD > units[j].CostUSD })
		_, _ = fmt.Fprintf(stdout, "\n  %-10s %-24s %8s %10s %10s %7s\n", "session", "model", "requests", "cost", "avoidable", "breaks")
		for _, u := range units {
			_, _ = fmt.Fprintf(stdout, "  %-10s %-24s %8d %10s %10s %7d\n", u.ID, u.Model, u.Requests,
				fmt.Sprintf("$%.2f", u.CostUSD), fmt.Sprintf("$%.2f", u.AvoidableUSD), u.Breaks)
		}
	}
	return nil
}

// sessionTime is when a session ran, taken from its first request.
func sessionTime(rep *analysis.LaneReport) time.Time {
	if rep == nil || rep.Lane == nil || len(rep.Lane.Requests) == 0 {
		return time.Time{}
	}
	return rep.Lane.Requests[0].Timestamp
}

// costHeaderLine names both dated documents, because they are different
// documents that move independently and only one of them sets the money.
// This previously cited the rules version beside a dollar total, which reads
// as the price date: the rules govern what gets cached, the price table
// governs what that costs, and on 2026-09-05 they were 73 days apart.
func costHeaderLine(tasks int) string {
	line := fmt.Sprintf("Cost per task, across %d sessions at list prices dated %s (caching rules %s).",
		tasks, cachemodel.PriceTableVersion, cachemodel.RulesVersionInEffect())
	return line + cachemodel.PriceTableAgeNote(time.Now())
}
