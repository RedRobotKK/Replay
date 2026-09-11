package transcript

import (
	"os"
	"testing"
)

// A repeatable baseline for the parse path, against a real transcript.
//
// Env-gated because the corpus is the operator's, not the repository's: there
// is no 78 MB transcript to commit and a fixture small enough to commit does
// not exercise what this measures. Run it as
//
//	PROF_FILE=~/.claude/projects/<project>/<session>.jsonl \
//	  go test ./internal/transcript -run XXX -bench ParseRealTranscript \
//	  -benchtime 3x -count 2
//
// Figures recorded on an Apple M1 Pro against a 78 MB transcript, so a later
// reading has something to be compared against rather than an impression:
//
//	before decode-once   1010.0 ms   292.8 MB   744,534 allocs
//	after  decode-once   1008.4 ms   283.7 MB   657,522 allocs
//	after  chain reuse    1013.7 ms   214.1 MB   615,003 allocs
//
// The time has not moved across either change and the individual runs straddle
// the baseline in both directions, so nothing should be read into it.
//
// The time difference there is noise — the individual runs straddle the
// baseline — and saying so is the point of writing both down.
func BenchmarkParseRealTranscript(b *testing.B) {
	path := os.Getenv("PROF_FILE")
	if path == "" {
		b.Skip("set PROF_FILE to a Claude Code transcript")
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s, err := ParseClaudeCodeFile(path)
		if err != nil {
			b.Fatal(err)
		}
		if s == nil {
			b.Fatal("nil session")
		}
	}
}
