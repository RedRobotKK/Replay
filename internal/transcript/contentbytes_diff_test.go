package transcript

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// contentBytesReference is the implementation ContentBytes had before it
// stopped decoding what it was measuring. It is the oracle for the scanner
// that replaced it, and it is kept verbatim — including the fallback to
// len(raw) on any error, which is a different measure from the one the
// function is named for and has to be reproduced exactly rather than fixed.
//
// The subtleties it carries, none of them obvious from reading it:
//
//   - Decoder.Token does not stop at the end of the first value. Two values
//     side by side are both measured and summed: `1 2` is 2, `"a" "b"` is 2.
//   - An escape is measured decoded: "a\nb" is 3, not 4.
//   - A lone surrogate, and any byte sequence that is not valid UTF-8,
//     measures as U+FFFD, which is three bytes.
//   - Object keys are measured, and duplicate keys are measured twice.
//   - There is no nesting limit on this path. Twenty thousand open brackets
//     are measured, not refused.
func contentBytesReference(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	n := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return n
			}
			return len(raw)
		}
		switch v := tok.(type) {
		case string:
			n += len(v)
		case json.Number:
			n += len(v.String())
		case bool:
			if v {
				n += len("true")
			} else {
				n += len("false")
			}
		case nil:
			n += len("null")
		}
	}
}

// diffCorpus is every input either implementation has been reasoned about,
// plus the shapes that separate them. Fuzzing covers what nobody thought of;
// this covers what somebody did, and runs on every `go test`.
func diffCorpus() []string {
	return []string{
		// Nothing, and nothing but space.
		"", " ", "\t\n\r ", "\n\n",
		// Scalars.
		"null", "true", "false", `""`, `"a"`, "0", "-0", "1", "1.5e10", "0.0",
		"123456789012345678901234567890", "-1.25E-7",
		// Numbers that are not numbers.
		"1.", ".1", "01", "1e", "1e+", "-", "+1", "NaN", "Infinity", "00", "-.5", "1..2",
		// Containers.
		"{}", "[]", "[[]]", "{\"a\":{}}", `{"a":1}`, `[1,2,3]`,
		`{"a":{"b":[1,2,{"c":"d"}]}}`, "[[[[1]]]]",
		`{"dup":1,"dup":2}`,
		// Structure that is not structure.
		`{"a":1,}`, "[1,]", "{a:1}", "'x'", "nul", "[1 2]", "1 ]", `{"a"}`, `{"a":}`,
		`{:1}`, "[,]", "}", "]", "{", "[", `{"a":1`, `["a"`,
		// More than one top-level value, which this path allows.
		"1 2", "{} {}", `"a" "b"`, "null true", "1\n2\n3",
		// Trailing rubbish after a complete value.
		`{"a":1}x`, "1x", `"a"x`, "[] [",
		// Escapes.
		`"a\nb"`, `"\/"`, `"\\"`, `"\""`, `"\b\f\n\r\t"`, `"\q"`, `"\"`,
		// Unicode escapes, including the surrogate rules.
		`"A"`, `"é"`, `"€"`, `"😀"`, `"\ud800"`,
		`"\ud800x"`, `"\ud800\ud800"`, `"\udc00"`, `"\ud800A"`,
		`"\uZZZZ"`, `"\u00"`, `"\u"`,
		// Literal UTF-8, and bytes that are not.
		`"é"`, `"😀"`, `"日本語"`, "\"\xff\"", "\"a\xffb\"", "\"\xc3\"", "\"\xed\xa0\x80\"",
		// Control characters are a syntax error inside a string.
		"\"a\x00b\"", "\"\x1f\"", "\"\x7f\"",
		// Keys carrying the same tricks as values.
		`{"a\nb":1}`, "{\"\xff\":1}", `{"\ud800":1}`,
		// Shapes from real transcripts.
		`{"file_path":"/tmp/x.go","content":"package x\n"}`,
		`[{"type":"text","text":"hello"}]`,
		`{"command":"ls -la","description":"list"}`,
	}
}

// TestCB5ScannerAgreesWithTheDecoderItReplaced is the net for the rewrite.
// ContentBytes feeds every cost figure the tool reports, so a divergence here
// is not a slow path, it is a wrong number with nothing to flag it.
func TestCB5ScannerAgreesWithTheDecoderItReplaced(t *testing.T) {
	for _, s := range diffCorpus() {
		raw := json.RawMessage(s)
		want := contentBytesReference(raw)
		got := ContentBytes(raw)
		if got != want {
			t.Errorf("ContentBytes(%q) = %d, decoder said %d", s, got, want)
		}
	}
	// Deep nesting, which the decoder path does not refuse and neither may
	// the scanner. Kept out of the corpus above so the failure message
	// stays readable.
	for _, depth := range []int{32, 33, 1000, 20000} {
		s := strings.Repeat("[", depth) + `"x"` + strings.Repeat("]", depth)
		raw := json.RawMessage(s)
		if got, want := ContentBytes(raw), contentBytesReference(raw); got != want {
			t.Errorf("ContentBytes(depth %d) = %d, decoder said %d", depth, got, want)
		}
	}
}

// TestCB6MeasuringDoesNotAllocate pins why the scanner exists. The decoder
// path built a json.Decoder and a bytes.Reader per call, copied the input
// into the decoder's own buffer, and then called Decode once per scalar —
// which allocated a boxed value for each one and constructed, and discarded,
// a syntax error at the end of each. It was 26% of every allocation the
// parser made.
func TestCB6MeasuringDoesNotAllocate(t *testing.T) {
	raw := json.RawMessage(`{"file_path":"/a/b/c.go","content":"package x\nfunc main() {}\n",` +
		`"nested":{"list":[1,2.5,true,null,"héllo €"],"deep":{"k":"v"}}}`)

	want := contentBytesReference(raw)
	var got int
	allocs := testing.AllocsPerRun(100, func() { got = ContentBytes(raw) })
	if got != want {
		t.Fatalf("ContentBytes = %d, decoder said %d", got, want)
	}
	if allocs != 0 {
		t.Fatalf("ContentBytes allocates %.0f times, want 0: measuring is reading, not decoding", allocs)
	}
}

// FuzzContentBytesMatchesTheDecoder is the part that covers what neither of
// us thought of. The seeds are the corpus; the value is in what the fuzzer
// builds from them. Run it with:
//
//	go test ./internal/transcript/ -run XXX -fuzz FuzzContentBytes -fuzztime 2m
func FuzzContentBytesMatchesTheDecoder(f *testing.F) {
	for _, s := range diffCorpus() {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		raw := json.RawMessage(in)
		want := contentBytesReference(raw)
		got := ContentBytes(raw)
		if got != want {
			t.Fatalf("ContentBytes(%q) = %d, decoder said %d", in, got, want)
		}
	})
}
