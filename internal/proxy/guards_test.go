package proxy

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/ledger"
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
	if ok, _ := b.Allow(); !ok {
		t.Fatal("closed breaker must allow")
	}
	b.Observe(true)
	if ok, _ := b.Allow(); !ok {
		t.Fatal("one failure must not open")
	}
	b.Observe(true)
	if ok, wait := b.Allow(); ok || wait <= 0 {
		t.Fatalf("two failures must open and refuse with a wait: ok=%v wait=%s", ok, wait)
	}
	now = now.Add(time.Minute)
	if ok, _ := b.Allow(); !ok {
		t.Fatal("after cooldown one probe must pass")
	}
	if ok, _ := b.Allow(); ok {
		t.Fatal("only one probe may pass while half-open")
	}
	// A probe that never reached an outcome is given back.
	b.Release()
	if ok, _ := b.Allow(); !ok {
		t.Fatal("after Release the next request must be allowed to probe")
	}
	b.Observe(false)
	if ok, _ := b.Allow(); !ok {
		t.Fatal("a successful probe must close the breaker")
	}
	if ok, _ := b.Allow(); !ok {
		t.Fatal("a closed breaker allows every request, not one probe")
	}
}

func TestBreakerDisabled(t *testing.T) {
	var b *Breaker
	if ok, _ := b.Allow(); !ok {
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
