package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// DC3: a save that snapshotted first cannot rename its file in last.
//
// DC2 cannot kill saveMu's removal: under free scheduling the race is too
// narrow. This test forces the one interleaving saveMu exists to prevent:
//
//	A: snapshot (total 3) .......................... write tmp(3), rename
//	B:            record (total 6), snapshot, write tmp(6), rename
//
// Without saveMu, B completes while A is parked between snapshot and write,
// then A renames its older snapshot over B's newer one and the file says 3
// when 6 was spent: a crash now restores a cap with headroom already used.
// With saveMu, B cannot snapshot until A has renamed, so B's file is last.
//
// The parking is done through saveStateAfterSnapshot, a no-op in production.
// Whether B is held off by saveMu is observed, not guessed with a sleep:
// the test waits until B either finishes (no lock: RED) or is seen in the
// goroutine dump blocked on a mutex inside SaveState (lock: GREEN).
//
// PASS: a fresh guard restores 6 tokens.
// FAIL: it restores 3, the older snapshot.
func TestDC3_SaveMuStopsAnOlderSnapshotRenamingOverANewerOne(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC)

	g := NewSpendGuard(SpendLimits{DayTokens: 1000})
	g.now = fixedClock(day)
	g.LoadState(dir)

	parked := make(chan struct{})
	release := make(chan struct{})
	first := true
	prev := saveStateAfterSnapshot
	saveStateAfterSnapshot = func(sg *SpendGuard) {
		// Only the test's guard, and only the first save, is parked. first is
		// read only by A (which runs alone until parked) and then by B, after
		// A has written false and signalled parked: ordered by the channel.
		if sg != g || !first {
			return
		}
		first = false
		close(parked)
		<-release
	}
	t.Cleanup(func() { saveStateAfterSnapshot = prev })

	aDone := make(chan struct{})
	go func() {
		defer close(aDone)
		g.Record(tenancy.LocalTenant, "sess", 3, 0, false)
	}()
	<-parked

	bDone := make(chan struct{})
	go func() {
		defer close(bDone)
		g.Record(tenancy.LocalTenant, "sess", 3, 0, false)
	}()

	deadline := time.After(10 * time.Second)
wait:
	for {
		select {
		case <-bDone:
			break wait // no serialisation: B's save finished under A's snapshot
		case <-deadline:
			close(release)
			t.Fatal("B neither finished nor blocked on saveMu within 10s")
		default:
		}
		if blockedOnMutexInSaveState() {
			break wait // serialised: B waits for A to rename
		}
		runtime.Gosched()
	}
	close(release)
	<-aDone
	<-bDone
	// Crash: no shutdown save.

	// The bytes on disk must be valid JSON before LoadState reads them.
	// LoadState discards a torn file as a fresh zero day, so without this
	// check a torn write (0) and an older snapshot winning (3) would be
	// indistinguishable failures of the total below.
	raw, err := os.ReadFile(filepath.Join(dir, spendStateFile))
	if err != nil {
		t.Fatalf("read %s after both saves: %v", spendStateFile, err)
	}
	if !json.Valid(raw) {
		t.Fatalf("%s after both saves is not valid JSON (%d bytes): a torn write, not a stale rename", spendStateFile, len(raw))
	}

	after := NewSpendGuard(SpendLimits{DayTokens: 1000})
	after.now = fixedClock(day)
	after.LoadState(dir)
	after.mu.Lock()
	var got int
	if du, ok := after.dayUsed[tenancy.LocalTenant]; ok {
		got = du.tokens
	}
	after.mu.Unlock()
	if got != 6 {
		t.Fatalf("restored day total = %d tokens, want 6: an older snapshot was renamed over a newer one", got)
	}
}

// blockedOnMutexInSaveState reports whether some goroutine is waiting on a
// sync.Mutex from inside SpendGuard.SaveState. A, the parked save, waits on a
// channel, so the only goroutine this can match is B on saveMu.
func blockedOnMutexInSaveState() bool {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	for _, gr := range strings.Split(string(buf), "\n\n") {
		header, _, _ := strings.Cut(gr, "\n")
		if strings.Contains(header, "[sync.Mutex.Lock") && strings.Contains(gr, "(*SpendGuard).SaveState") {
			return true
		}
	}
	return false
}
