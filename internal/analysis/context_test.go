package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Attribution of what a session's context is made of.
//
// The honesty problem this carries, and the reason the type is named for what
// it measures: Blame never subtracts. A block that entered the context and was
// later cleared by provider context editing, by compaction, or by a
// history-shrinking break is still counted. So this is content that ENTERED the
// context, not content that is in it, and the two diverge in the
// over-reporting direction on exactly the sessions where this tool's own
// leading recommendation has been followed.

func TestEnteredContextExcludesTheRebillRow(t *testing.T) {
	entries := []BlameEntry{
		{Label: RebillLabel, Tokens: Figure{Value: 900_000}, Occurrences: 4},
		{Label: "tool result: Bash ls -la", Tokens: Figure{Value: 100_000}, Occurrences: 20},
	}
	got := EnteredContext(entries)
	for _, e := range got {
		if e.Label == RebillLabel {
			t.Fatal("the rebill row describes tokens re-billed by a cache break, " +
				"which are by construction not content in the context")
		}
	}
	if len(got) != 1 || got[0].Label != "Bash" {
		t.Fatalf("want one Bash row, got %+v", got)
	}
}

// transcript.ToolLabel embeds the invocation, so a single `gh pr create` with a
// long argument list becomes its own label and outranks every other Bash call
// combined. Real sessions carry 370 to 611 distinct labels; grouped by tool
// name they collapse to a few dozen.
func TestEnteredContextGroupsByToolName(t *testing.T) {
	entries := []BlameEntry{
		{Label: "tool call: Bash", Tokens: Figure{Value: 10}, Occurrences: 1},
		{Label: "tool result: Bash gh pr create --base main --head feature", Tokens: Figure{Value: 50}, Occurrences: 1},
		{Label: "tool result: Bash npm test", Tokens: Figure{Value: 40}, Occurrences: 2},
		{Label: "tool result: WebFetch https://example.invalid/a", Tokens: Figure{Value: 30}, Occurrences: 1},
	}
	got := EnteredContext(entries)
	if len(got) != 2 {
		t.Fatalf("want Bash and WebFetch, got %d rows: %+v", len(got), got)
	}
	if got[0].Label != "Bash" || got[0].Tokens != 100 {
		t.Fatalf("Bash should sum to 100 across its three labels, got %+v", got[0])
	}
	if got[0].Occurrences != 4 {
		t.Fatalf("occurrences should sum too, got %d", got[0].Occurrences)
	}
}

func TestEnteredContextRanksByTokensNotPromptTokens(t *testing.T) {
	entries := []BlameEntry{
		// Small but carried by every request: huge PromptTokens, little context.
		{Label: "tool result: Read a.go", Tokens: Figure{Value: 100}, PromptTokens: Figure{Value: 90_000}},
		// Large but recent: little PromptTokens, dominates the context.
		{Label: "tool result: WebFetch page", Tokens: Figure{Value: 9_000}, PromptTokens: Figure{Value: 9_000}},
	}
	got := EnteredContext(entries)
	if got[0].Label != "WebFetch" {
		t.Fatalf("context is about size now, not cost across the session; got %+v", got)
	}
}

// Labels are built from tool arguments, which are model and tool supplied. They
// are sanitised where they are constructed, but anything rendering them has to
// bound them too: observed labels reach 424 runes.
func TestEnteredContextTruncatesAndStripsControlBytes(t *testing.T) {
	entries := []BlameEntry{
		{Label: "tool result: Bash " + strings.Repeat("x", 500) + "\x1b[31mred", Tokens: Figure{Value: 1}},
	}
	got := EnteredContext(entries)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if strings.ContainsRune(got[0].Label, 0x1b) {
		t.Fatalf("an escape sequence survived into a rendered label: %q", got[0].Label)
	}
	if len([]rune(got[0].Label)) > MaxContextLabel {
		t.Fatalf("label is %d runes, cap is %d", len([]rune(got[0].Label)), MaxContextLabel)
	}
}

func TestEnteredContextSharesAreOfTheAttributedTotal(t *testing.T) {
	entries := []BlameEntry{
		{Label: "tool result: Bash x", Tokens: Figure{Value: 750}},
		{Label: "tool result: Read y", Tokens: Figure{Value: 250}},
	}
	got := EnteredContext(entries)
	if got[0].Share < 0.749 || got[0].Share > 0.751 {
		t.Fatalf("want 0.75, got %v", got[0].Share)
	}
	var sum float64
	for _, e := range got {
		sum += e.Share
	}
	if sum < 0.999 || sum > 1.001 {
		t.Fatalf("shares must sum to 1 over the attributed total, got %v", sum)
	}
}

func TestEnteredContextHandlesNothing(t *testing.T) {
	if got := EnteredContext(nil); len(got) != 0 {
		t.Fatalf("want no rows, got %+v", got)
	}
	if got := EnteredContext([]BlameEntry{{Label: RebillLabel, Tokens: Figure{Value: 5}}}); len(got) != 0 {
		t.Fatalf("a rebill-only session attributes nothing, got %+v", got)
	}
}

// The gap is not a disclaimer, it is a measurement.
//
// Blame never subtracts, so an attribution overstates any session where content
// left the context. Rather than warn on every session and be ignored, detect
// the sessions where it actually happened: the provider reports what it cleared
// on the request itself, and this tool already reads those fields elsewhere.
func TestEvictionIsDetectedNotAssumed(t *testing.T) {
	clean := ContextGap{}
	if clean.Overstated() {
		t.Fatal("a session where nothing was cleared is not overstated")
	}
	if !strings.Contains(clean.Note(), "nothing was cleared") {
		t.Fatalf("a clean session should say so plainly: %q", clean.Note())
	}

	edited := ContextGap{ClearedTokens: 120_000, ContextEdits: 3, AttributedTokens: 400_000}
	if !edited.Overstated() {
		t.Fatal("a session with cleared tokens IS overstated and must say so")
	}
	note := edited.Note()
	for _, want := range []string{"120k", "3", "overstat"} {
		if !strings.Contains(note, want) {
			t.Fatalf("the note must quantify the gap; %q is missing %q", note, want)
		}
	}
	// The share is of the attributed total, which is the number being corrected.
	if s := edited.OverstatedShare(); s < 0.29 || s > 0.31 {
		t.Fatalf("120k cleared against 400k attributed is ~30%%, got %v", s)
	}
}

// A compacted session is the worst case: the attribution can exceed the window
// several times over, and saying "roughly 30%" would itself be a guess.
func TestCompactionIsCalledOutSeparately(t *testing.T) {
	g := ContextGap{Compactions: 2, AttributedTokens: 900_000}
	if !g.Overstated() {
		t.Fatal("a compacted session is overstated")
	}
	if !strings.Contains(g.Note(), "compact") {
		t.Fatalf("compaction must be named: %q", g.Note())
	}
	// No cleared-token count is reported for compaction, so no share is claimed.
	if g.OverstatedShare() != 0 {
		t.Fatalf("compaction gives no measured size, so no share may be quoted; got %v", g.OverstatedShare())
	}
}

// The overstatement a compaction causes is measured, not waved at.
//
// OverstatedShare returned zero whenever the only content leaving the context
// was a compaction, and the comment justified it: "Compaction reports no size,
// so a compacted session returns zero rather than a guess". The premise was
// false. Claude Code records preTokens and postTokens on every compaction, and
// the parser was simply not reading them.
//
// The cost of the gap is concentrated: the transcripts containing a compaction
// hold 31.8% of all re-billed tokens in the measured corpus, so the sessions
// the tool declined to quantify are the expensive ones.
//
// PASS: a recorded compaction contributes its dropped tokens to the share.
// FAIL: zero, which reports "nothing measurable" over a number on disk.
func TestGapCountsRecordedCompaction(t *testing.T) {
	g := ContextGap{AttributedTokens: 1_000_000}
	if g.OverstatedShare() != 0 {
		t.Fatal("setup: an empty gap overstates nothing")
	}
	g.CompactedTokens = 500_000
	got := g.OverstatedShare()
	if got != 0.5 {
		t.Errorf("OverstatedShare = %.3f, want 0.500: a compaction that dropped half the "+
			"attributed total overstates by half", got)
	}
	if !g.Overstated() {
		t.Error("a session with a sized compaction is overstated")
	}
}

// A compaction whose size the client did not record still says so.
//
// PASS: counted, but contributing no share, so "compacted, size unknown" stays
// distinguishable from "compacted, dropped nothing".
// FAIL: a share invented from a count.
func TestGapSizelessCompactionAddsNoShare(t *testing.T) {
	g := ContextGap{AttributedTokens: 1_000_000, Compactions: 1}
	if g.OverstatedShare() != 0 {
		t.Errorf("a compaction with no recorded size must add no share, got %.3f",
			g.OverstatedShare())
	}
	if !g.Overstated() {
		t.Error("it is still a known overstatement, just an unmeasured one")
	}
}

// A share above 100% means the denominator is wrong, and saying so beats
// printing it.
//
// Observed live: a session compacted 16 times, dropping 7.0M tokens the client
// recorded, against roughly 1M attributed. The ratio is 700%, and "these
// figures overstate by at least 700%" is not a sentence about the world - an
// attribution cannot overstate itself sevenfold. It means the attributed total
// describes only what SURVIVED, while the dropped total covers everything that
// ever passed through.
//
// PASS: the note reports the absolute figure and says the attribution covers
// what remains, without printing an impossible percentage.
// FAIL: a percentage over 100, which reads as a bug and discredits the real
// number beside it.
func TestGapNoteRefusesAnImpossibleShare(t *testing.T) {
	g := ContextGap{AttributedTokens: 1_000_000, Compactions: 16, CompactedTokens: 7_000_000}
	note := g.Note()
	if strings.Contains(note, "700%") {
		t.Errorf("printed an impossible overstatement share: %q", note)
	}
	if !strings.Contains(note, "7.0M") {
		t.Errorf("the absolute figure is real and must survive: %q", note)
	}
	// And the ordinary case still reports a share.
	ok := ContextGap{AttributedTokens: 1_000_000, Compactions: 1, CompactedTokens: 250_000}
	if !strings.Contains(ok.Note(), "25%") {
		t.Errorf("a share under 100%% must still be reported: %q", ok.Note())
	}
}

// Each recorded compaction is paired with the first prompt after it.
//
// The client records what a compaction kept; the transcript records what the
// next request then carried. The two together are the only measured account
// of what a compaction costs the turn that follows, and the pairing is by the
// boundary's instant against the lane's request timestamps.
//
// PASS: the first event pairs with the request after its boundary, not the
// one before; a boundary with no request after it reports zero.
// FAIL: every event paired with the first request, or with nothing.
func TestGapPairsEachCompactionWithTheFirstPromptAfterIt(t *testing.T) {
	t0 := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{Timestamp: at(0), Usage: transcript.Usage{CacheRead: 966_000}},
		{Timestamp: at(10), Usage: transcript.Usage{Input: 79_383}},
		{Timestamp: at(20), Usage: transcript.Usage{Input: 90_000}},
	}}
	session := &transcript.Session{Lanes: []*transcript.Lane{lane}, Compactions: []transcript.Compaction{
		{Trigger: "auto", PreTokens: 969_218, PostTokens: 26_970, At: at(5)},
		{Trigger: "auto", PreTokens: 500_000, PostTokens: 20_000, At: at(30)},
	}}
	g := MeasureGap(session, lane, 1_000_000)
	if len(g.CompactionEvents) != 2 {
		t.Fatalf("want 2 events, got %d: %+v", len(g.CompactionEvents), g.CompactionEvents)
	}
	if got := g.CompactionEvents[0].FirstPromptAfter; got != 79_383 {
		t.Errorf("first event pairs with the request AFTER its boundary (79,383), got %d", got)
	}
	if got := g.CompactionEvents[0].PostTokens; got != 26_970 {
		t.Errorf("the event carries the client's kept size, got %d", got)
	}
	if got := g.CompactionEvents[1].FirstPromptAfter; got != 0 {
		t.Errorf("no request follows the second boundary, so nothing may be reported; got %d", got)
	}
}

// The detail says what was kept and what the next prompt carried, with each
// figure labelled by how it is known.
//
// PASS: before and kept as the client recorded them, the kept share, the
// first prompt after, and the remainder marked calculated. No forward-looking
// word: the panel of 2026-10-05 ruled that nothing here predicts anything.
// FAIL: a missing figure, or a sentence that reads as a forecast.
func TestCompactionDetailSaysWhatWasKeptAndWhatCameBack(t *testing.T) {
	g := ContextGap{Compactions: 1, CompactionEvents: []CompactionEvent{
		{Trigger: "auto", PreTokens: 969_218, PostTokens: 26_970, FirstPromptAfter: 79_383},
	}}
	joined := strings.Join(g.CompactionDetail(), "\n")
	for _, want := range []string{"1 of 1 (auto):", "969k", "26k kept", "2.8%", "79k", "52k", "calculated", "client"} {
		if !strings.Contains(joined, want) {
			t.Errorf("detail lacks %q:\n%s", want, joined)
		}
	}
	for _, banned := range []string{"approach", "will ", "expect", "forecast", "window", "predict"} {
		if strings.Contains(strings.ToLower(joined), banned) {
			t.Errorf("detail must not read as a forecast, found %q:\n%s", banned, joined)
		}
	}
}

// A boundary with no request after it says so instead of subtracting zero.
func TestCompactionDetailWithoutAFollowingPromptSaysSo(t *testing.T) {
	g := ContextGap{Compactions: 1, CompactionEvents: []CompactionEvent{
		{PreTokens: 969_218, PostTokens: 26_970},
	}}
	joined := strings.Join(g.CompactionDetail(), "\n")
	if !strings.Contains(joined, "no prompt is recorded after the boundary") {
		t.Errorf("an absent next prompt must be named, not computed as zero:\n%s", joined)
	}
	if strings.Contains(joined, "calculated") {
		t.Errorf("nothing is calculated from an absent prompt:\n%s", joined)
	}
}

// A rewrite recorded without sizes is still listed, and says so.
func TestCompactionDetailWithoutSizesSaysSo(t *testing.T) {
	g := ContextGap{Compactions: 1, CompactionEvents: []CompactionEvent{{FirstPromptAfter: 50_000}}}
	joined := strings.Join(g.CompactionDetail(), "\n")
	if !strings.Contains(joined, "1 of 1: the rewrite was recorded without its sizes") {
		t.Errorf("an unsized rewrite must be named as such, with no trigger invented for it:\n%s", joined)
	}
	if strings.Contains(joined, "0 tokens before") || strings.Contains(joined, "kept (") {
		t.Errorf("no size may be printed for an unsized rewrite:\n%s", joined)
	}
}

// An inferred compaction has no client record to detail.
func TestCompactionDetailIsSilentOnAnInferredCompaction(t *testing.T) {
	g := ContextGap{Compactions: 1, InferredCompactions: 1}
	if got := g.CompactionDetail(); got != nil {
		t.Errorf("nothing was recorded, so there is nothing to detail; got %q", got)
	}
}

// A record that kept more than it had gets no share.
//
// One record in the measured corpus reports postTokens above preTokens
// (22,303 to 296,742). A kept share of 1330% would discredit every figure
// beside it, so the sizes are printed as recorded and the share is withheld.
func TestCompactionDetailWithholdsAShareAboveOneHundred(t *testing.T) {
	g := ContextGap{Compactions: 1, CompactionEvents: []CompactionEvent{
		{PreTokens: 22_303, PostTokens: 296_742, FirstPromptAfter: 300_000},
	}}
	joined := strings.Join(g.CompactionDetail(), "\n")
	if strings.Contains(joined, "kept (") || strings.Contains(joined, "%") {
		t.Errorf("no share may be derived when kept exceeds before:\n%s", joined)
	}
	if !strings.Contains(joined, "more than before as the client recorded it") {
		t.Errorf("the record must be named as it is:\n%s", joined)
	}
}
