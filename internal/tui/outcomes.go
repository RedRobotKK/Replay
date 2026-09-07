package tui

import (
	"fmt"
	"strings"
)

// The outcome of pressing a key.
//
// One question in, one screen out, inside eighty columns and twenty-four rows.
// The shape is the same every time, which is the point: a user who learns to
// read one of these can read all eight, and the thing that changes between them
// is the answer rather than the furniture.
//
//	title        what this screen is
//	answer       one sentence, the thing they asked for
//	body         the detail that supports it
//	ran          the command that produced it, copyable
//	hints        every other question, one keystroke away
//
// The answer comes first because the alternative is a table the reader has to
// interpret, and interpreting is the work they came here to avoid.

// Screen is a rendered outcome.
type Screen struct {
	Key   rune
	Title string
	Lines []string
	// From is where the figures came from. Every screen states it, and the
	// unmeasured ones say so on screen rather than in a comment.
	From Provenance
	// Rows is how many selectable lines the screen has, so the loop knows
	// whether the movement keys apply here.
	Rows int
}

// answerBlock renders the headline: a figure and the sentence that reads it.
func answerBlock(figure, sentence string) []string {
	return []string{"  " + figure, "  " + sentence}
}

// Outcome renders the screen for one shortcut with the values it would carry.
func Outcome(key rune) Screen {
	var sc Shortcut
	for _, s := range Shortcuts() {
		if s.Key == key {
			sc = s
		}
	}
	head := []string{"  replay " + sc.Label + strings.Repeat(" ",
		BudgetCols-12-len(sc.Label)-6) + "v0.4.0", ""}

	var body []string
	switch key {
	case 'c':
		body = append(answerBlock("$3,089.84 across 1,599 transcripts",
			"Median task $0.63. The top ten tasks are 31% of the bill."), "",
			Row(costCols, "task", "cost", "tokens", "when"),
			Row(costCols, "----------------------------", "---------", "-----------", "------"),
			Row(costCols, "refactor the proxy state map", "$41.20", "8,204,551", "14:02"),
			Row(costCols, "chase the windows consent bug", "$22.87", "4,551,209", "11:40"),
			Row(costCols, "wire-family capture", "$18.04", "3,880,112", "09:15"),
			Row(costCols, "installer copy pass", "$9.61", "1,902,338", "08:31"),
			"",
			"  "+"* estimated through the byte-to-token fit; the rest is provider usage.")
	case 'w':
		body = append(answerBlock("336,060 tokens re-billed, 4.2% of the bill",
			"Three cache breaks. Every one is an MCP tool block arriving mid-session."), "",
			Row(blameCols, "cost", "cause", "detail"),
			Row(blameCols, "---------", "----------------------", "--------------------------"),
			Row(blameCols, "157,080", "tool definitions changed", "added 3 Otter tools"),
			Row(blameCols, "140,623", "tool definitions changed", "added 39 Calendly tools"),
			Row(blameCols, "38,357", "tool definitions changed", "added 199 MCP resources"),
			"",
			"  the fix is client-side: bind MCP tools before the first cached request,",
			"  not after. Nothing here is Replay's to change for you.")
	case 'x':
		body = append(answerBlock("184,220 tokens entered this context",
			"Tool results are 71% of it. Two files account for a third."), "",
			Row(ctxCols, "what", "share", "tokens", "times"),
			Row(ctxCols, "------------------------------", "------", "---------", "-----"),
			Row(ctxCols, "tool result: Read internal/prox", "18.2%", "33,528", "9"),
			Row(ctxCols, "tool result: Read cmd/replay/se", "14.9%", "27,449", "7"),
			Row(ctxCols, "system prompt and tool defs", "12.1%", "22,290", "1"),
			Row(ctxCols, "tool result: Bash go test ./...", "9.4%", "17,317", "12"),
			"",
			"  * estimated: this session fitted 14 turns, so the ratio is its own.")
	case 'a':
		body = append(answerBlock("2 changes worth making, 1 not yet",
			"Both are measured on your sessions, not on a default."), "",
			"  1  bind MCP tools before the first request",
			"     evidence  3 sessions, 336,060 tokens re-billed by late tool blocks",
			"     saving    4.2% of prompt tokens        status  advice only",
			"",
			"  2  cache the system prompt explicitly",
			"     evidence  12 sessions, prefix stable across every one",
			"     saving    not measured on this corpus  status  pending",
			"",
			"  ! one suggestion is withheld: it needs a live trial and --trial-share",
			"    is unset, so nothing has been tested on your traffic.")
	case 'g':
		body = append(answerBlock("4 guards armed, 1 cannot fire",
			"The day dollar cap is set and is not being enforced."), "",
			Row(guardCols, "guard", "setting", "state"),
			Row(guardCols, "---------------------", "--------------", "----------------------"),
			Row(guardCols, "day cap, dollars", "$5.00", "$2.41   NOT ENFORCED"),
			Row(guardCols, "day cap, tokens", "unset", "not set"),
			Row(guardCols, "error budget", "15%", "3.1%   armed"),
			Row(guardCols, "loop block", "12 repeats", "0   armed"),
			Row(guardCols, "breaker", "3 failures", "closed"),
			"",
			"  ! 12 requests could not be priced. They add nothing to the dollar",
			"    total, so that cap will never be reached. You do not have it.")
	case 'm':
		body = append(answerBlock("haiku would have cost $283, against $2,623",
			"Wide error bars: the band is $18 to $548. Do not spend on this yet."), "",
			Row(routeCols, "model", "projected", "band", "turns"),
			Row(routeCols, "--------------------", "----------", "----------------", "------"),
			Row(routeCols, "claude-haiku-4-5", "$283.18", "$18 to $548", "17,560"),
			Row(routeCols, "claude-sonnet-5", "$1,019.74", "$61 to $1,978", "17,560"),
			Row(routeCols, "claude-fable-5-1", "$1,696.63", "$102 to $3,291", "17,560"),
			"",
			"  the band is the estimator's own floor, not this corpus being small.",
			"  More sessions will not narrow it. See routing-baseline-2026-09-06.")
	case 's':
		body = append(answerBlock("masking is on, and does not cover two paths",
			"Anything sent on those paths leaves this machine unmasked."), "",
			Row(safeCols, "path", "parsed", "masked"),
			Row(safeCols, "------------------------", "----------------", "----------------"),
			Row(safeCols, "/v1/messages", "yes", "yes, 14 rules"),
			Row(safeCols, "/v1/chat/completions", "against a stub", "no"),
			Row(safeCols, "/responses", "no", "no"),
			"",
			"  ! a path Replay does not parse cannot be masked. If you point a Grok",
			"    or OpenAI-compatible client here, its payloads are forwarded whole.",
			"",
			"  corpus contribution  refused, 2026-09-04    update checks  undecided")
	case 'd':
		body = append(answerBlock("1,599 transcripts across 12 projects",
			"Everything Replay needs is readable. Two things it cannot see."), "",
			Row(docCols, "check", "result"),
			Row(docCols, "------------------------------", "-------------------------------"),
			Row(docCols, "transcripts", "1,599 files, 12 projects"),
			Row(docCols, "ledger", "~/.replay/ledger, writable"),
			Row(docCols, "price table", "dated 2026-06-24, 74 days old"),
			Row(docCols, "probe readings", "4, one per model"),
			"",
			"  - one reading per model means no within-model variance at all.",
			"  - 74-day-old prices: figures are list price on that date, not today's.")
	}

	lines := make([]string, 0, BudgetRows)
	lines = append(lines, head...)
	lines = append(lines, body...)

	// Every figure on these screens was typed. Until each is wired to its
	// source, the screen says so above the numbers rather than letting a
	// reader take them for their own.
	from := Example
	lines = WithBanner(lines, from, "")
	for len(lines) < BudgetRows-4 {
		lines = append(lines, "")
	}
	lines = append(lines, Ran(sc)...)
	// No footer here.
	//
	// The loop appends Footer(key) to whatever a source returns, so a screen
	// carrying its own produced two, and the one baked in here was the old
	// eight-key strip that progressive disclosure moved behind "?". The
	// --once path printed that strip while the interactive path printed the
	// three-key floor: the same screen, two different promises, depending on
	// how you looked at it.
	lines = append(lines, "")
	return Screen{Key: key, Title: sc.Label, Lines: lines, From: from}
}

var (
	costCols  = []Column{{"task", 28}, {"cost", 9}, {"tokens", 11}, {"when", 6}}
	blameCols = []Column{{"cost", 9}, {"cause", 22}, {"detail", 26}}
	ctxCols   = []Column{{"what", 30}, {"share", 6}, {"tokens", 9}, {"times", 5}}
	guardCols = []Column{{"guard", 21}, {"setting", 14}, {"state", 22}}
	routeCols = []Column{{"model", 20}, {"projected", 10}, {"band", 16}, {"turns", 6}}
	safeCols  = []Column{{"path", 24}, {"parsed", 16}, {"masked", 16}}
	docCols   = []Column{{"check", 30}, {"result", 31}}
)

// Outcomes renders every screen, in key order.
func Outcomes() []Screen {
	var out []Screen
	for _, s := range Shortcuts() {
		out = append(out, Outcome(s.Key))
	}
	return out
}

// String renders a screen for a golden file or a terminal.
func (s Screen) String() string {
	return fmt.Sprintf("%s\n", strings.Join(s.Lines, "\n"))
}
