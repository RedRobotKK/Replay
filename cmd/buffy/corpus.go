package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Buffy/internal/analysis"
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
	tier     analysis.Tier
	causes   []analysis.BreakCause
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
	for _, f := range files {
		session, err := loadSession(f)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		lane := analysis.MainLane(session)
		if lane == nil {
			failures = append(failures, "no requests")
			continue
		}
		rep := analysis.AnalyzeLane(session, lane)
		row := corpusRow{
			id:       prefixID(session.ID),
			client:   session.ClientVersion,
			requests: len(lane.Requests),
			compared: rep.Calibration.Compared(),
			matched:  rep.Calibration.Reproduced + rep.Calibration.Exceeded,
			breaks:   rep.Calibration.Broken,
			fit:      rep.Fit,
			tier:     rep.Tier,
		}
		for _, b := range rep.Breaks {
			row.causes = append(row.causes, b.Cause)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return fmt.Errorf("no session could be analyzed (%d failures)", len(failures))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].requests > rows[j].requests })
	return writeCorpus(stdout, rows, failures)
}

func writeCorpus(w io.Writer, rows []corpusRow, failures []string) error {
	p := &corpusPrinter{w: w}
	p.printf("# Calibration Corpus\n\n")
	p.printf("How well the replay engine reproduces the provider's cache reads across %d sessions found on one machine on %s. Rows carry a session id prefix, never a path, project name, or content.\n\n", len(rows), time.Now().UTC().Format("2006-01-02"))
	p.printf("| Session | Client | Tier | Requests | Compared | Matched | Breaks | Match rate | Fit tokens/byte | Fit ±%% |\n")
	p.printf("|---|---|---|---:|---:|---:|---:|---:|---:|---:|\n")
	totalTurns, totalMatched, totalBreaks, below := 0, 0, 0, 0
	causes := map[analysis.BreakCause]int{}
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
		tier := "estimated"
		if r.tier == analysis.TierMeasured {
			tier = "measured"
		}
		p.printf("| %s | %s | %s | %d | %d | %d | %d | %.1f%% | %.3f | %.0f |\n", r.id, r.client, tier, r.requests, r.compared, r.matched, r.breaks, rate*100, r.fit.TokensPerByte, r.fit.RelativeError*100)
	}
	overall := 1.0
	if totalTurns > 0 {
		overall = float64(totalMatched) / float64(totalTurns)
	}
	p.printf("\n## Totals\n\n")
	p.printf("- Sessions: %d (%d below the %.0f%% threshold)\n", len(rows), below, analysis.CalibrationThreshold*100)
	p.printf("- Compared turns: %d, matched: %d, breaks: %d\n", totalTurns, totalMatched, totalBreaks)
	p.printf("- Overall match rate: %.2f%%\n", overall*100)
	if len(rows) < corpusTarget {
		p.printf("- Fewer than %d sessions: the roadmap gate for spikes 1 and 2 is not met by this corpus alone\n", corpusTarget)
	}

	p.printf("\n## Break causes\n\n")
	if len(causes) == 0 {
		p.printf("No cache breaks in any session.\n")
	} else {
		p.printf("| Cause | Count |\n|---|---:|\n")
		keys := make([]string, 0, len(causes))
		for c := range causes {
			keys = append(keys, string(c))
		}
		sort.Slice(keys, func(i, j int) bool {
			ci, cj := causes[analysis.BreakCause(keys[i])], causes[analysis.BreakCause(keys[j])]
			if ci != cj {
				return ci > cj
			}
			return keys[i] < keys[j]
		})
		for _, k := range keys {
			p.printf("| %s | %d |\n", k, causes[analysis.BreakCause(k)])
		}
	}

	p.printf("\n## Sessions not analyzed\n\n")
	if len(failures) == 0 {
		p.printf("None.\n")
	}
	for _, f := range failures {
		p.printf("- %s\n", scrubPath(f))
	}
	return p.err
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

type corpusPrinter struct {
	w   io.Writer
	err error
}

func (p *corpusPrinter) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}
