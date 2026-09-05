package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

func TestSpendGuardCapsSessionAndDay(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 100, DayTokens: 150})
	if g.Check("a") != "" {
		t.Fatal("fresh session must be allowed")
	}
	g.Record("a", 60, 0)
	if g.Check("a") != "" {
		t.Fatal("under the cap must be allowed")
	}
	g.Record("a", 40, 0)
	if reason := g.Check("a"); reason == "" {
		t.Fatal("session cap must refuse the next request")
	}
	if g.Check("b") != "" {
		t.Fatal("another session is under its own cap")
	}
	g.Record("b", 60, 0)
	if reason := g.Check("b"); reason == "" {
		t.Fatal("daily cap must refuse across sessions")
	}
}

func TestSpendGuardRollsOverAtMidnightUTC(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	day := time.Date(2026, 9, 2, 23, 59, 0, 0, time.UTC)
	g.now = func() time.Time { return day }
	g.Record("a", 10, 0)
	if g.Check("a") == "" {
		t.Fatal("cap must apply today")
	}
	g.now = func() time.Time { return day.Add(2 * time.Minute) }
	if g.Check("a") != "" {
		t.Fatal("daily counter must reset after midnight")
	}
}

func TestSpendGuardDisabledIsNoop(t *testing.T) {
	var g *SpendGuard
	if g.Enabled() || g.Check("x") != "" {
		t.Fatal("nil guard must allow everything")
	}
	g.Record("x", 1, 0)
}

func TestDetectLoop(t *testing.T) {
	body := `{"messages":[
	 {"role":"user","content":"go"},
	 {"role":"assistant","content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"go test ./..."}}]},
	 {"role":"user","content":[{"type":"tool_result","tool_use_id":"1","content":"FAIL"}]},
	 {"role":"assistant","content":[{"type":"tool_use","id":"2","name":"Bash","input":{"command":"go test ./..."}}]},
	 {"role":"user","content":[{"type":"tool_result","tool_use_id":"2","content":"FAIL"}]},
	 {"role":"assistant","content":[{"type":"tool_use","id":"3","name":"Bash","input":{"command":"go test ./..."}}]},
	 {"role":"user","content":[{"type":"tool_result","tool_use_id":"3","content":"FAIL"}]}
	]}`
	v := DetectLoop(summarize(t, body), LoopLimits{Warn: 3, Block: 5})
	if v.Repeats != 3 || v.Label != "Bash" || !v.Warn || v.Block {
		t.Fatalf("verdict = %+v", v)
	}
	// A different call at the tail ends the run: history alone never blocks.
	broken := strings.Replace(body, `{"type":"tool_result","tool_use_id":"3","content":"FAIL"}]}`, `{"type":"tool_result","tool_use_id":"3","content":"FAIL"}]},
	 {"role":"assistant","content":[{"type":"tool_use","id":"4","name":"Read","input":{"file_path":"a.go"}}]}`, 1)
	if v := DetectLoop(summarize(t, broken), LoopLimits{Warn: 3, Block: 3}); v.Repeats != 1 || v.Warn || v.Block {
		t.Fatalf("a differing tail call must reset the run: %+v", v)
	}
	v = DetectLoop(summarize(t, body), LoopLimits{Warn: 2, Block: 3})
	if !v.Block {
		t.Fatalf("block threshold not applied: %+v", v)
	}
	if v := DetectLoop(summarize(t, body), LoopLimits{}); v.Warn || v.Block || v.Repeats != 0 {
		t.Fatalf("disabled detector must be silent: %+v", v)
	}
	if v := DetectLoop(ledger.Prompt{}, LoopLimits{Warn: 1}); v.Warn {
		t.Fatal("an empty prompt must not warn")
	}
}

// summarize reduces a request body the way the proxy does before the
// guards see it.
func summarize(t *testing.T, body string) ledger.Prompt {
	t.Helper()
	sum, err := ledger.SummarizeRequest([]byte(body), ledger.NewLabeler([]byte("k")))
	if err != nil {
		t.Fatal(err)
	}
	return sum.Prompt
}

func TestBreakerOpensAndProbes(t *testing.T) {
	b := NewBreaker(BreakerSettings{Failures: 2, Cooldown: time.Minute})
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	if ok, _, _ := b.Allow(); !ok {
		t.Fatal("closed breaker must allow")
	}
	b.Observe(true)
	if ok, _, _ := b.Allow(); !ok {
		t.Fatal("one failure must not open")
	}
	b.Observe(true)
	if ok, _, wait := b.Allow(); ok || wait <= 0 {
		t.Fatalf("two failures must open and refuse with a wait: ok=%v wait=%s", ok, wait)
	}
	now = now.Add(time.Minute)
	if ok, probe, _ := b.Allow(); !ok || !probe {
		t.Fatal("after cooldown one probe must pass and be marked as the probe")
	}
	if ok, _, _ := b.Allow(); ok {
		t.Fatal("only one probe may pass while half-open")
	}
	// A probe that never reached an outcome is given back.
	b.Release()
	if ok, _, _ := b.Allow(); !ok {
		t.Fatal("after Release the next request must be allowed to probe")
	}
	b.Observe(false)
	if ok, _, _ := b.Allow(); !ok {
		t.Fatal("a successful probe must close the breaker")
	}
	if ok, _, _ := b.Allow(); !ok {
		t.Fatal("a closed breaker allows every request, not one probe")
	}
}

func TestBreakerDisabled(t *testing.T) {
	var b *Breaker
	if ok, _, _ := b.Allow(); !ok {
		t.Fatal("nil breaker must allow everything")
	}
	b.Observe(true)
}

func TestIsRetryableStatus(t *testing.T) {
	for status, want := range map[int]bool{200: false, 400: false, 401: false, 429: true, 500: true, 503: true, 529: true} {
		if got := IsRetryableStatus(status); got != want {
			t.Errorf("%d: %v", status, got)
		}
	}
}

func TestSpendGuardDollarCaps(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionUSD: 1, DayUSD: 1.5})
	g.Record("a", 100, 0.6)
	if g.Check("a") != "" {
		t.Fatal("under the dollar cap must be allowed")
	}
	g.Record("a", 100, 0.4)
	if reason := g.Check("a"); !strings.Contains(reason, "$1.00 of $1.00") {
		t.Fatalf("session dollar cap must refuse: %q", reason)
	}
	g.Record("b", 100, 0.5)
	if reason := g.Check("b"); !strings.Contains(reason, "daily spend cap reached: $1.50") {
		t.Fatalf("daily dollar cap must refuse across sessions: %q", reason)
	}
}

func TestErrorBudgetJudgesOnlyLargeSessions(t *testing.T) {
	b := ErrorBudget{Share: 0.3}
	if b.Check(9000, 9000) != "" {
		t.Fatal("a session under the minimum size must not be judged")
	}
	if b.Check(2999, errorBudgetMinPromptTokens) != "" {
		t.Fatal("under the share must pass")
	}
	if reason := b.Check(3000, errorBudgetMinPromptTokens); !strings.Contains(reason, "30% of this session") {
		t.Fatalf("at the share must refuse: %q", reason)
	}
	if (ErrorBudget{}).Check(1e9, 1e9) != "" {
		t.Fatal("off must pass everything")
	}
}

// A dollar cap on a model the price table does not know never fires: listCost
// returns 0, the running total never grows, and `used >= limit` is never true.
// The user asked for a cap and silently got none. Failing closed would block
// traffic over a missing price, which this proxy does not do, so the guard has
// to be able to say that the cap it was given is not being enforced.
func TestDollarCapOnAnUnpricedModelIsReportedNotSilentlyIgnored(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionUSD: 20})
	if g.CapNotEnforced() {
		t.Fatal("nothing has happened yet")
	}
	// Tokens were spent, but the model could not be priced.
	g.Record("s1", 500_000, 0)
	if !g.CapNotEnforced() {
		t.Fatal("a dollar cap that cannot be enforced must be reportable, not silent")
	}
	if msg := g.Check("s1"); msg != "" {
		t.Fatalf("the cap must not fire on an unpriceable session: %q", msg)
	}

	// A priced session behaves exactly as before.
	h := NewSpendGuard(SpendLimits{SessionUSD: 20})
	h.Record("s2", 500_000, 25)
	if h.CapNotEnforced() {
		t.Fatal("a priced session enforces its cap normally")
	}
	if msg := h.Check("s2"); msg == "" {
		t.Fatal("the cap should have fired at $25 of $20")
	}
}

// A daily cap that resets when the proxy restarts is worse than no cap,
// because the user believes it. The overnight runaway is the exact threat the
// day cap exists to stop, and a crash, a machine sleep, or a routine restart
// silently removes the protection they configured.
func TestDayCapSurvivesARestart(t *testing.T) {
	dir := t.TempDir()

	first := NewSpendGuard(SpendLimits{DayUSD: 10})
	first.LoadState(dir)
	first.Record("s1", 500_000, 7.50)
	if msg := first.Check("s1"); msg != "" {
		t.Fatalf("fixture: $7.50 of $10 should not refuse yet: %q", msg)
	}
	first.SaveState(dir)

	// The process dies and comes back. The day's spend must come back with it.
	second := NewSpendGuard(SpendLimits{DayUSD: 10})
	second.LoadState(dir)
	second.Record("s2", 200_000, 3.00) // takes the day to $10.50
	if msg := second.Check("s2"); msg == "" {
		t.Fatal("the day cap did not survive the restart: $10.50 of $10 was allowed")
	}
}

// State from a previous day must not carry over, or the first session of a new
// day inherits yesterday's spend and is refused immediately.
func TestPersistedStateFromAnotherDayIsDiscarded(t *testing.T) {
	dir := t.TempDir()
	g := NewSpendGuard(SpendLimits{DayUSD: 10})
	g.now = func() time.Time { return time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC) }
	g.LoadState(dir)
	g.Record("s1", 0, 50) // way over
	g.SaveState(dir)

	next := NewSpendGuard(SpendLimits{DayUSD: 10})
	next.now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	next.LoadState(dir)
	if msg := next.Check("s1"); msg != "" {
		t.Fatalf("yesterday's spend leaked into today: %q", msg)
	}
}

// Persistence must never be the reason a request fails.
func TestSpendStateFailsOpenOnAnUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, spendStateFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	g := NewSpendGuard(SpendLimits{DayUSD: 10})
	g.LoadState(dir) // must not panic, must not error out
	if msg := g.Check("s1"); msg != "" {
		t.Fatalf("a corrupt state file must not refuse traffic: %q", msg)
	}
}
