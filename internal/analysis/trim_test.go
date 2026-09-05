package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

func toolResult(tool, path, body string) *transcript.Block {
	return &transcript.Block{
		Kind: transcript.KindToolResult, ToolName: tool,
		Label: "tool result: " + tool + " " + path,
		Bytes: len(body), Text: body,
	}
}

func toolUse(tool, input string) *transcript.Block {
	return &transcript.Block{
		Kind: transcript.KindToolUse, ToolName: tool,
		Label: "tool call: " + tool, Bytes: len(input), Text: input,
	}
}

func assistantText(s string) *transcript.Block {
	return &transcript.Block{Kind: transcript.KindText, Label: "assistant text", Bytes: len(s), Text: s}
}

// laneOf builds a lane where request i carries every block up to i, which is
// how a conversation actually accumulates: an early tool result is resent on
// every later turn, and that resending is the whole cost trimming attacks.
func laneOf(blocks ...*transcript.Block) *transcript.Lane {
	lane := &transcript.Lane{ID: "main"}
	for i := range blocks {
		msg := &transcript.Message{}
		for _, b := range blocks[:i+1] {
			msg.Blocks = append(msg.Blocks, *b)
		}
		lane.Requests = append(lane.Requests, &transcript.Request{
			ID: "r", Model: "claude-opus-5", Timestamp: time.Unix(int64(i), 0),
			Context: []*transcript.Message{msg},
			Usage:   transcript.Usage{Input: 1000, CacheRead: 9000},
		})
	}
	return lane
}

const big = 4000

func body(head, mid, tail string) string {
	pad := strings.Repeat("x\n", big)
	return head + "\n" + pad + mid + "\n" + pad + tail
}

// The probe's first signal: a later Edit whose old_string sits only in the part
// the cap would have removed. Trimming there would have removed the content the
// agent needed to make its next edit.
func TestHarmProbeFindsALaterEditIntoARemovedRegion(t *testing.T) {
	doomed := "func criticalHelper() error { return nil }"
	lane := laneOf(
		toolResult("Read", "a.go", body("package main", "middle marker", doomed)),
		assistantText("thinking"),
		toolUse("Edit", `{"file_path":"a.go","old_string":"func criticalHelper() error { return nil }","new_string":"x"}`),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)
	if len(plan.Harms) == 0 {
		t.Fatal("a later Edit depended on removed content and the probe missed it")
	}
	var found bool
	for _, h := range plan.Harms {
		if h.Kind == HarmLaterEdit && strings.Contains(h.Detail, "a.go") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a later-edit harm naming a.go, got %+v", plan.Harms)
	}
}

// Content that survives the cap is not harm. Without this the probe could
// report every later Edit and look impressively thorough while measuring
// nothing.
func TestHarmProbeIgnoresAnEditIntoRetainedContent(t *testing.T) {
	lane := laneOf(
		toolResult("Read", "a.go", body("package main", "middle", "tail")),
		toolUse("Edit", `{"file_path":"a.go","old_string":"package main","new_string":"x"}`),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)
	for _, h := range plan.Harms {
		if h.Kind == HarmLaterEdit {
			t.Fatalf("old_string is inside the retained head, so nothing was harmed: %+v", h)
		}
	}
}

// The second signal: the agent read the same path again later. Trimming the
// first read is cheaper than it looks, because the re-read already paid for it,
// but it is still evidence the content mattered.
func TestHarmProbeFindsALaterReReadOfATrimmedPath(t *testing.T) {
	lane := laneOf(
		toolResult("Read", "b.go", body("package main", "middle", "tail")),
		assistantText("thinking"),
		toolResult("Read", "b.go", body("package main", "middle", "tail")),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)
	var found bool
	for _, h := range plan.Harms {
		if h.Kind == HarmReRead && strings.Contains(h.Detail, "b.go") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a re-read of a trimmed path was not reported: %+v", plan.Harms)
	}
}

// The third signal: the assistant quoted a line that only existed in the
// removed region.
func TestHarmProbeFindsAQuoteOfARemovedLine(t *testing.T) {
	secret := "the answer is fourty-two and nothing else"
	lane := laneOf(
		toolResult("Bash", "test", body("run", "middle", secret)),
		assistantText("I see that "+secret+" so we are done"),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)
	var found bool
	for _, h := range plan.Harms {
		if h.Kind == HarmQuote {
			found = true
		}
	}
	if !found {
		t.Fatalf("a quote of a removed line was not reported: %+v", plan.Harms)
	}
}

// Savings are reported in dollars at the cache-read price, because a resent
// byte is a cache read at a fraction of input price. Reporting the token share
// oversells by about eight times on this corpus's hit rate.
func TestSavingsArePricedAsCacheReadsNotFreshInput(t *testing.T) {
	lane := laneOf(
		toolResult("Read", "a.go", body("package main", "middle", "tail")),
		assistantText("one"),
		assistantText("two"),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)
	if plan.RemovedBytes <= 0 {
		t.Fatal("the cap removed nothing")
	}
	if plan.SavedUSD <= 0 {
		t.Fatalf("no saving was priced: %+v", plan)
	}
	if plan.SavedInputUSD <= plan.SavedUSD {
		t.Fatalf("the cache-read figure must be the smaller one: read $%.6f vs input $%.6f", plan.SavedUSD, plan.SavedInputUSD)
	}
	// The overstatement the PRD names, made explicit rather than implied.
	if plan.Overstatement() < 2 {
		t.Fatalf("pricing resent bytes as fresh input should overstate several times over, got %.2fx", plan.Overstatement())
	}
}

// The head/tail split is derived from where later dependencies actually landed,
// not fixed at 60/40. For Read the middle is function bodies; for test output
// the failure detail is in the middle; a fixed split is wrong in opposite
// directions for the two dominant shapes.
func TestSplitIsDerivedFromWhereDependenciesLanded(t *testing.T) {
	lane := laneOf(
		toolResult("Read", "a.go", body("package main", "middle", "func tailThing() {}")),
		toolUse("Edit", `{"file_path":"a.go","old_string":"func tailThing() {}","new_string":"x"}`),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)
	var split *ToolSplit
	for i := range plan.Splits {
		if plan.Splits[i].Tool == "Read" {
			split = &plan.Splits[i]
		}
	}
	if split == nil {
		t.Fatalf("no split was derived for Read: %+v", plan.Splits)
	}
	if split.Samples == 0 {
		t.Fatal("a split with no samples behind it is a guess wearing a number")
	}
	if split.TailShare <= split.HeadShare {
		t.Fatalf("the dependency was in the tail, so the tail must carry the weight: head %.2f tail %.2f", split.HeadShare, split.TailShare)
	}
}

// The probe is a lower bound and has to say so, naming what it cannot see.
func TestProbeDeclaresItselfALowerBound(t *testing.T) {
	notes := ProbeBlindSpots()
	if len(notes) < 3 {
		t.Fatalf("the blind spots are named in the PRD and must be printed: %v", notes)
	}
	joined := strings.ToLower(strings.Join(notes, " "))
	for _, want := range []string{"write", "line number", "lower bound"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing blind spot %q in: %v", want, notes)
		}
	}
}

// A cap nothing exceeds must produce no plan at all, rather than a confident
// zero that reads as "trimming is safe here".
func TestACapNothingExceedsProducesNothing(t *testing.T) {
	lane := laneOf(toolResult("Read", "a.go", "tiny"), assistantText("ok"))
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1<<20)
	if plan.Blocks != 0 || plan.RemovedBytes != 0 {
		t.Fatalf("nothing was over the cap: %+v", plan)
	}
}

// A re-read says the block mattered. It says nothing about WHERE in the block
// the dependency sat, because the whole block was fetched again. Feeding it
// into the head/tail derivation manufactures a position out of evidence that
// has none, and the resulting table reads as a finding.
func TestReReadsCarryNoPositionAndDoNotShapeTheSplit(t *testing.T) {
	lane := laneOf(
		toolResult("Read", "b.go", body("package main", "middle", "tail")),
		assistantText("thinking"),
		toolResult("Read", "b.go", body("package main", "middle", "tail")),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)

	var reReads int
	for _, h := range plan.Harms {
		if h.Kind == HarmReRead {
			reReads++
		}
	}
	if reReads == 0 {
		t.Fatal("the fixture must produce a re-read")
	}
	for _, s := range plan.Splits {
		t.Fatalf("re-reads alone produced a split table claiming %.0f%% tail over %d samples; "+
			"there is no positional evidence here at all", s.TailShare*100, s.Samples)
	}
}

// And a real positional harm still shapes it, so the exclusion above did not
// simply switch the feature off.
func TestPositionalHarmsStillShapeTheSplit(t *testing.T) {
	lane := laneOf(
		toolResult("Read", "a.go", body("package main", "middle", "func tailThing() {}")),
		toolUse("Edit", `{"file_path":"a.go","old_string":"func tailThing() {}","new_string":"x"}`),
	)
	plan := ScoreTrim(lane, TokenFit{TokensPerByte: 0.25, Turns: 9}, 1024)
	if len(plan.Splits) == 0 {
		t.Fatal("a later-edit harm carries a real offset and must still derive a split")
	}
}
