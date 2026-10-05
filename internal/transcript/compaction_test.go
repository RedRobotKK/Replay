package transcript

import (
	"strings"
	"testing"
	"time"
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

// CM-5: a compaction is a PAIR of records, and counts once.
//
// The client writes two lines. A boundary - type "system", subtype
// "compact_boundary", carrying compactMetadata and no message - and then the
// summary on the next line: type "user", isCompactSummary true, the text in
// message.content.
//
// The parser accepted either marker and appended one Compaction per RECORD, so
// every compaction was counted twice. Measured on a real session: 2 boundary
// records and 2 summary records, and `replay context` reported 4 compactions.
// The overstatement note derived from that count was doubled with it.
//
// PASS: one boundary plus its summary is one compaction, carrying the sizes
// from the boundary.
// FAIL: two, which is what shipped.
func TestCM5_ABoundaryAndItsSummaryAreOneCompaction(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(`
{"type":"system","subtype":"compact_boundary","uuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:00Z","compactMetadata":{"trigger":"auto","preTokens":999029,"postTokens":23218}}
{"type":"user","uuid":"u1","parentUuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:01Z","isCompactSummary":true,"message":{"role":"user","content":"## 1. Primary Request and Intent\nsomething"}}
{"type":"assistant","uuid":"a1","sessionId":"s","timestamp":"2026-09-06T00:00:02Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":2}}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Compactions) != 1 {
		t.Fatalf("a boundary and its summary are ONE compaction, got %d: %+v",
			len(s.Compactions), s.Compactions)
	}
	if got := s.Compactions[0].PreTokens; got != 999029 {
		t.Errorf("the pair must carry the boundary's sizes, got PreTokens %d", got)
	}
}

// A compaction is placed in time by the boundary record that carries it.
//
// The sizes were read and the instant was not, so nothing downstream could
// say which request came first after the rewrite, and the one figure that
// describes what a compaction costs the next turn, the first prompt after it,
// could not be paired with it.
//
// PASS: the parsed compaction carries the boundary's timestamp.
// FAIL: a zero time, which places every compaction before every request.
func TestCompactionCarriesTheBoundaryInstant(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(`
{"type":"system","subtype":"compact_boundary","uuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:05Z","compactMetadata":{"trigger":"auto","preTokens":999029,"postTokens":23218}}
{"type":"user","uuid":"u1","parentUuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:06Z","isCompactSummary":true,"message":{"role":"user","content":"summary"}}
{"type":"assistant","uuid":"a1","sessionId":"s","timestamp":"2026-09-06T00:00:07Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"output_tokens":2}}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Compactions) != 1 {
		t.Fatalf("want one compaction, got %d", len(s.Compactions))
	}
	want := time.Date(2026, 9, 6, 0, 0, 5, 0, time.UTC)
	if got := s.Compactions[0].At; !got.Equal(want) {
		t.Errorf("Compaction.At = %v, want the boundary's own timestamp %v", got, want)
	}
}

// The conversation after a compaction is the same lane as the one before.
//
// A lane is keyed on the first conversational line of a request's parent
// chain. The client starts a compaction with a parentless boundary, hangs the
// summary off it, and hangs every later turn off the summary, so the chain
// of a post-compaction request is rooted at the summary and the parser filed
// it as a new lane. Measured on one real session: 46 boundaries, 46
// parentless records, all of them boundaries, no sidechain record at all,
// and `replay context` reported "one lane of 46, 45 sub-agent lanes, carrying
// 24,666 requests, are not counted above". Every offline command reports on
// MainLane, so each of them saw one segment of a compacted session and called
// the rest sub-agents.
//
// PASS: one lane carrying both requests; a true sidechain stays its own lane.
// FAIL: two lanes, which is what shipped.
func TestCM6_AContinuationAfterACompactionIsTheSameLane(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(`
{"type":"user","uuid":"u0","sessionId":"s","timestamp":"2026-09-06T00:00:00Z","message":{"role":"user","content":"start"}}
{"type":"assistant","uuid":"a0","parentUuid":"u0","sessionId":"s","timestamp":"2026-09-06T00:00:01Z","requestId":"r0","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"working"}],"usage":{"input_tokens":1000,"cache_read_input_tokens":965000,"output_tokens":20}}}
{"type":"user","uuid":"s0","sessionId":"s","timestamp":"2026-09-06T00:00:02Z","isSidechain":true,"message":{"role":"user","content":"sub-agent brief"}}
{"type":"assistant","uuid":"sa0","parentUuid":"s0","sessionId":"s","timestamp":"2026-09-06T00:00:03Z","isSidechain":true,"requestId":"rs","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"sub"}],"usage":{"input_tokens":50,"output_tokens":5}}}
{"type":"system","subtype":"compact_boundary","uuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:05Z","compactMetadata":{"trigger":"auto","preTokens":969218,"postTokens":26970}}
{"type":"user","uuid":"u1","parentUuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:06Z","isCompactSummary":true,"message":{"role":"user","content":"summary"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","sessionId":"s","timestamp":"2026-09-06T00:00:07Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"continuing"}],"usage":{"input_tokens":79383,"output_tokens":20}}}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var main, side int
	for _, l := range s.Lanes {
		if l.Sidechain {
			side++
			continue
		}
		main++
		if len(l.Requests) != 2 {
			t.Errorf("the main lane carries the request before and after the compaction, got %d", len(l.Requests))
		}
	}
	if main != 1 || side != 1 {
		t.Fatalf("want one main lane and one sidechain, got %d main, %d sidechain (%d lanes)", main, side, len(s.Lanes))
	}
}
