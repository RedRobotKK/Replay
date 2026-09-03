package proxy

import (
	"fmt"
	"sync"
	"time"

	"github.com/RedRobotKK/Buffy/internal/ledger"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// Guards are off unless configured. Each is a pure decision component the
// handler consults before forwarding (spend, loop) or after a response
// (breaker); none of them touches request or response bytes.

// SpendLimits caps prompt-plus-output tokens per session and per UTC day.
// Zero means no cap.
type SpendLimits struct {
	SessionTokens int
	DayTokens     int
}

// SpendGuard accounts tokens from provider usage and fails closed before
// the next request once a cap is reached. It never interrupts a response
// in flight.
type SpendGuard struct {
	limits  SpendLimits
	mu      sync.Mutex
	session map[string]int
	day     string
	dayUsed int
	now     func() time.Time
}

// NewSpendGuard builds a guard; a zero limits value disables it.
func NewSpendGuard(limits SpendLimits) *SpendGuard {
	return &SpendGuard{limits: limits, session: map[string]int{}, now: time.Now}
}

// Enabled reports whether any cap is set.
func (g *SpendGuard) Enabled() bool {
	return g != nil && (g.limits.SessionTokens > 0 || g.limits.DayTokens > 0)
}

// Record adds a completed request's tokens.
func (g *SpendGuard) Record(sessionID string, tokens int) {
	if !g.Enabled() || tokens <= 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rollDay()
	g.session[sessionID] += tokens
	g.dayUsed += tokens
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
	if g.limits.SessionTokens > 0 && g.session[sessionID] >= g.limits.SessionTokens {
		return fmt.Sprintf("session spend cap reached: %d of %d tokens", g.session[sessionID], g.limits.SessionTokens)
	}
	if g.limits.DayTokens > 0 && g.dayUsed >= g.limits.DayTokens {
		return fmt.Sprintf("daily spend cap reached: %d of %d tokens", g.dayUsed, g.limits.DayTokens)
	}
	return ""
}

// rollDay resets the daily counter at UTC midnight. Callers hold the lock.
func (g *SpendGuard) rollDay() {
	today := g.now().UTC().Format("2006-01-02")
	if today != g.day {
		g.day = today
		g.dayUsed = 0
	}
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

// Allow reports whether a request may go upstream, and when not, how long
// the caller should wait.
func (b *Breaker) Allow() (bool, time.Duration) {
	if !b.Enabled() {
		return true, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openedAt.IsZero() {
		return true, 0
	}
	elapsed := b.now().Sub(b.openedAt)
	if elapsed < b.settings.Cooldown {
		return false, b.settings.Cooldown - elapsed
	}
	if b.probing {
		return false, b.settings.Cooldown
	}
	// Half-open: let exactly one request probe the provider.
	b.probing = true
	return true, 0
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
