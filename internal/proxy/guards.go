package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/tenancy"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Guards are off unless configured. Each is a pure decision component the
// handler consults before forwarding (spend, loop) or after a response
// (breaker); none of them touches request or response bytes.

// SpendLimits caps prompt-plus-output tokens and list-price dollars per
// session and per UTC day. Zero means no cap. Dollar caps count only
// requests whose model is in the price table; the status endpoint shows
// the same figure, so a model with no price shows zero there too.
type SpendLimits struct {
	SessionTokens int
	DayTokens     int
	SessionUSD    float64
	DayUSD        float64
}

// spend is one counter pair.
//
// tokens and usd run for the life of the session; dayTokens and dayUSD are the
// part of that spent on the UTC day named by day. The two windows are different
// and SP-7 needs the second: a lane that ran all of yesterday has a large
// lifetime total and may have spent nothing today, so attributing today's cap
// on the lifetime figure blames it for a budget it never touched.
type spend struct {
	tokens int
	usd    float64
	seen   time.Time
	// order is the guard's touch counter at the last Record. seen stays for
	// attribution's staleness filter, which wants a wall-clock instant;
	// eviction uses this instead, because it needs a total order and the
	// clock does not always provide one.
	order uint64

	day       string
	dayTokens int
	dayUSD    float64
}

// maxSpendSessions bounds the guard's per-session table, per tenant; the
// least recently seen sessions within a given tenant's own table are
// dropped past it. A tenant with no traffic costs nothing: the table for it
// does not exist until its first Record.
const maxSpendSessions = 1024

// SpendGuard accounts tokens and dollars from provider usage and fails
// closed before the next request once a cap is reached. It never
// interrupts a response in flight.
//
// session and dayUsed are tenant-scoped (SP-6, docs/requirements.md): ADR-
// 0015 names the spend guard's accumulators as one of the four live
// surfaces that must gain a tenant dimension before any shared deployment
// is built on top of them, and this is that dimension. Both are keyed by
// tenancy.TenantID as the OUTER map, rather than by a single map keyed on a
// composed "tenant:session" string: there is then no byte sequence a
// session id could contain that selects another tenant's inner map, the
// same reason tenancy.Registry composes its own keys by length-prefixing
// rather than by a separator a caller's key might also contain
// (tenancy/ownership.go's scopedKey). A single mutex still guards both,
// unchanged from before SP-6: the lock already serialized every access to
// the one flat map, and nesting the map one level deeper changes nothing
// about who may read or write while the lock is held, so there is nothing
// here for two tenants' concurrent requests to race on beyond what already
// could not race.
type SpendGuard struct {
	limits  SpendLimits
	mu      sync.Mutex
	session map[tenancy.TenantID]map[string]*spend
	day     string
	// dayUsed is tenant-scoped; g.day (the UTC date string) is not, because
	// midnight is the same instant for every tenant and only the spend
	// recorded against that day needs to be told apart.
	dayUsed map[tenancy.TenantID]*spend
	now     func() time.Time
	// unpriceable records that a dollar cap was configured and at least one
	// request could not be priced from the table, so its cost in the running
	// total is the dearest known row standing in for it.
	//
	// The total is then an UPPER BOUND rather than a measurement. That is the
	// deliberate choice: counting an unknown model as zero failed OPEN, and an
	// operator who asked to stop at $20 had no cap at all on exactly the
	// traffic most likely to be expensive. Erring high costs them a cap that
	// fires early, which they can see and raise. What they cannot see, unless
	// this flag says so, is which of the two they are reading.
	unpriceable bool
	// order increments on every touch and breaks ties that the clock cannot.
	// Eviction scanned seen alone, which is only least-recently-used if
	// time.Now can separate two records. On Windows it cannot: its resolution
	// is coarse enough that a burst lands on one instant, every seen compares
	// equal, and the victim becomes whichever key Go's randomised map
	// iteration yields first. A counter has no resolution to run out of.
	order uint64
}

// NewSpendGuard builds a guard; a zero limits value disables it.
func NewSpendGuard(limits SpendLimits) *SpendGuard {
	return &SpendGuard{
		limits:  limits,
		session: map[tenancy.TenantID]map[string]*spend{},
		dayUsed: map[tenancy.TenantID]*spend{},
		now:     time.Now,
	}
}

// SetClock replaces the guard's clock. A replay of recorded requests must roll
// the day on the records' own timestamps, not on the wall clock of the replay.
func (g *SpendGuard) SetClock(now func() time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.now = now
}

// CapNotEnforced reports that a dollar cap is configured but at least one
// request could not be priced and was charged at the dearest known rate, as an
// upper bound, so the cap is applied to an over-estimate of that traffic and
// can fire early. The name predates the dearest-row rule (#261) and is kept
// because it is the status key readers already look for.
func (g *SpendGuard) CapNotEnforced() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.unpriceable
}

// Enabled reports whether any cap is set.
func (g *SpendGuard) Enabled() bool {
	return g != nil && (g.limits.SessionTokens > 0 || g.limits.DayTokens > 0 || g.limits.SessionUSD > 0 || g.limits.DayUSD > 0)
}

// Record adds a completed request's tokens and list-price cost, scoped to
// tenant. upperBound says the cost is the dearest known row standing in for
// a model the table could not price, so the running total is a bound and
// not a measurement.
//
// tenant must already be resolved (tenancy.ResolveTenant, at the proxy
// boundary). TenantUnknown and the empty TenantID are refused by not
// recording at all, rather than by landing in some shared "unresolved"
// bucket: SP-5 (docs/requirements.md) requires a request whose tenant
// cannot be resolved to be refused before any cap is consulted and to
// contribute to no accumulator, and the proxy boundary is where that
// refusal happens (internal/proxy/passthrough.go). This is the belt under
// that brace — the one case it protects against is a second, different
// caller that also failed to resolve a tenant and would otherwise silently
// come to share a bucket with the first.
func (g *SpendGuard) Record(tenant tenancy.TenantID, sessionID string, tokens int, usd float64, upperBound bool) {
	if !g.Enabled() || (tokens <= 0 && usd <= 0) {
		return
	}
	if tenant == tenancy.TenantUnknown || tenant == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	// Armed where the substitution happened, not where the cost came out zero.
	// The old condition described the behaviour before the dearest-row
	// substitution and became unreachable when that landed, so the one signal
	// an operator had that their cap total is an over-estimate stopped firing.
	if upperBound && (g.limits.SessionUSD > 0 || g.limits.DayUSD > 0) {
		g.unpriceable = true
	}
	g.rollDay()
	sessions, ok := g.session[tenant]
	if !ok {
		sessions = map[string]*spend{}
		g.session[tenant] = sessions
	}
	st, ok := sessions[sessionID]
	if !ok {
		for len(sessions) >= maxSpendSessions {
			oldest, oldestOrder := "", uint64(0)
			for k, v := range sessions {
				if oldest == "" || v.order < oldestOrder {
					oldest, oldestOrder = k, v.order
				}
			}
			delete(sessions, oldest)
		}
		st = &spend{}
		sessions[sessionID] = st
	}
	st.seen = g.now()
	g.order++
	st.order = g.order
	// Lazily, so a day roll costs nothing until a session is next seen. A
	// session never seen again keeps a stale stamp and is filtered out of
	// attribution by it, rather than needing a sweep.
	if st.day != g.day {
		st.day, st.dayTokens, st.dayUSD = g.day, 0, 0
	}
	st.dayTokens += tokens
	st.dayUSD += usd
	st.tokens += tokens
	st.usd += usd
	du, ok := g.dayUsed[tenant]
	if !ok {
		du = &spend{}
		g.dayUsed[tenant] = du
	}
	du.tokens += tokens
	du.usd += usd
}

// Check returns a human-readable reason when the next request for the
// session, under tenant, must be refused, or an empty string when it may
// proceed.
//
// tenant must already be resolved. An unresolved tenant fails CLOSED here
// rather than open: SP-5 requires an unresolvable tenant to be refused, and
// Check deciding "no identity, so nothing to check, so allow" would let
// exactly the traffic SP-5 names - a request nobody can attribute - through
// uncapped. The proxy boundary is expected to refuse such a request before
// Check is ever reached (internal/proxy/passthrough.go); this is the same
// belt-and-brace as Record's refusal to account for one.
func (g *SpendGuard) Check(tenant tenancy.TenantID, sessionID string) string {
	if !g.Enabled() {
		return ""
	}
	if tenant == tenancy.TenantUnknown || tenant == "" {
		return "spend guard: tenant identity is not resolved"
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rollDay()
	var used spend
	if sessions, ok := g.session[tenant]; ok {
		if st, ok := sessions[sessionID]; ok {
			used = *st
		}
	}
	var dayUsed spend
	if du, ok := g.dayUsed[tenant]; ok {
		dayUsed = *du
	}
	switch {
	case g.limits.SessionTokens > 0 && used.tokens >= g.limits.SessionTokens:
		return fmt.Sprintf("session spend cap reached: %d of %d tokens", used.tokens, g.limits.SessionTokens)
	case g.limits.SessionUSD > 0 && used.usd >= g.limits.SessionUSD:
		return fmt.Sprintf("session spend cap reached: $%.2f of $%.2f at list price", used.usd, g.limits.SessionUSD)
	case g.limits.DayTokens > 0 && dayUsed.tokens >= g.limits.DayTokens:
		return g.attributeDay(tenant, fmt.Sprintf("daily spend cap reached: %d of %d tokens",
			dayUsed.tokens, g.limits.DayTokens), false)
	case g.limits.DayUSD > 0 && dayUsed.usd >= g.limits.DayUSD:
		return g.attributeDay(tenant, fmt.Sprintf("daily spend cap reached: $%.2f of $%.2f at list price",
			dayUsed.usd, g.limits.DayUSD), true)
	}
	return ""
}

// dayLeader reports the session that spent the most of today's budget for
// tenant, in the unit that tripped, and whether the surviving sessions
// account for the whole day total for that tenant.
//
// Completeness is the point. The session table evicts least-recently-seen past
// maxSpendSessions and discards that session's spend with it, while the day
// total is untouched, so after enough churn the survivors no longer add up.
// The largest survivor is then not the largest spender, and naming it would
// blame a small lane for someone else's overrun. Callers hold the lock.
//
// Ranging g.session[tenant] on a tenant with no table yet (a nil map) is a
// zero-iteration range, not a nil-map panic — the same reason this never
// needed a presence check before tenant scoping existed.
func (g *SpendGuard) dayLeader(tenant tenancy.TenantID, byUSD bool) (id string, tokens int, usd float64, complete bool) {
	var accTokens int
	var accUSD float64
	var bestTokens int
	var bestUSD float64
	for k, st := range g.session[tenant] {
		if st.day != g.day {
			continue
		}
		accTokens += st.dayTokens
		accUSD += st.dayUSD
		if byUSD {
			if st.dayUSD > bestUSD {
				id, bestTokens, bestUSD = k, st.dayTokens, st.dayUSD
			}
			continue
		}
		if st.dayTokens > bestTokens {
			id, bestTokens, bestUSD = k, st.dayTokens, st.dayUSD
		}
	}
	var dayTokens int
	var dayUSD float64
	if du, ok := g.dayUsed[tenant]; ok {
		dayTokens, dayUSD = du.tokens, du.usd
	}
	// A cent of slack: dollar figures are summed floats, and a rounding
	// residue is not an accounting gap.
	complete = accTokens >= dayTokens && accUSD+0.005 >= dayUSD
	return id, bestTokens, bestUSD, complete
}

// attributeDay appends who spent tenant's day budget to a day-cap refusal, or
// says the accounting cannot support a name. Callers hold the lock.
func (g *SpendGuard) attributeDay(tenant tenancy.TenantID, reason string, byUSD bool) string {
	id, tokens, usd, complete := g.dayLeader(tenant, byUSD)
	if id == "" {
		// Nothing recorded today under a live session: the spend is real and
		// entirely unattributable, which is worth saying rather than hiding.
		return reason + "; no live session accounts for it"
	}
	if !complete {
		return fmt.Sprintf("%s; attribution is partial, the largest session still accounted for "+
			"holds %d tokens and earlier sessions were dropped", reason, tokens)
	}
	if byUSD {
		return fmt.Sprintf("%s; most of it from session %s ($%.2f)", reason, id, usd)
	}
	return fmt.Sprintf("%s; most of it from session %s (%d tokens)", reason, id, tokens)
}

// rollDay resets the daily counters, for every tenant at once, at UTC
// midnight. Callers hold the lock.
//
// One shared g.day and a full-map reset, not a per-tenant lazy rollover like
// the per-session one above: UTC midnight is the same instant for every
// tenant, so there is no "this tenant's day has not rolled yet" case to
// preserve, unlike a session that has not been touched since before the
// roll.
func (g *SpendGuard) rollDay() {
	today := g.now().UTC().Format("2006-01-02")
	if today != g.day {
		g.day = today
		g.dayUsed = map[tenancy.TenantID]*spend{}
	}
}

// ErrorBudget trips a session before its spend cap when too large a share
// of its prompt tokens carried error content: failed tools, failed edits,
// repeated identical calls, overflow notices. Share is that fraction;
// zero is off. Small sessions are never judged, since one early failure
// would dominate them.
type ErrorBudget struct {
	Share float64
}

// errorBudgetMinPromptTokens is the session size below which the budget
// is not evaluated.
const errorBudgetMinPromptTokens = 10_000

// Enabled reports whether the budget is set.
func (b ErrorBudget) Enabled() bool { return b.Share > 0 }

// Check returns the refusal reason for a session whose error share of
// prompt tokens exceeds the budget, or an empty string.
func (b ErrorBudget) Check(errorTokens, promptTokens int) string {
	if !b.Enabled() || promptTokens < errorBudgetMinPromptTokens {
		return ""
	}
	share := float64(errorTokens) / float64(promptTokens)
	if share < b.Share {
		return ""
	}
	return fmt.Sprintf("error budget exceeded: %.0f%% of this session's prompt tokens (%d of %d) carried failed tools, failed edits, repeated identical calls, or overflow notices; the budget is %.0f%%", share*100, errorTokens, promptTokens, b.Share*100)
}

// LoopLimits set how many identical tool calls in one prompt warn and block.
// Zero disables the respective action.
type LoopLimits struct {
	Warn  int
	Block int
}

// LoopVerdict is what the loop detector concluded about a prompt.
type LoopVerdict struct {
	// Repeats is the highest count of one identical call in the prompt.
	Repeats int
	// Label names the repeated call for the warning text (tool name only).
	Label string
	Warn  bool
	Block bool
}

// DetectLoop measures the run of identical tool calls (same tool, same
// input) at the tail of a summarized conversation: how many times in a row
// the agent has just made the same call. Counting the tail rather than the
// whole history means a legitimate repeated command earlier in a long
// session cannot block it forever. Identity is the block's content-free
// call key, so the body is not parsed a second time.
func DetectLoop(prompt ledger.Prompt, limits LoopLimits) LoopVerdict {
	if limits.Warn <= 0 && limits.Block <= 0 {
		return LoopVerdict{}
	}
	var v LoopVerdict
	lastKey := ""
	for _, m := range prompt.Messages {
		if m.Role != transcript.RoleAssistant {
			continue
		}
		for _, b := range m.Blocks {
			if b.Kind != transcript.KindToolUse || b.CallKey == "" {
				continue
			}
			if b.CallKey == lastKey {
				v.Repeats++
			} else {
				lastKey = b.CallKey
				v.Repeats = 1
				v.Label = b.ToolName
			}
		}
	}
	v.Warn = limits.Warn > 0 && v.Repeats >= limits.Warn
	v.Block = limits.Block > 0 && v.Repeats >= limits.Block
	return v
}

// BreakerSettings open the circuit after a run of provider failures.
type BreakerSettings struct {
	// Failures is how many consecutive retryable failures open the circuit.
	Failures int
	// Cooldown is how long the circuit stays open before one probe passes.
	Cooldown time.Duration
}

// Breaker is a circuit breaker over upstream health. While open, requests
// are refused locally with a retry-after so the agent stops burning its
// own retries against a provider that is already saying no.
type Breaker struct {
	settings BreakerSettings
	mu       sync.Mutex
	failures int
	openedAt time.Time
	probing  bool
	now      func() time.Time
}

// NewBreaker builds a breaker; zero Failures disables it.
func NewBreaker(s BreakerSettings) *Breaker {
	return &Breaker{settings: s, now: time.Now}
}

// Enabled reports whether the breaker can open.
func (b *Breaker) Enabled() bool {
	return b != nil && b.settings.Failures > 0
}

// Allow reports whether a request may go upstream, whether it is the one
// half-open probe, and when refused, how long the caller should wait.
func (b *Breaker) Allow() (ok bool, probe bool, wait time.Duration) {
	if !b.Enabled() {
		return true, false, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openedAt.IsZero() {
		return true, false, 0
	}
	elapsed := b.now().Sub(b.openedAt)
	if elapsed < b.settings.Cooldown {
		return false, false, b.settings.Cooldown - elapsed
	}
	if b.probing {
		return false, false, b.settings.Cooldown
	}
	// Half-open: let exactly one request probe the provider.
	b.probing = true
	return true, true, 0
}

// Release gives back a half-open probe that never reached the provider
// (the request was refused or aborted before an outcome), so the next
// request can probe instead of every request being refused until restart.
func (b *Breaker) Release() {
	if !b.Enabled() {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
}

// Observe records an upstream outcome. Retryable is true for rate limit,
// overload, server error, and connection failure.
func (b *Breaker) Observe(retryable bool) {
	if !b.Enabled() {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !retryable {
		b.failures = 0
		b.openedAt = time.Time{}
		b.probing = false
		return
	}
	b.failures++
	b.probing = false
	if b.failures >= b.settings.Failures {
		b.openedAt = b.now()
	}
}

// IsRetryableStatus classifies provider responses the breaker counts:
// rate limits and every server-side status, which includes the provider's
// overload code.
func IsRetryableStatus(status int) bool {
	return status == 429 || (status >= 500 && status <= 599)
}

// spendStateFile holds the day's running total beside the ledger.
//
// Without it a daily cap resets whenever the proxy restarts, which is the
// protection silently disappearing for the exact threat it exists to stop: an
// agent looping overnight through a crash, a machine sleep, or a routine
// restart. A cap that resets is worse than no cap, because the operator
// believes it.
const spendStateFile = "spend-day.json"

// tenantSpendState is one tenant's persisted day total (SP-6).
type tenantSpendState struct {
	Tokens int     `json:"tokens"`
	USD    float64 `json:"usd"`
}

type spendState struct {
	Day string `json:"day"`
	// Tenants is the current, tenant-scoped shape: SP-6 names spendState as
	// the second accumulator, beside dayUsed, that gains a tenant dimension.
	// Keyed by TenantID's string form.
	Tenants map[string]tenantSpendState `json:"tenants,omitempty"`
	// Tokens and USD are the pre-SP-6 flat shape every released version
	// before this one wrote: one process-wide total, no tenant key at all.
	// SaveState never writes them again (see below); LoadState still reads
	// them, so an existing solo-developer install's spend-day.json from
	// before this change still restores across the upgrade, rather than a
	// day cap silently starting over on every machine that had one
	// configured. That silent reset is exactly what spendStateFile's own
	// doc comment calls worse than no cap, and it is the reason this is a
	// read-only compatibility path rather than a new migration surface:
	// nothing here converts, rewrites, or deletes an old file, it only
	// gives it one tenant to land under when both sides agree there was
	// only ever one.
	Tokens int     `json:"tokens,omitempty"`
	USD    float64 `json:"usd,omitempty"`
}

// persistedTenantKey validates one key of a loaded spendState.Tenants map
// before it is allowed to become a live tenant bucket.
//
// This is Tenant.UnmarshalJSON's own rule (internal/tenancy/serialization.go),
// restated here rather than imported, because spendState's tenant map is a
// map[string]tenantSpendState and not a tenancy.Tenant: ValidateTenantID
// alone is the wrong check since it reserves LocalTenant for fresh input,
// and LocalTenant is exactly the key the local/default workflow's own
// persisted day total legitimately carries forward from one run to the
// next. TenantUnknown and every other reserved or malformed string are
// refused, so a corrupted or hand-edited key can neither forge the local
// default nor land as a new tenant with a name nothing resolved.
func persistedTenantKey(raw string) (tenancy.TenantID, bool) {
	if raw == string(tenancy.LocalTenant) {
		return tenancy.LocalTenant, true
	}
	if tenancy.ValidateTenantID(raw) != nil {
		return "", false
	}
	return tenancy.TenantID(raw), true
}

// LoadState restores today's running total, per tenant. Anything unreadable,
// unparseable, or from another day is discarded entirely: yesterday's spend
// leaking into today would refuse the first session of the morning. Within a
// same-day file, one tenant entry that fails to validate is skipped on its
// own (see persistedTenantKey) rather than discarding every other tenant's
// legitimate entry alongside it.
func (g *SpendGuard) LoadState(dir string) {
	if g == nil || dir == "" {
		return
	}
	body, err := os.ReadFile(filepath.Join(dir, spendStateFile))
	if err != nil {
		return
	}
	var st spendState
	if json.Unmarshal(body, &st) != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if st.Day != g.now().UTC().Format("2006-01-02") {
		return
	}
	g.day = st.Day
	g.dayUsed = map[tenancy.TenantID]*spend{}
	if len(st.Tenants) > 0 {
		// The tenant-scoped shape takes precedence over the legacy fields
		// below whenever both are present, rather than merging them: no
		// version of SaveState writes both into the same file, so a file
		// that somehow carries both is already untrustworthy in a way
		// guessing at a merge would not fix, and the tenant-scoped shape is
		// the one that can actually name who the legacy figure belongs to.
		for raw, ts := range st.Tenants {
			tenant, ok := persistedTenantKey(raw)
			if !ok {
				// Skipped, not fatal to the whole file: a corrupted tenant
				// key must lose only its own day's accounting, the same
				// "discard what cannot be trusted" rule this function
				// already applies to the file as a whole, applied at the
				// per-tenant grain SP-6 adds.
				continue
			}
			g.dayUsed[tenant] = &spend{tokens: ts.Tokens, usd: ts.USD}
		}
		return
	}
	if st.Tokens != 0 || st.USD != 0 {
		// A pre-SP-6 file. Every workflow that could have produced one ran
		// with no tenant identity configured at all, which is exactly
		// tenancy.LocalTenant — ResolveTenant("") returns it by
		// construction — so this is not a guess about which tenant spent
		// it; LocalTenant is the only tenant pre-SP-6 code could have been.
		g.dayUsed[tenancy.LocalTenant] = &spend{tokens: st.Tokens, usd: st.USD}
	}
}

// SaveState persists today's running total, per tenant. A write failure is
// ignored on purpose: bookkeeping must never be the reason a request fails.
func (g *SpendGuard) SaveState(dir string) {
	if g == nil || dir == "" {
		return
	}
	g.mu.Lock()
	// An idle proxy has no day recorded yet. Stamp today rather than skip the
	// write: a restart that loses the marker is how a cap silently starts over.
	if g.day == "" {
		g.day = g.now().UTC().Format("2006-01-02")
	}
	st := spendState{Day: g.day}
	if len(g.dayUsed) > 0 {
		st.Tenants = make(map[string]tenantSpendState, len(g.dayUsed))
		for tenant, du := range g.dayUsed {
			st.Tenants[string(tenant)] = tenantSpendState{Tokens: du.tokens, USD: du.usd}
		}
	}
	g.mu.Unlock()
	body, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(dir, 0o700)
	tmp := filepath.Join(dir, spendStateFile+".tmp")
	if os.WriteFile(tmp, body, 0o600) == nil {
		_ = os.Rename(tmp, filepath.Join(dir, spendStateFile))
	}
}

// Configured reports which caps are set, for a diagnostic that cannot see the
// flags. It reports existence and never a value.
func (g *SpendGuard) Configured() CapStatus {
	if g == nil {
		return CapStatus{}
	}
	return CapStatus{
		SessionTokens: g.limits.SessionTokens > 0,
		DayTokens:     g.limits.DayTokens > 0,
		SessionUSD:    g.limits.SessionUSD > 0,
		DayUSD:        g.limits.DayUSD > 0,
	}
}
