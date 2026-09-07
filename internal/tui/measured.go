package tui

import (
	"fmt"
	"os"
	"strings"
)

// Measured screens.
//
// Everything else on this surface is example data and says so. These read the
// machine, so they carry no notice and the provenance test requires them to
// name their source when they change.
//
// The rule the tests enforce, and the reason this file is separate: a screen is
// Measured only when every figure on it came from somewhere a reader could go
// and check. A screen that measures three numbers and invents a fourth is an
// example screen, because the notice is what tells a reader how much to trust,
// and a partial one tells them wrong.

// Machine is what the local filesystem can answer without a proxy running.
//
// Populated by the caller, because internal/tui does no I/O: the boundary that
// keeps every frame testable without a disk is the same boundary that keeps
// this package honest about what it knows.
type Machine struct {
	// Transcripts, Lanes and Projects come from the same walk `replay doctor`
	// uses, so the two commands cannot disagree about how much is there.
	Transcripts int
	Lanes       int
	Projects    int
	// ProjectsDir is where the walk looked, so a reader can go and count.
	ProjectsDir string
	// LedgerDir and LedgerWritable describe where a proxy would record.
	LedgerDir      string
	LedgerWritable bool
	// PriceTableDate and PriceAgeDays date the compiled price table.
	PriceTableDate string
	PriceAgeDays   int
	// Readings is how many probe measurements exist, and Models how many
	// distinct models they cover. One reading per model means no within-model
	// variance at all, which is worth saying rather than implying.
	Readings int
	Models   int
	// Found is false when the machine could not be read at all, which is a
	// different answer from zero and is rendered as one.
	Found bool

	// Cost is what the corpus adds up to. It arrives late: the walk takes
	// seconds on a real corpus, and blocking the first frame on it would hand
	// the delay to the keyboard. Until CostReady the screen shows the field
	// waiting rather than a zero, which is the same rule the doctor screen
	// keeps about nothing-found.
	CostReady       bool
	Tasks           int
	TotalUSD        float64
	MedianUSD       float64
	P90USD          float64
	AvoidableUSD    float64
	AvoidableShare  float64
	AvoidableTokens int
	// PriceDate and CorpusFiles say what the figures were computed from, so a
	// reader can check the total against the same two numbers replay cost
	// prints.
	PriceDate   string
	CorpusFiles int
}

// DoctorScreen renders what Replay can see here.
//
// It is Measured when the walk succeeded and Unavailable when it did not.
// Never Example: this screen has no illustrative version, because a made-up
// answer to "what can you see on my machine" is worse than no answer.
func DoctorScreen(m Machine) Screen {
	head := []string{"  replay doctor" + spaces(BudgetCols-15-6) + "v0.4.0", ""}

	if !m.Found {
		lines := make([]string, 0, BudgetRows)
		lines = append(lines, head...)
		lines = append(lines,
			"  Nothing readable here yet.", "",
			"  "+cell("transcripts", 22)+"none found",
			"  "+cell("looked in", 22)+shortPath(m.ProjectsDir),
			"", "  notes",
			note(false, "point a client at replay serve, or name a corpus:"),
			"      REPLAY_TRANSCRIPTS=/path/to/projects replay tui")
		lines = WithBanner(lines, Unavailable, "no transcripts under "+shortPath(m.ProjectsDir))
		return Screen{Key: 'd', Title: "doctor", Lines: pad(lines), From: Unavailable}
	}

	lines := make([]string, 0, BudgetRows)
	lines = append(lines, head...)
	lines = append(lines,
		"  "+fmt.Sprintf("%s transcripts across %s projects",
			commas(m.Transcripts), commas(m.Projects)),
		"  Everything Replay reads is on this machine.", "",
		Row(docCols, "check", "result"),
		Row(docCols, "------------------------------", "-------------------------------"),
		Row(docCols, "transcripts", fmt.Sprintf("%s files, %s projects",
			commas(m.Transcripts), commas(m.Projects))),
		Row(docCols, "sub-agent lanes", commas(m.Lanes)),
		Row(docCols, "ledger", ledgerLine(m)),
		Row(docCols, "price table", fmt.Sprintf("dated %s, %d days old",
			m.PriceTableDate, m.PriceAgeDays)),
		Row(docCols, "probe readings", readingsLine(m)),
		"", "  notes")

	if m.Readings > 0 && m.Readings == m.Models {
		lines = append(lines,
			note(false, "one reading per model means no within-model variance at all."))
	}
	if m.PriceAgeDays > 30 {
		lines = append(lines,
			note(false, fmt.Sprintf("prices are %d days old: figures are list price on "+
				"that date,", m.PriceAgeDays)),
			"      not today's.")
	}
	return Screen{Key: 'd', Title: "doctor", Lines: pad(lines), From: Measured}
}

func ledgerLine(m Machine) string {
	if m.LedgerDir == "" {
		return "not configured"
	}
	if !m.LedgerWritable {
		return shortPath(m.LedgerDir) + ", NOT writable"
	}
	return shortPath(m.LedgerDir) + ", writable"
}

func readingsLine(m Machine) string {
	switch {
	case m.Readings == 0:
		return "none. replay probe --execute takes one"
	case m.Models == 0:
		return commas(m.Readings)
	default:
		return fmt.Sprintf("%d, across %d model(s)", m.Readings, m.Models)
	}
}

// pad fills a screen to the budget and gives it the footer the loop expects.
func pad(lines []string) []string {
	for len(lines) < BudgetRows-3 {
		lines = append(lines, "")
	}
	if len(lines) > BudgetRows-3 {
		lines = lines[:BudgetRows-3]
	}
	return append(lines, "", "  ran   replay doctor",
		"  "+Dim("copy it and you never need this screen again."))
}

func spaces(n int) string {
	if n < 0 {
		n = 0
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

// shortPath replaces the home directory with ~ so a path fits and a reader
// still recognises it.
func shortPath(p string) string {
	if p == "" {
		return "not set"
	}
	// Replace the home directory with ~, so the path fits the column and a
	// reader still recognises it. Truncating it instead produced
	// "/Users/daniel/.replay/ledger, ~" on this machine, which is a path
	// nobody can act on.
	if home, err := os.UserHomeDir(); err == nil && home != "" &&
		strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// commas groups thousands, because a seven-digit figure without them is a
// figure nobody reads at a glance.
func commas(n int) string {
	s := fmt.Sprint(n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// CostScreen renders what the corpus cost, or that it is still counting.
//
// Two states, and the waiting one is not a placeholder. A corpus of a thousand
// transcripts takes seconds to walk, and a surface that showed zeros until it
// finished would be showing a wrong number, not a pending one. The pinwheel is
// the difference between "nothing" and "not yet", and it is the reason the
// liveness cue exists at all.
func CostScreen(m Machine, tick int) Screen {
	head := []string{"  replay cost" + spaces(BudgetCols-13-6) + "v0.4.0", ""}

	if !m.Found {
		lines := make([]string, 0, BudgetRows)
		lines = append(lines, head...)
		lines = append(lines,
			"  No transcripts to add up.", "",
			"  "+cell("looked in", 22)+shortPath(m.ProjectsDir),
			"", "  notes",
			note(false, "REPLAY_TRANSCRIPTS=/path/to/projects replay tui"))
		lines = WithBanner(lines, Unavailable, "no transcripts under "+shortPath(m.ProjectsDir))
		return Screen{Key: 'c', Title: "cost", Lines: padCost(lines), From: Unavailable}
	}

	if !m.CostReady {
		lines := make([]string, 0, BudgetRows)
		lines = append(lines, head...)
		lines = append(lines,
			"  "+Pinwheel(tick)+" adding up "+commas(m.CorpusFiles)+" transcripts",
			"  This takes a few seconds the first time and is cached after.", "",
			Awaiting("total", tick),
			Awaiting("median task", tick),
			Awaiting("avoidable", tick),
			"", "  notes",
			note(false, "the keys still work while this counts. Nothing is blocked."))
		lines = WithBanner(lines, Unavailable, "still counting")
		return Screen{Key: 'c', Title: "cost", Lines: padCost(lines), From: Unavailable}
	}

	lines := make([]string, 0, BudgetRows)
	lines = append(lines, head...)
	lines = append(lines,
		"  "+money(m.TotalUSD)+" across "+commas(m.Tasks)+" tasks",
		"  Median task "+money(m.MedianUSD)+", ninetieth percentile "+money(m.P90USD)+".",
		"",
		Row(costCols2, "figure", "value", "what it is"),
		Row(costCols2, "--------------", "------------", "----------------------------"),
		Row(costCols2, "total", money(m.TotalUSD), "list price, not your invoice"),
		Row(costCols2, "median task", money(m.MedianUSD), "half cost less than this"),
		Row(costCols2, "p90 task", money(m.P90USD), "one task in ten costs more"),
		Row(costCols2, "avoidable", money(m.AvoidableUSD), avoidableWhat(m)),
		"", "  notes",
		note(false, "on a flat seat the dollars are not your bill. "+
			commas(m.AvoidableTokens)+" tokens is"),
		"      what the waste actually cost you: context the work did not get.",
		note(false, "prices dated "+m.PriceDate+", across "+commas(m.CorpusFiles)+" transcripts."))
	return Screen{Key: 'c', Title: "cost", Lines: padCost(lines), From: Measured}
}

func avoidableWhat(m Machine) string {
	if m.TotalUSD <= 0 {
		return "re-billed by cache breaks"
	}
	return fmt.Sprintf("%.0f%% of the total", m.AvoidableShare*100)
}

// money formats a dollar figure the way the cost report does.
func money(v float64) string { return fmt.Sprintf("$%.2f", v) }

var costCols2 = []Column{{"figure", 14}, {"value", 12}, {"what it is", 28}}

// padCost fills to the budget and names the command that produced the screen.
func padCost(lines []string) []string {
	for len(lines) < BudgetRows-3 {
		lines = append(lines, "")
	}
	if len(lines) > BudgetRows-3 {
		lines = lines[:BudgetRows-3]
	}
	return append(lines, "", "  ran   replay cost --per-task",
		"  "+Dim("copy it and you never need this screen again."))
}
