package proxy

import (
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// The day cap must survive a crash, not only a clean shutdown.
//
// spend-day.json exists because "a daily cap resets whenever the proxy
// restarts", and serve.go says the crash is "exactly what happens during the
// overnight run the cap exists to bound". SaveState ran only from a deferred
// call in runServe, which a SIGKILL, an OOM kill or a panic on a handler
// goroutine never reaches, so the file kept whatever the previous clean exit
// (or nothing) had written and the restarted proxy handed the agent a fresh
// day budget.
//
// The crash is simulated by never calling SaveState: the deferred call is
// the only thing a kill skips, and nothing else in the process is torn down
// differently from the guard's point of view.

// DC1: a day cap reached before a crash is still reached after the restart.
//
// PASS: a fresh guard loading the same directory refuses the next request.
// FAIL: the restarted guard allows it, because nothing was persisted.
func TestDC1_DayCapSurvivesACrashThatSkipsTheShutdownSave(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC)
	const tenant tenancy.TenantID = "acme-co"

	before := NewSpendGuard(SpendLimits{DayTokens: 1000})
	before.now = fixedClock(day)
	before.LoadState(dir) // what runServe does at start; nothing on disk yet
	before.Record(tenancy.LocalTenant, "sess-a", 1200, 0, false)
	before.Record(tenant, "sess-b", 1500, 0, false)
	if before.Check(tenancy.LocalTenant, "sess-a") == "" || before.Check(tenant, "sess-b") == "" {
		t.Fatal("precondition: both tenants are past the day cap before the crash")
	}
	// Crash: the deferred SaveState never runs.

	after := NewSpendGuard(SpendLimits{DayTokens: 1000})
	after.now = fixedClock(day.Add(time.Minute))
	after.LoadState(dir)
	if msg := after.Check(tenancy.LocalTenant, "sess-new"); msg == "" {
		t.Fatal("the local tenant's day cap reset across a crash: 1200 of 1000 tokens was forgotten")
	}
	if msg := after.Check(tenant, "sess-new"); msg == "" {
		t.Fatal("tenant acme-co's day cap reset across a crash: 1500 of 1000 tokens was forgotten")
	}
}

// DC2: concurrent Records persist the final total, not an older snapshot.
//
// Persisting from every Record means several goroutines save at once. If a
// snapshot taken earlier could be renamed into place after a later one, the
// file would hold less than was spent and a crash would restore a cap with
// headroom that was already used.
//
// PASS: after N concurrent Records and no shutdown save, a fresh guard
// restores exactly the sum.
// FAIL: the restored total is lower than what was recorded.
func TestDC2_ConcurrentRecordsPersistTheLatestTotal(t *testing.T) {
	const (
		writers = 16
		each    = 25
		tokens  = 3
	)
	dir := t.TempDir()
	day := time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC)
	want := writers * each * tokens

	g := NewSpendGuard(SpendLimits{DayTokens: want})
	g.now = fixedClock(day)
	g.LoadState(dir)
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				g.Record(tenancy.LocalTenant, "sess", tokens, 0, false)
			}
		}()
	}
	wg.Wait()
	// Crash: no SaveState.

	after := NewSpendGuard(SpendLimits{DayTokens: want})
	after.now = fixedClock(day)
	after.LoadState(dir)
	after.mu.Lock()
	var got int
	if du, ok := after.dayUsed[tenancy.LocalTenant]; ok {
		got = du.tokens
	}
	after.mu.Unlock()
	if got != want {
		t.Fatalf("restored day total = %d tokens, want %d: an older snapshot overwrote a newer one, or nothing was saved", got, want)
	}
}
