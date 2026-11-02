package transcript

import (
	"strings"
	"testing"
)

// Scanner unit tests: the evidence capture must correctly recover is_error
// values, preserve duplicates, and handle escaped keys and nested content.

func flagsFor(t *testing.T, content string) []*ErrorFlag {
	t.Helper()
	// The content array carries one or more tool_result blocks. Decode it the
	// way the production path does, then scan raw bytes for evidence.
	_, blocks, isText, err := DecodeContent([]byte(content))
	if err != nil || isText {
		t.Fatalf("fixture did not decode as a block list: err=%v isText=%v", err, isText)
	}
	return errorFlags([]byte(content), blocks)
}

func TestErrorFlagCapture(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []ErrorFlag
	}{
		{"unique_true", `[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"x"}]`,
			[]ErrorFlag{ErrorFlagTrue}},
		{"unique_false", `[{"type":"tool_result","tool_use_id":"t1","is_error":false,"content":"x"}]`,
			[]ErrorFlag{ErrorFlagFalse}},
		{"absent", `[{"type":"tool_result","tool_use_id":"t1","content":"x"}]`,
			[]ErrorFlag{ErrorFlagAbsent}},
		{"conflict", `[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"x"}]`,
			[]ErrorFlag{ErrorFlagConflict}},
		{"conflict_reversed", `[{"type":"tool_result","tool_use_id":"t1","is_error":false,"is_error":true,"content":"x"}]`,
			[]ErrorFlag{ErrorFlagConflict}},
		{"identical_duplicates", `[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":true,"content":"x"}]`,
			[]ErrorFlag{ErrorFlagTrue}},
		{"escaped_key", `[{"type":"tool_result","tool_use_id":"t1","\u0069s_error":true,"content":"x"}]`,
			[]ErrorFlag{ErrorFlagTrue}},
		{"content_mentions_is_error", `[{"type":"tool_result","tool_use_id":"t1","content":"the string is_error appears here but is not a key"}]`,
			[]ErrorFlag{ErrorFlagAbsent}},
		{"non_tool_result_ignored", `[{"type":"text","text":"x"},{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"x"}]`,
			[]ErrorFlag{ErrorFlagNone, ErrorFlagTrue}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := flagsFor(t, tc.content)
			if len(got) != len(tc.want) {
				t.Fatalf("flag count = %d, want %d (%v)", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] == nil {
					if tc.want[i] != ErrorFlagNone {
						t.Fatalf("block %d: got nil, want %v", i, tc.want[i])
					}
					continue
				}
				if *got[i] != tc.want[i] {
					t.Fatalf("block %d: got %v, want %v", i, *got[i], tc.want[i])
				}
			}
		})
	}
}

// The flag scanner must never panic on malformed or adversarial input, even
// though it is only called on encoding/json-accepted content.
func FuzzErrorFlagScannerNeverPanics(f *testing.F) {
	f.Add([]byte(`[{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false}]`))
	f.Add([]byte(`[{"type":"tool_result"`))
	f.Add([]byte(`{`))
	f.Add([]byte(``))
	f.Fuzz(func(_ *testing.T, data []byte) {
		// Feed the scanner directly; it must never panic regardless of bytes.
		s := &flagScanner{b: data}
		s.skipWS()
		if s.peek() == '[' {
			// Not strictly validated; only exercised for panic-freedom.
			_ = errorFlags(data, []RawBlock{{Type: KindToolResult}})
		}
	})
}

func TestAttachEvidenceIsParallel(t *testing.T) {
	content := `[{"type":"text","text":"x"},{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"x"}]`
	_, blocks, isText, err := DecodeContent([]byte(content))
	if err != nil || isText {
		t.Fatalf("decode: err=%v isText=%v", err, isText)
	}
	blks := DecodeBlocks(blocks, RoleUser, nil, nil)
	attachEvidence(blks, []byte(content), blocks)
	if len(blks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(blks))
	}
	if blks[0].Evidence != nil {
		t.Fatalf("text block gained evidence: %v", *blks[0].Evidence)
	}
	if blks[1].Evidence == nil || *blks[1].Evidence != ErrorFlagTrue {
		t.Fatalf("tool_result block evidence = %v, want ErrorFlagTrue", blks[1].Evidence)
	}
	_ = strings.TrimSpace
}
