package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/card"
	"github.com/RedRobotKK/Replay/internal/money"
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
//
// It was not, until 2026-09-07. Claude Code writes one transcript per agent
// LANE — a session that fanned out to sub-agents writes
// <session>/subagents/agent-*.jsonl, and every one of those files carries the
// parent's sessionId — and this command priced one file at a time. So a row
// labelled `session` was a lane, and `--per-task --json` reported 1614 "tasks"
// over 114 distinct sessions with one id on 1014 separate rows.
//
// The same conflation was published once and retracted: docs/evidence/README.md
// records a figure of "1363 sessions" that was a file count, one session
// supplying 1020 of them. It was corrected in the calibration corpus and left
// live here. foldSessions is the correction; --per-lane is the lane view, kept
// because fan-out analysis genuinely needs it.

// unitSession and unitLane name what one row of this report is. They are
// printed, and they appear in the JSON as summary.unit, because a reader who
// cannot tell which one they are holding is the reader this file exists for.
const (
	unitSession = "session"
	unitLane    = "lane"
)

type costUnit struct {
	ID string `json:"session"`
	// Lane names the agent lane this row was priced from, and is set on every
	// unit as it is built: "main" for the session's own transcript, the agent
	// id for a sub-agent lane. It survives into a --per-lane row and is
	// cleared when lanes are folded into a session, because a session is not
	// one lane. It is a label, not a key — eight hex characters of an agent id
	// are enough to tell two rows apart on screen and not enough to promise
	// uniqueness across thousands of them.
	Lane string `json:"lane,omitempty"`
	// Lanes is how many agent lanes were folded into this row. Present only on
	// a session row, where it is the fan-out, and it is the number that makes
	// the difference between the two units visible instead of inferred.
	Lanes           int     `json:"lanes,omitempty"`
	Model           string  `json:"model"`
	Requests        int     `json:"requests"`
	CostUSD         float64 `json:"costUsd"`
	AvoidableUSD    float64 `json:"avoidableUsd"`
	AvoidableTokens int     `json:"avoidableTokens,omitempty"`
	Breaks          int     `json:"breaks"`
	// Repeated and Errored are the waste distribution ADR-0009 asks the corpus
	// to carry: content-free counts, additive across lanes like Breaks.
	Repeated int       `json:"repeated,omitempty"`
	Errored  int       `json:"errored,omitempty"`
	At       time.Time `json:"at"`
}

// costLaneRow is a --per-lane row on its way to JSON.
//
// Its id field is `ofSession`, never `session`. Several lanes share one
// session id by construction, so a field named `session` on a row that is not
// one is precisely the defect this file was opened to fix; naming it as the
// parent link it actually is means a consumer that groups on `session` cannot
// accidentally be handed lanes. The lane figures themselves are copied
// unchanged — this is a renaming, not a second calculation.
type costLaneRow struct {
	Lane            string    `json:"lane,omitempty"`
	OfSession       string    `json:"ofSession"`
	Model           string    `json:"model"`
	Requests        int       `json:"requests"`
	CostUSD         float64   `json:"costUsd"`
	AvoidableUSD    float64   `json:"avoidableUsd"`
	AvoidableTokens int       `json:"avoidableTokens,omitempty"`
	Breaks          int       `json:"breaks"`
	At              time.Time `json:"at"`
}

func laneRows(units []costUnit) []costLaneRow {
	rows := make([]costLaneRow, 0, len(units))
	for _, u := range units {
		rows = append(rows, costLaneRow{Lane: u.Lane, OfSession: u.ID, Model: u.Model,
			Requests: u.Requests, CostUSD: u.CostUSD, AvoidableUSD: u.AvoidableUSD,
			AvoidableTokens: u.AvoidableTokens, Breaks: u.Breaks, At: u.At})
	}
	return rows
}

// laneID names one agent lane from the transcript it was priced from.
//
// Claude Code writes a sub-agent lane to <session>/subagents/agent-<id>.jsonl,
// so the file's own name carries the only lane identity available; the parent
// lane's file is named after the session, and repeating the session id in a
// lane column would say nothing, so it is called "main".
func laneID(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	if rest, ok := strings.CutPrefix(name, "agent-"); ok {
		return prefixID(rest)
	}
	return unitMain
}

const unitMain = "main"

// foldSessions turns lane rows into session rows: one row, one session.
//
// Which fields may be summed was checked one at a time, because the fast way
// to get an aggregation wrong is to sum something that is not a quantity:
//
//	requests, breaks, avoidableTokens  counts of events within one lane. Additive.
//	costUsd, avoidableUsd              money already spent on that lane. Additive.
//	at                                 a point in time, not a quantity. A session
//	                                   began when its earliest lane began, so this
//	                                   is a minimum, never a sum and never a mean.
//	model                              a category. Neither summable nor averageable.
//	                                   The row names the model that ran the largest
//	                                   share of the money, which is the one a
//	                                   routing decision is about; see runCost for
//	                                   why the route line is still derived from
//	                                   every lane rather than from these rows.
//
// No rate or percentile passes through here. avoidableShare, the median and
// the p90 are all derived in summarise from the rows this returns, so folding
// cannot silently average an average.
//
// The sums carry a known overlap and do not create it. A sub-agent lane
// re-renders some of its parent's requests, so a request id can be present in
// two of a session's files and is priced in both — measured over this corpus,
// 430 of 32,559 ids, 1.3%. That residue is already disclosed as
// duplicatedRequests, and it was already inside the corpus total, which sums
// every lane. Folding changes which row a figure is shown on, not what is in
// the figure: the grand total is identical before and after.
// errorShare is errored requests over requests, or nothing.
//
// A share over no requests is a division, not a measurement. Returning 0.0
// would tell a pooled figure "this corpus had no errors" when what happened is
// that nothing was counted, and ADR-0018 is that absence and zero are different
// values. It is a function rather than three lines at the call site because a
// decision inlined into a caller cannot be tested at the boundary where it is
// wrong — ADR-0014.
func errorShare(errored, requests int) *float64 {
	if requests <= 0 {
		return nil
	}
	share := float64(errored) / float64(requests)
	return &share
}

func foldSessions(units []costUnit) []costUnit {
	var order []string
	by := map[string]*costUnit{}
	// The cost of the lane whose model each row currently names, so a later,
	// larger lane can take the column over.
	dominant := map[string]float64{}
	for _, u := range units {
		s, ok := by[u.ID]
		if !ok {
			row := u
			// A session is not one lane, so it does not carry one lane's id.
			row.Lane = ""
			row.Lanes = 1
			by[u.ID] = &row
			order = append(order, u.ID)
			dominant[u.ID] = u.CostUSD
			continue
		}
		s.Lanes++
		s.Requests += u.Requests
		s.CostUSD += u.CostUSD
		s.AvoidableUSD += u.AvoidableUSD
		s.AvoidableTokens += u.AvoidableTokens
		s.Breaks += u.Breaks
		s.Repeated += u.Repeated
		s.Errored += u.Errored
		// Earliest, not first-seen: files arrive in directory order, which is
		// not time order.
		if !u.At.IsZero() && (s.At.IsZero() || u.At.Before(s.At)) {
			s.At = u.At
		}
		if u.CostUSD > dominant[u.ID] {
			dominant[u.ID] = u.CostUSD
			s.Model = u.Model
		}
	}
	// First-seen order, so the fold is deterministic before the callers sort.
	out := make([]costUnit, 0, len(order))
	for _, id := range order {
		out = append(out, *by[id])
	}
	return out
}

type costSummary struct {
	// Tasks counts the rows, whatever the rows are. Unit says which they are.
	//
	// The two travel together deliberately. This count was a file count under
	// a name that reads as a count of work, which is the whole defect; a
	// consumer that reads Tasks without reading Unit is making the same
	// mistake in their own code, and Unit is there so that they cannot say
	// they were not told.
	Tasks int    `json:"tasks"`
	Unit  string `json:"unit"`
	// Lanes is the agent-lane count behind those rows — the transcript files
	// actually read and priced. On a session-unit report it is the larger
	// number and printing both is what stops either being mistaken for the
	// other. On a lane-unit report it equals Tasks.
	Lanes          int     `json:"lanes"`
	TotalUSD       float64 `json:"totalUsd"`
	MedianUSD      float64 `json:"medianUsd"`
	P90USD         float64 `json:"p90Usd"`
	AvoidableUSD   float64 `json:"avoidableUsd"`
	AvoidableShare float64 `json:"avoidableShare"`
	// AvoidableTokens is the same waste before it is multiplied by a price.
	//
	// The dollar figure is meaningless to a flat-seat subscriber, who is not
	// billed per token and is most of the readership. The tokens are what they
	// actually lost, and the token count is where this stops: what those
	// tokens go on to cost such a reader is a quota question, and the one
	// measurement of it came back null (README.md:228-235). The deficit was
	// always in tokens first, and in tokens is where it can be stated.
	AvoidableTokens int `json:"avoidableTokens,omitempty"`
	// Which route the traffic took. A category, never an id — see
	// namespace.go for why the billing mode is only claimed where the model
	// id actually settles it.
	Route string `json:"route,omitempty"`
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
	models := make([]string, 0, len(units))
	for _, u := range units {
		models = append(models, u.Model)
	}
	s.Route = routeLine(models)
	for _, u := range units {
		s.AvoidableTokens += u.AvoidableTokens
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

func renderCost(s costSummary, unpriced int, out io.Writer, stateDir string) string {
	var b strings.Builder
	if s.Tasks == 0 {
		fmt.Fprintf(&b, "No transcript could be priced. %d were read but their model is not in the price table.\n", unpriced)
		return b.String()
	}
	fmt.Fprintf(&b, "%s\n\n", costHeaderLine(s))
	// The reader's own currency, beside the dollars and never instead of them.
	//
	// A reader in Tokyo should not have to do arithmetic to know whether a
	// number is large. They also must not be handed a yen figure as their
	// cost: the provider bills dollars, and a card issuer converts at its own
	// rate on its settlement date and adds a foreign transaction fee. The
	// conversion is an indication of size, and the column header, the rate,
	// its date and the note under the block all say so.
	fx := money.Detect(os.LookupEnv, time.Now())
	fmt.Fprintf(&b, "  total          %s\n", fxCol(fx, s.TotalUSD))
	// "task" only where a row is a task. Under --per-lane the same two figures
	// describe agent lanes, and calling a lane a task on the line beneath a
	// header that just said "lanes" is how one word came to mean two things
	// here in the first place.
	noun := "task"
	if s.Unit == unitLane {
		noun = "lane"
	}
	fmt.Fprintf(&b, "  median %-8s%s\n", noun, fxCol(fx, s.MedianUSD))
	fmt.Fprintf(&b, "  p90 %-11s%s\n", noun, fxCol(fx, s.P90USD))
	fmt.Fprintf(&b, "  avoidable      %s  (%.0f%% of the total)\n", fxCol(fx, s.AvoidableUSD), s.AvoidableShare*100)
	if s.AvoidableTokens > 0 {
		fmt.Fprintf(&b, "                 %s tokens re-billed\n", shortTokens(s.AvoidableTokens))
	}
	if n := fx.Note(); n != "" {
		fmt.Fprintf(&b, "\n%s\n", wrapAt(n, 78, ""))
	}
	fmt.Fprintf(&b, "\nAvoidable is the part nobody chose: tokens re-billed because a prompt cache\nbroke. It is not a forecast of savings, it is what was already spent twice.\n")
	if s.AvoidableTokens > 0 {
		// What this paragraph may and may not assert.
		//
		// It may say what re-billed tokens ARE: that is arithmetic from the
		// transcript. It may not say what they COST a subscriber, because that
		// is a quota measurement, and the only one anyone has run came back
		// null — matched cold-write and warm-read arms, 3.09M tokens, and the
		// utilisation counter moved zero steps (README.md:228-235).
		//
		// Two claims were removed here. "Rate-limit budget spent on nothing"
		// asserted on the reader's screen exactly what the README refuses to
		// assert two clicks away. "Context the work did not get" was wrong on
		// its own terms: a broken cache changes what a prompt is billed, not
		// what it contains, so the work got the context either way.
		//
		// Saying nothing was the other option and it is worse. A subscriber's
		// whole reason to care about re-billed tokens is what they cost them,
		// so leaving the question out invites them to assume an answer. The
		// null is more useful than the silence and more honest than the claim.
		fmt.Fprintf(&b, "\nOn a subscription seat - Claude Pro or Max, Copilot, Cursor - none of that is\n"+
			"money: you are not billed per token, so the dollars above are list price for\n"+
			"someone who is. The tokens are still yours.\n\n"+
			"What they cost you instead is not established. Whether a re-billed token draws\n"+
			"down a rate-limit window was measured here across 3.09M tokens and the\n"+
			"utilisation counter did not move: a null result, not a saving. `replay advise`\n"+
			"ranks what to cut by token count, which is the part that was measured.\n")
	}
	if unpriced > 0 {
		fmt.Fprintf(&b, "\n%d further transcripts were read but not priced, because their model is not in\nthe price table. They are excluded rather than counted as free.\n", unpriced)
	}
	// tipLine names a coffee count and returns nothing below its floor, so a
	// modest corpus produced a result and no ask at all. Below the floor the
	// ask still belongs; it just cannot quote a share of a figure this small.
	// The ask is rate-limited to once a month per machine, because reciprocity
	// is what makes it work and repetition is what destroys it. stateDir empty
	// means "no state available": ask, rather than fall silent.
	arm := "A"
	ask := true
	if stateDir != "" {
		arm = tipVariant(tipSeed(stateDir))
		ask = shouldAsk(stateDir, s.AvoidableUSD, time.Now())
	}
	if tip := tipLineArm(arm, s.AvoidableUSD, canHyperlink(out)); ask && tip != "" {
		if stateDir != "" {
			noteAsked(stateDir, s.AvoidableUSD, time.Now())
		}
		b.WriteString(tip)
	} else {
		b.WriteString(supportLine(describeResult("cost"), out))
	}
	return b.String()
}

// tipStateDir is where the ask remembers itself, or "" when there is no home
// to remember it in.
func tipStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".replay")
}

func runCost(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("cost", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the figures as JSON")
	perTask := fs.Bool("per-task", false, "list every priced session, most expensive first")
	perLane := fs.Bool("per-lane", false, "report agent lanes instead of sessions: a session that spawned sub-agents wrote one transcript per lane, and this is the fan-out view of them")
	since := fs.String("compare", "", "split at this date (YYYY-MM-DD) and report cost per task before and after")
	predicted := fs.Float64("predicted", 0, "with --compare, the fractional change you predicted (e.g. -0.2 for a 20% saving)")
	maxAvoidable := fs.Float64("max-avoidable-usd", 0,
		"fail the build when measured avoidable spend exceeds this many dollars (0 = off). "+
			"Refuses to pass when nothing was priced")
	share := fs.Bool("share", false, "print a paste-ready summary: the avoidable rate and the task spread, with no spend total, no paths and no project names")
	png := fs.String("png", "", "with --share, also write the same figures as a 1200x630 social card at this path")
	design := fs.String("card", "", "which card design --png writes: b (the dark receipt) or c (the paper statement, the default)")
	tone := fs.String("tone", "", "the register the card is written in: measured (what was found, stated, the default) or rekt (the same figures, exact and deadpan)")
	contributeTo := fs.String("contribute", "", "build a corpus submission for this campaign from the figures below; writes a file, sends nothing. Unlike the probe submission, this one CARRIES SPEND")
	contributeDir := fs.String("contribute-dir", ".", "where --contribute writes its file")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	// Resolved before any transcript is read, so a typo in --card costs
	// nothing and is reported as what it is rather than after a full scan.
	variant, err := pickVariant(*design)
	if err != nil {
		return err
	}
	// Same reasoning as --card: resolved before any transcript is read, so a
	// typo costs nothing and is reported as what it is rather than after a
	// full scan.
	register, err := card.ParseTone(*tone)
	if err != nil {
		return err
	}
	if *png != "" && !*share {
		// A second way to produce the card would be a second way to produce it
		// without the guard: the check for whether anything was measured well
		// enough to stand behind lives on the --share path.
		return fmt.Errorf("--png writes the share card, so it needs --share as well")
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
			//
			// Unless a ceiling was asked for. Passing --max-avoidable-usd is a
			// request to assert that spend is under a number, and that cannot
			// be asserted over nothing: a CI runner has no transcripts, so a
			// gate that stayed silent here would go green having measured
			// nothing, which is the failure this flag exists to prevent.
			return checkAvoidableCeiling(*maxAvoidable, costSummary{Unit: unitSession}, 0, stdout)
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

	// Requests seen in an earlier transcript. A sub-agent lane re-renders its
	// parent's requests, so the same requestId can appear in several files;
	// MainLane skips sidechains and absorbs most of that, and the residue is
	// disclosed rather than silently carried.
	seenReq := map[string]bool{}
	duplicated, totalReq := 0, 0

	// An index over transcripts already understood. Transcripts are
	// append-only and most never change again, so re-deriving all of them to
	// learn about the few that grew is a full scan of a table that wanted an
	// index. Keyed on the price table and rules version as well as file
	// identity: a figure priced from a table that has since moved is a wrong
	// number that arrives fast.
	cache := newCostCache(filepath.Join(tipStateDir(), "cost-index.json"), costIndexKey())
	_ = cache.load()
	var cold []string
	var units []costUnit
	for _, f := range files {
		if u, ids, ok := cache.get(f); ok {
			units = append(units, u)
			for _, id := range ids {
				totalReq++
				if seenReq[id] {
					duplicated++
					continue
				}
				seenReq[id] = true
			}
			continue
		}
		cold = append(cold, f)
	}
	warm := len(units)
	files = cold
	unpriced := 0
	_ = forEachSession(files, func(path string, session *transcript.Session, rep *analysis.LaneReport, err error) error {
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
		var reqIDs []string
		if rep.Lane != nil {
			for _, r := range rep.Lane.Requests {
				if r.ID == "" {
					continue
				}
				reqIDs = append(reqIDs, r.ID)
				totalReq++
				if seenReq[r.ID] {
					duplicated++
					continue
				}
				seenReq[r.ID] = true
			}
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
			Lane:     laneID(path),
			Model:    model,
			Requests: asRun.Requests,
			CostUSD:  asRun.CostUSD,
			Breaks:   len(rep.Breaks),
			Repeated: rep.ReReads.Repeated,
		}
		for _, e := range rep.Errors {
			u.Errored += e.Count
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
			u.AvoidableTokens = deficit
		}
		units = append(units, u)
		cache.put(path, u, reqIDs)
		return nil
	})

	// One row, one session — and one unit for the whole report.
	//
	// The fold happens here, above every consumer, so that the compare split,
	// the summary, the share card, the JSON rows and the printed table are all
	// counting the same thing. A report that folded only the rows it printed
	// would still say "tasks" over a lane count in the line above them, which
	// is the defect wearing a smaller hat.
	//
	// --per-lane moves that single unit down to the agent lane. It does not
	// add a second unit alongside the first: whichever one is in force, every
	// figure below is in it.
	lanesRead := len(units)
	laneUnits := units
	unit := unitSession
	if *perLane {
		unit = unitLane
	} else {
		units = foldSessions(units)
	}

	// The before/after comparison is the only test here that could be
	// contradicted by a provider invoice, which makes it the only one worth
	// much. Everything else measures the engine against its own model.
	if *since != "" {
		cut, err := time.Parse("2006-01-02", *since)
		if err != nil {
			return fmt.Errorf("--compare wants a date like 2026-09-01: %w", err)
		}
		before, after := splitAt(units, cut.UTC())
		_, err = io.WriteString(stdout, renderCompare(compare(before, after, unit), *predicted))
		return err
	}

	s := summarise(units)
	gateCeiling := *maxAvoidable
	s.Unit, s.Lanes = unit, lanesRead
	// The route is the one summary field that must not be derived from the
	// rows.
	//
	// routeLine reports the SET of routes the traffic took, and a set shrinks
	// when its members are folded together: a session whose parent lane ran
	// first-party and whose sub-agent lane ran on Bedrock collapses to one
	// model, and "Bedrock, metered" would vanish from a card that gets posted
	// publicly. Route is a statement about requests, not about rows, so it is
	// computed over every lane whatever the row unit is.
	models := make([]string, 0, len(laneUnits))
	for _, u := range laneUnits {
		models = append(models, u.Model)
	}
	s.Route = routeLine(models)

	if err := cache.save(); err != nil {
		// A slow next run is the whole consequence, so it is mentioned and
		// never fatal.
		_, _ = fmt.Fprintf(stderr, "note: could not write the transcript index (%v); the next run "+
			"will be a full scan\n", err)
	}
	if warm > 0 {
		_, _ = fmt.Fprintf(stderr, "%d transcript(s) reused from the index, %d re-read\n", warm, len(files))
	}

	// The submission is built from the summary just computed, never recomputed,
	// so what is pooled is what the contributor's own terminal printed. It runs
	// before every rendering branch because --share short-circuits them, and a
	// contribution that silently did not happen under one flag combination is
	// the kind of absence nobody notices until the pool is short.
	contribution, supersedes := "", []string(nil)
	if *contributeTo != "" {
		// The waste distribution is totalled from the same units the summary
		// above was reduced from, never recomputed, for the reason
		// contributeCorpus states: a second implementation of the arithmetic is
		// free to disagree with the one on the contributor's screen.
		breaks, repeated, errored, requests := 0, 0, 0, 0
		for _, u := range units {
			breaks += u.Breaks
			repeated += u.Repeated
			errored += u.Errored
			requests += u.Requests
		}
		f := corpusFigures{
			Tasks: s.Tasks, Unpriced: unpriced, TotalUSD: s.TotalUSD,
			AvoidableUSD: s.AvoidableUSD, AvoidableShare: s.AvoidableShare,
			MedianTaskUSD: s.MedianUSD,
			CacheBreaks:   &breaks,
			ReReads:       &repeated,
		}
		f.ErrorShare = errorShare(errored, requests)
		p, old, err := contributeCorpus(*contributeTo, *contributeDir, f, time.Now())
		if err != nil {
			return err
		}
		contribution, supersedes = p, old
	}

	// --share short-circuits every other rendering. A card that also printed
	// the full report would defeat its own purpose: the point is that what is
	// on screen is exactly what is safe to paste.
	if *share {
		// Over the rows, not over the lanes: the two sums are equal because
		// breaks are additive and the fold does not drop any, and summing the
		// rows is the version that stays correct if a row unit is ever added
		// that is not a partition of the lanes.
		breaks := 0
		for _, u := range units {
			breaks += u.Breaks
		}
		text := shareCard(s, breaks)
		if text == "" {
			return fmt.Errorf("nothing measured enough to share: %d priced sessions", s.Tasks)
		}
		if _, err := io.WriteString(stdout, text); err != nil {
			return err
		}
		if _, err := io.WriteString(stderr, shareNote()); err != nil {
			return err
		}
		// To stderr, not stdout. --share exists so that what is on screen is
		// exactly what is safe to paste, and a path under the contributor's
		// home is the one thing on this branch that is not.
		if contribution != "" {
			if _, err := io.WriteString(stderr, corpusContributionNote(contribution, supersedes)); err != nil {
				return err
			}
		}
		if *png == "" {
			return nil
		}
		// Same guard, same figures, one extra: the peak row's re-billed tokens,
		// which the picture needs and the text card does not carry. See
		// sharepng.go for why it is a peak and not the corpus sum.
		return writeCard(*png, variant, register, cardData(s, breaks, peakAvoidableTokens(units)), stderr)
	}

	if *asJSON {
		sort.Slice(units, func(i, j int) bool { return units[i].CostUSD > units[j].CostUSD })
		// v2, because `tasks` changed meaning. A consumer pinned to v1 was
		// handed one row per agent lane under that key; handing them session
		// rows under the same version string would be the silent kind of
		// break, where nothing errors and every figure moves.
		out := map[string]any{"schema": "replay.cost.v2", "summary": s, "unpriced": unpriced,
			"duplicatedRequests": duplicated, "totalRequests": totalReq}
		// A key rather than a printed line, because stdout on this branch is a
		// document a machine parses. The statement the human needs still has to
		// reach a human, so it goes to stderr alongside it.
		if contribution != "" {
			out["contribution"] = contribution
			if len(supersedes) > 0 {
				out["supersedes"] = supersedes
			}
			_, _ = io.WriteString(stderr, corpusContributionNote(contribution, supersedes))
		}
		if *perTask {
			// Lanes never appear under `tasks`. A separate key, with rows whose
			// id field is `ofSession`, means a consumer cannot be handed one
			// unit while reading code written for the other.
			if *perLane {
				out["lanes"] = laneRows(units)
			} else {
				out["tasks"] = units
			}
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s\n", b)
		return err
	}

	if _, err := io.WriteString(stdout, renderCost(s, unpriced, stdout, tipStateDir())); err != nil {
		return err
	}
	if note := overlapNote(duplicated, totalReq); note != "" {
		if _, err := io.WriteString(stdout, note); err != nil {
			return err
		}
	}
	// The one line a first-time reader can act on: their peak session against
	// their own median. Silent unless it is far enough out to be worth saying.
	if note := outlierNote(units, s); note != "" {
		if _, err := io.WriteString(stdout, note); err != nil {
			return err
		}
	}
	if contribution != "" {
		if _, err := io.WriteString(stdout, corpusContributionNote(contribution, supersedes)); err != nil {
			return err
		}
	}
	if *perTask && len(units) > 0 {
		sort.Slice(units, func(i, j int) bool { return units[i].CostUSD > units[j].CostUSD })
		// The first column names the row's own unit, and under --per-lane the
		// session it belongs to is a second column rather than the first one
		// relabelled. A lane table whose only id column was the parent session
		// would print the same id on a thousand rows, which is the screen the
		// JSON defect looked like.
		if *perLane {
			_, _ = fmt.Fprintf(stdout, "\n  %-10s %-10s %-24s %8s %10s %10s %7s\n",
				"lane", "of session", "model", "requests", "cost", "avoidable", "breaks")
			for _, u := range units {
				_, _ = fmt.Fprintf(stdout, "  %-10s %-10s %-24s %8d %10s %10s %7d\n", u.Lane, u.ID, u.Model,
					u.Requests, fmt.Sprintf("$%.2f", u.CostUSD), fmt.Sprintf("$%.2f", u.AvoidableUSD), u.Breaks)
			}
			return nil
		}
		_, _ = fmt.Fprintf(stdout, "\n  %-10s %5s %-24s %8s %10s %10s %7s\n",
			"session", "lanes", "model", "requests", "cost", "avoidable", "breaks")
		for _, u := range units {
			_, _ = fmt.Fprintf(stdout, "  %-10s %5d %-24s %8d %10s %10s %7d\n", u.ID, u.Lanes, u.Model,
				u.Requests, fmt.Sprintf("$%.2f", u.CostUSD), fmt.Sprintf("$%.2f", u.AvoidableUSD), u.Breaks)
		}
	}
	return checkAvoidableCeiling(gateCeiling, s, unpriced, stdout)
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
func costHeaderLine(s costSummary) string {
	// Both counts, named, whenever they differ.
	//
	// This said "N transcripts" because the row unit WAS a transcript file and
	// calling that a session count had overstated the corpus roughly
	// twentyfold in the published evidence. The rows are sessions now, so the
	// headline is a session count — and the file count is printed beside it
	// rather than dropped, because the gap between 114 and 1614 is the fact
	// that made the original figure wrong and a reader who cannot see it is
	// one retraction away from the same mistake.
	per, subject := "task", fmt.Sprintf("%d sessions", s.Tasks)
	if s.Unit == unitLane {
		// "Cost per task" over a lane count is the sentence this whole change
		// exists to stop being printed.
		per, subject = "agent lane", fmt.Sprintf("%d agent lanes", s.Tasks)
	} else if s.Lanes > s.Tasks {
		subject = fmt.Sprintf("%d sessions (%d agent lanes)", s.Tasks, s.Lanes)
	}
	line := fmt.Sprintf("Cost per %s, across %s at list prices dated %s (caching rules %s).",
		per, subject, cachemodel.PriceTableVersion, cachemodel.RulesVersionInEffect())
	return line + cachemodel.PriceTableAgeNote(time.Now())
}

// shortTokens renders a token count the way a reader scans it, not the way it
// is stored. 31,264,349 is a figure to parse; 31.3M is a figure to read.
func shortTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// fxCol renders one money figure for the summary block.
//
// The dollars keep their column exactly as they had it, so a reader who does
// not want this feature sees a byte-identical report. The local figure is
// appended after the column rather than inside it, because padding a string
// that already carries a currency name would shift everything under it.
func fxCol(fx money.Display, usd float64) string {
	base := fmt.Sprintf("$%-13.2f", usd)
	if a := fx.Short(usd); a != "" {
		return strings.TrimRight(base, " ") + "  = " + a
	}
	return strings.TrimRight(base, " ")
}
