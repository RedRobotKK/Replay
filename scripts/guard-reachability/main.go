//go:build ignore

// guard-reachability neutralises each conditional a change touches and reports
// the ones whose removal changes nothing observable.
//
// The gap it fills is not a missing test, it is a missing reader. On
// 2026-09-09 this repository merged 23 pull requests in fourteen hours, every
// one of them authored and merged by the same agent, with CI green throughout.
// Three carried defects. All three were found by disabling a guard and watching
// the suite stay green — none by review, and none by the tests that shipped
// alongside them:
//
//   - an advisor fix that left Verified unreachable, reported green from a
//     filtered run that excluded the failing test
//   - an absence fix whose four tests all passed with half the fix reverted,
//     because every one called the inner function directly and none crossed
//     the join
//   - a guard written to close a vacuous test which was itself vacuous on
//     arrival, asserting on wording a different refusal also produces
//
// A reviewer would plausibly have caught none of those by reading. A machine
// that disables each new conditional and reruns the tests catches all three,
// and cannot get bored on the twenty-third pull request of the day.
//
// ADR-0014 names this practice and describes an implementation in a sibling
// repository. This is the Go one, scoped to a diff so it costs seconds rather
// than the eleven minutes the full mutant catalogue needs.
//
// Usage:
//
//	go run scripts/guard-reachability/main.go [base-ref]
//
// It compares the working tree against base-ref (default origin/main), finds
// conditionals added or changed in non-test Go files, and for each one runs the
// packages that could observe it with the condition forced false.
//
// A conditional whose neutralisation leaves every test passing is reported,
// and the report says which of two things it is, because they have different
// fixes:
//
//   - UNREACHED: no test ever makes the condition true. The branch is not
//     exercised at all, and the fix is a test.
//   - INERT: the branch runs, and no test depends on whether it did. Either
//     the statement is redundant with the code below it — delete it — or it
//     changes something real that nothing asserts on. Both happened on the
//     first run of this verdict against its own package: two error returns
//     stated an outcome the fall-through already produced, and one guard
//     changed which line a scan stopped at, with no test watching. So the
//     verdict names the situation and leaves the choice to a reader, rather
//     than asking for a test that would freeze dead code in place.
//
// Coverage separates them: `go test -covermode=count` at the baseline says
// whether any test entered the body. Where coverage has no block for a guard,
// the verdict stays UNOBSERVED and says so — absence, zero and unknown are
// three values (ADR-0018).
//
// A mutant the compiler rejects was never put to the suite, so it is counted
// and reported as UNCHECKED rather than passed over. An unchecked guard is the
// false green this tool exists to prevent, in the tool itself.
//
// # What a verdict here does not mean
//
// Every verdict is scoped to the host that produced it. Coverage is measured
// by running this machine's tests on this machine's OS, so a branch that only
// fires elsewhere is UNREACHED here and load-bearing there.
//
// This is not hypothetical. On the run that introduced these verdicts, the
// tool reported a name comparison in its own coverage matcher as never true,
// the comparison was deleted as dead, and Windows CI went red: the deleted
// line was the one normalising a backslash path, and it could not be true on
// a host whose separator is already a slash. The verdict was accurate and the
// conclusion drawn from it was wrong.
//
// So UNREACHED means "no test on THIS host makes this true". Before deleting
// a branch on its authority, ask whether the condition is one another
// platform, another build tag, or another configuration could satisfy. The
// tool cannot ask that question; it only runs here.

// The same caution applies across packages, for a different reason.
//
// Each mutant is put only to its own package's tests, because that is what
// makes the run cost seconds rather than the whole suite per guard. A test
// that would have caught the mutant from a neighbouring package is therefore
// never run against it, and the guard comes back SURVIVED with that test
// passing all along.
//
// Found by use, on the pull request after this tool shipped: a test written in
// cmd/replay for a guard in internal/usage reported SURVIVED while the test
// itself was green. It is the mirror of the defect ADR-0018 names — there the
// test sat too close to the thing it checked, here too far from it.
//
// So a survivor is a claim about the guard's own package. Before writing a
// test to satisfy one, check whether a test somewhere else already covers it;
// if it does, the honest fix is usually to move the test next to the guard,
// not to add a second.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/guardcheck"
)

func main() {
	base := "origin/main"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}

	changed, err := changedGoFiles(base)
	if err != nil {
		fail("listing changed files: %v", err)
	}
	if len(changed) == 0 {
		fmt.Println("guard-reachability: no non-test Go files changed against " + base)
		return
	}

	var guards, unbuilt []guardcheck.Guard
	for file, lines := range changed {
		// A file excluded from every build has no guard that can affect
		// anyone, and its package cannot be tested at all.
		//
		// Found by running this tool on the pull request that adds it. It
		// analysed its own source, found 28 conditionals, and tried to
		// `go test ./scripts/guard-reachability` — a package carrying
		// //go:build ignore, so the toolchain reports "build constraints
		// exclude all Go files" and the baseline is red. The refusal worked;
		// the analysis should never have got that far.
		// Order matters, and the two questions are different.
		//
		// ExcludedFromBuild evaluates with every tag false, under which
		// //go:build ignore and //go:build linux both come back excluded. Only
		// the first is correct to skip: an ignore file is built nowhere and
		// ships in no binary, while a linux file ships on Linux and skipping
		// it silently here hides a guard that reaches users.
		if excluded, why := guardcheck.ExcludedFromBuild(file); excluded &&
			!guardcheck.BuiltOnSomePlatform(file) {
			fmt.Printf("guard-reachability: skipping %s (%s)\n", file, why)
			continue
		}
		// A file this host does not compile is not skipped silently.
		//
		// Its conditionals cannot be neutralised — there is no build to run
		// them against — but leaving them out of the count entirely is how a
		// pass reports "0 survived, 0 unchecked" over guards nothing looked
		// at. They are counted and named instead.
		if buildable, why := guardcheck.BuildableHere(file); !buildable {
			gs, err := guardcheck.Conditionals(file, lines)
			if err != nil {
				fmt.Printf("guard-reachability: %s is not built here (%s) and will not parse: %v\n", file, why, err)
				continue
			}
			for _, g := range gs {
				g.Src = g.Src + "   [" + why + "]"
				unbuilt = append(unbuilt, g)
			}
			continue
		}
		gs, err := guardcheck.Conditionals(file, lines)
		if err != nil {
			fail("parsing %s: %v", file, err)
		}
		guards = append(guards, gs...)
	}
	if len(guards) == 0 && len(unbuilt) == 0 {
		fmt.Printf("guard-reachability: %d changed file(s), no new or changed conditionals\n", len(changed))
		return
	}
	if len(guards) == 0 {
		fmt.Printf("guard-reachability: %d changed file(s), and every conditional is in a "+
			"file this host does not build\n", len(changed))
		report("These were not scored. They are in files this host does not compile, so no\n"+
			"build exists to neutralise them against. Run the reviewer on a host that\n"+
			"builds them, or treat them as unreviewed:", unbuilt)
		os.Exit(1)
	}

	// A red baseline makes every guard look caught, which is the failure this
	// tool exists to prevent in others. ADR-0014 refuses to run against one.
	//
	// The baseline is also where coverage is measured, on unmutated code: it
	// is the record of which branches the suite enters when nothing has been
	// tampered with.
	fmt.Printf("guard-reachability: baseline over %d guard(s) in %d file(s)\n", len(guards), len(changed))
	profile := filepath.Join(os.TempDir(), fmt.Sprintf("guard-cover-%d.out", os.Getpid()))
	defer func() { _ = os.Remove(profile) }()
	baselineStart := time.Now()
	out, ok := runTestsWithCoverage(pkgsOf(guards), profile)
	baseline := time.Since(baselineStart)
	if !ok {
		fail("the baseline is red, so every mutant would look caught:\n%s", out)
	}
	cov, err := guardcheck.ParseCoverage(profile)
	if err != nil {
		// Coverage is an enrichment, not the verdict. Without it every
		// survivor is reported as before, which is less useful and still true.
		fmt.Printf("guard-reachability: no coverage detail (%v); survivors will not be classified\n", err)
		cov = nil
	}
	if cov != nil && cov.Unparsed > 0 {
		// The profile format is an assumption. When it stops holding, every
		// block stops matching and every survivor degrades to UNOBSERVED —
		// which reads as a suite problem rather than an instrument one.
		fmt.Printf("guard-reachability: %d line(s) of the coverage profile did not parse; "+
			"survivor classification may be degraded\n", cov.Unparsed)
	}

	var unreached, inert, unobserved, unchecked []guardcheck.Guard
	for i, g := range guards {
		fmt.Printf("  [%d/%d] %s:%d  %s\n", i+1, len(guards), g.File, g.Line, short(g.Src))
		restore, err := guardcheck.Neutralise(g)
		if err != nil {
			fmt.Printf("        skipped: %v\n", err)
			continue
		}
		out, green := runTests([]string{g.Pkg}, guardcheck.NeutralisedTimeout(baseline))
		restore()
		switch {
		case strings.Contains(out, "build failed") || strings.Contains(out, "cannot use"):
			// A mutant the compiler rejected was never put to the suite.
			// Counting it as caught is how a score is inflated.
			fmt.Printf("        UNCHECKED: the neutralised form does not compile\n")
			unchecked = append(unchecked, g)
		case green:
			taken, known := cov.BranchTaken(g)
			switch {
			case known && !taken:
				fmt.Printf("        UNREACHED: no test makes this condition true\n")
				unreached = append(unreached, g)
			case known:
				fmt.Printf("        INERT: the branch runs and nothing depends on it\n")
				inert = append(inert, g)
			default:
				fmt.Printf("        UNOBSERVED: nothing observed this guard, and coverage has no block for it\n")
				unobserved = append(unobserved, g)
			}
		default:
			fmt.Printf("        caught\n")
		}
	}

	total := len(unreached) + len(inert) + len(unobserved)
	fmt.Printf("\nguard-reachability: %d guard(s), %d survived, %d unchecked, %d not built here\n",
		len(guards), total, len(unchecked), len(unbuilt))
	if total == 0 && len(unchecked) == 0 && len(unbuilt) == 0 {
		return
	}

	report("These branches are never entered by any test. The fix is a test that\n"+
		"makes the condition true:", unreached)
	report("These branches run, and no test depends on whether they did. Either the\n"+
		"statement is redundant with the code below it — delete it — or it changes\n"+
		"something real that nothing asserts on. Read it before choosing: a test\n"+
		"written to satisfy this verdict can freeze dead code in place:", inert)
	report("These survived and coverage carried no block for them, so which of the\n"+
		"two above they are is NOT MEASURED:", unobserved)
	report("These were not scored at all. They are in files this host does not\n"+
		"compile, so there is no build to neutralise them against — and a pass that\n"+
		"left them out of the count would print a clean line over guards nothing\n"+
		"looked at. Run the reviewer on a host that builds them, or treat them as\n"+
		"unreviewed:", unbuilt)
	report("These were never put to the suite at all: the compiler rejected the\n"+
		"neutralised form, so nothing about them was measured. An unchecked guard\n"+
		"is the false green this tool exists to prevent:", unchecked)
	os.Exit(1)
}

// report prints one verdict's guards under its heading, or nothing.
func report(heading string, gs []guardcheck.Guard) {
	if len(gs) == 0 {
		return
	}
	fmt.Println("\n" + heading)
	for _, g := range gs {
		fmt.Printf("  %s:%d  %s\n", g.File, g.Line, short(g.Src))
	}
}

// pkgsOf lists the distinct packages the guards live in.
func pkgsOf(gs []guardcheck.Guard) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range gs {
		if !seen[g.Pkg] {
			seen[g.Pkg] = true
			out = append(out, g.Pkg)
		}
	}
	return out
}

// short trims a source line to something a terminal can hold.
func short(s string) string {
	const max = 90
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "guard-reachability: "+format+"\n", a...)
	os.Exit(1)
}

// changedGoFiles asks git what moved and hands the text to the parser.
//
// The git invocation lives here rather than in internal/guardcheck because
// os/exec is confined to the mutation build tag, and this file is the one
// place exempted from that confinement. Keeping the analysis package free of
// process invocation is the point of the split; it also leaves every branch of
// the diff parser reachable from a test with a string.
//
// Committed work first, then the working tree, and the union of both. The
// first version fell back only when the three-dot diff ERRORED. On a branch
// with no commits yet that diff succeeds and is empty, so uncommitted changes
// were invisible and the tool reported "no non-test Go files changed" over a
// file it had just been pointed at. Found by planting a guard and watching it
// say nothing.
func changedGoFiles(base string) (map[string]map[int]bool, error) {
	var out []byte
	for _, args := range [][]string{
		{"diff", "-U0", base + "...HEAD"},
		{"diff", "-U0", base},
	} {
		b, err := exec.Command("git", args...).Output()
		if err != nil {
			continue
		}
		out = append(out, b...)
	}
	return guardcheck.ParseDiff(string(out)), nil
}

// runTests runs the packages under a bound and reports whether they passed.
//
// The bound matters here and not at the baseline. A neutralised guard can turn
// a bounded walk into one that never ends, and a suite that hangs is a mutant
// caught, not one worth waiting on: without the bound each such guard costs
// `go test`'s ten-minute default, and a change touching forty conditionals
// exhausts a CI runner before it reaches the twentieth. That is not a
// hypothetical — it is why this reviewer was killed at exit 143 on the change
// that added it.
func runTests(pkgs []string, limit time.Duration) (string, bool) {
	args := append([]string{"test", "-count=1", "-timeout", limit.String()}, pkgs...)
	cmd := exec.Command("go", args...)
	// Declare the neutralisation to the suite being run. A tree with a
	// `false &&` in it is exactly what guardcheck.GC6 refuses, and without
	// this the reviewer fails that test on every guard — which makes every
	// mutant look caught and the whole verdict worthless. The marker is set
	// only while a guard is neutralised; the baseline runs without it, so
	// GC6 still guards the tree the reviewer started from.
	cmd.Env = append(os.Environ(), guardcheck.NeutralisingEnv+"=1")
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// runTestsWithCoverage runs the packages and writes a count-mode profile.
//
// Count mode, not set mode: the reviewer only asks whether a block ran at
// least once, and count mode answers that as well while leaving the door open
// to asking how often.
func runTestsWithCoverage(pkgs []string, profile string) (string, bool) {
	args := append([]string{"test", "-count=1", "-covermode=count", "-coverprofile=" + profile}, pkgs...)
	out, err := exec.Command("go", args...).CombinedOutput()
	return string(out), err == nil
}
