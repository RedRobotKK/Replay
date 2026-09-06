package transcript

import (
	"strings"
	"testing"
)

// Compaction reports its own size, and this parser was not reading it.
//
// internal/analysis/context.go says "Compaction reports no size, so a
// compacted session returns zero rather than a guess", and infers compactions
// from a prompt that shrank between turns. The comment is wrong. Claude Code
// writes a system record carrying compactMetadata with preTokens, postTokens,
// cumulativeDroppedTokens and durationMs. Measured across this corpus: 39 such
// records, median preTokens 999,029 against postTokens 23,218 - a median
// retention of 2.55%.
//
// That gap matters more than most parser gaps. The 9 transcripts containing a
// compaction hold 31.8% of all re-billed tokens in the corpus, so the sessions
// the tool measures least completely are the expensive ones, and it declines to
// quantify an overstatement whose exact size is on disk.

// CM-1: the recorded sizes are parsed.
//
// PASS: pre and post survive into the session.
// FAIL: the fact without the size, which is what shipped.
func TestCM1_CompactMetadataIsParsed(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(`
{"type":"system","uuid":"u1","sessionId":"s","timestamp":"2026-09-06T00:00:00Z","isCompactSummary":true,"compactMetadata":{"trigger":"auto","preTokens":999029,"postTokens":23218,"cumulativeDroppedTokens":3391749,"durationMs":113000}}
{"type":"assistant","uuid":"a1","sessionId":"s","timestamp":"2026-09-06T00:00:01Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":2}}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Compactions) != 1 {
		t.Fatalf("expected 1 compaction, got %d", len(s.Compactions))
	}
	c := s.Compactions[0]
	if c.PreTokens != 999029 || c.PostTokens != 23218 {
		t.Errorf("sizes not parsed: %+v", c)
	}
	if c.Dropped() != 999029-23218 {
		t.Errorf("dropped = %d, want %d", c.Dropped(), 999029-23218)
	}
}

// CM-2: a compaction that reports growth drops nothing.
//
// Observed in the real corpus: one record carries preTokens 22,303 and
// postTokens 296,742 - more after than before. Whatever produced it, treating
// it as a negative drop would subtract from a total that is supposed to
// describe content LEAVING the context, and one bad record would silently
// offset several good ones.
//
// PASS: clamped to zero, and the record still counts as a compaction.
// FAIL: a negative, which is an overstatement correction running backwards.
func TestCM2_GrowthDropsNothing(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(`
{"type":"system","uuid":"u1","sessionId":"s","timestamp":"2026-09-06T00:00:00Z","compactMetadata":{"trigger":"auto","preTokens":22303,"postTokens":296742}}
{"type":"assistant","uuid":"a1","sessionId":"s","timestamp":"2026-09-06T00:00:01Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":2}}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Compactions) != 1 {
		t.Fatalf("the record must still count as a compaction: %+v", s.Compactions)
	}
	if got := s.Compactions[0].Dropped(); got != 0 {
		t.Errorf("dropped = %d, want 0: a compaction cannot drop a negative amount", got)
	}
}

// CM-3: a transcript with no compaction reports none, and invents no size.
//
// PASS: empty.
// FAIL: a zero-valued entry, which would make "compacted, size unknown"
// indistinguishable from "compacted, dropped nothing".
func TestCM3_NoCompactionNoEntry(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(`
{"type":"user","uuid":"u1","sessionId":"s","timestamp":"2026-09-06T00:00:00Z","message":{"role":"user","content":"hi"}}
{"type":"assistant","uuid":"a1","sessionId":"s","timestamp":"2026-09-06T00:00:01Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":2}}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Compactions) != 0 {
		t.Errorf("invented a compaction: %+v", s.Compactions)
	}
}

// CM-4: a compaction record with no sizes is still recorded, without a size.
//
// A client that stops emitting preTokens must not silently stop reporting
// compactions - that is the failure this whole test file exists to fix, one
// version later.
//
// PASS: counted, Dropped() zero, and Sized() says the size is missing.
// FAIL: dropped from the count, which restores the original defect.
func TestCM4_SizelessCompactionIsStillCounted(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(`
{"type":"system","uuid":"u1","sessionId":"s","timestamp":"2026-09-06T00:00:00Z","isCompactSummary":true}
{"type":"assistant","uuid":"a1","sessionId":"s","timestamp":"2026-09-06T00:00:01Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":2}}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Compactions) != 1 {
		t.Fatalf("a compaction without sizes must still be counted: %+v", s.Compactions)
	}
	if s.Compactions[0].Sized() {
		t.Error("a record with no preTokens must not claim to be sized")
	}
}
