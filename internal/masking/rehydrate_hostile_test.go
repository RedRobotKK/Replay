package masking

import (
	"encoding/json"
	"strings"
	"testing"
)

// Body does not report an error for any document the proxy can be sent.
//
// This exists to hold up a claim made in a comment at
// internal/proxy/masking.go, where the `if err != nil` on Body's result is
// reported UNREACHED by guard-reachability. The reason it is unreached is
// structural: Body returns a non-nil error from exactly one place,
// literals(), and literals() is only reached after json.Unmarshal has
// accepted the same bytes — so the documents literals rejects are the
// documents Unmarshal rejected first, and Body has already returned.
//
// A comment asserting that is only as good as the probe behind it, so the
// probe lives here and runs. If Body ever grows a second error path that one
// of these inputs reaches, this goes red and the comment in proxy/masking.go
// is out of date.
//
// This is deliberately NOT a test written to satisfy the UNREACHED verdict.
// It does not make the proxy's guard run. It records why nothing can.
func TestBodyReportsNoErrorOnHostileDocuments(t *testing.T) {
	cases := map[string]string{
		"plain message":        `{"type":"message","content":[]}`,
		"message with content": `{"type":"message","content":[{"type":"text","text":"hi"}]}`,
		"not a message":        `{"type":"error","content":[]}`,
		"truncated":            `{"type":"message","content":[`,
		"trailing garbage":     `{"type":"message"} xyz`,
		"two top-level values": `{"type":"message"} {"a":1}`,
		"empty":                ``,
		"nest 100":             `{"type":"message","d":` + strings.Repeat("[", 100) + strings.Repeat("]", 100) + `}`,
		"nest 10001":           `{"type":"message","d":` + strings.Repeat("[", 10001) + strings.Repeat("]", 10001) + `}`,
		"invalid utf8":         "{\"type\":\"message\",\"k\":\"a\xffb\"}",
		"lone surrogate":       `{"type":"message","s":"\ud800"}`,
		"surrogate pair":       `{"type":"message","s":"𐀀"}`,
		"duplicate keys":       `{"type":"message","x":1,"x":2}`,
		"100k digits":          `{"type":"message","n":` + strings.Repeat("9", 100000) + `}`,
		"huge exponent":        `{"type":"message","n":1e999999}`,
	}
	r := &Rehydrator{}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := r.Body([]byte(body))
			if err != nil {
				t.Fatalf("Body reported an error for %q: %v\n"+
					"      internal/proxy/masking.go carries a comment saying no input reaches\n"+
					"      that error return. This input does, so either the comment is wrong or\n"+
					"      Body grew an error path. Read both before changing either.",
					name, err)
			}
		})
	}
}

// The mechanism, asserted directly rather than inferred from the table above:
// for a document Unmarshal refuses, Body returns before literals() can run.
// If this ever fails, the dominance argument in proxy/masking.go is void even
// if every case above still passes.
func TestUnmarshalRefusalIsWhatKeepsLiteralsUnreached(t *testing.T) {
	// 10001 levels is past encoding/json's max depth, so Unmarshal refuses
	// it; literals() would report "unexpected end of document" on a
	// truncated one, which is the error Body would otherwise surface.
	deep := `{"type":"message","d":` + strings.Repeat("[", 10001) + strings.Repeat("]", 10001) + `}`

	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(deep), &msg); err == nil {
		t.Fatal("Unmarshal accepted a 10001-deep document; this test no longer " +
			"demonstrates the early return it was written to demonstrate")
	}
	if _, _, err := r(t).Body([]byte(deep)); err != nil {
		t.Fatalf("Body surfaced %v for a document Unmarshal refuses. The early "+
			"return is gone, and the UNREACHED comment in proxy/masking.go with it", err)
	}
}

func r(t *testing.T) *Rehydrator { t.Helper(); return &Rehydrator{} }
