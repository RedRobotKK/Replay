package main

import (
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

// sessionIDPrefixLen is how much of a session id the corpus report shows:
// enough to tell sessions apart, not enough to look one up.
const sessionIDPrefixLen = 8

// corpusRow is one session's calibration summary. It carries no path, no
// project name, and no content.
type corpusRow struct {
	id       string
	client   string
	requests int
	compared int
	matched  int
	breaks   int
	fit      analysis.TokenFit
	source   transcript.Source
	causes   []cachemodel.BreakCause
}

func (r corpusRow) matchRate() float64 {
	if r.compared == 0 {
		return 1
	}
	return float64(r.matched) / float64(r.compared)
}

// runCorpus calibrates every session under the given paths and prints a
// Markdown report suitable for committing under docs/reviews.
func runCorpus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("corpus", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("one or more transcript directories are required: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}

	var rows []corpusRow
	var failures []string
	// The visitor never fails, so the walk cannot either.
	var reports []*analysis.LaneReport
	_ = forEachSession(files, func(_ string, session *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil {
			failures = append(failures, err.Error())
			return nil
		}
		reports = append(reports, rep)
		row := corpusRow{
			id:       prefixID(session.ID),
			client:   session.ClientVersion,
			requests: len(rep.Lane.Requests),
			compared: rep.Calibration.Compared(),
			matched:  rep.Calibration.Reproduced + rep.Calibration.Exceeded,
			breaks:   rep.Calibration.Broken,
			fit:      rep.Fit,
			source:   session.Source,
		}
		for _, b := range rep.Breaks {
			row.causes = append(row.causes, b.Cause)
		}
		rows = append(rows, row)
		return nil
	})
	if len(rows) == 0 {
		return fmt.Errorf("no session could be analyzed (%d failures)", len(failures))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].requests > rows[j].requests })
	return writeCorpus(stdout, rows, analysis.ModelCalibrations(reports), failures)
}

func writeCorpus(w io.Writer, rows []corpusRow, models []analysis.ModelCalibration, failures []string) error {
	p := analysis.NewPrinter(w)
	p.Printf("# Calibration Corpus\n\n")
	p.Printf("How well the replay engine reproduces the provider's cache reads across %d sessions found on one machine on %s. Rows carry a session id prefix, never a path, project name, or content.\n\n", len(rows), time.Now().UTC().Format("2006-01-02"))
	p.Printf("| Session | Client | Tier | Requests | Compared | Matched | Breaks | Match rate | Fit tokens/byte | Fit ±%% |\n")
	p.Printf("|---|---|---|---:|---:|---:|---:|---:|---:|---:|\n")
	totalTurns, totalMatched, totalBreaks, below := 0, 0, 0, 0
	causes := map[cachemodel.BreakCause]int{}
	for _, r := range rows {
		rate := r.matchRate()
		if rate < analysis.CalibrationThreshold {
			below++
		}
		totalTurns += r.compared
		totalMatched += r.matched
		totalBreaks += r.breaks
		for _, c := range r.causes {
			causes[c]++
		}
		p.Printf("| %s | %s | %s | %d | %d | %d | %d | %.1f%% | %.3f | %.0f |\n", r.id, r.client, tierName(r.source), r.requests, r.compared, r.matched, r.breaks, rate*100, r.fit.TokensPerByte, r.fit.RelativeError*100)
	}
	overall := 1.0
	if totalTurns > 0 {
		overall = float64(totalMatched) / float64(totalTurns)
	}
	p.Printf("\n## Totals\n\n")
	p.Printf("- Sessions: %d (%d below the %.0f%% threshold)\n", len(rows), below, analysis.CalibrationThreshold*100)
	p.Printf("- Compared turns: %d, matched: %d, breaks: %d\n", totalTurns, totalMatched, totalBreaks)
	p.Printf("- Overall match rate: %.2f%%\n", overall*100)
	if len(rows) < corpusTarget {
		p.Printf("- Fewer than %d sessions: the roadmap gate for spikes 1 and 2 is not met by this corpus alone\n", corpusTarget)
	}

	p.Printf("\n## Per model\n\n")
	p.Printf("Calibration by the model of each session's first request, with the newest %d sessions judged on their own so a provider rule change shows as a drop (ST-1). The minimum cacheable prefix is bounded from usage: the largest uncached prompt lies below it, the smallest cached prefix at or above it.\n\n", analysis.StalenessRecentSessions)
	p.Printf("| Model | Sessions | Match rate | Recent sessions | Recent match rate | Verdict |\n")
	p.Printf("|---|---:|---:|---:|---:|---|\n")
	for _, m := range models {
		verdict := "calibrated"
		switch {
		case m.Stale:
			verdict = "stale: provider behavior changed"
		case m.MatchRate() < analysis.CalibrationThreshold:
			verdict = "below threshold"
		}
		p.Printf("| %s | %d | %.1f%% | %d | %.1f%% | %s |\n", m.Model, m.Sessions, m.MatchRate()*100, m.RecentSessions, m.RecentMatchRate()*100, verdict)
	}
	for _, m := range models {
		p.Printf("\n- %s: %s", m.Model, m.MinPrefix)
		if m.MinPrefix.Disagrees() {
			p.Printf("; the rules file disagrees with the observations")
		}
		if m.Stale {
			p.Printf("\n  %s", m.Reason)
		}
	}
	if len(models) > 0 {
		p.Printf("\n")
	}

	p.Printf("\n## Break causes\n\n")
	if len(causes) == 0 {
		p.Printf("No cache breaks in any session.\n")
	} else {
		p.Printf("| Cause | Count |\n|---|---:|\n")
		keys := make([]cachemodel.BreakCause, 0, len(causes))
		for c := range causes {
			keys = append(keys, c)
		}
		sort.Slice(keys, func(i, j int) bool {
			if causes[keys[i]] != causes[keys[j]] {
				return causes[keys[i]] > causes[keys[j]]
			}
			return keys[i] < keys[j]
		})
		for _, k := range keys {
			p.Printf("| %s | %d |\n", k, causes[k])
		}
	}

	p.Printf("\n## Sessions not analyzed\n\n")
	if len(failures) == 0 {
		p.Printf("None.\n")
	}
	for _, f := range failures {
		p.Printf("- %s\n", scrubPath(f))
	}
	return p.Err()
}

// tierName is the one-word tier for a table cell.
func tierName(src transcript.Source) string {
	if src.PrefixVisible() {
		return "measured"
	}
	return "estimated"
}

// corpusTarget is the session count the roadmap gate asks for.
const corpusTarget = 20

func prefixID(id string) string {
	if len(id) > sessionIDPrefixLen {
		return id[:sessionIDPrefixLen]
	}
	return id
}

// scrubPath keeps an error message useful without leaking a directory.
func scrubPath(msg string) string {
	if i := strings.LastIndex(msg, "/"); i >= 0 {
		return msg[i+1:]
	}
	return msg
}
