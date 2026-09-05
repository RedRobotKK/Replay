package proxy

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// sessionState is what the proxy remembers about one client session so it
// can classify each new cache read the moment the response arrives and
// score what other layouts would have cost.
type sessionState struct {
	last       transcript.Usage
	lastSeen   time.Time
	model      string
	prefixHash string
	tally      analysis.Tally
	breaks     int
	// prefixChanges counts requests whose system prompt or tool definitions
	// differed from the request before, which a transcript cannot see.
	prefixChanges int
	// policy is the decision pinned at the session's first request and
	// edit the parameters it was made for; both empty until then. applied
	// and cleared are the policy's measured side.
	policy  policy.Decision
	edit    *policy.ContextEdit
	applied int
	cleared int
	// builder accumulates the session in the analysis's own shape, so the
	// live what-if figures are the ones replay replay prints for the ledger.
	// scoreMu serializes additions and simulations for one session without
	// holding the proxy-wide lock through a walk of the whole session.
	scoreMu sync.Mutex
	builder *ledger.SessionBuilder
	whatIf  []WhatIf
	context []analysis.ContextEntry
	reReads analysis.ReReads
	// errorByLane is the estimated prompt cost of error content carried by
	// each agent lane, from the same analysis replay prints, keyed by AgentID
	// with "" for the main loop.
	//
	// Per lane rather than one figure because the analysis that produces it
	// sees one lane at a time, while the denominator it is divided by,
	// tally.PromptTokens, counts every request of the session. A single field
	// meant the last lane to rescore decided the numerator and a quiet
	// sub-agent could erase a busy one. The sum is what the budget wants,
	// because the sum is scoped like the denominator.
	errorByLane map[string]int
	// breached is set once the session's guardrail tripped.
	breached bool
	// masked counts secrets replaced in this session's requests;
	// rehydrated and denied count placeholders restored and left in place
	// in its responses.
	masked     int
	rehydrated int
	denied     int
	// held counts this session's requests held behind a sibling, and
	// heldMS their total wait.
	held   int
	heldMS int64
	// generated is the generation time of the policy file the pinned
	// policy came from, for tying a revert to it.
	generated time.Time
}

// WhatIf is one candidate layout scored against the session so far. It is
// dry-run only: the candidate is simulated from measured usage and never
// sent to the provider.
type WhatIf struct {
	Policy string `json:"policy"`
	// EffectiveTokens prices writes and reads at the provider multipliers.
	EffectiveTokens float64 `json:"effective_tokens"`
	// VsAsRun is the change in effective tokens relative to what ran:
	// negative is a saving.
	VsAsRun     float64 `json:"vs_as_run"`
	CachedShare float64 `json:"cached_share"`
	ListCostUSD float64 `json:"list_cost_usd"`
	// Estimated is true when the score depends on the byte-to-token fit.
	Estimated bool `json:"estimated"`
	// ReachableLive says how a user would turn the candidate on.
	ReachableLive string `json:"reachable_live"`
}

// whatIfLogEvery is how many requests pass between what-if log lines for
// a session; the status endpoint always has the latest figures.
const whatIfLogEvery = 10

// maxSessions bounds the in-memory session state; the least recently
// seen sessions are dropped past it. The ledger is the durable record.
const maxSessions = 256

// stats is the proxy's in-memory observability state. It is derived data
// only and is lost on restart; the ledger is the durable record.
type stats struct {
	mu            sync.Mutex
	now           func() time.Time
	started       time.Time
	sessions      map[string]*sessionState
	requests      map[string]int // by status class: 2xx, 4xx, 5xx, refused
	upstreamErrs  map[int]int
	breakCauses   map[cachemodel.BreakCause]int
	latencySum    time.Duration
	latencyCount  int
	refusedByKind map[string]int
	policyApplied int
	retries       int
	// breaches and reverted are keyed by the policy the breaching sessions
	// were actually pinned to. A single counter meant evidence gathered
	// against one trigger reverted a different one after `replay learn`
	// wrote a newer file, and a single reverted flag meant that once any
	// policy had been reverted the guardrail was disarmed for every policy
	// after it.
	breaches     map[string]int
	reverted     map[string]bool
	revertReason string
	masked       map[string]int
	rehydrated   map[string]int
	denied       map[string]int
	held         int
	heldMS       int64
	// costUSD is list-price cost since start; dayCostUSD is the same for the
	// current UTC day, which dayStamp names. Sourced here rather than from
	// SpendGuard because the guard only records when a cap is configured, and
	// the question "what did today cost" is not conditional on having set one.
	// Lifetime token counters. These were once summed over the live session
	// map, which evicts past maxSessions, so they under-reported by whatever
	// had been evicted and could fall between scrapes. A Prometheus counter
	// that decreases is read as a reset, and every rate() over it is then
	// wrong. Accumulated here instead, once per request, so they only ever
	// climb.
	promptTokens int
	cacheReads   int
	cacheWrites  int
	breaksTotal  int
	costUSD      float64
	dayCostUSD   float64
	dayStamp     string
	// unpriced counts requests whose model the rules could not price. They
	// contribute nothing to the totals, so without this the totals would read
	// as complete when they are not.
	unpriced int
	// unparsed counts requests on paths this build cannot read, by path.
	// Everything Replay does hangs off parsing, so these requests were
	// forwarded with every guard and the masker inert.
	unparsed map[string]int
	// unmasked counts requests on paths the masker does not understand.
	unmasked map[string]int
}

func newStats() *stats {
	return &stats{
		now:           time.Now,
		started:       time.Now(),
		sessions:      map[string]*sessionState{},
		requests:      map[string]int{},
		upstreamErrs:  map[int]int{},
		breakCauses:   map[cachemodel.BreakCause]int{},
		refusedByKind: map[string]int{},
		breaches:      map[string]int{},
		unparsed:      map[string]int{},
		unmasked:      map[string]int{},
		reverted:      map[string]bool{},
		masked:        map[string]int{},
		rehydrated:    map[string]int{},
		denied:        map[string]int{},
	}
}

// observe records a completed request and returns the cache outcome for
// the ledger and the log line. Causes that need the message history are
// left to the offline diff; the live classification names what usage and
// timing alone can settle.
func (s *stats) observe(rec *ledger.Record) *ledger.CacheOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latencySum += time.Duration(rec.LatencyMS) * time.Millisecond
	s.latencyCount++
	s.retries += rec.Retries
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
	if rec.Response.Usage == nil || rec.SessionID == "" {
		return nil
	}
	cur := *rec.Response.Usage
	st := s.session(rec.SessionID)
	var out *ledger.CacheOutcome
	prefixChanged := st.tally.Requests > 0 && rec.PrefixHash != st.prefixHash
	if prefixChanged {
		st.prefixChanges++
	}
	if st.tally.Requests > 0 {
		outcome, expected := cachemodel.ClassifyRead(st.last, cur)
		out = &ledger.CacheOutcome{Outcome: outcome.String(), Expected: expected}
		if outcome == cachemodel.ReadBroken {
			out.Deficit = expected - cur.CacheRead
			out.Cause = s.breakCause(st, rec, prefixChanged)
			st.breaks++
			s.breaksTotal++
			s.breakCauses[out.Cause]++
		}
	}
	st.last, st.lastSeen, st.model, st.prefixHash = cur, rec.Timestamp, rec.Model, rec.PrefixHash
	before := st.tally.CostUSD
	st.tally.Add(cur, rec.Model)
	s.addCost(st.tally.CostUSD - before)
	s.promptTokens += cur.PromptTotal()
	s.cacheReads += cur.CacheRead
	s.cacheWrites += cur.CacheCreation
	if rec.Policy != "" {
		st.applied++
		s.policyApplied++
	}
	for name, n := range rec.Masked {
		st.masked += n
		s.masked[name] += n
	}
	for dest, n := range rec.Rehydrated {
		st.rehydrated += n
		s.rehydrated[dest] += n
	}
	for dest, n := range rec.RehydrationDenied {
		st.denied += n
		s.denied[dest] += n
	}
	if rec.HeldMS > 0 {
		st.held++
		st.heldMS += rec.HeldMS
		s.held++
		s.heldMS += rec.HeldMS
	}
	st.cleared += rec.Response.ClearedInputTokens
	return out
}

// totalErrorTokens sums every lane. Callers hold the lock.
func (st *sessionState) totalErrorTokens() (n int) {
	for _, v := range st.errorByLane {
		n += v
	}
	return n
}

// errorTokens returns a session's error-content prompt tokens and its prompt
// tokens so far, for the error budget.
//
// Both figures cover the whole session. Summing the lanes is the point: the
// denominator counts every request whatever lane it came from, so a numerator
// from one lane would be a ratio between two different populations, and the
// guard that reads it refuses live traffic.
func (s *stats) errorTokens(sessionID string) (errorTokens, promptTokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.sessions[sessionID]
	if !ok {
		return 0, 0
	}
	return st.totalErrorTokens(), st.tally.PromptTokens
}

// setLaneErrors records one lane's error-content total. Lanes share the
// session's state: keying the session map by agent instead would give every
// sub-agent its own policy pin, which ADR-0003 forbids, and would split the
// spend cap per agent as well.
func (s *stats) setLaneErrors(sessionID, agentID string, tokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.session(sessionID)
	if st.errorByLane == nil {
		st.errorByLane = map[string]int{}
	}
	st.errorByLane[agentID] = tokens
}

// session finds or creates a session's state, evicting the least recently
// seen ones past maxSessions. Callers hold the lock.
func (s *stats) session(id string) *sessionState {
	st, ok := s.sessions[id]
	if ok {
		return st
	}
	for len(s.sessions) >= maxSessions {
		oldest, oldestSeen := "", time.Time{}
		for k, v := range s.sessions {
			if oldest == "" || v.lastSeen.Before(oldestSeen) {
				oldest, oldestSeen = k, v.lastSeen
			}
		}
		delete(s.sessions, oldest)
	}
	st = &sessionState{lastSeen: time.Now()}
	s.sessions[id] = st
	return st
}

// pinned returns a session's policy decision and parameters when one was
// made in this process, and false otherwise.
func (s *stats) pinned(sessionID string) (*policy.ContextEdit, policy.Decision, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.sessions[sessionID]
	if !ok || st.policy == "" {
		return nil, "", false
	}
	return st.edit, st.policy, true
}

// pin records a session's decision. The session is created here when its
// first request has not completed yet, so the pin exists before any
// usage does. A decision already made is kept.
func (s *stats) pin(sessionID string, edit *policy.ContextEdit, decision policy.Decision, generated time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.session(sessionID)
	if st.policy == "" {
		st.policy, st.edit, st.generated = decision, edit, generated
	}
}

// trialSession returns a treated session's policy and file generation
// time, for the guardrail; false for controls and flag-set policies.
func (s *stats) trialSession(sessionID string) (*policy.ContextEdit, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.sessions[sessionID]
	if !ok || st.policy != policy.Applied || st.edit == nil || st.generated.IsZero() {
		return nil, time.Time{}, false
	}
	return st.edit, st.generated, true
}

// breakCause names a break from what the proxy can see. A changed prefix
// is certain, since the proxy hashed both requests; the usage-and-timing
// causes come next; the rest is left to the offline diff.
func (s *stats) breakCause(st *sessionState, rec *ledger.Record, prefixChanged bool) cachemodel.BreakCause {
	if prefixChanged {
		return cachemodel.CausePrefixChange
	}
	cause, ok := cachemodel.ClassifyBreak(st.last, *rec.Response.Usage, st.model, rec.Model, rec.Timestamp.Sub(st.lastSeen))
	if !ok {
		return cachemodel.CauseUnknown
	}
	return cause
}

// rescore adds the record to the session's analysis shape, simulates the
// candidate layouts over the session as it stands, and stores the result
// for the status endpoint. It runs after the response has been delivered
// and takes only the session's own lock while it walks the session, so
// other sessions are never held up. It returns a log line every
// whatIfLogEvery requests and an empty string otherwise.
func (s *stats) rescore(rec *ledger.Record) (string, analysis.ReReads) {
	s.mu.Lock()
	st, ok := s.sessions[rec.SessionID]
	if !ok {
		s.mu.Unlock()
		return "", analysis.ReReads{}
	}
	if st.builder == nil {
		st.builder = ledger.NewSessionBuilder(rec.SessionID, "")
	}
	requests := st.tally.Requests
	s.mu.Unlock()

	st.scoreMu.Lock()
	st.builder.Add(*rec)
	session := st.builder.Session()
	lane := session.Lane(rec.AgentID, rec.AgentID != "")
	report := analysis.AnalyzeLane(session, lane)
	policies := report.Policies()
	st.scoreMu.Unlock()

	asRun := policies[0]
	rows := make([]WhatIf, 0, len(policies))
	for _, p := range policies {
		row := WhatIf{Policy: p.Name, EffectiveTokens: p.EffectiveTokens, CachedShare: p.CachedShare(), ListCostUSD: p.CostUSD, Estimated: p.Estimated, ReachableLive: p.ReachableLive}
		if asRun.EffectiveTokens > 0 {
			row.VsAsRun = (p.EffectiveTokens - asRun.EffectiveTokens) / asRun.EffectiveTokens
		}
		rows = append(rows, row)
	}
	errorTokens := 0
	for _, e := range report.Errors {
		errorTokens += e.PromptTokens.Value
	}
	s.mu.Lock()
	st.whatIf = rows
	st.reReads = report.ReReads
	// Blame was computed and discarded here. It is the only attribution of what
	// a session's context is made of, and the proxy is the one place it can be
	// produced from provider usage rather than from the byte-to-token fit.
	st.context = analysis.EnteredContext(report.Blame)
	if st.errorByLane == nil {
		st.errorByLane = map[string]int{}
	}
	// Replaces this lane's figure rather than adding to it: report.Errors is
	// that lane's running total, not a delta.
	st.errorByLane[rec.AgentID] = errorTokens
	s.mu.Unlock()

	if requests%whatIfLogEvery != 0 || len(rows) < 2 {
		return "", report.ReReads
	}
	best := rows[1]
	for _, r := range rows[2:] {
		if r.VsAsRun < best.VsAsRun {
			best = r
		}
	}
	if best.VsAsRun >= 0 {
		return fmt.Sprintf("what-if session=%s requests=%d as-run %.0f effective tokens; no candidate layout beats it", short(rec.SessionID), requests, asRun.EffectiveTokens), report.ReReads
	}
	tier := "measured"
	if best.Estimated {
		tier = "estimated"
	}
	return fmt.Sprintf("what-if session=%s requests=%d as-run %.0f effective tokens; best candidate %s %+.0f%% (%s); live: %s", short(rec.SessionID), requests, asRun.EffectiveTokens, best.Policy, best.VsAsRun*100, tier, best.ReachableLive), report.ReReads
}

func (s *stats) refused(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests["refused"]++
	s.refusedByKind[kind]++
}

// SessionSummary is one row of the status endpoint.
type SessionSummary struct {
	Session      string  `json:"session"`
	Model        string  `json:"model"`
	Requests     int     `json:"requests"`
	PromptTokens int     `json:"prompt_tokens"`
	CachedShare  float64 `json:"cached_share"`
	Breaks       int     `json:"cache_breaks"`
	// PrefixChanges counts requests whose system prompt or tools differed
	// from the previous request; each one rewrites the cache from the top.
	PrefixChanges int       `json:"prefix_changes"`
	ListCostUSD   float64   `json:"list_cost_usd"`
	LastSeen      time.Time `json:"last_seen"`
	// Policy is the decision pinned at the session's first request and
	// PinnedPolicy the policy it was made for. PolicyApplied counts
	// requests that carried the parameter; ClearedInputTokens is what the
	// provider's edits removed from prompts.
	Policy             string `json:"policy,omitempty"`
	PinnedPolicy       string `json:"pinned_policy,omitempty"`
	PolicyApplied      int    `json:"policy_applied,omitempty"`
	ClearedInputTokens int    `json:"cleared_input_tokens,omitempty"`
	// ErrorShare is the share of the session's prompt tokens that carried
	// error content, as the error budget judges it.
	ErrorShare float64 `json:"error_share"`
	// Masked counts secrets replaced with placeholders in this session's
	// requests, so the user can check coverage (MK-7).
	Masked int `json:"masked,omitempty"`
	// Rehydrated counts placeholders restored in this session's responses;
	// RehydrationDenied counts those left in place.
	Rehydrated        int `json:"rehydrated,omitempty"`
	RehydrationDenied int `json:"rehydration_denied,omitempty"`
	// Held counts requests held behind a sibling with the same prefix;
	// HeldMS is their total wait.
	Held   int   `json:"held,omitempty"`
	HeldMS int64 `json:"held_ms,omitempty"`
	// ReReads is the context-editing guardrail: file reads that repeated a
	// path already in context, before and after the provider's first clear.
	// Context is what entered this session's context, by tool. It does not
	// subtract cleared or compacted content; see analysis.ContextEntry.
	Context []analysis.ContextEntry `json:"context,omitempty"`
	ReReads analysis.ReReads        `json:"re_reads"`
	// WhatIf scores candidate layouts over the session so far; as-run is
	// first. Nothing here was sent to the provider.
	WhatIf []WhatIf `json:"what_if,omitempty"`
}

// Status is the status endpoint's body.
type Status struct {
	UptimeSeconds int64            `json:"uptime_seconds"`
	Requests      map[string]int   `json:"requests"`
	Sessions      []SessionSummary `json:"sessions"`
	PriceTable    string           `json:"price_table"`
	Rules         string           `json:"rules"`
	// SpendCapNotEnforced is true when a dollar cap is configured and at least
	// one request could not be priced, so that traffic is not capped at all.
	SpendCapNotEnforced bool `json:"spend_cap_not_enforced,omitempty"`
	// Refusals counts requests answered locally, by guard. The total is in
	// Requests["refused"]; this names which guard did it, which is the part a
	// person needs to act on.
	Refusals map[string]int `json:"refusals,omitempty"`
	// Caps says which spend limits are configured, so a reader that cannot
	// see serve's flags, `replay doctor` over HTTP for instance, can tell
	// whether a blind dollar cap is already covered by a token one.
	Caps CapStatus `json:"caps"`
	// CostUSD and DayCostUSD are list-price cost since start and for the
	// current UTC day.
	CostUSD    float64 `json:"cost_usd"`
	DayCostUSD float64 `json:"day_cost_usd"`
	// Trial reports the live trial of a learned policy, when one runs.
	Trial TrialStatus `json:"trial"`
}

// CapStatus is which spend limits are set, not their values. The values are
// the operator's business and a status endpoint is a thing other software
// reads; whether a limit exists is what a diagnostic needs.
type CapStatus struct {
	SessionTokens bool `json:"session_tokens,omitempty"`
	DayTokens     bool `json:"day_tokens,omitempty"`
	SessionUSD    bool `json:"session_usd,omitempty"`
	DayUSD        bool `json:"day_usd,omitempty"`
}

// TrialStatus is the trial's arms and guardrail state.
type TrialStatus struct {
	Treated  int    `json:"treated"`
	Control  int    `json:"control"`
	Breached int    `json:"breached"`
	Reverted string `json:"reverted,omitempty"`
}

func (s *stats) status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Status{UptimeSeconds: int64(time.Since(s.started).Seconds()), Requests: map[string]int{}, PriceTable: cachemodel.PriceTableVersion, Rules: cachemodel.RulesVersionInEffect()}
	for k, v := range s.requests {
		out.Requests[k] = v
	}
	// Every session that breached, whatever policy it was pinned to. The
	// revert decision is per-policy; this figure answers "how many sessions
	// tripped the guardrail", which is a different and legitimate question.
	breached := 0
	for _, n := range s.breaches {
		breached += n
	}
	out.Trial = TrialStatus{Breached: breached, Reverted: s.revertReason}
	if len(s.refusedByKind) > 0 {
		out.Refusals = map[string]int{}
		for k, v := range s.refusedByKind {
			out.Refusals[k] = v
		}
	}
	out.CostUSD = s.costUSD
	if s.now().UTC().Format("2006-01-02") == s.dayStamp {
		out.DayCostUSD = s.dayCostUSD
	}
	for id, st := range s.sessions {
		switch st.policy {
		case policy.Control:
			out.Trial.Control++
		case policy.Applied:
			if st.edit != nil && !st.generated.IsZero() {
				out.Trial.Treated++
			}
		}
		out.Sessions = append(out.Sessions, SessionSummary{Session: short(id), Model: st.model, Requests: st.tally.Requests, PromptTokens: st.tally.PromptTokens, CachedShare: st.tally.CachedShare(), Breaks: st.breaks, PrefixChanges: st.prefixChanges, ListCostUSD: st.tally.CostUSD, LastSeen: st.lastSeen, Policy: string(st.policy), PinnedPolicy: pinnedName(st.edit), PolicyApplied: st.applied, ClearedInputTokens: st.cleared, Context: st.context, ReReads: st.reReads, WhatIf: st.whatIf, ErrorShare: share(st.totalErrorTokens(), st.tally.PromptTokens), Masked: st.masked, Rehydrated: st.rehydrated, RehydrationDenied: st.denied, Held: st.held, HeldMS: st.heldMS})
	}
	sort.Slice(out.Sessions, func(i, j int) bool { return out.Sessions[i].LastSeen.After(out.Sessions[j].LastSeen) })
	return out
}

// metrics renders the Prometheus text exposition format by hand; the
// metric set is small and the binary carries no client library.
func (s *stats) metrics() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Lifetime totals, not a walk of the live session map: that map evicts,
	// and a counter must never fall. CachedShare is still a ratio of the two
	// lifetime figures.
	total := analysis.Tally{PromptTokens: s.promptTokens, Reads: s.cacheReads, Writes: s.cacheWrites}
	breaks := s.breaksTotal
	var b []byte
	line := func(format string, args ...any) { b = fmt.Appendf(b, format+"\n", args...) }
	line("# HELP replay_requests_total Requests handled, by outcome class.")
	line("# TYPE replay_requests_total counter")
	for _, k := range sortedKeys(s.requests) {
		line(`replay_requests_total{class=%q} %d`, k, s.requests[k])
	}
	line("# HELP replay_prompt_tokens_total Prompt tokens the provider processed.")
	line("# TYPE replay_prompt_tokens_total counter")
	line("replay_prompt_tokens_total %d", total.PromptTokens)
	line("# HELP replay_cache_read_tokens_total Prompt tokens served from cache.")
	line("# TYPE replay_cache_read_tokens_total counter")
	line("replay_cache_read_tokens_total %d", total.Reads)
	line("# HELP replay_cache_write_tokens_total Prompt tokens written to cache.")
	line("# TYPE replay_cache_write_tokens_total counter")
	line("replay_cache_write_tokens_total %d", total.Writes)
	line("# HELP replay_cached_share Cache reads divided by prompt tokens, all sessions.")
	line("# TYPE replay_cached_share gauge")
	line("replay_cached_share %.4f", total.CachedShare())
	line("# HELP replay_cache_break_total Responses whose cache read fell short of the expectation, by cause.")
	line("# TYPE replay_cache_break_total counter")
	line("replay_cache_break_total %d", breaks)
	causes := make([]string, 0, len(s.breakCauses))
	for c := range s.breakCauses {
		causes = append(causes, string(c))
	}
	sort.Strings(causes)
	for _, c := range causes {
		line(`replay_cache_break_total{cause=%q} %d`, c, s.breakCauses[cachemodel.BreakCause(c)])
	}
	dayCost := s.dayCostUSD
	if s.now().UTC().Format("2006-01-02") != s.dayStamp {
		dayCost = 0
	}
	line("# HELP replay_cost_usd_total List-price cost of all traffic since start.")
	line("# TYPE replay_cost_usd_total counter")
	line("replay_cost_usd_total %.6f", s.costUSD)
	line("# HELP replay_cost_usd_day List-price cost for the current UTC day.")
	line("# TYPE replay_cost_usd_day gauge")
	line("replay_cost_usd_day %.6f", dayCost)
	line("# HELP replay_cost_unpriced_requests_total Requests whose model the rules could not price, so they are in no cost figure.")
	line("# TYPE replay_cost_unpriced_requests_total counter")
	line("replay_cost_unpriced_requests_total %d", s.unpriced)
	unparsed := 0
	for _, v := range s.unparsed {
		unparsed += v
	}
	unmasked := 0
	for _, v := range s.unmasked {
		unmasked += v
	}
	line("# HELP replay_unmasked_requests_total Requests on a path the secret masker does not understand, forwarded without masking.")
	line("# TYPE replay_unmasked_requests_total counter")
	line("replay_unmasked_requests_total %d", unmasked)
	line("# HELP replay_unparsed_requests_total Requests forwarded on a path this build cannot read, so no guard, masker or ledger applied to them.")
	line("# TYPE replay_unparsed_requests_total counter")
	line("replay_unparsed_requests_total %d", unparsed)
	line("# HELP replay_upstream_errors_total Provider responses with an error status.")
	line("# TYPE replay_upstream_errors_total counter")
	codes := make([]int, 0, len(s.upstreamErrs))
	for c := range s.upstreamErrs {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	for _, c := range codes {
		line(`replay_upstream_errors_total{status="%d"} %d`, c, s.upstreamErrs[c])
	}
	line("# HELP replay_refused_total Requests Replay refused locally, by guard.")
	line("# TYPE replay_refused_total counter")
	for _, k := range sortedKeys(s.refusedByKind) {
		line(`replay_refused_total{guard=%q} %d`, k, s.refusedByKind[k])
	}
	line("# HELP replay_retries_total Requests the proxy resent after a retryable provider failure.")
	line("# TYPE replay_retries_total counter")
	line("replay_retries_total %d", s.retries)
	line("# HELP replay_held_total Requests held behind a sibling with the same prefix.")
	line("# TYPE replay_held_total counter")
	line("replay_held_total %d", s.held)
	line("# HELP replay_held_milliseconds_total Time requests spent held behind a sibling.")
	line("# TYPE replay_held_milliseconds_total counter")
	line("replay_held_milliseconds_total %d", s.heldMS)
	line("# HELP replay_masked_total Secrets replaced with placeholders, by pattern.")
	line("# TYPE replay_masked_total counter")
	for _, k := range sortedKeys(s.masked) {
		line(`replay_masked_total{pattern=%q} %d`, k, s.masked[k])
	}
	line("# HELP replay_rehydrated_total Placeholders restored in responses, by destination.")
	line("# TYPE replay_rehydrated_total counter")
	for _, k := range sortedKeys(s.rehydrated) {
		line(`replay_rehydrated_total{destination=%q} %d`, k, s.rehydrated[k])
	}
	line("# HELP replay_rehydration_denied_total Placeholders left in place, by destination and reason.")
	line("# TYPE replay_rehydration_denied_total counter")
	for _, k := range sortedKeys(s.denied) {
		line(`replay_rehydration_denied_total{destination=%q} %d`, k, s.denied[k])
	}
	line("# HELP replay_policy_applied_total Requests that carried a Replay-added request parameter.")
	line("# TYPE replay_policy_applied_total counter")
	line(`replay_policy_applied_total{policy=%q} %d`, policy.Name, s.policyApplied)
	line("# HELP replay_request_latency_seconds Time from request received to response finished, including the provider.")
	line("# TYPE replay_request_latency_seconds summary")
	line("replay_request_latency_seconds_sum %.6f", s.latencySum.Seconds())
	line("replay_request_latency_seconds_count %d", s.latencyCount)
	return string(b)
}

func pinnedName(edit *policy.ContextEdit) string {
	if edit == nil {
		return ""
	}
	return edit.String()
}

func share(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// addCost folds one request's list-price cost into the running and UTC-day
// totals. Callers hold the lock.
//
// A zero delta means the rules could not price the model. That is counted
// rather than added, because a total that silently omits unpriced traffic
// reads as complete and is not.
func (s *stats) addCost(delta float64) {
	if delta <= 0 {
		s.unpriced++
		return
	}
	stamp := s.now().UTC().Format("2006-01-02")
	if stamp != s.dayStamp {
		s.dayStamp, s.dayCostUSD = stamp, 0
	}
	s.costUSD += delta
	s.dayCostUSD += delta
}

// costs returns list-price cost since start and for the current UTC day.
//
// The day figure is rolled on read as well as on write, so a proxy that has
// been idle across midnight reports today's zero rather than yesterday's spend.
func (s *stats) costs() (total, day float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.now().UTC().Format("2006-01-02") != s.dayStamp {
		return s.costUSD, 0
	}
	return s.costUSD, s.dayCostUSD
}

// noteUnparsed records a request on a path this build cannot read, and reports
// whether this is the first time that path has been seen.
func (s *stats) noteUnparsed(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unparsed == nil {
		s.unparsed = map[string]int{}
	}
	first := s.unparsed[path] == 0
	s.unparsed[path]++
	return first
}

// unparsedTotal is every request forwarded without being understood.
func (s *stats) unparsedTotal() (n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.unparsed {
		n += v
	}
	return n
}

// noteUnmasked records a request the masker did not cover, reporting whether
// this is the first time that path has been seen.
func (s *stats) noteUnmasked(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unmasked == nil {
		s.unmasked = map[string]int{}
	}
	first := s.unmasked[path] == 0
	s.unmasked[path]++
	return first
}
