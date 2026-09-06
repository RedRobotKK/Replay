//go:build mutation

// Package mutation re-runs the frozen mutants.
//
// This exists because of a question with an embarrassing answer: what happens
// to a mutant after it is caught? Nothing. It is applied, the suite goes red,
// it is reverted, and it is gone. What survives is a number in a commit
// message — "9 mutants, 9 caught" — which is a claim no reader can check and
// no future run can repeat. The mutant was the evidence and it was thrown away.
//
// So they are frozen instead: testdata/mutants.json carries each one as data,
// and this harness thaws them one at a time against a scratch copy of the
// tree, runs the tests that are supposed to kill them, and puts them back.
// The score becomes reproducible, and a defect that was fixed once cannot
// return quietly — the mutant that represents it is still here, still being
// asked whether the check that caught it works.
//
// Two rules make it evidence rather than ceremony, both from ADR-0014:
//
//   - It refuses to run on a red baseline. A mutant "caught" by a suite that
//     was already failing has been caught by nothing.
//   - A mutant whose anchor no longer exists is a FAILURE, not a skip. The
//     code moved and nobody checked whether the guard moved with it, which is
//     precisely how a test stops covering the thing it was written for.
//
// It is behind a build tag because it copies the tree and runs the suite once
// per mutant. Run it with:
//
//	go test -tags mutation ./internal/mutation/ -v
package mutation

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type edit struct {
	File        string `json:"file"`
	Anchor      string `json:"anchor"`
	Replacement string `json:"replacement"`
}

type mutant struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Class      string `json:"class"`
	Discovered string `json:"discovered"`
	Note       string `json:"note"`

	File        string `json:"file"`
	Anchor      string `json:"anchor"`
	Replacement string `json:"replacement"`
	// Also carries the further edits of a multi-part mutant, so a defect that
	// took two changes to ship can be reproduced as it actually shipped.
	Also []edit `json:"also"`

	KilledBy []string `json:"killedBy"`
}

func (m mutant) edits() []edit {
	return append([]edit{{File: m.File, Anchor: m.Anchor, Replacement: m.Replacement}}, m.Also...)
}

type catalogue struct {
	Schema  string            `json:"schema"`
	Classes map[string]string `json:"classes"`
	Mutants []mutant          `json:"mutants"`
}

func load(t *testing.T) catalogue {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "mutants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c catalogue
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("the catalogue does not parse: %v", err)
	}
	if c.Schema != "replay.mutants.v1" {
		t.Fatalf("unknown catalogue schema %q", c.Schema)
	}
	return c
}

// repoRoot is two levels up from internal/mutation.
func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// thaw copies the tree somewhere writable, so a mutant is never applied to the
// working copy. A harness that edits the repository leaves it mutated whenever
// it is interrupted, and the interruption is exactly when nobody is looking.
func thaw(t *testing.T, root string) string {
	t.Helper()
	dst := t.TempDir()
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
	for _, name := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if name == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue // a tracked file that is not on disk right now
		}
		target := filepath.Join(dst, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// outcome is what happened to a mutant. The three are not interchangeable and
// collapsing any two of them is how a mutation score stops meaning anything.
type outcome int

const (
	// killed: the mutant compiled, ran, and at least one test failed. This is
	// the only outcome that is evidence about the test suite.
	killed outcome = iota
	// survived: the mutant compiled, ran, and every test passed. Either the
	// suite has a hole, or the mutant is equivalent — it changes no behaviour
	// anyone can observe, in which case not killing it is correct. Mutation
	// testing cannot tell those apart automatically; a human must say which.
	survived
	// stillborn: the mutant did not compile. The compiler refused it, so the
	// test suite was never asked anything. Counting this as a kill is the
	// most common way a mutation score is inflated, and it is a real risk
	// here: `go test` exits non-zero for a build failure exactly as it does
	// for a test failure, so the naive check reports a kill for a mutant no
	// test ever saw.
	stillborn
)

func (o outcome) String() string {
	switch o {
	case killed:
		return "killed"
	case survived:
		return "survived"
	default:
		return "stillborn"
	}
}

// buildFailed distinguishes a compiler refusal from a test failure.
func buildFailed(out string) bool {
	return strings.Contains(out, "[build failed]") ||
		strings.Contains(out, "undefined:") ||
		strings.Contains(out, "declared and not used") ||
		strings.Contains(out, "syntax error")
}

// failedTests returns the names of the tests that failed in one run.
func failedTests(out string) map[string]bool {
	failed := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--- FAIL: ") {
			continue
		}
		name := strings.TrimPrefix(line, "--- FAIL: ")
		if i := strings.IndexAny(name, " \t"); i > 0 {
			name = name[:i]
		}
		// Subtests belong to their parent for this purpose.
		if i := strings.Index(name, "/"); i > 0 {
			name = name[:i]
		}
		failed[name] = true
	}
	return failed
}

// runNamed runs the given tests in one pass and reports which of them failed.
// One pass rather than one per test, because the tree has to be compiled
// either way and compiling it is most of the cost.
func runNamed(t *testing.T, dir string, names []string) (outcome, map[string]bool, string) {
	t.Helper()
	// Ask the compiler directly rather than matching strings in the test
	// output. Whether a mutant built is a fact `go build` knows exactly, and
	// a heuristic that misses one phrase produces the score inflation this
	// distinction exists to prevent.
	build := exec.Command("go", "build", "./...")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		return stillborn, nil, string(out)
	}
	cmd := exec.Command("go", "test", "./...", "-count=1", "-run", "^("+strings.Join(names, "|")+")$")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := string(out)
	if buildFailed(text) {
		// Test files are not covered by `go build`, so a mutant that breaks
		// only a _test.go still reaches here.
		return stillborn, nil, text
	}
	if err == nil {
		return survived, nil, text
	}
	return killed, failedTests(text), text
}

// runTests reports whether one named test passed, distinguishing a build
// failure so a mutant the compiler rejected is never counted as caught.
func runTests(t *testing.T, dir, name string) (bool, string) {
	t.Helper()
	o, _, out := runNamed(t, dir, []string{name})
	return o == survived, out
}

// Admission criteria. What makes an edit a mutant worth freezing.
//
// Generic mutation testing perturbs operators at random and drowns in
// equivalent mutants. This catalogue is not that. An edit earns a place only
// if all four hold, and the harness checks the ones a machine can check:
//
//  1. VIABLE — it compiles. A mutant the compiler refuses asks the test suite
//     nothing, so counting it is inflation. Asserted: stillborn fails the run.
//  2. OBSERVABLE — it changes something a person could see. An edit that
//     changes no behaviour is an equivalent mutant, and surviving is then the
//     correct answer rather than a hole. Not machine-checkable; the note field
//     must say what changes on screen.
//  3. HISTORICAL — it reintroduces a defect that actually shipped or was
//     actually found here. This is the filter that makes the catalogue small
//     and worth re-running: every entry is a bug that got past a review once.
//     Asserted weakly: every mutant carries the date it was found.
//  4. SPECIFICALLY GUARDED — a named test dies. A mutant killed by "the suite"
//     records nothing; a mutant killed by a named test records which check is
//     load-bearing, so weakening that check is visible. Asserted: killedBy is
//     non-empty and every name in it exists.

// TestFrozenMutantsStillDie is the score, recomputed.
func TestFrozenMutantsStillDie(t *testing.T) {
	c := load(t)
	root := repoRoot(t)

	// The baseline first. Every killer test must pass before any mutant is
	// applied, or a red result proves nothing about the mutant.
	base := thaw(t, root)
	o, _, out := runNamed(t, base, killerNames(c))
	if o != survived {
		t.Fatalf("baseline is %s: the named tests do not all pass before any mutant is applied. "+
			"A mutant caught by an already-failing suite has been caught by nothing.\n%s", o, tail(out))
	}

	var nKilled, nSurvived, nStillborn int
	for _, m := range c.Mutants {
		t.Run(m.ID+"_"+m.Name, func(t *testing.T) {
			if c.Classes[m.Class] == "" {
				t.Fatalf("%s has class %q, which the catalogue does not define", m.ID, m.Class)
			}
			if len(m.KilledBy) == 0 {
				t.Fatalf("%s names no test that kills it, so it asserts nothing", m.ID)
			}
			if m.Discovered == "" || m.Note == "" {
				t.Fatalf("%s records neither when it was found nor what it changes; both are the "+
					"reason to keep it rather than a random operator mutation", m.ID)
			}
			dir := thaw(t, root)
			apply(t, dir, m)

			o, failed, out := runNamed(t, dir, m.KilledBy)
			switch o {
			case stillborn:
				nStillborn++
				t.Fatalf("%s (%s) did not compile, so no test was ever asked about it. "+
					"A mutant the compiler refuses is not evidence about the suite; "+
					"rewrite it so it builds.\n%s", m.ID, m.Name, tail(out))
			case survived:
				nSurvived++
				t.Errorf("%s (%s) SURVIVED %s.\n\n%s\n\nEither the defect this represents can "+
					"ship again, or the mutant is equivalent and changes nothing observable — "+
					"say which, in the catalogue.", m.ID, m.Name, strings.Join(m.KilledBy, ", "), m.Note)
			default:
				nKilled++
				for _, name := range m.KilledBy {
					if !failed[name] {
						t.Errorf("%s names %s as a killer, but %s passed with the mutant applied. "+
							"Something else caught it, and this entry misrecords which check is "+
							"load-bearing.", m.ID, name, name)
					}
				}
			}
		})
	}
	t.Logf("%d mutants: %d killed, %d survived, %d stillborn", len(c.Mutants), nKilled, nSurvived, nStillborn)
}

// TestKillMatrix is the permutation analysis: every mutant against every
// killer test in the catalogue, not just the ones it claims.
//
// It answers two questions the score cannot. Which mutants are guarded by
// exactly one test — those tests are load-bearing and weakening one silently
// removes the only thing standing between a known defect and a release. And
// which tests kill nothing that another test does not already kill — those are
// candidates for deletion, or a sign the suite is testing one thing twice and
// something else not at all.
func TestKillMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("the matrix compiles the tree once per mutant")
	}
	c := load(t)
	root := repoRoot(t)
	all := killerNames(c)

	// The baseline rule is per experiment, not per file. Without it, a tree
	// where one killer test is already red reports every mutant as killed by
	// that test — a fully fictitious matrix, produced by the instrument built
	// to find which checks are load-bearing.
	if o, _, out := runNamed(t, thaw(t, root), all); o != survived {
		t.Fatalf("baseline is %s; the matrix would attribute that failure to every mutant.\n%s", o, tail(out))
	}

	kills := map[string]map[string]bool{} // mutant id -> tests that killed it
	for _, m := range c.Mutants {
		dir := thaw(t, root)
		apply(t, dir, m)
		o, failed, _ := runNamed(t, dir, all)
		if o == stillborn {
			t.Errorf("%s did not compile; excluded from the matrix", m.ID)
			continue
		}
		kills[m.ID] = failed
	}

	t.Log("kill matrix — mutant: tests that detect it")
	var singly []string
	for _, m := range c.Mutants {
		f := kills[m.ID]
		names := make([]string, 0, len(f))
		for n := range f {
			names = append(names, n)
		}
		sort.Strings(names)
		t.Logf("  %-4s %-32s %d: %s", m.ID, m.Name, len(names), strings.Join(names, ", "))
		if len(names) == 1 {
			singly = append(singly, m.ID+" ("+m.Name+") guarded only by "+names[0])
		}
		if len(names) == 0 {
			t.Errorf("%s is detected by no test in the catalogue", m.ID)
		}
	}
	if len(singly) > 0 {
		t.Logf("singly guarded — weakening any of these removes the only detection:")
		for _, s := range singly {
			t.Logf("  %s", s)
		}
	}

	// A test that detects nothing another test does not already detect is not
	// pulling its weight as a guard. That is a finding, not a failure: it may
	// still be the clearest statement of the property.
	for _, name := range all {
		unique := 0
		for id, f := range kills {
			if f[name] && len(f) == 1 {
				unique++
				_ = id
			}
		}
		if unique == 0 {
			t.Logf("note: %s is the sole detector of nothing; every mutant it catches is caught elsewhere too", name)
		}
	}
}

// apply edits a thawed tree, refusing a mutant whose anchor has moved.
func apply(t *testing.T, dir string, m mutant) {
	t.Helper()
	for _, e := range m.edits() {
		path := filepath.Join(dir, e.File)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s targets %s, which is gone: %v", m.ID, e.File, err)
		}
		// Exactly one site, or the edit is ambiguous: Replace takes the first
		// occurrence, so a duplicated anchor would silently mutate somewhere
		// other than the place this mutant is about and still report a kill.
		if n := strings.Count(string(body), e.Anchor); n > 1 {
			t.Fatalf("%s matches %s in %d places. The mutation would land on the first of them, "+
				"which is not necessarily the one this mutant is about. Lengthen the anchor "+
				"until it is unique.", m.ID, e.File, n)
		}
		if !strings.Contains(string(body), e.Anchor) {
			// Not a skip. The code moved and nobody checked whether the guard
			// moved with it.
			t.Fatalf("%s no longer applies: %s does not contain its anchor.\n\n%s\n\n"+
				"Either the defect became impossible to express, in which case delete the "+
				"mutant and say so, or the code was refactored past the check that caught "+
				"it, in which case the check needs re-earning.", m.ID, e.File, e.Anchor)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(body), e.Anchor, e.Replacement, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEveryClassIsRepresented keeps the taxonomy honest: a class nothing is
// filed under is a category somebody invented rather than observed.
func TestEveryClassIsRepresented(t *testing.T) {
	c := load(t)
	seen := map[string]int{}
	for _, m := range c.Mutants {
		seen[m.Class]++
	}
	for class, desc := range c.Classes {
		if seen[class] == 0 {
			t.Errorf("class %q (%s) has no mutants. Either something belongs in it or it is not a real class here.", class, desc)
		}
	}
}

func killerNames(c catalogue) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range c.Mutants {
		for _, n := range m.KilledBy {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 15 {
		lines = lines[len(lines)-15:]
	}
	return strings.Join(lines, "\n")
}
