package main

import (
	"flag"
	"fmt"
	"io"
	"os"
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
	// exact is the subset of matched whose read was reproduced EXACTLY, and
	// exceeded is the rest of it: turns where the provider served MORE cached
	// prefix than the model predicted. Both are carried because matched alone
	// cannot be decomposed by a reader, and the published evidence document
	// never broke it out. See analysis.Calibration.ExactRate.
	exact    int
	exceeded int
	breaks   int
	fit      analysis.TokenFit
	source   transcript.Source
	causes   []cachemodel.BreakCause
}

// matchRate is the share of this transcript's compared turns that matched.
//
// Zero when nothing was compared. It returned 1 until 2026-09-06, which
// reported a transcript offering no turn to check as a perfect one and counted
// it as above the calibration threshold in the tally below.
func (r corpusRow) matchRate() float64 {
	if r.compared == 0 {
		return 0
	}
	return float64(r.matched) / float64(r.compared)
}

// exactRate is the share reproduced EXACTLY, with exceeded reads excluded.
//
// Zero when nothing was compared, for the same reason matchRate gives.
func (r corpusRow) exactRate() float64 {
	if r.compared == 0 {
		return 0
	}
	return float64(r.exact) / float64(r.compared)
}

// distinctSessions counts session ids, not transcripts.
//
// A session writes one transcript per lane — the main one plus one per
// subagent — and they all carry the same session id. So a file count is not a
// sample size: on the corpus this was built from, 1450 transcripts carry 78
// session ids, and one session supplies 1020 of 1363 published rows. Turns
// inside one session share a client version, an account, a machine, a project
// and one person's habits, so counting files overstates the independent sample
// by roughly twenty times.
func distinctSessions(rows []corpusRow) int {
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		seen[r.id] = true
	}
	return len(seen)
}

// runCorpus calibrates every session under the given paths and prints a
// Markdown report suitable for committing under docs/reviews.
func runCorpus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("corpus", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// ADR-0007 named this report the unit of contribution and ADR-0007's own
	// implementation note records that `replay corpus --submit` exited with
	// "flag provided but not defined" because this command defined no flags at
	// all. It defines one now, and it writes rather than submits: nothing here
	// transmits, and internal/observation's import allowlist keeps it that way.
	contributeTo := fs.String("contribute", "", "write a calibration report for this campaign: what the provider's caching did, not what it cost you")
	contributeDir := fs.String("contribute-dir", ".", "directory to write the report into")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		// The binary already knows where Claude Code writes. Demanding the
		// path again is the funnel dying at step one.
		home, _ := os.UserHomeDir()
		roots := defaultTranscriptRoots(home)
		if len(roots) == 0 {
			// Say where it looked and what that means, rather than asking for
			// an argument the reader does not have. This is the branch a new
			// user reaches, and a usage error here is the funnel dying at step
			// one, which is the thing the comment above set out to prevent.
			explainNoCorpus(home, stderr)
			// Not an error. A machine that has never run the agent has nothing
			// to report and that is a fact about the machine, not a failure of
			// the command, so the exit status says so.
			return nil
		}
		_, _ = fmt.Fprintf(stderr, "reading %s\n", roots[0])
		args = append(args, roots...)
		if err := parseArgs(fs, args, stdout); err != nil {
			return err
		}
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
			exact:    rep.Calibration.Reproduced,
			exceeded: rep.Calibration.Exceeded,
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
	cals := analysis.ModelCalibrations(reports)
	if *contributeTo != "" {
		path, err := contributeCalibration(*contributeTo, *contributeDir, rows, cals, time.Now())
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stderr, "wrote %s\n", path)
		_, _ = fmt.Fprintf(stderr, "%s", calibrationContributionNote())
	}
	return writeCorpus(stdout, rows, cals, failures)
}

func writeCorpus(w io.Writer, rows []corpusRow, models []analysis.ModelCalibration, failures []string) error {
	p := analysis.NewPrinter(w)
	p.Printf("# Calibration Corpus\n\n")
	p.Printf("How well the replay engine reproduces the provider's cache reads across %d transcripts, "+
		"from %d distinct sessions, found on one machine on %s. **One row is one transcript, not one "+
		"session**: a session writes one per lane, so a session that spawned subagents contributes "+
		"several rows that share its id and its conditions. Rows carry a session id prefix, never a "+
		"path, project name, or content.\n\n",
		len(rows), distinctSessions(rows), time.Now().UTC().Format("2006-01-02"))
	// Matched is split into Exact and Exceeded in the table itself, not only
	// in the totals. An exceeded turn is one the provider served MORE cached
	// prefix for than the model predicted; it is counted as a match because
	// the predicted prefix was served, and it is still a prediction that was
	// wrong. Every row of the published evidence document reported the folded
	// figure alone, so a reader could not tell an exactly reproduced lane from
	// one carried by its siblings.
	p.Printf("| Session | Client | Tier | Requests | Compared | Exact | Exceeded | Matched | Breaks | Exact rate | Match rate | Fit tokens/byte | Fit ±%% |\n")
	p.Printf("|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	totalTurns, totalMatched, totalExact, totalExceeded, totalBreaks, below := 0, 0, 0, 0, 0, 0
	causes := map[cachemodel.BreakCause]int{}
	for _, r := range rows {
		rate := r.matchRate()
		if rate < analysis.CalibrationThreshold {
			below++
		}
		totalTurns += r.compared
		totalMatched += r.matched
		totalExact += r.exact
		totalExceeded += r.exceeded
		totalBreaks += r.breaks
		for _, c := range r.causes {
			causes[c]++
		}
		p.Printf("| %s | %s | %s | %d | %d | %d | %d | %d | %d | %.1f%% | %.1f%% | %.3f | %.0f |\n", r.id, r.client, tierName(r.source), r.requests, r.compared, r.exact, r.exceeded, r.matched, r.breaks, r.exactRate()*100, rate*100, r.fit.TokensPerByte, r.fit.RelativeError*100)
	}
	overall, exactOverall := 0.0, 0.0
	if totalTurns > 0 {
		overall = float64(totalMatched) / float64(totalTurns)
		exactOverall = float64(totalExact) / float64(totalTurns)
	}
	sessions := distinctSessions(rows)
	p.Printf("\n## Totals\n\n")
	p.Printf("- Transcripts: %d (%d below the %.0f%% threshold)\n", len(rows), below, analysis.CalibrationThreshold*100)
	p.Printf("- Sessions: %d\n", sessions)
	if len(rows) > sessions {
		p.Printf("  A session writes one transcript per lane, so these differ whenever subagents ran.\n" +
			"  Sessions is the independent count: turns inside one session share a client version,\n" +
			"  an account, a machine and one person's habits, and are not independent draws.\n")
	}
	p.Printf("- Compared turns: %d, matched: %d, breaks: %d\n", totalTurns, totalMatched, totalBreaks)
	// The decomposition, stated rather than left for the reader to subtract.
	// Until 2026-09-11 this report printed only the line above, and the
	// published evidence document inherited that: it gave a matched total and
	// never said how much of it was exact.
	p.Printf("  Of those matched, reproduced exactly: %d; read more than predicted: %d\n", totalExact, totalExceeded)
	if totalExceeded > 0 {
		p.Printf("  A read larger than predicted usually means a concurrent sibling lane extended the\n" +
			"  prefix. It is counted as a match because the provider served at least what was\n" +
			"  predicted, and it is still a prediction that was wrong, so it is NOT counted as exact.\n")
	}
	p.Printf("- Overall match rate: %.2f%% (exact reproduction rate: %.2f%%)", overall*100, exactOverall*100)
	if totalTurns == 0 {
		p.Printf(" (nothing was compared, so this is an absence rather than a result)")
	}
	p.Printf("\n")
	if sessions < corpusTarget {
		p.Printf("- Fewer than %d sessions: the roadmap gate for spikes 1 and 2 is not met by this corpus alone\n", corpusTarget)
	}

	p.Printf("\n## Per model\n\n")
	p.Printf("Calibration by the model of each session's first request, with the newest %d sessions judged on their own so a provider rule change shows as a drop (ST-1). The minimum cacheable prefix is bounded from usage: the largest uncached prompt lies below it, the smallest cached prefix at or above it.\n\n", analysis.StalenessRecentSessions)
	p.Printf("| Model | Sessions | Exact rate | Match rate | Recent sessions | Recent turns | Recent exact rate | Recent match rate | Verdict |\n")
	p.Printf("|---|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, m := range models {
		// A model with nothing compared is not a model that scored badly, and
		// printing a percentage for it says it was measured. `<synthetic>` is
		// the case that forced this: it is Claude Code's label for assistant
		// messages generated locally that never reached the API, so its turns
		// carry zero usage and none of them can be compared. It published as
		// 100.0% "calibrated" until 2026-09-08. Now the row says which of the
		// two it is, and the percentage is only printed where a comparison
		// actually happened.
		verdict := "calibrated"
		switch {
		case m.Compared == 0:
			verdict = "not measured: no turn to compare"
		case m.Stale:
			verdict = "stale: provider behavior changed"
		case m.MatchRate() < analysis.CalibrationThreshold:
			verdict = "below threshold"
		}
		// The recent window's denominator is printed, not just its rates.
		//
		// Without it the table reports 87.5% for a lane of EIGHT turns beside
		// 93.3% for a lane of 539, and marks both calibrated. Seven of eight is
		// exactly 87.5%, and a reader cannot tell that from the rate alone.
		//
		// This does not fix the gate, and is not meant to. CalibrationThreshold
		// has no minimum sample size, so a lane clears 95% on eight turns as
		// easily as on eight hundred — splitting exact from match leaves that
		// untouched. Showing n is the part that can be done without changing
		// which lanes pass, which is a decision for whoever owns the threshold.
		p.Printf("| %s | %d | %s | %s | %d | %d | %s | %s | %s |\n", m.Model, m.Sessions,
			matchRateCell(m.Exact, m.Compared), matchRateCell(m.Matched, m.Compared),
			m.RecentSessions, m.RecentCompared,
			matchRateCell(m.RecentExact, m.RecentCompared), matchRateCell(m.RecentMatched, m.RecentCompared),
			verdict)
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

// scrubPath keeps an error message useful without leaking a directory or a
// session identifier.
//
// Dropping the directory is not enough. Claude Code names a transcript after
// its session UUID, so the bare filename is still an identifier, and this list
// is part of the report ADR-0007 designed for submission. Nothing transmits it:
// `corpus` defines no flags and no submit path was built. Analysed rows are
// already shortened by prefixID; skipped ones were not, which quietly
// contradicted this report's own promise of "a session id prefix, never a
// path".
func scrubPath(msg string) string {
	if i := strings.LastIndex(msg, "/"); i >= 0 {
		msg = msg[i+1:]
	}
	if i := strings.Index(msg, ".jsonl"); i > 0 {
		return prefixID(msg[:i]) + msg[i+len(".jsonl"):]
	}
	return msg
}

// matchRateCell renders a match rate, or says there was nothing to rate.
//
// The distinction the table has to keep is between a model that was measured
// and matched badly and a model that was never measured at all. Both used to
// print as a percentage, which made the second one indistinguishable from the
// first at a glance and, while rate() returned 1 for an empty comparison, made
// it indistinguishable from a perfect score.
//
// "no evidence" is the same phrase MinPrefixFit already uses for the same
// condition in the notes below this table, so the two halves of the report
// describe an absent measurement the same way.
func matchRateCell(matched, compared int) string {
	if compared == 0 {
		return "no evidence"
	}
	return fmt.Sprintf("%.1f%%", float64(matched)/float64(compared)*100)
}
