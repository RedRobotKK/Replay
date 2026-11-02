package transcript

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/RedRobotKK/Replay/internal/evidenceverify"
)

// e2eLocator builds a locator for the first occurrence of token in a raw line,
// using the create half of the locator lifecycle.
func e2eLocator(t *testing.T, raw []byte, token string) evidenceverify.Locator {
	t.Helper()
	off := bytes.Index(raw, []byte(token))
	if off < 0 {
		t.Fatalf("token %q not found in fixture", token)
	}
	loc, ok := evidenceverify.LocateValue(raw, int64(off))
	if !ok {
		t.Fatalf("LocateValue failed for %q at offset %d", token, off)
	}
	return loc
}

// The ordinary decode path collapses duplicate keys and absence into a single
// bool. The forensic path must preserve all four states on the same bytes.
func TestOccurrenceVerificationEndToEnd(t *testing.T) {
	// Case C — AMBIGUOUS: conflicting same-scope duplicate key.
	ambiguous := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"ok"}]}}`

	// Ordinary decode resolves last-wins, destroying the recorded true.
	_, blocks, isText, err := DecodeContent(json.RawMessage(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"ok"}]`))
	if err != nil || isText || len(blocks) != 1 {
		t.Fatalf("DecodeContent: err=%v isText=%v blocks=%d", err, isText, len(blocks))
	}
	if blocks[0].IsError != false {
		t.Fatalf("ordinary decode collapsed the duplicate to %v, want last-wins false", blocks[0].IsError)
	}
	if got := VerifyIsErrorOccurrence([]byte(ambiguous), e2eLocator(t, []byte(ambiguous), "true"), true); got.Verdict != evidenceverify.Ambiguous {
		t.Fatalf("AMBIGUOUS case: verdict=%s reason=%s, want AMBIGUOUS", got.Verdict, got.Reason)
	}

	// Case A — CONFIRMED: unique matching literal.
	confirmed := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]}}`
	if got := VerifyIsErrorOccurrence([]byte(confirmed), e2eLocator(t, []byte(confirmed), "true"), true); got.Verdict != evidenceverify.Confirmed {
		t.Fatalf("CONFIRMED case: verdict=%s reason=%s", got.Verdict, got.Reason)
	}

	// Case B — REFUTED: unique opposite literal, fully established basis.
	refuted := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","is_error":false,"content":"ok"}]}}`
	if got := VerifyIsErrorOccurrence([]byte(refuted), e2eLocator(t, []byte(refuted), "false"), true); got.Verdict != evidenceverify.Refuted {
		t.Fatalf("REFUTED case: verdict=%s reason=%s", got.Verdict, got.Reason)
	}

	// Case D — UNVERIFIED: transcript binding broken.
	broken := confirmed
	loc := e2eLocator(t, []byte(broken), "true")
	loc.TranscriptSHA256[0] ^= 0xFF
	if got := VerifyIsErrorOccurrence([]byte(broken), loc, true); got.Verdict != evidenceverify.Unverified {
		t.Fatalf("UNVERIFIED case: verdict=%s reason=%s", got.Verdict, got.Reason)
	}
}

// Absence is not false. The ordinary view reads an absent is_error as false;
// the forensic view has no occurrence to pin and reports no basis.
func TestAbsentIsErrorIsNotFalseForensically(t *testing.T) {
	// Ordinary decode: absent is_error becomes the zero bool (false).
	_, blocks, _, err := DecodeContent(json.RawMessage(`[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]`))
	if err != nil || len(blocks) != 1 {
		t.Fatalf("DecodeContent: err=%v blocks=%d", err, len(blocks))
	}
	if blocks[0].IsError {
		t.Fatalf("absent is_error must not read true")
	}

	// Forensic view: there is no is_error value to pin, so no basis exists.
	absent := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`
	if _, ok := evidenceverify.LocateValue([]byte(absent), 4); ok {
		t.Fatalf("LocateValue must not build a locator for a non-occurrence")
	}
}

// Same bytes + same locator + same predicate must yield the same verdict,
// whatever evaluation path produced the bytes (determinism).
func TestOccurrenceVerificationIsDeterministic(t *testing.T) {
	raw := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"ok"}]}}`)
	loc := e2eLocator(t, raw, "true")
	first := VerifyIsErrorOccurrence(raw, loc, true)
	second := VerifyIsErrorOccurrence(raw, loc, true)
	if first != second {
		t.Fatalf("non-deterministic verdict: %+v vs %+v", first, second)
	}
	if first.Verdict != evidenceverify.Confirmed {
		t.Fatalf("determinism fixture: verdict=%s", first.Verdict)
	}
}
