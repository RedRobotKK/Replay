package transcript

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// sourceBlock decodes one image block and returns it, through the real
// RawBlock rather than a mock: a mock would pass whether or not the field
// on the shipped struct ever changed.
func sourceBlock(t *testing.T, raw string) Block {
	t.Helper()
	var rb RawBlock
	if err := json.Unmarshal([]byte(raw), &rb); err != nil {
		t.Fatalf("unmarshal %.40s: %v", raw, err)
	}
	return DecodeBlock(rb, RoleUser, nil, nil)
}

// TestMS1_APayloadMeasuresWhatItMeasuredBefore is the regression net. The
// figure an image contributes is a billed quantity, so it has to survive the
// field changing type underneath it.
func TestMS1_APayloadMeasuresWhatItMeasuredBefore(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{
			"the object shape Anthropic sends",
			`{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}`,
			// type(6)+base64(6) + media_type(10)+image/png(9) + data(4)+the payload(12)
			len("type") + len("base64") + len("media_type") + len("image/png") + len("data") + len("iVBORw0KGgo="),
		},
		{
			"a bare string source",
			`{"type":"image","source":"iVBORw0KGgo="}`,
			len("iVBORw0KGgo="),
		},
		{
			"content and source are summed, not chosen between",
			`{"type":"image","content":"ab","source":"cde"}`,
			len("ab") + len("cde"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sourceBlock(t, c.raw).Bytes; got != c.want {
				t.Fatalf("Bytes = %d, want %d", got, c.want)
			}
		})
	}
}

// TestMS2_NullIsFourAndAbsentIsZero pins the two states that look identical
// from the outside and are not.
//
// encoding/json calls UnmarshalJSON for a JSON null, so a null source is
// measured — as the four bytes of the keyword, which is what a
// json.RawMessage holding `null` measured. An ABSENT key never calls the
// method at all, so the field keeps its zero value. Neither number is
// obvious from reading the code, and a change that collapsed them would look
// like a simplification.
func TestMS2_NullIsFourAndAbsentIsZero(t *testing.T) {
	if got := sourceBlock(t, `{"type":"image","source":null}`).Bytes; got != 4 {
		t.Errorf(`"source":null measured %d, want 4: Unmarshal calls UnmarshalJSON `+
			`for a JSON null, and the keyword is four bytes — which is what a `+
			`json.RawMessage holding null measured`, got)
	}
	if got := sourceBlock(t, `{"type":"image"}`).Bytes; got != 0 {
		t.Errorf("an absent source measured %d, want 0: the method is never called, "+
			"so the zero value stands", got)
	}
}

// TestMS3_AMeasuredPayloadIsNotKept is why this change exists. A screenshot
// arrives as a few hundred kilobytes of base64, its only use in the whole
// repo is the measurement above, and json.RawMessage copies every byte to
// hand back a number.
func TestMS3_AMeasuredPayloadIsNotKept(t *testing.T) {
	const payload = 400 << 10
	raw := `{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` +
		strings.Repeat("A", payload) + `"}}`

	// Convert the input OUTSIDE the measured region. The caller owns the
	// bytes of the line either way, and counting that conversion here would
	// put a 400 KB floor under a measurement whose whole point is what the
	// decode adds on top of it.
	in := []byte(raw)

	var before, after runtime.MemStats
	var rb RawBlock
	runtime.GC()
	runtime.ReadMemStats(&before)
	if err := json.Unmarshal(in, &rb); err != nil {
		t.Fatal(err)
	}
	b := DecodeBlock(rb, RoleUser, nil, nil)
	runtime.ReadMemStats(&after)
	used := after.TotalAlloc - before.TotalAlloc

	if b.Bytes < payload {
		t.Fatalf("Bytes = %d, want at least the payload (%d): the fixture is wrong", b.Bytes, payload)
	}
	// With the field holding an int, the decode allocates the small strings
	// of the block and nothing proportional to the payload. A tenth of the
	// payload is far above what the small fields cost and far below a single
	// copy of it, so neither a copy nor ordinary allocator noise can be
	// mistaken for the other.
	if used > payload/10 {
		t.Fatalf("decoding a %d-byte payload allocated %d bytes, want under %d:\n"+
			"the payload is being copied into a field whose only use is len()",
			payload, used, payload/10)
	}
	t.Logf("%d bytes allocated decoding a %d-byte payload", used, payload)
}
