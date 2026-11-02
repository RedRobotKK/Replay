package evidenceverify

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Benchmarks compare the two-pass occurrence scanner against the naive
// encoding/json decode that the verifier exists to replace. The point is not
// benchmark theater; it is to establish that the security architecture does
// not impose unreasonable cost. ns/op, B/op and allocs/op are the numbers
// worth comparing; absolute times vary by machine.

func benchLoc(b []byte, off, fragLen int) Locator {
	return Locator{
		TranscriptSHA256: sha256.Sum256(b),
		ValueOffset:      int64(off),
		FragmentLength:   int64(fragLen),
		FragmentSHA256:   sha256.Sum256(b[off : off+fragLen]),
	}
}

func benchVerify(b *testing.B, doc string, key string, expect Literal) {
	raw := []byte(doc)
	off := strings.Index(doc, "true")
	if off < 0 {
		off = 0
	}
	fragLen := 4
	if off+4 > len(doc) {
		fragLen = len(doc) - off
	}
	loc := benchLoc(raw, off, fragLen)
	pred := Predicate{Key: key, Expect: expect}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Verify(raw, loc, pred)
	}
}

// naiveDecode is the flattened-map oracle this verifier replaces. It resolves
// duplicate keys last-wins and collapses absence into false, which is exactly
// what the verifier must not do.
func naiveDecode(raw []byte) (bool, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false, err
	}
	v, ok := m["is_error"].(bool)
	if !ok {
		return false, nil
	}
	return v, nil
}

func BenchmarkVerifySmallObject(b *testing.B) {
	benchVerify(b, `{"is_error":true}`, "is_error", BoolLiteral(true))
}

func BenchmarkVerifyTranscriptShaped(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"role":"user","content":[`)
	for i := 0; i < 200; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"type":"tool_result","tool_use_id":"t%d","is_error":%v,"content":"ok"}`, i, i%2 == 0)
	}
	sb.WriteString(`]}`)
	benchVerify(b, sb.String(), "is_error", BoolLiteral(true))
}

func BenchmarkVerifyLargeTranscript(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"role":"user","content":[`)
	for i := 0; i < 20000; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"type":"tool_result","tool_use_id":"t%d","is_error":%v,"content":"some tool output text"}`, i, i%2 == 0)
	}
	sb.WriteString(`]}`)
	benchVerify(b, sb.String(), "is_error", BoolLiteral(true))
}

func BenchmarkVerifyDeeplyNested(b *testing.B) {
	doc := strings.Repeat(`{"a":`, 200) + "true" + strings.Repeat("}", 200)
	raw := []byte(doc)
	off := strings.Index(doc, "true")
	loc := benchLoc(raw, off, 4)
	pred := Predicate{Key: "a", Expect: BoolLiteral(true)}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Verify(raw, loc, pred)
	}
}

func BenchmarkVerifyManyMembers(b *testing.B) {
	var sb strings.Builder
	sb.WriteByte('{')
	for i := 0; i < 100000; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `"k%d":%d`, i, i)
	}
	sb.WriteString(`,"is_error":true}`)
	benchVerify(b, sb.String(), "is_error", BoolLiteral(true))
}

func BenchmarkVerifyDuplicateHeavy(b *testing.B) {
	doc := "{" + strings.Repeat(`"is_error":true,`, 1000) + `"is_error":true}`
	benchVerify(b, doc, "is_error", BoolLiteral(true))
}

func BenchmarkVerifyTargetNearEnd(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"role":"user","content":[`)
	for i := 0; i < 20000; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"type":"tool_result","tool_use_id":"t%d","content":"ok"}`, i)
	}
	sb.WriteString(`],"is_error":true}`)
	benchVerify(b, sb.String(), "is_error", BoolLiteral(true))
}

// Naive baseline for the same inputs, so the cost of strictness is visible.
func BenchmarkNaiveDecodeTranscriptShaped(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"role":"user","content":[`)
	for i := 0; i < 200; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"type":"tool_result","tool_use_id":"t%d","is_error":%v,"content":"ok"}`, i, i%2 == 0)
	}
	sb.WriteString(`]}`)
	raw := []byte(sb.String())
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = naiveDecode(raw)
	}
}

func BenchmarkNaiveDecodeLargeTranscript(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"role":"user","content":[`)
	for i := 0; i < 20000; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"type":"tool_result","tool_use_id":"t%d","is_error":%v,"content":"some tool output text"}`, i, i%2 == 0)
	}
	sb.WriteString(`]}`)
	raw := []byte(sb.String())
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = naiveDecode(raw)
	}
}
