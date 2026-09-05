package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
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
type spend struct {
	tokens int
	usd    float64
	seen   time.Time
}

// maxSpendSessions bounds the guard's per-session table; the least
// recently seen sessions are dropped past it.
const maxSpendSessions = 1024

// SpendGuard accounts tokens and dollars from provider usage and fails
// closed before the next request once a cap is reached. It never
// interrupts a response in flight.
type SpendGuard struct {
	limits  SpendLimits
	mu      sync.Mutex
	session map[string]*spend
	day     string
	dayUsed spend
	now     func() time.Time
	// unpriceable records that a dollar cap was configured and at least one
	// request could not be priced. Such a request adds nothing to the running
	// total, so the cap can never be reached and the user silently has no cap
	// at all. Refusing traffic over a missing price is not this proxy's
	// behaviour, so the guard has to be able to say so instead.
	unpriceable bool
}

// NewSpendGuard builds a guard; a zero limits value disables it.
func NewSpendGuard(limits SpendLimits) *SpendGuard {
	return &SpendGuard{limits: limits, session: map[string]*spend{}, now: time.Now}
}

// CapNotEnforced reports that a dollar cap is configured but at least one
// request could not be priced, so the cap is not being applied to that traffic.
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

// Record adds a completed request's tokens and list-price cost.
func (g *SpendGuard) Record(sessionID string, tokens int, usd float64) {
	if !g.Enabled() || (tokens <= 0 && usd <= 0) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if usd <= 0 && tokens > 0 && (g.limits.SessionUSD > 0 || g.limits.DayUSD > 0) {
		g.unpriceable = true
	}
	g.rollDay()
	st, ok := g.session[sessionID]
	if !ok {
		for len(g.session) >= maxSpendSessions {
			oldest, oldestSeen := "", time.Time{}
			for k, v := range g.session {
				if oldest == "" || v.seen.Before(oldestSeen) {
					oldest, oldestSeen = k, v.seen
				}
			}
			delete(g.session, oldest)
		}
		st = &spend{}
		g.session[sessionID] = st
	}
	st.seen = g.now()
	st.tokens += tokens
	st.usd += usd
	g.dayUsed.tokens += tokens
	g.dayUsed.usd += usd
}

// Check returns a human-readable reason when the next request for the
// session must be refused, or an empty string when it may proceed.
func (g *SpendGuard) Check(sessionID string) string {
	if !g.Enabled() {
		return ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rollDay()
	var used spend
	if st, ok := g.session[sessionID]; ok {
		used = *st
	}
	switch {
	case g.limits.SessionTokens > 0 && used.tokens >= g.limits.SessionTokens:
		return fmt.Sprintf("session spend cap reached: %d of %d tokens", used.tokens, g.limits.SessionTokens)
	case g.limits.SessionUSD > 0 && used.usd >= g.limits.SessionUSD:
		return fmt.Sprintf("session spend cap reached: $%.2f of $%.2f at list price", used.usd, g.limits.SessionUSD)
	case g.limits.DayTokens > 0 && g.dayUsed.tokens >= g.limits.DayTokens:
		return fmt.Sprintf("daily spend cap reached: %d of %d tokens", g.dayUsed.tokens, g.limits.DayTokens)
	case g.limits.DayUSD > 0 && g.dayUsed.usd >= g.limits.DayUSD:
		return fmt.Sprintf("daily spend cap reached: $%.2f of $%.2f at list price", g.dayUsed.usd, g.limits.DayUSD)
	}
	return ""
}

// rollDay resets the daily counters at UTC midnight. Callers hold the lock.
func (g *SpendGuard) rollDay() {
	today := g.now().UTC().Format("2006-01-02")
	if today != g.day {
		g.day = today
		g.dayUsed = spend{}
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

type spendState struct {
	Day    string  `json:"day"`
	Tokens int     `json:"tokens"`
	USD    float64 `json:"usd"`
}

// LoadState restores today's running total. Anything unreadable, unparseable,
// or from another day is discarded: yesterday's spend leaking into today would
// refuse the first session of the morning.
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
	g.dayUsed = spend{tokens: st.Tokens, usd: st.USD}
}

// SaveState persists today's running total. A write failure is ignored on
// purpose: bookkeeping must never be the reason a request fails.
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
	st := spendState{Day: g.day, Tokens: g.dayUsed.tokens, USD: g.dayUsed.usd}
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
