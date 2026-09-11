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
//	after  label truncate   989.0 ms   201.9 MB   611,786 allocs
//	after  label once       918.0 ms   193.5 MB   587,147 allocs
//	after  scanner          935.0 ms   158.8 MB   386,573 allocs
//	after  measured source  947.0 ms   129.4 MB   386,292 allocs
//
// The scanner ContentBytes uses is checked against the decoder it replaced by
// FuzzContentBytesMatchesTheDecoder. What that is worth is not the execution
// count: 145 million executions is a number, and a number is not a bound. What
// bounds it is that the corpus stopped growing — 489 interesting inputs after
// ten minutes, 512 after three more, nine of them new. A discovery curve that
// has flattened is evidence; a big number on its own is not.
//
// Both real defects came from the hand-written corpus, not the fuzzer.
//
// Read no timing signal out of this benchmark at this scale. Twelve runs of
// one unchanged binary spanned 955 to 1193 ms on this machine, a band of 12%
// either side of the middle, which is wider than any of the three changes
// above moved the median. The bytes and the allocation counts are stable to
// four figures run to run and are the only numbers here worth comparing.
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
