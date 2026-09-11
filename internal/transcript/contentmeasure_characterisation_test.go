package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// What ContentBytes and CallKey return today, pinned, before anything moves.
//
// Both are load-bearing in ways a refactor cannot see. ContentBytes measures a
// JSON value by its DECODED content — "independent of escaping and key order,
// so a redacted transcript measures exactly like the original" (wire.go:212).
// CallKey identifies a tool call "without retaining the input", and identical
// calls must share a key (wire.go:199).
//
// Both are reached only through json.RawMessage fields, and the fix under
// consideration is to stop copying those bytes. A zero-copy rewrite that
// changed escaping handling, key ordering, or hash input would move a published
// figure with nothing to catch it: every token attribution in every report is
// downstream of ContentBytes, and the blame table's grouping is downstream of
// CallKey.
//
// So this file is written first and deliberately says nothing about how either
// function works. It records what they RETURN. If a later change is a pure
// optimisation these values do not move; if any of them moves, the change was
// not what it claimed to be.

// CH1: ContentBytes is independent of escaping, key order and whitespace.
//
// These are the three properties the doc comment claims, stated as equalities
// between inputs that differ only in those respects. A rewrite that started
// counting raw bytes instead of decoded ones passes none of them.
func TestCH1_ContentBytesIgnoresEscapingKeyOrderAndWhitespace(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b string
	}{
		{"escaped vs literal slash", `"a/b"`, `"a\/b"`},
		{"escaped unicode vs literal", `"é"`, `"é"`},
		{"key order", `{"a":1,"b":2}`, `{"b":2,"a":1}`},
		{"whitespace", `{"a":1,"b":2}`, `{ "a" : 1 , "b" : 2 }`},
		{"array whitespace", `[1,2,3]`, `[ 1, 2, 3 ]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			ga, gb := ContentBytes(json.RawMessage(c.a)), ContentBytes(json.RawMessage(c.b))
			if ga != gb {
				t.Errorf("%s: %q measures %d, %q measures %d. The comment says this "+
					"measure is independent of exactly this difference, and a redacted "+
					"transcript must measure like the original", c.name, c.a, ga, c.b, gb)
			}
		})
	}
}

// CH2: and the absolute values, so "independent" cannot be satisfied by
// returning a constant.
//
// CH1 alone passes if ContentBytes returns zero for everything. These are the
// figures it returns today.
func TestCH2_ContentBytesValuesArePinned(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{`""`, 0},
		{`"abc"`, 3},
		{`"a/b"`, 3},
		{`"é"`, 2},
		{`123`, 3},
		{`true`, 4},
		{`false`, 5},
		{`null`, 4},
		{`{}`, 0},
		{`[]`, 0},
		{`{"a":1}`, 2},
		{`{"a":"bc"}`, 3},
		{`[1,2,3]`, 3},
		{`{"a":{"b":"c"}}`, 3},
		{`{"text":"hello world"}`, 15},
	} {
		if got := ContentBytes(json.RawMessage(c.in)); got != c.want {
			t.Errorf("ContentBytes(%s) = %d, was %d", c.in, got, c.want)
		}
	}
}

// CH3: CallKey is stable, sensitive to both inputs, and fixed width.
func TestCH3_CallKeyIsStableAndSensitive(t *testing.T) {
	in := json.RawMessage(`{"path":"/etc/hosts"}`)
	first := CallKey("Read", in)
	if first != CallKey("Read", in) {
		t.Fatal("CallKey is not deterministic")
	}
	if len(first) != 16 {
		t.Errorf("CallKey is %d characters, was 16", len(first))
	}
	if CallKey("Write", in) == first {
		t.Error("a different tool name produced the same key")
	}
	if CallKey("Read", json.RawMessage(`{"path":"/etc/passwd"}`)) == first {
		t.Error("a different input produced the same key")
	}
	// It must not leak the argument. The comment says the key "reveals nothing
	// about the arguments".
	if strings.Contains(first, "etc") || strings.Contains(first, "hosts") {
		t.Errorf("the key carries the argument: %q", first)
	}
	// Unlike ContentBytes, CallKey hashes the RAW bytes, so escaping DOES
	// change it. Recorded rather than judged: a rewrite that normalised here
	// would regroup every tool call in the blame table.
	if CallKey("Read", json.RawMessage(`{"path":"/etc\/hosts"}`)) == first {
		t.Log("note: CallKey now normalises escaping; it did not before")
	}
}

// CH4: the whole shipped corpus, as one digest.
//
// The table cases above cover shapes I thought of. This covers the shapes that
// are actually in a transcript: every block of the redacted session this
// repository ships, measured and hashed in decode order. It is the assertion
// that catches a change to a block type nobody remembered to enumerate.
func TestCH4_TheShippedCorpusMeasuresTheSame(t *testing.T) {
	path := filepath.Join("testdata", "session-redacted.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no shipped corpus: %v", err)
	}
	s, err := ParseClaudeCodeFile(path)
	if err != nil {
		t.Fatalf("parsing the shipped corpus: %v", err)
	}

	var parts []string
	blocks := 0
	for _, lane := range s.Lanes {
		for _, req := range lane.Requests {
			for _, b := range req.Context {
				for _, blk := range b.Blocks {
					blocks++
					parts = append(parts, fmt.Sprintf("%s|%d|%s|%s",
						blk.Kind, blk.Bytes, blk.Label, blk.CallKey))
				}
			}
		}
	}
	if blocks == 0 {
		t.Fatal("the shipped corpus produced no blocks, so this digest pins nothing")
	}
	// Sorted, so a change to iteration order is not mistaken for a change to
	// the measurements themselves. Order is pinned separately below.
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	got := hex.EncodeToString(sum[:])

	t.Logf("corpus: %d blocks, digest %s", blocks, got)
	const (
		want       = "c4b6dc7fd5e27dfbb43a01b0af8d4c6dfa7c069b8a8a75d0601ec9fe72dfa3b3"
		wantBlocks = 12018
	)
	if blocks != wantBlocks {
		t.Errorf("the shipped corpus now yields %d blocks, was %d; the digest below "+
			"cannot be compared across a different set", blocks, wantBlocks)
	}
	if got != want {
		t.Errorf("the shipped corpus now measures differently.\n got %s\nwant %s\n"+
			"Every token attribution in every report is downstream of these "+
			"figures; a change here moves published numbers.", got, want)
	}
}
