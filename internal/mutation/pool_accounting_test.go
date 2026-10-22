//go:build mutation

package mutation

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The pool's own guarantees, checked without compiling anything.
//
// Running the catalogue concurrently is only safe if three things hold: no two
// mutants share a directory, each mutant starts from the snapshot and not from
// whatever the last holder left behind, and the score counts every mutant
// exactly once whatever order they finish in. Each is cheap to break and
// invisible in a green run, so each has a test here.

func TestWorkerCountFollowsTheCPUsAndRefusesABadOverride(t *testing.T) {
	for _, c := range []struct{ cpus, want int }{{1, 1}, {2, 1}, {3, 1}, {4, 2}, {8, 2}, {64, 2}} {
		if got, err := workerCount("", c.cpus); err != nil || got != c.want {
			t.Errorf("%d CPUs: got %d, %v; want %d", c.cpus, got, err, c.want)
		}
	}
	if got, err := workerCount("3", 4); err != nil || got != 3 {
		t.Errorf("override 3: got %d, %v", got, err)
	}
	for _, bad := range []string{"0", "-1", "two", "2.5", " 2"} {
		if _, err := workerCount(bad, 4); err == nil {
			t.Errorf("%s=%q was accepted; a bad override must fail the run, not fall back", workersEnv, bad)
		}
	}
}

func snapshotFixture() []file {
	return []file{
		{name: "go.mod", body: []byte("module x\n"), mode: 0o644},
		{name: "a/a.go", body: []byte("package a\n"), mode: 0o644},
	}
}

// A mutant's edit, and any file it created, are gone before the next holder
// gets the directory.
func TestRefillLeavesNothingOfThePreviousMutant(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "w0")
	files := snapshotFixture()
	if err := refill(dir, files); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "a.go"), []byte("package a // mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "left.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refill(dir, files); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a", "a.go")); string(b) != "package a\n" {
		t.Errorf("a.go after refill = %q, want the snapshot", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "left.go")); !os.IsNotExist(err) {
		t.Errorf("a file the previous mutant created survived the refill: %v", err)
	}
}

// At most n holders at once, never two in one directory, and every holder
// starts from the snapshot even though the one before it scribbled on it.
func TestPoolNeverSharesADirectory(t *testing.T) {
	const n, holders = 3, 24
	p := newPool(t, snapshotFixture(), n)
	var inFlight, peak atomic.Int32
	var mu sync.Mutex
	held := map[string]bool{}
	var wg sync.WaitGroup
	errs := make(chan string, holders)
	for i := 0; i < holders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dir, release, err := p.acquire()
			if err != nil {
				errs <- err.Error()
				return
			}
			defer release()
			now := inFlight.Add(1)
			defer inFlight.Add(-1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			mu.Lock()
			if held[dir] {
				errs <- dir + " handed to two holders at once"
			}
			held[dir] = true
			mu.Unlock()
			if b, _ := os.ReadFile(filepath.Join(dir, "a", "a.go")); string(b) != "package a\n" {
				errs <- "a holder started from " + string(b) + ", not the snapshot"
			}
			_ = os.WriteFile(filepath.Join(dir, "a", "a.go"), []byte("package a // mine\n"), 0o644)
			time.Sleep(2 * time.Millisecond)
			mu.Lock()
			delete(held, dir)
			mu.Unlock()
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if got := peak.Load(); got > n {
		t.Errorf("%d holders at once from a pool of %d", got, n)
	}
}

// A mutant that never reached an outcome, one counted twice, and an outcome
// for a mutant that never started are each named; a complete run is not.
func TestAccountingNamesEveryMissingOrDoubledMutant(t *testing.T) {
	r := newResults()
	for _, id := range []string{"M1", "M2", "M3"} {
		r.start(id)
	}
	r.record("M1", "one", killed, map[string]bool{"TestA": true}, "")
	r.record("M2", "two", survived, nil, "")
	r.record("M2", "two", survived, nil, "")
	r.record("M9", "nine", killed, nil, "")
	err := r.accounted()
	if err == nil {
		t.Fatal("an incomplete run was accounted complete")
	}
	for _, want := range []string{"M3", "M2", "M9 (never started)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("accounting did not name %s: %v", want, err)
		}
	}

	ok := newResults()
	for _, id := range []string{"M1", "M2"} {
		ok.start(id)
	}
	ok.record("M2", "two", stillborn, nil, "refused")
	ok.record("M1", "one", killed, nil, "")
	if err := ok.accounted(); err != nil {
		t.Errorf("a complete run was refused: %v", err)
	}
}

// The score does not depend on completion order: the same outcomes recorded in
// two orders give the same verdict, and the stillborn list comes back in
// catalogue order.
func TestTheVerdictIsIndependentOfCompletionOrder(t *testing.T) {
	type rec struct {
		id string
		o  outcome
	}
	recs := []rec{{"M1", killed}, {"M2", stillborn}, {"M3", survived}, {"M4", stillborn}, {"M5", killed}}
	order := []string{"M1", "M2", "M3", "M4", "M5"}
	run := func(perm []int) *results {
		r := newResults()
		for _, id := range order {
			r.start(id)
		}
		for _, i := range perm {
			r.record(recs[i].id, recs[i].id, recs[i].o, nil, "")
		}
		r.stillbornSorted(order)
		return r
	}
	a, b := run([]int{0, 1, 2, 3, 4}), run([]int{4, 3, 2, 1, 0})
	if a.v.String() != b.v.String() {
		t.Errorf("verdict depends on order: %s vs %s", a.v, b.v)
	}
	ids := func(r *results) string {
		var s []string
		for _, x := range r.v.stillborn {
			s = append(s, x.id)
		}
		return strings.Join(s, ",")
	}
	if ids(a) != "M2,M4" || ids(b) != "M2,M4" {
		t.Errorf("stillborn order = %s and %s, want M2,M4 both times", ids(a), ids(b))
	}
}
