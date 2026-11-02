package evidenceverify

import (
	"crypto/sha256"
	"strconv"
	"strings"
	"testing"
)

// The fuzz targets below are property checks, not proofs. They bound hostile
// input and catch panics, but a green fuzz run is evidence of robustness, not
// a guarantee. The load-bearing invariant they encode is the one from the
// epistemic contract: a failure to inspect evidence is UNVERIFIED, never
// REFUTED.

// FuzzVerifierNeverPanics feeds arbitrary bytes through Verify with a locator
// derived from the input. A panic is a test failure, which is the point.
func FuzzVerifierNeverPanics(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"is_error":true}`),
		[]byte(`{"is_error":false}`),
		[]byte(`{"is_error":true,"is_error":false}`),
		[]byte(`{"a":{"is_error":true},"b":{"is_error":false}}`),
		[]byte(`[{"is_error":true},{"is_error":false}]`),
		[]byte(`{"is_error":"\uD800"}`),
		[]byte(`{"is_error":01}`),
		[]byte(`{`),
		[]byte(`}}}`),
		[]byte(`[][][][][][][][][][]`),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		fragLen := len(data)
		if fragLen > 8 {
			fragLen = 8
		}
		loc := Locator{
			TranscriptSHA256: sha256.Sum256(data),
			ValueOffset:      0,
			FragmentLength:   int64(fragLen),
			FragmentSHA256:   sha256.Sum256(data[:fragLen]),
		}
		r := Verify(data, loc, Predicate{Key: "is_error", Expect: BoolLiteral(true)})
		switch r.Verdict {
		case Unverified, Refuted, Ambiguous, Confirmed:
		default:
			t.Fatalf("invalid verdict %d", r.Verdict)
		}
	})
}

// FuzzReportedSpansRemainWithinInput asserts that whenever the verifier
// establishes a span or object offset, that span stays inside the transcript.
func FuzzReportedSpansRemainWithinInput(f *testing.F) {
	f.Add([]byte(`{"is_error":true}`), int64(12), int64(4))
	f.Add([]byte(`{"a":{"is_error":false}}`), int64(16), int64(5))
	f.Fuzz(func(t *testing.T, data []byte, off, fragLen int64) {
		if len(data) == 0 {
			return
		}
		if off < 0 {
			off = 0
		}
		if fragLen <= 0 {
			fragLen = 1
		}
		if off+fragLen > int64(len(data)) {
			return
		}
		loc := Locator{
			TranscriptSHA256: sha256.Sum256(data),
			ValueOffset:      off,
			FragmentLength:   fragLen,
			FragmentSHA256:   sha256.Sum256(data[off : off+fragLen]),
		}
		r := Verify(data, loc, Predicate{Key: "is_error", Expect: BoolLiteral(true)})
		n := int64(len(data))
		if r.ObjectOffset != -1 && (r.ObjectOffset < 0 || r.ObjectOffset >= n) {
			t.Fatalf("object offset %d outside input len %d", r.ObjectOffset, n)
		}
		if r.KeySpan != (Span{}) && (r.KeySpan.Start < 0 || r.KeySpan.End > n || r.KeySpan.Start > r.KeySpan.End) {
			t.Fatalf("key span %+v outside input len %d", r.KeySpan, n)
		}
		if r.ValueSpan != (Span{}) && (r.ValueSpan.Start < 0 || r.ValueSpan.End > n || r.ValueSpan.Start > r.ValueSpan.End) {
			t.Fatalf("value span %+v outside input len %d", r.ValueSpan, n)
		}
	})
}

// FuzzInvalidBasisCannotProduceRefuted is the load-bearing property. If a
// mutation destroys transcript identity or fragment identity, the result may
// become UNVERIFIED but must never become REFUTED, because REFUTED is only
// reachable after the evidence basis is established.
func FuzzInvalidBasisCannotProduceRefuted(f *testing.F) {
	f.Add([]byte(`{"is_error":false}`), int64(12), int64(5), true, false)
	f.Add([]byte(`{"is_error":false}`), int64(12), int64(5), false, true)
	f.Add([]byte(`{"is_error":true}`), int64(12), int64(4), true, true)
	f.Fuzz(func(t *testing.T, data []byte, off, fragLen int64, flipTranscript, flipFragment bool) {
		if len(data) == 0 {
			return
		}
		if off < 0 || off >= int64(len(data)) {
			return
		}
		if fragLen <= 0 || off+fragLen > int64(len(data)) {
			return
		}
		loc := Locator{
			TranscriptSHA256: sha256.Sum256(data),
			ValueOffset:      off,
			FragmentLength:   fragLen,
			FragmentSHA256:   sha256.Sum256(data[off : off+fragLen]),
		}
		if flipTranscript {
			loc.TranscriptSHA256[0] ^= 0xFF
		}
		if flipFragment {
			loc.FragmentSHA256[0] ^= 0xFF
		}
		r := Verify(data, loc, Predicate{Key: "is_error", Expect: BoolLiteral(true)})
		if (flipTranscript || flipFragment) && r.Verdict == Refuted {
			t.Fatalf("corrupted basis produced REFUTED: %+v", r)
		}
	})
}

// FuzzDuplicateConflictIsOrderIndependent asserts that conflicting duplicate
// keys stay AMBIGUOUS whichever order they appear in, and that identical
// duplicates stay determined.
func FuzzDuplicateConflictIsOrderIndependent(f *testing.F) {
	f.Add(true, false)
	f.Add(false, true)
	f.Add(true, true)
	f.Add(false, false)
	f.Fuzz(func(t *testing.T, a, b bool) {
		doc := `{"is_error":` + strconv.FormatBool(a) + `,"is_error":` + strconv.FormatBool(b) + `}`
		off := strings.Index(doc, strconv.FormatBool(a))
		r := Verify([]byte(doc), mkLoc(doc, off, len(strconv.FormatBool(a))), Predicate{Key: "is_error", Expect: BoolLiteral(true)})
		if a != b {
			if r.Verdict != Ambiguous {
				t.Fatalf("conflicting duplicates (a=%v b=%v): verdict=%s, want AMBIGUOUS", a, b, r.Verdict)
			}
		} else if r.Verdict == Ambiguous {
			t.Fatalf("identical duplicates (a=%v b=%v) must not be ambiguous", a, b)
		}
	})
}
