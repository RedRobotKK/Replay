package transcript

import (
	"fmt"
	"strings"
	"testing"
)

// Benchmarks for the evidence capture. The point is to show the capture is
// O(content size), a single pass per message, not O(content size × block
// count). attachEvidence runs once per decoded message and scans the raw
// content bytes exactly once.

func benchContent(toolResultBlocks int) (raw []byte, blocks []RawBlock) {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < toolResultBlocks; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"type":"tool_result","tool_use_id":"t%d","is_error":%v,"content":"some output text"}`, i, i%2 == 0)
	}
	b.WriteByte(']')
	raw = []byte(b.String())
	_, blocks, _, _ = DecodeContent(raw)
	return raw, blocks
}

func BenchmarkAttachEvidence(b *testing.B) {
	for _, n := range []int{1, 20, 200, 2000} {
		b.Run(fmt.Sprintf("blocks_%d", n), func(b *testing.B) {
			raw, blocks := benchContent(n)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				blks := DecodeBlocks(blocks, RoleUser, nil, nil)
				attachEvidence(blks, raw, blocks)
			}
		})
	}
}
