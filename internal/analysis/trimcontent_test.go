package analysis

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Diff-shaped output is content by construction, and this pins that it is not.
//
// Not one of the four findings the 2026-09-04 review left open — it is the
// surface those findings sit next to. ScoreTrim is the code in this repository
// that comes closest to printing prompt content on purpose: it reads
// transcript.Block.Text for a tool result, computes which bytes a cap would
// remove, then looks through every later message's Text for a quoted line or
// an Edit's old_string that lives only in the removed part. Every one of those
// comparisons has a real line of the user's source on both sides.
//
// What it emits today is a label and a fraction. Nothing said so, and nothing
// would have noticed a `%s` gaining an argument — the change that turns
// "a later message quotes a line only present in read.go" into "a later
// message quotes `AWS_SECRET_ACCESS_KEY=...`" is one format verb, in a
// function whose whole job is to talk about the removed bytes.
//
// The reason to pin it here rather than at the printer: this is where the
// content is in scope. By the time a TrimHarm reaches a terminal the text is
// long out of reach, so a test at the printer can only confirm that what it
// was handed was clean.
//
// TC2 covers the same ground for the JSON form, because `--json` output lands
// in files and CI logs rather than scrollback.

// tcCanaries are strings that exist only inside block text. Any of them in a
// harm is content that escaped.
var tcCanaries = []string{
	"AKIAIOSFODNN7EXAMPLE",
	"postgres://deploy:hunter2@db.internal:5432/prod",
	"func settleInvoice(customerID string, amountJPY int64) error {",
	"// TODO(dsaito): the retry here double-charges on a 409",
}

// tcLane builds a lane whose over-cap tool result holds every canary, and
// whose later turns quote them back — the exact shape probeCut looks for, so
// each harm kind actually fires.
func tcLane(t *testing.T) *transcript.Lane {
	t.Helper()
	// The removed region has to be past the cap, so put filler first.
	head := strings.Repeat("padding line that is retained\n", 40)
	body := strings.Join(tcCanaries, "\n") + "\n"
	full := head + body + strings.Repeat("trailing filler\n", 40)

	result := transcript.Block{
		Kind: transcript.KindToolResult, Label: transcript.LabelToolResultPrefix + "Read src/billing/settle.go",
		ToolName: "Read", ToolUseID: "tu-1", Text: full, Bytes: len(full),
	}
	// A later assistant message quoting a removed line.
	quote := transcript.Block{
		Kind: transcript.KindText, Label: "assistant text",
		Text:  "I see the problem in " + tcCanaries[2] + " — the retry is wrong.",
		Bytes: 64,
	}
	// A later Edit whose old_string sits only in the removed region.
	edit := transcript.Block{
		Kind: transcript.KindToolUse, Label: transcript.LabelToolCallPrefix + "Edit", ToolName: "Edit", ToolUseID: "tu-2",
		Text:  `{"file_path":"src/billing/settle.go","old_string":"` + tcCanaries[3] + `","new_string":"// fixed"}`,
		Bytes: 80,
	}
	// The same file read again, after the cap would have trimmed it.
	reread := transcript.Block{
		Kind: transcript.KindToolResult, Label: transcript.LabelToolResultPrefix + "Read src/billing/settle.go",
		ToolName: "Read", ToolUseID: "tu-3", Text: full, Bytes: len(full),
	}

	msg := func(role string, blocks ...transcript.Block) *transcript.Message {
		return &transcript.Message{UUID: role + "-m", Role: role, Blocks: blocks}
	}
	first := msg(transcript.RoleUser, result)
	return &transcript.Lane{ID: "lane-1", Requests: []*transcript.Request{
		{ID: "r0", Context: []*transcript.Message{first}},
		{ID: "r1", Context: []*transcript.Message{first, msg(transcript.RoleAssistant, quote, edit)}},
		{ID: "r2", Context: []*transcript.Message{first, msg(transcript.RoleUser, reread)}},
	}}
}

// tcFit is the byte-to-token fit ScoreTrim needs. A quarter token per byte is
// close enough; this test is not about the arithmetic.
var tcFit = TokenFit{TokensPerByte: 0.25, Turns: 3}

// A note on what is NOT asserted here. The block LABEL is content by design:
// shortLabel's own comment says a Bash tool result's label is the whole
// invocation, "URLs and paths included", and a report that could not name the
// file it is talking about would be useless. The boundary this test defends is
// the one that is not obvious from reading the code — the block's BODY, the
// bytes a cap would remove, which nothing in the output is meant to carry.

// TC1: no harm detail carries a byte of the block it describes.
//
// PASS: every canary is absent from every Detail and Tool.
// FAIL: any of them present, which is a line of the user's source in the
// terminal, in scrollback, and in whatever recorded the session.
func TestTC1_TrimHarmsNameTheBlockAndNeverQuoteIt(t *testing.T) {
	plan := ScoreTrim(tcLane(t), tcFit, 512)
	if len(plan.Harms) == 0 {
		t.Fatal("no harms were produced, so this test asserts nothing; the fixture no longer exercises probeCut")
	}
	kinds := map[string]bool{}
	for _, h := range plan.Harms {
		kinds[h.Kind] = true
		for _, c := range tcCanaries {
			if strings.Contains(h.Detail, c) || strings.Contains(h.Tool, c) {
				t.Errorf("harm %q leaked block content: %q", h.Kind, h.Detail)
			}
		}
	}
	// All three harm kinds must have fired, or the assertion above only covers
	// the paths that happened to run.
	for _, want := range []string{HarmLaterEdit, HarmReRead, HarmQuote} {
		if !kinds[want] {
			t.Errorf("harm kind %q never fired; that path is unasserted", want)
		}
	}
}

// TC2: the JSON form carries no content either.
//
// `--json` goes into files and CI logs, which outlive a terminal. Serialising
// the whole plan rather than walking its fields is deliberate: a field added
// later that happens to hold text is caught without anybody remembering to
// extend this test.
func TestTC2_TheSerialisedTrimPlanCarriesNoContent(t *testing.T) {
	plan := ScoreTrim(tcLane(t), tcFit, 512)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Harms) == 0 {
		t.Fatal("no harms were produced, so this test asserts nothing")
	}
	for _, c := range tcCanaries {
		if strings.Contains(string(raw), c) {
			t.Errorf("the serialised trim plan carries %q", c)
		}
	}
}
