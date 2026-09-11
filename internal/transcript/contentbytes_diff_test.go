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
//
//   - An escape is measured decoded: "a\nb" is 3, not 4.
//
//   - A lone surrogate, and any byte sequence that is not valid UTF-8,
//     measures as U+FFFD, which is three bytes.
//
//   - Object keys are measured, and duplicate keys are measured twice.
//
//   - There IS a nesting limit on this path, and where it sits depends on the
//     toolchain. This said there was none, which was true of every Go the
//     corpus had been run on: go1.26.0 measures twenty thousand open brackets
//     without complaint. go1.27.1 caps Decoder.Token at ten thousand and
//     returns "exceeded max depth", at which point the fallback above turns a
//     refusal into len(raw) — forty thousand bytes reported for one byte of
//     content.
//
//     That is a property of this implementation rather than of JSON, and it
//     is one of the things the scanner was written to stop doing. TestCB5
//     compares against the oracle below the limit and asserts the scanner's
//     own answer above it.
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
		// Input that runs out at a boundary. Each of these stops cleanly and
		// keeps what it counted, because Decoder.Token returns a plain io.EOF
		// there rather than a syntax error. Four separate boundaries, each
		// its own return in the scanner, and a mutation pass found every one
		// of them unguarded when only `{"a":1` was here:
		//   `{"a"`    the key was read, the colon never arrived
		//   `{"a":`   a value was expected and the input ended
		//   `{"a":1`  a value was read, the comma or brace never arrived
		//   `{"a":1,` a comma was read, the next key never arrived
		`{"a"`, `{"a":`, `{"a":1,`, `{"a":{`, `{"a":{"b"`, "[", "[1,", `["a",`, "[{",
		// Numbers starting with zero, nested so that measuring them correctly
		// and failing to parse them give different answers. Bare `0` measures
		// 1 and len("0") is also 1, so a top-level zero cannot tell the two
		// apart and the leading-zero arm sat unguarded behind it.
		"[0]", "[-0]", "[0.0]", `{"z":0}`, "[0,0]", "[01]",
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

// base64ish builds n bytes of the base64 alphabet, deterministically, and
// includes the '+' and '/' a real payload carries. '/' is the interesting
// one: it is legal raw inside a JSON string and legal as "\/", so the two
// spellings have to measure the same, and no hand-typed corpus entry is long
// enough to contain one by accident.
func base64ish(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[i%len(alphabet)]
	}
	return string(b)
}

// payloadShapes returns the forms a source payload of n bytes arrives in: a
// bare string, the object RawBlock.Source actually holds, and the same object
// with every forward slash escaped. The last pairs with the second: both
// spellings of '/' must measure the same.
func payloadShapes(n int) []string {
	payload := base64ish(n)
	return []string{
		`"` + payload + `"`,
		`{"type":"base64","media_type":"image/png","data":"` + payload + `"}`,
		`{"type":"base64","data":"` + strings.ReplaceAll(payload, "/", `\/`) + `"}`,
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
	// Deep nesting. Kept out of the corpus above so the failure message stays
	// readable, and split at ten thousand because that is where the oracle
	// stops being a fixed point.
	//
	// encoding/json gained a nesting limit of 10000 on the Decoder.Token path
	// in Go 1.27. Measured on both toolchains, same input:
	//
	//	depth   go1.26.0   go1.27.0
	//	 10000         1          1
	//	 10001         1      20005   (= len(raw), the refusal)
	//	 20000         1      40003
	//
	// So below the limit the two agree and equality is the right assertion.
	// Above it, asserting equality would freeze Replay's answer to whichever
	// Go the tests happened to run on, and the figure a user reads would
	// change when their toolchain did.
	for _, depth := range []int{32, 33, 1000, 9999, 10000} {
		s := strings.Repeat("[", depth) + `"x"` + strings.Repeat("]", depth)
		raw := json.RawMessage(s)
		if got, want := ContentBytes(raw), contentBytesReference(raw); got != want {
			t.Errorf("ContentBytes(depth %d) = %d, decoder said %d", depth, got, want)
		}
	}
	// Past the limit, the scanner keeps measuring. That is now a decision
	// this package owns rather than one inherited from encoding/json: the
	// walk is iterative, holds one byte per level, and refusing a value it
	// can measure would report len(raw) — a different measure — for content
	// it read perfectly well. A transcript nested ten thousand deep does not
	// exist; what matters is that the answer does not depend on the compiler.
	for _, depth := range []int{10001, 20000} {
		s := strings.Repeat("[", depth) + `"x"` + strings.Repeat("]", depth)
		if got := ContentBytes(json.RawMessage(s)); got != 1 {
			t.Errorf("ContentBytes(depth %d) = %d, want 1: the scanner measures deep "+
				"nesting rather than refusing it, on every toolchain", depth, got)
		}
	}

	// Payload scale. RawBlock.Source carries base64 image data at a few
	// hundred kilobytes a block, and the longest entry in the corpus above is
	// three hundred bytes — three orders of magnitude short of the thing the
	// field actually holds. The scanner has no length threshold today, which
	// is exactly why a case at the real size has to exist: a future one would
	// otherwise land unmeasured, and the corpus would not notice.
	for _, n := range []int{1 << 10, 100 << 10, 400 << 10} {
		for i, shape := range payloadShapes(n) {
			raw := json.RawMessage(shape)
			if got, want := ContentBytes(raw), contentBytesReference(raw); got != want {
				// Never print the payload. A 400 KB failure message is a
				// failure nobody reads.
				t.Fatalf("ContentBytes(%d-byte payload, shape %d, %q…) = %d, decoder said %d",
					n, i, shape[:min(24, len(shape))], got, want)
			}
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

	// And at the size the field actually carries. Zero allocations on a
	// 200-byte object says nothing about whether the scanner starts
	// allocating when the input grows, which is the whole reason
	// RawBlock.Source can stop being a json.RawMessage.
	big := json.RawMessage(payloadShapes(400 << 10)[1])
	wantBig := contentBytesReference(big)
	var gotBig int
	bigAllocs := testing.AllocsPerRun(5, func() { gotBig = ContentBytes(big) })
	if gotBig != wantBig {
		t.Fatalf("ContentBytes(400 KB) = %d, decoder said %d", gotBig, wantBig)
	}
	if bigAllocs != 0 {
		t.Fatalf("ContentBytes allocates %.0f times on a %d-byte payload, want 0", bigAllocs, len(big))
	}
	t.Logf("%.0f allocations measuring %d bytes", bigAllocs, len(big))
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
