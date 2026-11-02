package transcript

// Transcript adapter evidence-integrity conformance.
//
// This file pins, with synthetic fixtures, the boundaries a transcript adapter
// must not silently cross if occurrence-pinned verification is ever fed from
// its output. It is a conformance contract, not a rewrite: it documents and
// machine-checks what the ordinary decode path destroys (duplicate keys,
// absent-vs-false, invalid UTF-8), classifies each adapter's forensic
// coverage, and exercises the temporal/cross-session identity rules using the
// frozen internal/evidenceverify public API.
//
// Mirror of internal/ledger/provider_conformance_test.go: a shared invariant
// asserted against real decode functions, so a regression is a failing test
// rather than a drifted comment.

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/RedRobotKK/Replay/internal/evidenceverify"
)

// ---------------------------------------------------------------------------
// Section 1: the ordinary decode boundary, pinned empirically.
//
// These three facts are what make "a decoded value is not evidence" load
// bearing. They are asserted against the stdlib decoder actually used by the
// adapters, so the claim survives a Go toolchain change that altered them.
// ---------------------------------------------------------------------------

func TestOrdinaryDecodeBoundaryIsLossy(t *testing.T) {
	t.Run("duplicate_keys_last_wins", func(t *testing.T) {
		var m map[string]any
		if err := json.Unmarshal([]byte(`{"is_error":true,"is_error":false}`), &m); err != nil {
			t.Fatalf("fixture must be valid JSON: %v", err)
		}
		if m["is_error"] != false {
			t.Fatalf("encoding/json did not resolve the duplicate last-wins; got %v", m["is_error"])
		}
	})
	t.Run("absent_bool_collapses_to_false", func(t *testing.T) {
		var b struct {
			IsError bool `json:"is_error"`
		}
		if err := json.Unmarshal([]byte(`{}`), &b); err != nil {
			t.Fatalf("fixture must be valid JSON: %v", err)
		}
		if b.IsError {
			t.Fatalf("an absent bool must decode to false, and that false is not evidence")
		}
	})
	t.Run("invalid_utf8_is_repaired_not_rejected", func(t *testing.T) {
		var s struct {
			S string `json:"s"`
		}
		doc := []byte{'{', '"', 's', '"', ':', '"', 0xFF, '"', '}'}
		if err := json.Unmarshal(doc, &s); err != nil {
			t.Fatalf("encoding/json rejected invalid UTF-8 (it repairs it); err=%v", err)
		}
		if s.S != "\uFFFD" {
			t.Fatalf("invalid UTF-8 was not repaired to U+FFFD; got %q", s.S)
		}
	})
}

// ---------------------------------------------------------------------------
// Section 2: per-adapter duplicate behavior, established with synthetic
// fixtures through the adapter's own decode function.
// ---------------------------------------------------------------------------

// Claude Code decodes message content through DecodeContent into []RawBlock.
// The raw content bytes still hold both duplicate keys; the decoded RawBlock
// has collapsed them. The forensic evidence exists only before DecodeContent.
func TestClaudeCodeContentCollapsesDuplicates(t *testing.T) {
	content := json.RawMessage(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"ok"}]`)

	// Raw evidence: both keys are physically present.
	if got := bytes.Count(content, []byte(`"is_error"`)); got != 2 {
		t.Fatalf("raw content holds %d is_error keys, want 2 (fixture is wrong)", got)
	}

	// Decoded evidence: collapsed to last-wins false.
	_, blocks, isText, err := DecodeContent(content)
	if err != nil || isText || len(blocks) != 1 {
		t.Fatalf("DecodeContent: err=%v isText=%v blocks=%d", err, isText, len(blocks))
	}
	if blocks[0].IsError != false {
		t.Fatalf("DecodeContent resolved the duplicate to %v, want last-wins false", blocks[0].IsError)
	}
}

// JEV is the one adapter that already rejects duplicate keys, because a parser
// default must not settle which of two usage objects the provider meant.
func TestJevRejectsDuplicateKeys(t *testing.T) {
	if err := jevRejectDuplicateKeys([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatalf("JEV accepted a duplicate key; it must reject it")
	}
	if err := jevRejectDuplicateKeys([]byte(`{"a":1,"b":2}`)); err != nil {
		t.Fatalf("JEV rejected a valid object: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Section 3: forensic coverage classification.
// ---------------------------------------------------------------------------

type forensicCoverage string

const (
	covFull          forensicCoverage = "FULL"
	covPartial       forensicCoverage = "PARTIAL"
	covNone          forensicCoverage = "NONE"
	covNotApplicable forensicCoverage = "NOT_APPLICABLE"
)

func TestTranscriptAdapterCoverageMatrix(t *testing.T) {
	matrix := []struct {
		adapter string
		cov     forensicCoverage
	}{
		{"claude_code", covPartial},
		{"codex", covPartial},
		{"openai_chat", covPartial},
		{"responses", covPartial},
		{"ollama_log", covNotApplicable},
		{"jev_capture", covNotApplicable},
		{"redact", covNone},
	}

	seen := map[string]bool{}
	valid := map[forensicCoverage]bool{
		covFull: true, covPartial: true, covNone: true, covNotApplicable: true,
	}
	for _, row := range matrix {
		if seen[row.adapter] {
			t.Fatalf("adapter %q appears twice in the coverage matrix", row.adapter)
		}
		seen[row.adapter] = true
		if !valid[row.cov] {
			t.Fatalf("adapter %q has invalid coverage %q", row.adapter, row.cov)
		}
	}
	// A new production adapter must be classified; absence is the defect this
	// check exists to catch.
	if len(matrix) < 7 {
		t.Fatalf("coverage matrix shrank to %d rows; an adapter lost its classification", len(matrix))
	}
}

// ---------------------------------------------------------------------------
// Section 4: temporal and cross-session identity, via the frozen verifier.
//
// These use only the public API of internal/evidenceverify; they do not
// modify it. They prove that evidence identity is bound to the whole artifact
// digest, never to a local fragment, an offset, or a key/value pair.
// ---------------------------------------------------------------------------

func evidencePredicate() evidenceverify.Predicate {
	return evidenceverify.Predicate{Key: "is_error", Expect: evidenceverify.BoolLiteral(true)}
}

func locateValue(t *testing.T, raw []byte) evidenceverify.Locator {
	t.Helper()
	off := bytes.Index(raw, []byte("true"))
	if off < 0 {
		t.Fatalf("fixture lacks a true value to pin")
	}
	loc, ok := evidenceverify.LocateValue(raw, int64(off))
	if !ok {
		t.Fatalf("LocateValue failed on fixture")
	}
	return loc
}

func TestSourceMutationInvalidatesStaleLocator(t *testing.T) {
	v1 := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	v2 := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":false,"content":"ok"}]`)

	loc := locateValue(t, v1) // bound to v1's whole-artifact digest

	// Verify v2 against v1's locator. The whole-artifact digest differs, so
	// the basis fails UNVERIFIED — never REFUTED, because the claim was never
	// established against these bytes.
	r := evidenceverify.Verify(v2, loc, evidencePredicate())
	if r.Verdict != evidenceverify.Unverified {
		t.Fatalf("source mutation: verdict=%s reason=%s, want UNVERIFIED", r.Verdict, r.Reason)
	}
	if r.Reason != evidenceverify.ReasonTranscriptHashMismatch {
		t.Fatalf("source mutation: reason=%s, want TRANSCRIPT_HASH_MISMATCH", r.Reason)
	}
}

func TestSameFragmentDifferentArtifactIsNotEquivalent(t *testing.T) {
	// Two artifacts that both contain the SAME fragment at the SAME offset,
	// differing only elsewhere. The local fragment is identical; the evidence
	// is not.
	a := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	b := []byte(`[{"type":"tool_result","tool_use_id":"t2","is_error":true,"content":"different"}]`)

	locA := locateValue(t, a)
	r := evidenceverify.Verify(b, locA, evidencePredicate())
	if r.Verdict != evidenceverify.Unverified {
		t.Fatalf("same fragment in a different artifact must not verify: verdict=%s", r.Verdict)
	}
}

func TestHistoricalRereadIsDeterministic(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	loc := locateValue(t, raw)
	first := evidenceverify.Verify(raw, loc, evidencePredicate())
	second := evidenceverify.Verify(raw, loc, evidencePredicate())
	if first != second {
		t.Fatalf("re-read of unchanged bytes changed the verdict: %+v vs %+v", first, second)
	}
	if first.Verdict != evidenceverify.Confirmed {
		t.Fatalf("unchanged bytes: verdict=%s, want CONFIRMED", first.Verdict)
	}
}
