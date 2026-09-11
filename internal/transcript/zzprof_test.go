package transcript

import (
	"os"
	"testing"
)

func BenchmarkParseRealTranscript(b *testing.B) {
	p := os.Getenv("PROF_FILE")
	if p == "" {
		b.Skip("set PROF_FILE")
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s, err := ParseClaudeCodeFile(p)
		if err != nil {
			b.Fatal(err)
		}
		if s == nil {
			b.Fatal("nil session")
		}
	}
}
