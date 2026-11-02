package analysis

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Hostile tests for the evidence-aware production path. Each pins one of the
// forbidden mappings from the bundle contract.

// H4: when forensic evidence is present, the collapsed decoded bool must never
// substitute for it. A conflicting duplicate decodes to IsError=false, but the
// evidence says conflict; the production path must not read false as clean.
func TestH4_EvidenceNeverFallsBackToDecodedBool(t *testing.T) {
	// Build a Block that already carries a conflict flag AND a decoded false
	// (exactly what the adapter produces after the collapse).
	b := transcript.Block{
		Kind: transcript.KindToolResult, Label: "tool result: x",
		Bytes: 4, Text: "boom", IsError: false, // collapsed
		Evidence: flagPtr(transcript.ErrorFlagConflict),
	}
	_, known := evidenceErrorState(b)
	if !known {
		t.Fatalf("evidence must be known when a flag is present")
	}
	// A conflict is neither error nor clean; classifyResult with the
	// evidence-derived isError must not report an error.
	class, ok := classifyResult(b, false)
	if ok {
		t.Fatalf("conflicting duplicate classified as %q; it must be unresolved", class)
	}
}

// H8/H3: a verified-false or absent block is not the same as a decoded false,
// but neither may be counted as an error. The distinction is carried as
// evidence, not guessed.
func TestH8_VerifiedNotErrorIsNotGuessed(t *testing.T) {
	b := transcript.Block{
		Kind: transcript.KindToolResult, Label: "tool result: x",
		Bytes: 4, Text: "ok", IsError: true, // decoded would be true, but evidence says false
		Evidence: flagPtr(transcript.ErrorFlagFalse),
	}
	isError, known := evidenceErrorState(b)
	if !known || isError {
		t.Fatalf("verified false must not read as error (known=%v isError=%v)", known, isError)
	}
}

// H7: an ambiguous occurrence must never contribute to the error cost.
func TestH7_AmbiguousContributesNoErrorCost(t *testing.T) {
	s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"boom"}`)
	costs, ev := errorCostsFor(t, s)
	if ev.Unresolved != 1 {
		t.Fatalf("unresolved = %d, want 1", ev.Unresolved)
	}
	for _, c := range costs {
		if c.Count > 0 && (c.Class == ErrorToolFailed || c.Class == ErrorEditAnchor) {
			t.Fatalf("ambiguous occurrence contributed to %s: %+v", c.Class, c)
		}
	}
}

// H11: the ErrorEvidence travels with the costs so a consumer cannot silently
// drop the distinction between verified clean and ambiguous.
func TestH11_ErrorEvidenceTravelsWithCosts(t *testing.T) {
	s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"boom"}`)
	lane := MainLane(s)
	rep := AnalyzeLane(s, lane)
	if rep.ErrorEvidence.Unresolved != 1 {
		t.Fatalf("LaneReport.ErrorEvidence.Unresolved = %d, want 1", rep.ErrorEvidence.Unresolved)
	}
}

// H10/H5: a non-Claude-Code adapter (no evidence) must never be labeled
// forensically verified — it is Legacy, and the decoded bool still decides.
func TestH10_LegacyIsNeverLabeledVerified(t *testing.T) {
	b := transcript.Block{
		Kind: transcript.KindToolResult, Label: "tool result: x",
		Bytes: 4, Text: "boom", IsError: true, Evidence: nil,
	}
	_, known := evidenceErrorState(b)
	if known {
		t.Fatalf("a block with no evidence must report unknown, not verified")
	}
	// Legacy falls back to the decoded bool.
	class, ok := classifyResult(b, b.IsError)
	if !ok || class != ErrorToolFailed {
		t.Fatalf("legacy decoded true not classified: %q %v", class, ok)
	}
}

func flagPtr(f transcript.ErrorFlag) *transcript.ErrorFlag { return &f }
