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
// A benchmark nobody runs is not a guard. BenchmarkAddedLatency deliberately
// asserts nothing, which is right for an absolute latency figure on noisy CI,
// but it leaves this path with no automated floor at all. So this is a test,
// and it asserts a RATIO rather than a duration: how much more it costs to
// serve twice as many requests. A ratio survives a slow or busy machine in a
// way "must finish in N ms" does not.
//
// Measured on an M1 Pro on 2026-09-06, doubling the session length cost 3.62x
// at 50->100 and 3.79x at 100->200. Quadratic is 4x. The bound is set at 6x:
// loose enough that noise and a slower machine do not fail it, tight enough
// that a regression to cubic, which would be 8x, does. It is a smoke alarm,
// not a stopwatch.
//
// If this fails, the likely cause is that rescore started doing more per
// request than walk the lane once, or that AnalyzeLane grew a nested walk.
//
// PASS: doubling the session costs less than 6x.
// FAIL: something made a long session disproportionately expensive, which is
// invisible to every other test here because the client never waits for it.
func TestRescore_SessionCostDoesNotGrowWorseThanQuadratic(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}

	cost := func(n int) time.Duration {
		// Best of three: the minimum is the run least disturbed by the
		// scheduler, which is what makes a timing assertion survivable on CI.
		best := time.Duration(1<<62 - 1)
		for try := 0; try < 3; try++ {
			s := newStats()
			recs := syntheticSession("guard", n)
			for i := range recs {
				s.observe(&recs[i])
			}
			start := time.Now()
			for i := range recs {
				s.rescore(&recs[i])
			}
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}

	small, large := cost(100), cost(200)
	if small <= 0 {
		t.Fatal("the 100-request session was not measurable, so this test cannot fail")
	}

	// The work must actually be happening, or the ratio is measuring nothing.
	// The first version of this file's benchmark called rescore without
	// observe, so every call returned at a map miss and reported a flat 250ns
	// with zero allocations. That looked like proof the walk was cheap.
	if small < 100*time.Microsecond {
		t.Fatalf("a 100-request session rescored in %v, which is too fast to be walking the "+
			"lane. rescore returns immediately for a session that does not exist; check "+
			"that observe ran first, or this test is measuring a map miss", small)
	}

	ratio := float64(large) / float64(small)
	t.Logf("100 requests: %v, 200 requests: %v, ratio %.2fx (quadratic is 4x)", small, large, ratio)
	if ratio > 6 {
		t.Errorf("doubling the session length cost %.2fx (%v -> %v), over the 6x bound. "+
			"rescore re-walks the whole lane on every request, so this path is already "+
			"quadratic across a session; worse than that means a nested walk was added. "+
			"Nothing else catches it: the client never waits for rescore, so "+
			"BenchmarkAddedLatency is structurally blind to this code.", ratio, small, large)
	}
}
