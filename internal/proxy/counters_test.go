package proxy

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

func metricValue(t *testing.T, out, name string) float64 {
	t.Helper()
	m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + ` ([0-9.e+-]+)$`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("metric %s not found", name)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// A Prometheus counter may never decrease. These four were summed over the live
// session map, and that map evicts past maxSessions, so a long-running proxy
// would report them falling. Prometheus reads any decrease as a counter reset
// and attributes the whole prior value as new traffic, so every rate() over
// these metrics was wrong on a busy machine and right on an idle one.
func TestTokenCountersNeverDecreaseAcrossEviction(t *testing.T) {
	s := newStats()
	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return at }

	rec := func(session string) *ledger.Record {
		r := &ledger.Record{Timestamp: at, SessionID: session}
		r.Model = "claude-opus-5"
		u := transcript.Usage{Input: 1_000, CacheRead: 9_000, CacheCreation: 500, Output: 100}
		r.Response.Usage = &u
		return r
	}

	names := []string{
		"replay_prompt_tokens_total",
		"replay_cache_read_tokens_total",
		"replay_cache_write_tokens_total",
	}
	peak := map[string]float64{}

	// Well past maxSessions, so the map evicts many times over.
	for i := 0; i < maxSessions*3; i++ {
		s.observe(rec("session-" + strconv.Itoa(i)))
		out := s.metrics()
		for _, n := range names {
			v := metricValue(t, out, n)
			if v < peak[n] {
				t.Fatalf("%s decreased from %.0f to %.0f after %d sessions; a counter that falls makes every rate() query wrong",
					n, peak[n], v, i+1)
			}
			peak[n] = v
		}
	}

	// And the total must reflect everything seen, not just what survived.
	if got := peak["replay_prompt_tokens_total"]; got != float64(maxSessions*3*10_500) {
		t.Fatalf("prompt tokens %.0f, want %d: evicted sessions were dropped from the total",
			got, maxSessions*3*10_500)
	}
}
