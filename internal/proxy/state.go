package proxy

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/ledger"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// sessionState is what the proxy remembers about one client session so it
// can classify each new cache read the moment the response arrives.
type sessionState struct {
	last      transcript.Usage
	lastModel string
	lastSeen  time.Time
	model     string
	requests  int
	prompt    int
	reads     int
	writes    int
	output    int
	breaks    int
	costUSD   float64
}

// CacheOutcome is the ledger's live classification type.
type CacheOutcome = ledger.CacheOutcome

// stats is the proxy's in-memory observability state. It is derived data
// only and is lost on restart; the ledger is the durable record.
type stats struct {
	mu            sync.Mutex
	started       time.Time
	sessions      map[string]*sessionState
	requests      map[string]int // by status class: 2xx, 4xx, 5xx, refused
	upstreamErrs  map[int]int
	breakCauses   map[string]int
	latencySum    time.Duration
	latencyCount  int
	refusedByKind map[string]int
}

func newStats() *stats {
	return &stats{
		started:       time.Now(),
		sessions:      map[string]*sessionState{},
		requests:      map[string]int{},
		upstreamErrs:  map[int]int{},
		breakCauses:   map[string]int{},
		refusedByKind: map[string]int{},
	}
}

// causeForLiveBreak names the most likely cause from what the proxy can
// see at response time. The offline diff does a fuller job with the
// message history; this is the version that can be logged immediately.
func causeForLiveBreak(prev transcript.Usage, prevModel string, gap time.Duration, cur transcript.Usage, model string) string {
	switch {
	case gap > cachemodel.TTLOf(prev):
		return "cache expired (gap longer than the TTL)"
	case prevModel != model:
		return "model changed"
	case cur.CacheRead == 0:
		return "system prompt or tools changed"
	default:
		return "prefix diverged inside the message history"
	}
}

// observe records a completed request and returns the cache outcome for
// the ledger and the log line.
func (s *stats) observe(rec *ledger.Record, latency time.Duration) *CacheOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latencySum += latency
	s.latencyCount++
	switch {
	case rec.Status >= 500:
		s.requests["5xx"]++
		s.upstreamErrs[rec.Status]++
	case rec.Status >= 400:
		s.requests["4xx"]++
		if rec.Status == 429 {
			s.upstreamErrs[rec.Status]++
		}
	case rec.Status >= 200:
		s.requests["2xx"]++
	}
	u := rec.Response.Usage
	if u == nil || rec.SessionID == "" {
		return nil
	}
	cur := transcript.Usage{Input: u.Input, CacheCreation: u.CacheCreation, CacheRead: u.CacheRead, Output: u.Output, Create5m: u.Create5m, Create1h: u.Create1h}
	st, ok := s.sessions[rec.SessionID]
	if !ok {
		st = &sessionState{}
		s.sessions[rec.SessionID] = st
	}
	var out *CacheOutcome
	if st.requests > 0 {
		outcome, expected := cachemodel.ClassifyRead(st.last, cur)
		out = &CacheOutcome{Outcome: outcome.String(), Expected: expected}
		if outcome == cachemodel.ReadBroken {
			out.Deficit = expected - cur.CacheRead
			out.Cause = causeForLiveBreak(st.last, st.lastModel, rec.Timestamp.Sub(st.lastSeen), cur, rec.Model)
			st.breaks++
			s.breakCauses[out.Cause]++
		}
	}
	st.last, st.lastModel, st.lastSeen, st.model = cur, rec.Model, rec.Timestamp, rec.Model
	st.requests++
	st.prompt += cur.PromptTotal()
	st.reads += cur.CacheRead
	st.writes += cur.CacheCreation
	st.output += cur.Output
	if p, ok := cachemodel.PriceFor(rec.Model); ok {
		st.costUSD += cachemodel.CostUSD(cur, p)
	}
	return out
}

func (s *stats) refused(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests["refused"]++
	s.refusedByKind[kind]++
}

// SessionSummary is one row of the status endpoint.
type SessionSummary struct {
	Session      string    `json:"session"`
	Model        string    `json:"model"`
	Requests     int       `json:"requests"`
	PromptTokens int       `json:"prompt_tokens"`
	CachedShare  float64   `json:"cached_share"`
	Breaks       int       `json:"cache_breaks"`
	ListCostUSD  float64   `json:"list_cost_usd"`
	LastSeen     time.Time `json:"last_seen"`
}

// Status is the status endpoint's body.
type Status struct {
	UptimeSeconds int64            `json:"uptime_seconds"`
	Requests      map[string]int   `json:"requests"`
	Sessions      []SessionSummary `json:"sessions"`
	PriceTable    string           `json:"price_table"`
	Rules         string           `json:"rules"`
}

func (s *stats) status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Status{UptimeSeconds: int64(time.Since(s.started).Seconds()), Requests: map[string]int{}, PriceTable: cachemodel.PriceTableVersion, Rules: cachemodel.RulesVersion}
	for k, v := range s.requests {
		out.Requests[k] = v
	}
	for id, st := range s.sessions {
		share := 0.0
		if st.prompt > 0 {
			share = float64(st.reads) / float64(st.prompt)
		}
		out.Sessions = append(out.Sessions, SessionSummary{Session: short(id), Model: st.model, Requests: st.requests, PromptTokens: st.prompt, CachedShare: share, Breaks: st.breaks, ListCostUSD: st.costUSD, LastSeen: st.lastSeen})
	}
	sort.Slice(out.Sessions, func(i, j int) bool { return out.Sessions[i].LastSeen.After(out.Sessions[j].LastSeen) })
	return out
}

// metrics renders the Prometheus text exposition format by hand; the
// metric set is small and the binary carries no client library.
func (s *stats) metrics() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var prompt, reads, writes, breaks int
	for _, st := range s.sessions {
		prompt += st.prompt
		reads += st.reads
		writes += st.writes
		breaks += st.breaks
	}
	share := 0.0
	if prompt > 0 {
		share = float64(reads) / float64(prompt)
	}
	var b []byte
	line := func(format string, args ...any) { b = fmt.Appendf(b, format+"\n", args...) }
	line("# HELP buffy_requests_total Requests handled, by outcome class.")
	line("# TYPE buffy_requests_total counter")
	for _, k := range sortedKeys(s.requests) {
		line(`buffy_requests_total{class=%q} %d`, k, s.requests[k])
	}
	line("# HELP buffy_prompt_tokens_total Prompt tokens the provider processed.")
	line("# TYPE buffy_prompt_tokens_total counter")
	line("buffy_prompt_tokens_total %d", prompt)
	line("# HELP buffy_cache_read_tokens_total Prompt tokens served from cache.")
	line("# TYPE buffy_cache_read_tokens_total counter")
	line("buffy_cache_read_tokens_total %d", reads)
	line("# HELP buffy_cache_write_tokens_total Prompt tokens written to cache.")
	line("# TYPE buffy_cache_write_tokens_total counter")
	line("buffy_cache_write_tokens_total %d", writes)
	line("# HELP buffy_cached_share Cache reads divided by prompt tokens, all sessions.")
	line("# TYPE buffy_cached_share gauge")
	line("buffy_cached_share %.4f", share)
	line("# HELP buffy_cache_break_total Responses whose cache read fell short of the expectation, by cause.")
	line("# TYPE buffy_cache_break_total counter")
	line("buffy_cache_break_total %d", breaks)
	for _, k := range sortedKeys(s.breakCauses) {
		line(`buffy_cache_break_total{cause=%q} %d`, k, s.breakCauses[k])
	}
	line("# HELP buffy_upstream_errors_total Provider responses with an error status.")
	line("# TYPE buffy_upstream_errors_total counter")
	codes := make([]int, 0, len(s.upstreamErrs))
	for c := range s.upstreamErrs {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	for _, c := range codes {
		line(`buffy_upstream_errors_total{status="%d"} %d`, c, s.upstreamErrs[c])
	}
	line("# HELP buffy_refused_total Requests Buffy refused locally, by guard.")
	line("# TYPE buffy_refused_total counter")
	for _, k := range sortedKeys(s.refusedByKind) {
		line(`buffy_refused_total{guard=%q} %d`, k, s.refusedByKind[k])
	}
	line("# HELP buffy_request_latency_seconds Time from request received to response finished, including the provider.")
	line("# TYPE buffy_request_latency_seconds summary")
	line("buffy_request_latency_seconds_sum %.6f", s.latencySum.Seconds())
	line("buffy_request_latency_seconds_count %d", s.latencyCount)
	return string(b)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
