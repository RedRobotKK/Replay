package analysis

import (
	"strings"
	"testing"
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
