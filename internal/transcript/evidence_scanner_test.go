package transcript

// Guard-reachability contract tests for the evidence-backed is_error scanner.
// Each test drives one structural branch of the forward-only scanner and
// asserts the fail-closed outcome it exists to protect: malformed raw bytes
// must yield nil (never a guessed flag) and valid shapes must yield the exact
// ErrorFlag.

import (
	"encoding/json"
	"testing"
)

// attachEvidence must tolerate empty raw content (evidence.go:47).
func TestAttachEvidenceEmptyRaw(t *testing.T) {
	blks := []Block{{}}
	attachEvidence(blks, json.RawMessage(""), []RawBlock{{Type: KindToolResult}})
	if blks[0].Evidence != nil {
		t.Fatalf("empty raw content must not attach evidence, got %v", *blks[0].Evidence)
	}
}

// errorFlags on an empty array yields a nil flag for the (phantom) block,
// entering the empty-array short circuit (evidence.go:71).
func TestErrorFlagsEmptyArray(t *testing.T) {
	out := errorFlags([]byte("[]"), []RawBlock{{Type: KindToolResult}})
	if len(out) != 1 || out[0] != nil {
		t.Fatalf("empty array must produce one nil flag, got %#v", out)
	}
}

// scanToolResult on an empty object records Absent (evidence.go:139).
func TestScanToolResultEmptyObject(t *testing.T) {
	s := &flagScanner{b: []byte("{}")}
	got := s.scanToolResult()
	if got == nil || *got != ErrorFlagAbsent {
		t.Fatalf("empty object = Absent, got %v", got)
	}
}

// A non-string member key fails closed (evidence.go:145).
func TestScanToolResultNonStringKey(t *testing.T) {
	s := &flagScanner{b: []byte(`{123:1}`)}
	if got := s.scanToolResult(); got != nil {
		t.Fatalf("non-string key must return nil, got %v", *got)
	}
}

// A truncated boolean literal fails closed (evidence.go:157).
func TestScanToolResultTruncatedBool(t *testing.T) {
	s := &flagScanner{b: []byte(`{"is_error":tru}`)}
	if got := s.scanToolResult(); got != nil {
		t.Fatalf("truncated bool must return nil, got %v", *got)
	}
}

// readString refuses a non-string token (evidence.go:230).
func TestReadStringNotAtQuote(t *testing.T) {
	s := &flagScanner{b: []byte("abc")}
	if _, ok := s.readString(); ok {
		t.Fatalf("readString must fail when not at a quote")
	}
}

// readString rejects an invalid escape (evidence.go:241).
func TestReadStringInvalidEscape(t *testing.T) {
	s := &flagScanner{b: []byte(`"ab\q"`)}
	if _, ok := s.readString(); ok {
		t.Fatalf("readString must fail on an invalid escape")
	}
}

// skipValue at end of input returns without panicking (evidence.go:258).
func TestSkipValueAtEOF(_ *testing.T) {
	s := &flagScanner{b: []byte{}}
	s.skipValue() // must not panic
}

// skipArray handles an empty array (evidence.go:305).
func TestSkipArrayEmpty(t *testing.T) {
	s := &flagScanner{b: []byte("[]")}
	s.skipArray() // must not panic and must consume the ']'
	if s.pos != 2 {
		t.Fatalf("empty array skip ended at pos %d, want 2", s.pos)
	}
}

// skipArray handles a comma-separated array (evidence.go:311).
func TestSkipArrayComma(t *testing.T) {
	s := &flagScanner{b: []byte("[1,2]")}
	s.skipArray() // must not panic and must consume the ']'
	if s.pos != len("[1,2]") {
		t.Fatalf("array skip ended at pos %d, want %d", s.pos, len("[1,2]"))
	}
}
