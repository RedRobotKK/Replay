package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/RedRobotKK/Replay/internal/card"
	// Aliased because this file already has a money() that formats dollars,
	// and a package name shadowed by a function is a compile error waiting for
	// whoever adds the next reference.
	currency "github.com/RedRobotKK/Replay/internal/money"
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

// RulesAge says which of three answers the caller has about the rules
// document's date.
//
// A day count alone can carry only one of them. Absence, zero and unknown are
// three values (ADR-0018), and here each is a different sentence: no document
// installed over the build, a document dated N days ago, and a document whose
// date this build cannot read. Collapsing the third into "0 days" would make
// the unreadable case render as the freshest possible one, which is the
// reading that hides the problem.
type RulesAge int

const (
	// RulesUntold is the zero value: the caller populated no rules fields, so
	// the screen draws no row. A screen that was not told cannot report, and a
	// default row would announce the compiled-in table on a machine running an
	// installed one.
	RulesUntold RulesAge = iota
	// RulesBuiltIn is the table this build shipped with. That is a floor, not
	// a stale document: ageing a compiled-in constant prints a number with
	// nothing behind it.
	RulesBuiltIn
	// RulesFetched is a document installed over the build, with a date this
	// build could read.
	RulesFetched
	// RulesUndated is a document carrying a fetch date this build cannot
	// parse. It must not pass as current.
	RulesUndated
)

// Machine is what the local filesystem can answer without a proxy running.
//
// Populated by the caller, because internal/tui does no I/O: the boundary that
// keeps every frame testable without a disk is the same boundary that keeps
// this package honest about what it knows.
type Machine struct {
	// Sessions, Transcripts, Lanes and Projects come from the same walk
	// `replay doctor` uses, so the two commands cannot disagree about how much
	// is there.
	//
	// Sessions and Transcripts are different questions and were once reported
	// under one word. A session writes one transcript per agent lane, so on a
	// fanned-out corpus the file count is an order of magnitude above the
	// session count: this screen said "1,809 transcripts across 12 projects"
	// where the command said "124 sessions across 12 projects", about the same
	// directory, in the same minute.
	Sessions    int
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
	// RulesVersion, RulesState and RulesAgeDays date the rules document, which
	// is a different question from the price table above and a more expensive
	// one to get wrong. The price table moves a column; the rules document
	// carries the cache-read multiple, and that decides what Replay
	// RECOMMENDS rather than only what it reports. `replay doctor` has said
	// this since cmd/replay/doctorrules.go landed. This screen said nothing,
	// and it is the surface the installer opens for a new operator.
	//
	// RulesAgeDays is meaningful only when RulesState is RulesFetched.
	RulesVersion string
	RulesState   RulesAge
	RulesAgeDays int
	// Readings is how many probe measurements exist, and Models how many
	// distinct models they cover. One reading per model means no within-model
	// variance at all, which is worth saying rather than implying.
	Readings int
	Models   int
	// Found is false when the machine could not be read at all, which is a
	// different answer from zero and is rendered as one.
	Found bool

	// FX is the reader's currency, resolved from their locale by the command.
	//
	// The zero value is dollars only, so a screen built without one renders
	// exactly what it rendered before this existed. That matters more here
	// than in the report: nine screens share this struct and most of them will
	// never set it.
	FX currency.Display

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

	// Tasks is the per-task breakdown, most expensive first. It is what makes
	// the cost screen answerable rather than merely true: a total tells you
	// there is a problem and a list tells you where it is.
	TaskRows []Task

	// Card is what a share card of this corpus would carry, built by the same
	// function `replay cost --share --png` builds it with. One construction, so
	// the screen and the command cannot disagree about what the card says.
	//
	// ShareOK is the command's own guard on whether there is anything worth
	// posting, asked rather than reimplemented. A screen that decided for
	// itself would be a second answer to a question the command already
	// answers, and the two would drift.
	Card    card.Data
	ShareOK bool
}

// Task is one session's cost, and enough to go and look at it.
type Task struct {
	Session   string
	Model     string
	CostUSD   float64
	Breaks    int
	Requests  int
	Avoidable float64
	// Path is the transcript, resolved from the session prefix. Empty when the
	// file could not be found, and a row with no path cannot be opened, which
	// the screen says rather than failing on enter.
	Path string
}

// DoctorScreen renders what Replay can see here.
//
// It is Measured when the walk succeeded and Unavailable when it did not.
// Never Example: this screen has no illustrative version, because a made-up
// answer to "what can you see on my machine" is worse than no answer.
func DoctorScreen(m Machine) Screen {
	head := screenHead("doctor")

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
		"  "+headlineCount(m),
		"  Everything Replay reads is on this machine.", "",
		Row(docCols, "check", "result"),
		Row(docCols, "------------------------------", "-------------------------------"),
		Row(docCols, "transcripts", fmt.Sprintf("%s files, %s projects",
			commas(m.Transcripts), commas(m.Projects))),
		Row(docCols, "sub-agent lanes", commas(m.Lanes)),
		Row(docCols, "ledger", ledgerLine(m)),
		Row(docCols, "price table", fmt.Sprintf("dated %s, %d days old",
			m.PriceTableDate, m.PriceAgeDays)),
	)
	// Directly under the price table, because it answers the question that row
	// provokes: priced against what, and how old is that? Conditional rather
	// than defaulted, so a caller that has not been told draws nothing.
	if v, ok := rulesLine(m); ok {
		lines = append(lines, Row(docCols, "rules", v))
	}
	lines = append(lines,
		Row(docCols, "probe readings", readingsLine(m)),
		"", "  notes")

	if m.Readings > 0 && m.Readings == m.Models {
		lines = append(lines,
			note(false, "one reading per model means no within-model variance at all."))
	}
	if m.PriceAgeDays > 30 {
		// One line, not two. The doctor screen's worst case — a stale price
		// table, a stale rules document and one probe reading per model — came
		// to 21 rows against a body budget of 20 once bodyRows landed, and the
		// row that lost was the last note appended, which is the rules
		// staleness warning. Two changes, each correct alone, arriving in the
		// same tree. The trailing "not today's." folds onto the sentence above
		// it without losing a word of the claim.
		lines = append(lines,
			note(false, fmt.Sprintf("prices are %d days old: figures are list price on "+
				"that date, not today's.", m.PriceAgeDays)))
	}
	if m.RulesState == RulesFetched && m.RulesAgeDays >= rulesStaleDays {
		// Why this one gets three lines when prices get two: an operator who
		// reads "prices are old" knows what to discount. The cache-read
		// multiple is not a column they can discount, it is the term that
		// picks which policy the comparison calls cheaper — so the note has to
		// say that the recommendation itself is affected, and name the command
		// that fixes it. A warning an operator cannot act on is decoration.
		lines = append(lines,
			note(false, fmt.Sprintf("the rules document is %d days old: the cache-read multiple is",
				m.RulesAgeDays)),
			"      what changes what replay recommends, not only what it reports.",
			"      next: replay rules --check-prices")
	}
	return Screen{Key: 'd', Title: "doctor", Lines: pad(lines), From: Measured}
}

// headlineCount is the sentence at the top of the doctor screen, in the same
// words `replay doctor` uses for it.
//
// Sessions is the figure, because that is the figure the command prints and a
// reader moving between the two surfaces is comparing sentences, not fields.
// The file count keeps its place in the table below, as files.
//
// A screen given no session count does not print "0 sessions" beside a
// four-figure file count: absence and zero are different values (ADR-0018),
// and of the two numbers in hand the one that was actually measured is the one
// worth saying.
func headlineCount(m Machine) string {
	if m.Sessions == 0 {
		return fmt.Sprintf("%s transcript files across %s projects",
			commas(m.Transcripts), commas(m.Projects))
	}
	return fmt.Sprintf("%s sessions across %s projects",
		commas(m.Sessions), commas(m.Projects))
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

// rulesStaleDays is the same thirty days cmd/replay/doctorrules.go uses, in the
// unit this struct carries. Two surfaces answering the same question must cross
// the same line on the same day, or an operator who checks both is told their
// table is fine by one and stale by the other.
const rulesStaleDays = 30

// RulesStaleDays exposes the threshold so the command that owns the other copy
// of it can assert the two are the same number. Exported for that assertion and
// for nothing else: a threshold duplicated across two packages is only safe
// while something is checking.
func RulesStaleDays() int { return rulesStaleDays }

// rulesLine is the result column for the rules row, and false when there is no
// row to draw.
//
// Every state renders as "<version>, <what we know about its date>", so the
// version stays in the same place whichever answer applies and a reader
// scanning the column finds it once. The words are short because docCols gives
// this column 31 cells and cell() truncates with '~' — and a truncated version
// string does not read as a cut value, it reads as a different document.
func rulesLine(m Machine) (string, bool) {
	if m.RulesVersion == "" || m.RulesState == RulesUntold {
		return "", false
	}
	switch m.RulesState {
	case RulesBuiltIn:
		return m.RulesVersion + ", built in", true
	case RulesUndated:
		return m.RulesVersion + ", undated", true
	default:
		return fmt.Sprintf("%s, %d days", m.RulesVersion, m.RulesAgeDays), true
	}
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
	for len(lines) < bodyRows-3 {
		lines = append(lines, "")
	}
	if len(lines) > bodyRows-3 {
		lines = lines[:bodyRows-3]
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
func CostScreen(m Machine, tick int, sel Selection) Screen {
	head := screenHead("cost")

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
	// The two figures that carry the screen, and only those two.
	//
	// Strong on the total because it is the answer to the question the screen
	// asks, and bold rather than a hue so it survives a monochrome terminal.
	// Alarm on the avoidable figure because it is the only number here that is
	// money already spent twice; median and p90 describe what the work cost,
	// which is not a fault. Painting all four would say they are all the same
	// kind of number, and the reader would stop reading any of them.
	//
	// Painted into a free-form sentence rather than a padded column, so the
	// escapes cannot narrow a field. TestCL1 checks that claim rather than
	// trusting it.
	// The reader's own currency, on the two figures that carry the screen.
	//
	// Named by ISO code rather than by symbol, because this is a grid: every
	// currency symbol is East Asian width class Ambiguous and would take two
	// cells in exactly the locales that need it. The report can use a symbol
	// because its local figure ends the line; here it cannot.
	//
	// Only the total. The second line already carries three figures and a
	// sentence, and adding the aside there took it to 85 cells: TestTC1 caught
	// it under a ja_JP locale, which is the locale nobody here develops in.
	// Putting a second currency in the table's cost column has the same
	// problem with less to gain, and the column header already says dollars.
	//
	// The shape leads, and the total follows it.
	//
	// It was the other way round, and the total was the only figure raised:
	// "$3739.80 across 116 tasks" on the first line, "Median $0.77, p90 $3.41"
	// dropped into a subtitle underneath. On this corpus the mean is $32 and
	// the median is $0.77, so the figure the eye was sent to describes no task
	// anybody ran. A reader taking one number off this screen took the one that
	// answers "what did I spend in total", which is a different question from
	// the one the screen is titled by.
	//
	// The total stays. It is a real answer to a question about a bill and
	// deleting it would trade one half-truth for another; what changes is that
	// it no longer has the emphasis to itself.
	lines = append(lines,
		"  Median "+paint(Strong, money(m.MedianUSD))+", p90 "+paint(Strong, money(m.P90USD))+
			" per task. List price, not your bill.",
		"  "+paint(Strong, money(m.TotalUSD))+alsoIn(m.FX, m.TotalUSD)+" across "+commas(m.Tasks)+
			" tasks, "+paint(Alarm, money(m.AvoidableUSD))+" avoidable.",
		"")

	rows := TaskLines(m.TaskRows)
	if len(rows) == 0 {
		lines = append(lines, "  No per-task breakdown available.")
		return Screen{Key: 'c', Title: "cost", Lines: padCost(lines), From: Measured}
	}
	// The list is capped so the notes below it survive the budget.
	//
	// They were being truncated away by padCost, which took the price date and
	// the token figure with them: the list grew until it pushed the provenance
	// off the screen. Rows are the cheapest thing here to give up, because
	// there is a whole screen of them one keystroke away and only one place
	// the date appears.
	// One row fewer when a conversion is on screen, because the rate note
	// costs a line and the provenance must not be what gets dropped.
	//
	// The comment above this already worked out the priority: rows are the
	// cheapest thing here to give up, since there is a whole screen of them
	// one keystroke away and only one place the rate appears. Without this the
	// note is appended and then padCost trims it off the bottom, which reads
	// on screen as the feature simply not working.
	maxRows := 8
	if m.FX.RateNote() != "" {
		maxRows = 7
	}
	if sel.Window <= 0 || sel.Window > maxRows {
		sel.Window = maxRows
	}
	vis, cur := sel.Visible(rows)
	// Two spaces for the cursor marker, so the headings sit above the row text
	// rather than above the marker. Without this the columns are offset by the
	// width of the thing that points at them.
	// Column headings and the rule under them are structure, not content.
	// Lowering them is what lets the figures rise without anything shouting.
	// Painted after Row has padded, never before.
	lines = append(lines,
		"  "+Dim(Row(taskCols, "task", "cost", "breaks", "model")),
		"  "+Dim(Row(taskCols, "--------", "----------", "------", "----------------------")))
	lines = append(lines, RenderRows(vis, cur)...)
	lines = append(lines, "", SelectedLine(rows, sel.At),
		"  "+Dim("enter opens why this one cost what it did"),
		"",
		// The price date and the token figure survive the list.
		//
		// Two tests caught their loss when the table replaced the notes block,
		// and both were right to. The date is the provenance: without it the
		// dollars are a number with no basis. The tokens are what a flat-seat
		// reader actually lost, and they are most of the readership, so a
		// screen that shows only dollars is talking to somebody else.
		note(false, commas(m.AvoidableTokens)+" tokens is what the waste cost you: "+
			"context the work did not get."),
		note(false, "prices dated "+m.PriceDate+", across "+commas(m.CorpusFiles)+
			" transcripts."))
	// The rate belongs with the other provenance, not beside each figure.
	//
	// Repeating it on both converted numbers is noise a reader learns to skip,
	// and a caveat nobody reads is not a caveat. This screen has no room for
	// the report's full paragraph, so the line says the two things that cannot
	// be dropped: the rate it used, and that the bill is in dollars.
	if rn := m.FX.RateNote(); rn != "" {
		lines = append(lines, note(false, rn))
	}
	return Screen{Key: 'c', Title: "cost", Lines: padCost(lines), From: Measured, Rows: len(rows)}
}

// TaskLines renders the per-task rows, most expensive first.
//
// Sorted by cost rather than by time, because the question the screen answers
// is "where did the money go" and the answer is almost never the most recent
// task. A list in arrival order makes the reader scroll to find it.
func TaskLines(tasks []Task) []Line {
	out := make([]Line, 0, len(tasks))
	for _, t := range tasks {
		// The breaks count is the one column where a glance should tell you
		// something before you read it. 558 and 2 are both just numbers in a
		// column of numbers; Severity says which is a session worth opening.
		// Every other column stays plain, because a row painted throughout is
		// a row with no emphasis in it.
		text := StyledRow(taskCols,
			[]Style{Plain, Plain, Severity(t.Breaks)},
			t.Session, money(t.CostUSD), fmt.Sprint(t.Breaks), t.Model)
		label := t.Session + "  " + money(t.CostUSD)
		if t.Path == "" {
			label += "   (transcript not found, cannot open)"
		}
		out = append(out, Line{Text: text, Path: t.Path, Label: label})
	}
	return out
}

var taskCols = []Column{{"task", 8}, {"cost", 10}, {"breaks", 6}, {"model", 22}}

// money formats a dollar figure the way the cost report does.
func money(v float64) string { return fmt.Sprintf("$%.2f", v) }

// padCost fills to the budget and names the command that produced the screen.
func padCost(lines []string) []string {
	for len(lines) < bodyRows-3 {
		lines = append(lines, "")
	}
	if len(lines) > bodyRows-3 {
		lines = lines[:bodyRows-3]
	}
	return append(lines, "", "  ran   replay cost --per-task",
		"  "+Dim("copy it and you never need this screen again."))
}

// WhyScreen answers why one session cost what it did.
//
// It needs a session. `replay blame` refuses without a transcript, and
// averaging "why was it expensive" over 1,599 tasks answers nobody: the
// question is always about the one that cost forty dollars. So this screen has
// a state the others do not, which is having nothing to answer about yet, and
// it says so rather than inventing a session to talk about.
//
// run is injected so this package still does no I/O and the screen stays
// testable without a corpus.
func WhyScreen(t *Task, run func(path string) (string, error)) Screen {
	head := screenHead("why")
	lines := make([]string, 0, BudgetRows)
	lines = append(lines, head...)

	if t == nil {
		lines = append(lines,
			"  Pick a session first.", "",
			"  This answers why ONE task cost what it did, so it needs to know which.",
			"", "  "+cell("press", 10)+"c   the cost list",
			"  "+cell("then", 10)+"j k to move, enter to open one here",
			"", "  notes",
			note(false, "averaged over every task this question has no useful answer."))
		lines = WithBanner(lines, Unavailable, "no session chosen yet")
		return Screen{Key: 'w', Title: "why", Lines: padWhy(lines), From: Unavailable}
	}

	out, err := run(t.Path)
	if err != nil {
		lines = append(lines,
			"  Could not read that session.", "",
			"  "+cell("session", 10)+t.Session,
			"  "+cell("error", 10)+truncate(err.Error(), Cols()-14))
		lines = WithBanner(lines, Unavailable, "the transcript could not be read")
		return Screen{Key: 'w', Title: "why", Lines: padWhy(lines), From: Unavailable}
	}

	lines = append(lines,
		"  "+t.Session+"  "+money(t.CostUSD)+"  "+fmt.Sprint(t.Breaks)+" cache break(s)",
		"  "+t.Model, "")
	for _, l := range blameBody(out) {
		lines = append(lines, "  "+truncate(l, Cols()-2))
	}
	return Screen{Key: 'w', Title: "why", Lines: padWhy(lines), From: Measured}
}

// blameBody keeps the lines of a blame report that answer the question, and
// drops the file path header, which the reader already knows.
func blameBody(out string) []string {
	all := strings.Split(strings.TrimRight(out, "\n"), "\n")
	keep := make([]string, 0, len(all))
	for _, l := range all {
		if strings.HasPrefix(l, "/") || strings.TrimSpace(l) == "" {
			continue
		}
		keep = append(keep, l)
		if len(keep) >= BudgetRows-11 {
			break
		}
	}
	return keep
}

func truncate(s string, w int) string {
	if w <= 1 || len(s) <= w {
		return s
	}
	return s[:w-1] + string(truncationMark)
}

func padWhy(lines []string) []string {
	for len(lines) < bodyRows-3 {
		lines = append(lines, "")
	}
	if len(lines) > bodyRows-3 {
		lines = lines[:bodyRows-3]
	}
	return append(lines, "", "  ran   replay blame <session>",
		"  "+Dim("copy it and you never need this screen again."))
}

// alsoIn renders " (N CODE)" for a figure, or nothing at all.
//
// Dimmed, because it is the second way of saying a number the reader has
// already been given. Painted after the parentheses so the whole aside lowers
// together rather than leaving punctuation at full contrast.
func alsoIn(fx currency.Display, usd float64) string {
	a := fx.ASCII(usd)
	if a == "" {
		return ""
	}
	return " " + Dim("("+a+")")
}
