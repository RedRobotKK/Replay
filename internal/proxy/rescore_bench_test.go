package proxy

import (
	"fmt"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The cost of rescore as a session grows.
//
// This exists because nothing measured it. BenchmarkAddedLatency reports what
// a client waits for, and correctly closes its window before rescore runs:
// rec.LatencyMS is taken at server.go:592 and rescore is called at server.go:614,
// after the response has already been streamed. Widening that window would fold
// in work no client waits for and corrupt a number that is currently honest.
//
// So this is a different instrument answering a different question: what does
// the proxy spend per request, on its own CPU, as the session it is serving
// gets longer.
//
// The shape being measured is real. rescore calls AnalyzeLane over
// st.builder.Session() on every request, and AnalyzeLane walks the whole lane
// each time rather than folding in the new record, so the work per request
// grows with the session and the work across a session grows with its square.
// It is serialized per session by scoreMu, so it is also a throughput ceiling
// for any one session's lanes.
func BenchmarkRescoreBySessionLength(b *testing.B) {
	for _, n := range []int{10, 50, 100, 200} {
		b.Run(fmt.Sprintf("session-len-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := newStats()
				recs := syntheticSession("bench", n)
				// Warm the session to length n-1 without timing it, so the
				// measured call is one request against a session of that
				// length rather than the cost of building one.
				//
				// observe must run too: rescore returns immediately when the
				// session does not exist, and observe is what creates it. The
				// first version of this benchmark called only rescore, so
				// every iteration measured a map miss and reported a flat
				// ~250ns with zero allocations at every length. It looked
				// like proof the walk was cheap.
				for _, rec := range recs[:n-1] {
					r := rec
					s.observe(&r)
					s.rescore(&r)
				}
				last := recs[n-1]
				s.observe(&last)
				b.StartTimer()

				s.rescore(&last)
			}
			// ns/op is the cost of ONE rescore against a session of length n.
			// Divide across n values to see the growth: flat means the walk
			// was made incremental, linear means it is still O(session).
		})
	}
}

// BenchmarkRescoreWholeSession measures the total cost of serving a session of
// each length, which is the figure an operator actually pays.
//
// If per-request cost is linear in session length, this is quadratic. Reported
// separately because the per-request number understates what a long session
// costs in aggregate.
func BenchmarkRescoreWholeSession(b *testing.B) {
	for _, n := range []int{10, 50, 100, 200} {
		b.Run(fmt.Sprintf("session-len-%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := newStats()
				recs := syntheticSession("bench", n)
				// observe untimed, so this reports rescore alone. It does not
				// change the work rescore does: rescore reads tally.Requests
				// only to decide whether to emit a log line, never to decide
				// what to analyse.
				for _, rec := range recs {
					r := rec
					s.observe(&r)
				}
				b.StartTimer()

				for _, rec := range recs {
					r := rec
					s.rescore(&r)
				}
			}
			b.ReportMetric(float64(n), "requests/session")
		})
	}
}

// syntheticSession builds n records for one session, each carrying enough
// structure for AnalyzeLane to do its real work: a system prompt, tool
// definitions, a growing message history and provider usage.
func syntheticSession(sessionID string, n int) []ledger.Record {
	base := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	out := make([]ledger.Record, 0, n)
	for i := 0; i < n; i++ {
		rec := ledger.Record{
			Schema:    ledger.SchemaVersion,
			Timestamp: base.Add(time.Duration(i) * 5 * time.Second),
			SessionID: sessionID,
			RequestID: fmt.Sprintf("req-%d", i),
			Path:      "/v1/messages",
			Status:    200,
			LatencyMS: 100,
		}
		rec.Model = "claude-opus-5"
		rec.PrefixHash = "stable-prefix"
		rec.Prompt.SystemBytes = 3000
		rec.Prompt.ToolBytes = 40000
		rec.Prompt.ToolCount = 20
		rec.Prompt.CacheControlCount = 1
		// The turn's own user message plus a tool result, which is what makes
		// the lane grow rather than repeat.
		rec.Prompt.Messages = []ledger.Message{
			{Role: "user", Blocks: []ledger.Block{
				{Kind: "text", Label: "user text", Bytes: 240},
			}},
			{Role: "assistant", Blocks: []ledger.Block{
				{Kind: "tool_use", Label: fmt.Sprintf("Read(file-%d.go)", i), Bytes: 120},
			}},
			{Role: "user", Blocks: []ledger.Block{
				{Kind: "tool_result", Label: fmt.Sprintf("Read(file-%d.go)", i), Bytes: 4000},
			}},
		}
		rec.Response.Blocks = []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 400}}
		// Warm reads after the first, so the lane looks like a real cached
		// session rather than a cold one.
		cacheRead := 0
		if i > 0 {
			cacheRead = 40000 + i*1000
		}
		rec.Response.Usage = &transcript.Usage{
			Input:         200,
			CacheCreation: 43000,
			CacheRead:     cacheRead,
			Output:        100,
		}
		out = append(out, rec)
	}
	return out
}

// The cost of a session must not grow worse than quadratically.
//
// rescore re-walks the whole lane on every request, so the requests analysed
// across a session of n is 1+2+...+n — quadratic by design, and known. What
// this guards is a nested walk being added on top, which would make it cubic
// and which nothing else catches: the client never waits for rescore, so
// BenchmarkAddedLatency is structurally blind to this code.
//
// Counted, not timed. The first version measured wall-clock and asserted a
// ratio under 32x. It failed roughly one run in three on a loaded machine —
// four separate agents hit it in one day, and it blocked merges each time for
// a reason that had nothing to do with the code under test. A gate that fails
// on the weather is one people learn to re-run rather than read.
//
// The counter is the same claim decided by arithmetic: quadrupling the session
// should quadruple squared, about 16x. Anything past 32x is a second walk.
func TestRescore_SessionCostDoesNotGrowWorseThanQuadratic(t *testing.T) {
	analysed := func(n int) int64 {
		s := newStats()
		recs := syntheticSession("quadratic", n)
		for i := range recs {
			s.observe(&recs[i])
		}
		s.analysed.Store(0)
		for i := range recs {
			s.rescore(&recs[i])
		}
		return s.analysed.Load()
	}

	small, large := analysed(100), analysed(400)

	// The work must actually be happening, or the ratio measures nothing. An
	// earlier benchmark called rescore without observe, so every call returned
	// at a map miss and reported a flat 250ns with zero allocations — which
	// looked like proof the walk was cheap.
	if small < 100 {
		t.Fatalf("a 100-request session analysed %d lane requests, which is too few to be "+
			"walking the lane. rescore returns immediately for a session that does not "+
			"exist; check that observe ran first, or this test is measuring a map miss", small)
	}

	ratio := float64(large) / float64(small)
	t.Logf("100 requests analysed %d, 400 analysed %d, ratio %.2fx "+
		"(quadratic predicts 16x, cubic 64x)", small, large, ratio)
	if ratio > 32 {
		t.Errorf("quadrupling the session length analysed %.2fx as many lane requests "+
			"(%d -> %d), over the 32x bound. rescore re-walking the lane once per request "+
			"is already quadratic; worse than that means a nested walk was added.",
			ratio, small, large)
	}
}
