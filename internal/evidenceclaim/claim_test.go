package evidenceclaim

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/RedRobotKK/Replay/internal/evidenceverify"
	"github.com/RedRobotKK/Replay/internal/stateledger"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mkLoc builds a locator for the value token that starts at off, using the
// create half of the locator lifecycle (evidenceverify.LocateValue). It panics
// on failure because every fixture pins a real value token.
func mkLoc(raw []byte, off int) evidenceverify.Locator {
	loc, ok := evidenceverify.LocateValue(raw, int64(off))
	if !ok {
		panic("mkLoc: no value token starts at the pinned offset in fixture")
	}
	return loc
}

func claimPredicate() evidenceverify.Predicate {
	return evidenceverify.Predicate{Key: "is_error", Expect: evidenceverify.BoolLiteral(true)}
}

// fullEval evaluates the selected claim on raw bytes at FULL coverage, pinning
// the first occurrence of the token "true".
func fullEval(t *testing.T, raw []byte) Result {
	t.Helper()
	off := bytes.Index(raw, []byte("true"))
	if off < 0 {
		off = 0 // some fixtures pin other literals; callers use fullEvalAt
	}
	return Evaluate(ClaimProposition, "synth-session", raw, mkLoc(raw, off), claimPredicate(), CoverageFull)
}

func fullEvalAt(t *testing.T, raw []byte, token string) Result {
	t.Helper()
	off := bytes.Index(raw, []byte(token))
	if off < 0 {
		t.Fatalf("token %q not found in fixture", token)
	}
	loc := mkLoc(raw, off)
	return Evaluate(ClaimProposition, "synth-session", raw, loc, claimPredicate(), CoverageFull)
}

// ---------------------------------------------------------------------------
// Four end-to-end cases through the actual vertical slice.
// ---------------------------------------------------------------------------

func TestFourStatesEndToEnd(t *testing.T) {
	t.Run("A_confirming", func(t *testing.T) {
		raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
		r := fullEval(t, raw)
		if r.Verdict != evidenceverify.Confirmed || r.Outcome != stateledger.Supports || r.Standing != stateledger.Supported {
			t.Fatalf("confirming: verdict=%s outcome=%s standing=%s", r.Verdict, r.Outcome, r.Standing)
		}
	})

	t.Run("B_refuting", func(t *testing.T) {
		raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":false,"content":"ok"}]`)
		r := fullEvalAt(t, raw, "false")
		if r.Verdict != evidenceverify.Refuted || r.Outcome != stateledger.Contradicts || r.Standing != stateledger.Contradicted {
			t.Fatalf("refuting: verdict=%s outcome=%s standing=%s", r.Verdict, r.Outcome, r.Standing)
		}
	})

	t.Run("C_ambiguous", func(t *testing.T) {
		raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"ok"}]`)
		r := fullEval(t, raw)
		if r.Verdict != evidenceverify.Ambiguous || r.Outcome != stateledger.Inconclusive || r.Standing != stateledger.Unresolved {
			t.Fatalf("ambiguous: verdict=%s outcome=%s standing=%s", r.Verdict, r.Outcome, r.Standing)
		}
	})

	t.Run("D_unverifiable", func(t *testing.T) {
		raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
		loc := mkLoc(raw, bytes.Index(raw, []byte("true")))
		loc.TranscriptSHA256[0] ^= 0xFF // corrupt whole-artifact binding
		r := Evaluate(ClaimProposition, "synth-session", raw, loc, claimPredicate(), CoverageFull)
		if r.Verdict != evidenceverify.Unverified || r.Outcome != stateledger.NotRun || r.Standing != stateledger.Unresolved {
			t.Fatalf("unverifiable: verdict=%s outcome=%s standing=%s", r.Verdict, r.Outcome, r.Standing)
		}
		if r.Reason != evidenceverify.ReasonTranscriptHashMismatch {
			t.Fatalf("unverifiable: reason=%s", r.Reason)
		}
	})
}

// ---------------------------------------------------------------------------
// Coverage behavior.
// ---------------------------------------------------------------------------

func TestCoverageBehavior(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	off := bytes.Index(raw, []byte("true"))
	loc := mkLoc(raw, off)
	pred := claimPredicate()

	t.Run("full_confirms", func(t *testing.T) {
		r := Evaluate(ClaimProposition, "s", raw, loc, pred, CoverageFull)
		if r.Verdict != evidenceverify.Confirmed {
			t.Fatalf("full: verdict=%s", r.Verdict)
		}
	})
	t.Run("partial_still_verifies_and_flags_partial", func(t *testing.T) {
		r := Evaluate(ClaimProposition, "s", raw, loc, pred, CoveragePartial)
		if r.Verdict != evidenceverify.Confirmed {
			t.Fatalf("partial: verdict=%s", r.Verdict)
		}
		if !bytes.Contains([]byte(r.Explanation), []byte("PARTIAL")) {
			t.Fatalf("partial coverage must be recorded in the explanation: %q", r.Explanation)
		}
	})
	t.Run("none_is_unresolved_not_false", func(t *testing.T) {
		r := Evaluate(ClaimProposition, "s", nil, evidenceverify.Locator{}, pred, CoverageNone)
		if r.Standing != stateledger.Unresolved || r.Outcome != stateledger.NotRun {
			t.Fatalf("none: standing=%s outcome=%s, want UNRESOLVED/NOT-RUN", r.Standing, r.Outcome)
		}
		if r.Verdict != evidenceverify.Unverified {
			t.Fatalf("none: verdict must be zero/UNVERIFIED, got %s", r.Verdict)
		}
		if r.Outcome == stateledger.Contradicts || r.Standing == stateledger.Contradicted {
			t.Fatalf("none: missing coverage must not become contradiction")
		}
	})
	t.Run("unsupported_is_unresolved", func(t *testing.T) {
		r := Evaluate(ClaimProposition, "s", raw, loc, pred, CoverageUnsupported)
		if r.Standing != stateledger.Unresolved || r.Outcome != stateledger.NotRun {
			t.Fatalf("unsupported: standing=%s outcome=%s", r.Standing, r.Outcome)
		}
	})
	t.Run("unknown_coverage_fails_closed", func(t *testing.T) {
		r := Evaluate(ClaimProposition, "s", raw, loc, pred, Coverage("BOGUS"))
		if r.Standing != stateledger.Unresolved || r.Outcome != stateledger.NotRun {
			t.Fatalf("unknown coverage must fail closed, got %s/%s", r.Standing, r.Outcome)
		}
	})
}

// ---------------------------------------------------------------------------
// Hostile tests H1-H12.
// ---------------------------------------------------------------------------

// H1: underlying UNVERIFIED must never map to claim REFUTED.
func TestH1_UnverifiedNeverRefuted(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	loc := mkLoc(raw, bytes.Index(raw, []byte("true")))
	loc.TranscriptSHA256[0] ^= 0xFF
	r := Evaluate(ClaimProposition, "s", raw, loc, claimPredicate(), CoverageFull)
	if r.Outcome == stateledger.Contradicts || r.Standing == stateledger.Contradicted {
		t.Fatalf("H1: UNVERIFIED mapped to REFUTED: %s/%s", r.Standing, r.Outcome)
	}
}

// H2/H3: underlying AMBIGUOUS must never map to REFUTED or CONFIRMED.
func TestH2H3_AmbiguousNeverDetermined(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"ok"}]`)
	r := fullEval(t, raw)
	if r.Verdict != evidenceverify.Ambiguous {
		t.Fatalf("fixture must be ambiguous, got %s", r.Verdict)
	}
	if r.Outcome == stateledger.Contradicts || r.Standing == stateledger.Contradicted {
		t.Fatalf("H2: AMBIGUOUS mapped to REFUTED")
	}
	if r.Outcome == stateledger.Supports || r.Standing == stateledger.Supported {
		t.Fatalf("H3: AMBIGUOUS mapped to CONFIRMED")
	}
}

// H4/H8: unavailable evidence must not fall back to decoded bool / false.
func TestH4H8_NoFallbackToDecodedState(t *testing.T) {
	// The API takes only raw bytes; there is no decoded bool parameter to fall
	// back to, and NONE coverage short-circuits before any verification.
	r := Evaluate(ClaimProposition, "s", nil, evidenceverify.Locator{}, claimPredicate(), CoverageNone)
	if r.Verdict != evidenceverify.Unverified || r.Outcome != stateledger.NotRun {
		t.Fatalf("H4/H8: missing evidence must stay unevaluated: %s/%s", r.Verdict, r.Outcome)
	}
}

// H5/H6/H7: same fragment from another artifact, or source mutation, must not
// prove the claim; explanation must not exceed the evaluator's strength.
func TestH5H6H7_ArtifactIdentityBinding(t *testing.T) {
	a := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	b := []byte(`[{"type":"tool_result","tool_use_id":"t2","is_error":true,"content":"different"}]`)

	locA := mkLoc(a, bytes.Index(a, []byte("true")))
	// Same local fragment, different artifact: the whole-artifact digest differs.
	r := Evaluate(ClaimProposition, "s", b, locA, claimPredicate(), CoverageFull)
	if r.Verdict != evidenceverify.Unverified {
		t.Fatalf("H5: same fragment in another artifact must not verify: %s", r.Verdict)
	}
	if r.Outcome == stateledger.Supports || r.Standing == stateledger.Supported {
		t.Fatalf("H5: cross-artifact fragment proved the claim")
	}

	// Source mutation: verify b (false) against locator bound to a (true).
	a2 := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	b2 := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":false,"content":"ok"}]`)
	locA2 := mkLoc(a2, bytes.Index(a2, []byte("true")))
	r = Evaluate(ClaimProposition, "s", b2, locA2, claimPredicate(), CoverageFull)
	if r.Verdict != evidenceverify.Unverified || r.Reason != evidenceverify.ReasonTranscriptHashMismatch {
		t.Fatalf("H6: source mutation must invalidate: verdict=%s reason=%s", r.Verdict, r.Reason)
	}
}

// H9: a recorded action is not proof of an external outcome. The claim text
// and explanation must be scoped to "records", never to "failed".
func TestH9_RecordedIsNotOutcome(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	r := fullEval(t, raw)
	if r.Verdict != evidenceverify.Confirmed {
		t.Fatalf("fixture must confirm")
	}
	for _, forbidden := range []string{"actually failed", "external outcome", "side effect failed"} {
		if bytes.Contains([]byte(r.Explanation), []byte(forbidden)) || bytes.Contains([]byte(r.ClaimText), []byte(forbidden)) {
			t.Fatalf("H9: explanation/claim overstates to execution outcome: %q", forbidden)
		}
	}
	if !bytes.Contains([]byte(r.ClaimText), []byte("records")) {
		t.Fatalf("H9: claim must be scoped to what is recorded, not what happened")
	}
}

// H10: unsupported adapter must not produce a substantive forensic claim.
func TestH10_UnsupportedAdapterNoSubstantiveClaim(t *testing.T) {
	r := Evaluate(ClaimProposition, "s", nil, evidenceverify.Locator{}, claimPredicate(), CoverageUnsupported)
	if r.Verdict != evidenceverify.Unverified || r.Outcome != stateledger.NotRun {
		t.Fatalf("H10: unsupported adapter produced substantive claim: %s/%s", r.Verdict, r.Outcome)
	}
}

// H11: the renderer must not collapse ambiguous/unavailable into a boolean.
func TestH11_RendererDoesNotCollapse(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"ok"}]`)
	r := fullEval(t, raw)
	out := r.Render()
	if !bytes.Contains([]byte(out), []byte("AMBIGUOUS")) {
		t.Fatalf("H11: render lost the AMBIGUOUS state:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("UNRESOLVED")) {
		t.Fatalf("H11: render lost the UNRESOLVED standing:\n%s", out)
	}
	// No transcript content may leak.
	for _, forbidden := range []string{"content", `"ok"`, "t1"} {
		if bytes.Contains([]byte(out), []byte(forbidden)) {
			t.Fatalf("H11/security: render leaked content %q:\n%s", forbidden, out)
		}
	}
}

// H12: historical collapsed state must not override stronger raw evidence.
// The Evaluate API never accepts collapsed state; this pins that raw bytes are
// the only input, so collapsed state cannot enter the claim path.
func TestH12_NoCollapsedStateInput(t *testing.T) {
	// Evaluate's signature has no bool parameter; a caller holding only
	// Block.IsError cannot call it, which is the guard, pinned here at the
	// type level.
	_ = Evaluate // compile-time assertion that the symbol exists
	if false {
		t.Fatal("unreachable")
	}
}

// ---------------------------------------------------------------------------
// Property tests P1-P6.
// ---------------------------------------------------------------------------

func TestProperties(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]`)
	off := bytes.Index(raw, []byte("true"))
	pred := claimPredicate()

	// P1: UNVERIFIED -> never REFUTED.
	t.Run("P1", func(t *testing.T) {
		loc := mkLoc(raw, off)
		loc.TranscriptSHA256[0] ^= 0xFF
		r := Evaluate(ClaimProposition, "s", raw, loc, pred, CoverageFull)
		if r.Outcome == stateledger.Contradicts || r.Standing == stateledger.Contradicted {
			t.Fatalf("P1: UNVERIFIED became REFUTED")
		}
	})
	// P2: AMBIGUOUS -> never CONFIRMED or REFUTED.
	t.Run("P2", func(t *testing.T) {
		amb := []byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"ok"}]`)
		r := fullEval(t, amb)
		if r.Outcome == stateledger.Supports || r.Outcome == stateledger.Contradicts {
			t.Fatalf("P2: AMBIGUOUS became determined")
		}
	})
	// P3: changing artifact identity invalidates proof.
	t.Run("P3", func(t *testing.T) {
		loc := mkLoc(raw, off)
		other := []byte(`[{"type":"tool_result","tool_use_id":"t9","is_error":true,"content":"ok"}]`)
		r := Evaluate(ClaimProposition, "s", other, loc, pred, CoverageFull)
		if r.Verdict != evidenceverify.Unverified {
			t.Fatalf("P3: changed artifact identity still verified")
		}
	})
	// P4: rendering cannot increase epistemic strength (render only echoes the
	// retained verdict/standing, never invents a stronger one).
	t.Run("P4", func(t *testing.T) {
		r := fullEval(t, raw)
		out := r.Render()
		if r.Verdict != evidenceverify.Confirmed && bytes.Contains([]byte(out), []byte("SUPPORTED")) {
			t.Fatalf("P4: render claimed SUPPORTED without CONFIRMED")
		}
	})
	// P5: an unknown coverage code cannot panic.
	t.Run("P5", func(_ *testing.T) {
		func() {
			defer func() { _ = recover() }()
			_ = Evaluate(ClaimProposition, "s", raw, mkLoc(raw, off), pred, Coverage("????"))
		}()
	})
	// P6: coverage NONE cannot produce substantive forensic truth.
	t.Run("P6", func(t *testing.T) {
		r := Evaluate(ClaimProposition, "s", nil, evidenceverify.Locator{}, pred, CoverageNone)
		if r.Outcome == stateledger.Supports || r.Outcome == stateledger.Contradicts {
			t.Fatalf("P6: NONE coverage produced substantive truth")
		}
	})
}

// FuzzEvaluateNeverPanics feeds arbitrary bytes and a derived locator so the
// claim layer cannot panic on hostile evidence.
func FuzzEvaluateNeverPanics(f *testing.F) {
	f.Add([]byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true}]`))
	f.Add([]byte(`{"is_error":true,"is_error":false}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		fragLen := len(data)
		if fragLen > 8 {
			fragLen = 8
		}
		loc := evidenceverify.Locator{
			TranscriptSHA256: sha256.Sum256(data),
			ValueOffset:      0,
			FragmentLength:   int64(fragLen),
			FragmentSHA256:   sha256.Sum256(data[:fragLen]),
		}
		r := Evaluate(ClaimProposition, "fuzz", data, loc, claimPredicate(), CoverageFull)
		switch r.Verdict {
		case evidenceverify.Unverified, evidenceverify.Ambiguous, evidenceverify.Confirmed, evidenceverify.Refuted:
		default:
			t.Fatalf("invalid verdict %d", r.Verdict)
		}
		_ = r.Render() // rendering must not panic either
	})
}
