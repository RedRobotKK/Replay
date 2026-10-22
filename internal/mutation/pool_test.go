//go:build mutation

package mutation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Why the catalogue runs in a pool of fixed directories.
//
// Measured 2026-10-21 on main a0d652b: the catalogue step took 49 to 80
// minutes on ubuntu-latest, and most of that was not the mutants. Every
// mutant was thawed into a new t.TempDir, and the Go build cache keys a
// package on its directory unless -trimpath is set, so the same tree at a
// new path recompiled all 41 packages (`go build -x`: 41 compile actions,
// against 0 at the same path). The test compile of the whole tree took
// 12.7 to 13.8s at a new path and 7.5s at the old one, for every mutant,
// twice per mutant (once here, once in the kill matrix).
//
// So each worker owns one directory for the whole run. Before every mutant
// it is emptied and refilled from the same snapshot, which is exactly the
// fresh copy thaw used to make, only at a path the cache has seen. A mutated
// file still hashes differently, so only the packages that can see the
// mutation recompile; the cache is content-addressed and a stale entry cannot
// be served for a changed file.
//
// And the workers run concurrently, because a mutant's run is a compile, a
// link of each test binary and a test run, and much of that is serial. How
// many is the next function's business.

// workersEnv overrides the worker count. Unset, the count follows the CPUs.
const workersEnv = "REPLAY_MUTATION_WORKERS"

// workerCount is how many mutants are in flight at once.
//
// Half the CPUs, between 1 and 2. Each mutant's `go build` and `go test`
// already parallelise across packages up to GOMAXPROCS, so a worker per CPU
// oversubscribes the machine. Measured 2026-10-21 on 13 mutants of the
// catalogue, both tests, on a 10-core machine: 395.7s for the old harness,
// 287.4s with 1 worker (the fixed directories alone), 220.2s with 2, and
// 260.1s with 4, so past 2 it got slower and the cap is 2 until a measurement
// says otherwise. ubuntu-latest has 4 CPUs (public repository), which gives 2. An unparseable or non-positive override is an error, not a
// fallback: a typo that silently ran the catalogue serially, or not at all,
// would be indistinguishable from the setting working.
func workerCount(env string, cpus int) (int, error) {
	if env != "" {
		n, err := strconv.Atoi(env)
		if err != nil || n < 1 {
			return 0, fmt.Errorf("%s=%q: want a whole number of at least 1", workersEnv, env)
		}
		return n, nil
	}
	n := cpus / 2
	if n < 1 {
		n = 1
	}
	if n > 2 {
		n = 2
	}
	return n, nil
}

// file is one entry of the tree snapshot.
type file struct {
	name string
	body []byte
	mode os.FileMode
}

// snapshot reads the tree once, so every mutant in a run is applied to the
// same tree. thaw re-read the working copy for each mutant, so an edit made
// during an hour-long run landed in some mutants and not others.
func snapshot(t *testing.T, root string) []file {
	t.Helper()
	// Cached AND untracked-but-not-ignored: a mutant must be run against the
	// tree as it is now, including work that has not been committed yet. That
	// is exactly when a guard is most likely to be missing.
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// Not a skip. ADR-0014 lists "a skip reported as a pass" among the
		// twenty defects it exists to stop, and a harness that goes green
		// having asserted nothing — in a source tarball, say — is that
		// defect wearing this tool's name.
		t.Fatalf("cannot enumerate the tree, so no mutant can be applied and nothing here "+
			"asserts anything: %v", err)
	}
	var files []file
	for _, name := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if name == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue // a tracked file that is not on disk right now
		}
		files = append(files, file{name: name, body: body, mode: 0o644})
	}
	if len(files) == 0 {
		t.Fatal("the tree snapshot is empty, so every mutant would run against nothing")
	}
	return files
}

// refill empties dir and writes the snapshot into it. The directory itself is
// kept, because its path is what the build cache recognises.
func refill(dir string, files []file) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("emptying %s: %w", dir, err)
	}
	for _, f := range files {
		target := filepath.Join(dir, f.name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, f.body, f.mode); err != nil {
			return err
		}
	}
	return nil
}

// pool hands out worker directories, one holder at a time.
type pool struct {
	dirs  chan string
	files []file
}

func newPool(t *testing.T, files []file, n int) *pool {
	t.Helper()
	base := t.TempDir()
	p := &pool{dirs: make(chan string, n), files: files}
	for i := 0; i < n; i++ {
		p.dirs <- filepath.Join(base, fmt.Sprintf("w%d", i))
	}
	return p
}

// acquire blocks until a worker directory is free, refills it from the
// snapshot, and returns it with the function that gives it back. No two
// holders ever share a directory, so one mutant cannot see another's edit.
// On error the directory has already been given back.
func (p *pool) acquire() (string, func(), error) {
	dir := <-p.dirs
	release := func() { p.dirs <- dir }
	if err := refill(dir, p.files); err != nil {
		release()
		return "", nil, err
	}
	return dir, release, nil
}

// take is acquire for a subtest: the directory goes back when t ends.
func (p *pool) take(t *testing.T) string {
	t.Helper()
	dir, release, err := p.acquire()
	if err != nil {
		t.Fatalf("cannot prepare a clean tree for this mutant, so it was not run: %v", err)
	}
	t.Cleanup(release)
	return dir
}

// baseline runs names on the unmutated snapshot in a worker directory and
// gives the directory straight back. It runs in the parent test, never as a
// subtest: `-run 'TestFrozenMutantsStillDie/^M14_'` would filter a subtest
// out, and a selected mutant would then be measured against a baseline
// nobody checked.
func (p *pool) baseline(t *testing.T, names []string) (outcome, string) {
	t.Helper()
	dir, release, err := p.acquire()
	if err != nil {
		t.Fatalf("cannot prepare the baseline tree: %v", err)
	}
	defer release()
	o, _, out := runNamed(t, dir, names)
	return o, out
}

// results records one outcome per mutant from concurrent subtests.
type results struct {
	mu sync.Mutex
	v  verdict
	// kills is mutant id -> tests that killed it, for every mutant that
	// reached the suite (stillborn ones are in v.stillborn instead).
	kills map[string]map[string]bool
	seen  map[string]int
	// started is every mutant whose subtest began, in the order they began,
	// which under -run is the selection and otherwise the whole catalogue.
	started []string
}

func newResults() *results {
	return &results{kills: map[string]map[string]bool{}, seen: map[string]int{}}
}

// start registers a mutant's subtest before any work that can fail.
func (r *results) start(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = append(r.started, id)
}

func (r *results) record(id, name string, o outcome, failed map[string]bool, out string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen[id]++
	switch o {
	case stillborn:
		r.v.stillborn = append(r.v.stillborn, stillbirth{id: id, name: name, out: out})
		return
	case survived:
		r.v.survived++
	default:
		r.v.killed++
	}
	r.kills[id] = failed
}

// accounted reports every started mutant that has no outcome or more than
// one, and any outcome for a mutant that never started. A mutant whose subtest failed before reaching the suite (its tree
// could not be prepared, its anchor moved) has none, and in a run where
// completion order is the scheduler's, saying so by id is what stops a
// missing result reading as a quiet pass.
func (r *results) accounted() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.started
	var missing, doubled []string
	for _, id := range ids {
		switch r.seen[id] {
		case 0:
			missing = append(missing, id)
		case 1:
		default:
			doubled = append(doubled, id)
		}
	}
	for id := range r.seen {
		if !contains(ids, id) {
			doubled = append(doubled, id+" (never started)")
		}
	}
	sort.Strings(doubled)
	if len(missing) == 0 && len(doubled) == 0 {
		return nil
	}
	return fmt.Errorf("%d started mutants reached no outcome: %v; %d have more than one: %v. "+
		"The score below is over the rest, not over the catalogue",
		len(missing), missing, len(doubled), doubled)
}

// stillbornSorted returns the stillborn list in catalogue order, because the
// order they completed in is the scheduler's.
func (r *results) stillbornSorted(order []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	sort.Slice(r.v.stillborn, func(i, j int) bool { return pos[r.v.stillborn[i].id] < pos[r.v.stillborn[j].id] })
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// poolWorkers reads the worker count for this run.
func poolWorkers(t *testing.T) int {
	t.Helper()
	n, err := workerCount(os.Getenv(workersEnv), runtime.NumCPU())
	if err != nil {
		t.Fatal(err)
	}
	return n
}
