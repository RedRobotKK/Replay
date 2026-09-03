package proxy

import (
	"testing"
	"time"
)

func TestSpendGuardCapsSessionAndDay(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 100, DayTokens: 150})
	if g.Check("a") != "" {
		t.Fatal("fresh session must be allowed")
	}
	g.Record("a", 60)
	if g.Check("a") != "" {
		t.Fatal("under the cap must be allowed")
	}
	g.Record("a", 40)
	if reason := g.Check("a"); reason == "" {
		t.Fatal("session cap must refuse the next request")
	}
	if g.Check("b") != "" {
		t.Fatal("another session is under its own cap")
	}
	g.Record("b", 60)
	if reason := g.Check("b"); reason == "" {
		t.Fatal("daily cap must refuse across sessions")
	}
}

func TestSpendGuardRollsOverAtMidnightUTC(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	day := time.Date(2026, 9, 2, 23, 59, 0, 0, time.UTC)
	g.now = func() time.Time { return day }
	g.Record("a", 10)
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
	g.Record("x", 1)
}

func TestDetectLoop(t *testing.T) {
	body := `{"messages":[
	 {"role":"user","content":"go"},
	 {"role":"assistant","content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"go test ./..."}}]},
	 {"role":"user","content":[{"type":"tool_result","tool_use_id":"1","content":"FAIL"}]},
	 {"role":"assistant","content":[{"type":"tool_use","id":"2","name":"Bash","input":{"command":"go test ./..."}}]},
	 {"role":"user","content":[{"type":"tool_result","tool_use_id":"2","content":"FAIL"}]},
	 {"role":"assistant","content":[{"type":"tool_use","id":"3","name":"Bash","input":{"command":"go test ./..."}}]},
	 {"role":"user","content":[{"type":"tool_result","tool_use_id":"3","content":"FAIL"}]},
	 {"role":"assistant","content":[{"type":"tool_use","id":"4","name":"Read","input":{"file_path":"a.go"}}]}
	]}`
	v := DetectLoop([]byte(body), LoopLimits{Warn: 3, Block: 5})
	if v.Repeats != 3 || v.Label != "Bash" || !v.Warn || v.Block {
		t.Fatalf("verdict = %+v", v)
	}
	v = DetectLoop([]byte(body), LoopLimits{Warn: 2, Block: 3})
	if !v.Block {
		t.Fatalf("block threshold not applied: %+v", v)
	}
	if v := DetectLoop([]byte(body), LoopLimits{}); v.Warn || v.Block || v.Repeats != 0 {
		t.Fatalf("disabled detector must be silent: %+v", v)
	}
	if v := DetectLoop([]byte("not json"), LoopLimits{Warn: 1}); v.Warn {
		t.Fatal("unparseable body must not warn")
	}
}

func TestBreakerOpensAndProbes(t *testing.T) {
	b := NewBreaker(BreakerSettings{Failures: 2, Cooldown: time.Minute})
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	if ok, _ := b.Allow(); !ok {
		t.Fatal("closed breaker must allow")
	}
	b.Observe(true)
	if b.Open() {
		t.Fatal("one failure must not open")
	}
	b.Observe(true)
	if !b.Open() {
		t.Fatal("two failures must open")
	}
	if ok, wait := b.Allow(); ok || wait <= 0 {
		t.Fatalf("open breaker must refuse with a wait: ok=%v wait=%s", ok, wait)
	}
	now = now.Add(time.Minute)
	if ok, _ := b.Allow(); !ok {
		t.Fatal("after cooldown one probe must pass")
	}
	if ok, _ := b.Allow(); ok {
		t.Fatal("only one probe may pass while half-open")
	}
	b.Observe(false)
	if b.Open() {
		t.Fatal("a successful probe must close the breaker")
	}
	if ok, _ := b.Allow(); !ok {
		t.Fatal("closed again must allow")
	}
}

func TestBreakerDisabled(t *testing.T) {
	var b *Breaker
	if ok, _ := b.Allow(); !ok || b.Open() {
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
