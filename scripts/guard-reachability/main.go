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
// WHAT "A CHANGE TOUCHES" MEANS, AND WHAT IT DOES NOT.
//
// The scope is a LINE diff, not a semantic one. For an edit that is the same
// thing; for a MOVE it is not, and the difference is large enough to read as a
// failure when nothing has broken.
//
// Splitting one file into several presents every relocated line as added, so
// this tool re-analyses essentially every conditional in the old file and
// reports the whole pre-existing untested surface at once, under new
// filenames. That happened on the pull request splitting internal/proxy:
// 125 guards, 39 survived. Coverage was 94.5% on both sides and the old file
// held exactly 27 zero-count blocks against exactly 27 across the new ones —
// the same holes, the same count, different names. The refactor did not create
// them. It revealed them.
//
// So a survivor on a move means "this branch is untested", never "this change
// broke something". Both are worth knowing and they are not the same verdict.
// This paragraph used to end by saying the tool could not tell them apart short
// of a rename-aware diff it deliberately does not do. It now can, by another
// route: it re-measures the survivor in the base tree. What that buys, and the
// four things it still cannot see, are below under "Telling a move from an
// edit".
//
// The failure runs in the safe direction — noisy rather than silent — and the
// cost of the alternative is what is being bought: a diff-scoped check is
// cheap enough to run on every pull request, which is the property that makes
// it useful at all.
//
// This is one instance of a rule the repository keeps rediscovering, and it is
// written here because this is the tool most likely to be believed:
//
//	A SOURCE SCAN MAY CLAIM ONLY WHAT IT TRAVERSES.
//
// Four scans were corrected in a single day for claiming more than they read.
// internal/otlp OT6 and internal/probe R3 both proved "this package cannot
// reach the network" and "the credential never comes from a flag" by grepping
// for a fixed list of literals; a working capability added under any other
// name passed both. internal/regression FC-PX described itself as checking
// comments while scanning only line-leading ones. A fourth counted comment
// text as code. None of them was wrong about what it found — each was wrong
// about what its finding covered.
//
// The repair in every case was the same: state the traversal in the test, and
// state what it cannot see. A scan that names its own blind spot is evidence.
// One that does not is a claim wearing evidence's clothes.
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
// A guard whose false arm exhausts MEMORY is unscoreable, and is not the same
// as one that merely runs long.
//
// The distinction is the whole finding, and the first draft of this paragraph
// lost it. A neutralised guard that turns a bounded walk into a slow one is
// already handled: runTests below bounds every neutralised run with
// guardcheck.NeutralisedTimeout, and its comment records that such a suite is
// a mutant CAUGHT rather than one worth waiting on. That bound is wall clock.
// It cannot fire on a guard whose false arm allocates without limit, because
// the OS kills the parent process before the child's deadline arrives — so the
// run reports zero survivors having never finished, and the two paragraphs in
// this file would appear to disagree about whether a hanging suite is a catch.
// They do not: one is about time, which is bounded, and this is about memory,
// which is not.
//
// Found on PR #246 — an allocation ceiling `if chars > ceiling` in sizedFiller
// was load-bearing in the strongest sense available, since deleting it
// exhausts the machine, and that is exactly why a tool which works by removing
// guards could not measure it. This job carries no timeout-minutes in CI, so
// the runner's own kill is the only bound there is.
//
// Write such a bound as arithmetic (`chars = min(chars, cap)`), which has no
// branch to neutralise and changes no behaviour. Do not exempt it.
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
//
// # Telling a move from an edit, by asking the base tree
//
// The section above names the defect and stops there, because when it was
// written nothing here could do better. The repair is not a rename-aware diff,
// which would still be guessing from text alone: when a conditional survives
// neutralisation, the reviewer asks the BASE TREE whether it survived there
// too, and measures the answer the same way it measured this one.
//
// It checks the base out into a temporary detached worktree, finds the same
// condition there by package, enclosing function and condition text — not by
// file and line, which a move changes — neutralises it THERE and runs that
// package's tests THERE. A survivor that also survived in the base is reported
// PRE-EXISTING and does not fail the build. It is still printed, with its
// UNREACHED or INERT verdict intact, because it is still a hole; it is just not
// this change's hole.
//
// On the split above, re-run against origin/main on 2026-09-11: 136 guards, 42
// survived, 35 of them found in the base tree and surviving there too, 7 left
// to fail. Every per-guard verdict was byte-identical to the run before this
// change — 94 caught, 26 UNREACHED, 16 INERT — so what moved is which
// survivors fail the build, not what any of them was judged to be.
//
// The seven are worth reading, because they are what this deliberately does not
// absorb. All seven come from the branch sitting twenty-five commits behind
// main rather than from the split: four tap conditions that main's
// openaistream_e2e_test.go catches and the branch has no copy of, and three in
// internal/cachemodel that main rewrote. Confirmed by hand — dropping that one
// test file into the branch and neutralising `case t.ostream != nil` turns it
// red. Comparing a working tree against the base's HEAD is not the same as
// comparing it against the merge, and this still does the former; ADR-0020
// names that family of defect for the compile case.
//
// The count differs from the 125 and 39 recorded above because main moved in
// between, and because the union diff this tool takes picks that drift up as
// well. The 94.5% and the 27-against-27 are that earlier diagnosis's
// measurement, not this one's.
//
// What this does NOT do, and every one of these is a way to be wrong:
//
//   - It does not pair identical conditions by name. Where one function holds
//     several `if err != nil`, nothing can say which is which, so the counts
//     are compared and the excess fails. "Pre-existing" there means one of them
//     was already unobserved, not that this one was.
//   - It does not read meaning. A condition whose text is unchanged but which
//     now reads a different value — `if ok {` after the call above it was
//     swapped — is a new guard wearing an old face, and this calls it
//     pre-existing.
//   - It does not follow a guard across packages or through a renamed
//     function. Both read as introduced, which is the loud direction.
//   - It does not certify anything when the base tree's own suite is red, when
//     the base cannot be checked out, or when the neutralised form does not
//     compile there. Each of those reports the survivor as introduced and
//     fails. The reviewer's value is that it fails noisily rather than
//     silently, and an exemption is earned per guard or not at all.
//
// It is not free. A run with survivors pays for one detached worktree and one
// package test run per surviving identity, bounded by how many survivors the
// change has for that identity — so a change with no survivors costs exactly
// what it did before, and the bill is proportional to the noise it saves.
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

	limit := guardcheck.NeutralisedTimeout(baseline)
	verdicts := map[guardcheck.Guard]string{}
	var survivors, unchecked []guardcheck.Guard
	for i, g := range guards {
		fmt.Printf("  [%d/%d] %s:%d  %s\n", i+1, len(guards), g.File, g.Line, short(g.Src))
		restore, err := guardcheck.Neutralise(g)
		if err != nil {
			fmt.Printf("        skipped: %v\n", err)
			continue
		}
		out, green := runTests("", []string{g.Pkg}, limit, true)
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
				verdicts[g] = unreachedVerdict
			case known:
				fmt.Printf("        INERT: the branch runs and nothing depends on it\n")
				verdicts[g] = inertVerdict
			default:
				fmt.Printf("        UNOBSERVED: nothing observed this guard, and coverage has no block for it\n")
				verdicts[g] = unobservedVerdict
			}
			survivors = append(survivors, g)
		default:
			fmt.Printf("        caught\n")
		}
	}

	// A survivor is only this change's business if the base tree did not
	// already have it. Asking costs a checkout and a run per survivor, so it is
	// asked once, after the loop, and only when there is something to ask about.
	var baseSurvivors map[guardcheck.Identity]int
	var baseErr error
	if len(survivors) > 0 {
		baseSurvivors, baseErr = baseSurvivorCounts(base, survivors, limit)
		if baseErr != nil {
			// Nothing certified is nothing exempted. The run fails on every
			// survivor, which is where it started before any of this existed.
			fmt.Printf("\nguard-reachability: the base tree could not answer (%v),\n"+
				"so no survivor can be shown to pre-date this change and every one is "+
				"reported as introduced\n", baseErr)
		}
	}
	// The split itself is guardcheck.ClassifySurvivors rather than an `if err`
	// here, because this file carries //go:build ignore and no test can reach
	// it. Fail-closed is the reviewer's one load-bearing property and it may
	// not live where a suite cannot watch it — see FC1..FC5 in that package.
	preExisting, introduced := guardcheck.ClassifySurvivors(survivors, baseSurvivors, baseErr)
	unreached := withVerdict(introduced, verdicts, unreachedVerdict)
	inert := withVerdict(introduced, verdicts, inertVerdict)
	unobserved := withVerdict(introduced, verdicts, unobservedVerdict)

	fmt.Printf("\nguard-reachability: %d guard(s), %d survived (%d pre-existing, %d introduced), "+
		"%d unchecked, %d not built here\n",
		len(guards), len(survivors), len(preExisting), len(introduced), len(unchecked), len(unbuilt))
	reportPreExisting(preExisting, verdicts, base)
	// Likewise the exit rule: classifying every survivor as introduced and then
	// exiting 0 would be the same silent fail-open by another route, so the
	// rule is decided where FC2, FC4 and FC5 can watch it.
	code := guardcheck.ExitCode(introduced, unchecked, unbuilt)
	if code == 0 {
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
	os.Exit(code)
}

// The three survivor verdicts, kept as names so the grouping below and the
// printing above cannot drift apart.
const (
	unreachedVerdict  = "UNREACHED"
	inertVerdict      = "INERT"
	unobservedVerdict = "UNOBSERVED"
)

// withVerdict selects the guards carrying one verdict, in the order they were
// judged.
func withVerdict(gs []guardcheck.Guard, verdicts map[guardcheck.Guard]string, want string) []guardcheck.Guard {
	var out []guardcheck.Guard
	for _, g := range gs {
		if verdicts[g] == want {
			out = append(out, g)
		}
	}
	return out
}

// baseSurvivorCounts asks the base tree how many survivors it already had per
// identity.
//
// The base is checked out into a temporary worktree and the SAME condition —
// same package, same enclosing function, same text — is neutralised there and
// put to that tree's own tests. Nothing is inferred from the text matching: a
// survivor is exempted only when its counterpart demonstrably survived in the
// base too, measured the same way.
//
// It measures and does not judge. Turning these counts into an exemption is
// guardcheck.ClassifySurvivors' job, and the split is not cosmetic: this
// function starts processes and creates worktrees, so it can only be exercised
// by running it, while the decision it feeds is pure and has FC1..FC5 on it.
//
// Three things make the whole fail closed rather than open, and all three are
// the point:
//
//   - a base tree that cannot be checked out, or whose own suite is red,
//     certifies nothing and returns an error, on which ClassifySurvivors
//     reports every survivor as introduced
//   - a base counterpart whose neutralised form does not compile there is not a
//     survivor there, so it certifies nothing
//   - the search is bounded by how many survivors the change has for an
//     identity, so a function whose three identical guards were two survivors
//     before and three now still reports one introduced
//
// Every error path returns a nil map as well as the error, so a caller that
// ignored the error would exempt nothing rather than something. That is belt
// and braces, not the contract: the contract is that the error dominates, and
// it is enforced in ClassifySurvivors where a test can see it.
func baseSurvivorCounts(base string, survivors []guardcheck.Guard, limit time.Duration) (map[guardcheck.Identity]int, error) {
	root, cleanup, err := baseWorktree(base)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	pkgs := pkgsOf(survivors)
	fmt.Printf("\nguard-reachability: %d survived; asking %s whether it had them too\n",
		len(survivors), base)
	if out, ok := runTests(root, pkgs, limit, false); !ok {
		return nil, fmt.Errorf("the base tree's own suite is red, so nothing there "+
			"can certify anything:\n%s", lastLines(out, 15))
	}
	index, err := baseConditionals(root, pkgs)
	if err != nil {
		return nil, err
	}

	// Iterated over the survivors rather than over the identity map, so the
	// order of the run — and of its output — does not depend on map iteration.
	wanted := guardcheck.CountByIdentity(survivors)
	baseSurvivors := map[guardcheck.Identity]int{}
	done := map[guardcheck.Identity]bool{}
	for _, g := range survivors {
		id := g.Identity()
		if done[id] {
			continue
		}
		done[id] = true
		if len(index[id]) == 0 {
			fmt.Printf("  %s has no `%s` in %s: introduced\n", base, short(g.Cond), idFunc(id))
			continue
		}
		for _, cand := range index[id] {
			if baseSurvivors[id] >= wanted[id] {
				break
			}
			rel := strings.TrimPrefix(strings.TrimPrefix(cand.File, root), string(os.PathSeparator))
			survived, why := survivesIn(root, cand, limit)
			if survived {
				baseSurvivors[id]++
				fmt.Printf("  %s:%d  %s  survives in %s too\n", rel, cand.Line, short(cand.Src), base)
				continue
			}
			fmt.Printf("  %s:%d  %s  does not certify it: %s\n", rel, cand.Line, short(cand.Src), why)
		}
	}
	return baseSurvivors, nil
}

// idFunc names an identity's function for a message, or says it has none.
func idFunc(id guardcheck.Identity) string {
	if id.Func == "" {
		return id.Pkg + " outside any function"
	}
	return id.Pkg + " " + id.Func
}

// survivesIn neutralises a guard in another tree and reports whether that
// tree's tests stayed green.
func survivesIn(root string, g guardcheck.Guard, limit time.Duration) (bool, string) {
	restore, err := guardcheck.Neutralise(g)
	if err != nil {
		return false, fmt.Sprintf("it could not be neutralised there (%v)", err)
	}
	out, green := runTests(root, []string{g.Pkg}, limit, true)
	restore()
	switch {
	case strings.Contains(out, "build failed") || strings.Contains(out, "cannot use"):
		// Unchecked there is not survived there. Treating it as survived would
		// exempt a guard on the strength of a mutant nobody ran.
		return false, "the neutralised form does not compile there"
	case green:
		return true, ""
	}
	return false, "a test there catches it"
}

// baseConditionals indexes every conditional in the base copies of the packages
// the survivors live in.
//
// Whole files, not a diff: the base has no diff to scope by, and the question
// is precisely where a moved guard used to live.
func baseConditionals(root string, pkgs []string) (map[guardcheck.Identity][]guardcheck.Guard, error) {
	out := map[guardcheck.Identity][]guardcheck.Guard{}
	for _, pkg := range pkgs {
		dir := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(pkg, "./")))
		entries, err := os.ReadDir(dir)
		if err != nil {
			// A package the base tree does not have is not an error. Every
			// survivor in it is new, which is what an empty index already says.
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			// The same two questions the working tree is put to, for the same
			// two reasons: a file built nowhere holds no guard that reaches
			// anyone, and a file this host does not compile cannot be
			// neutralised against a build that does not exist.
			if excluded, _ := guardcheck.ExcludedFromBuild(path); excluded &&
				!guardcheck.BuiltOnSomePlatform(path) {
				continue
			}
			if buildable, _ := guardcheck.BuildableHere(path); !buildable {
				continue
			}
			gs, err := guardcheck.Conditionals(path, nil)
			if err != nil {
				return nil, fmt.Errorf("parsing the base copy of %s: %w", name, err)
			}
			for _, g := range gs {
				// The package as the working tree names it, so an identity
				// built here compares equal to one built there. Set before
				// the identity is taken, which reads it.
				g.Pkg = pkg
				out[g.Identity()] = append(out[g.Identity()], g)
			}
		}
	}
	return out, nil
}

// baseWorktree checks the base ref out into a temporary directory.
//
// A worktree rather than `git show` per file, because the counterpart has to be
// TESTED, not just read: it needs its package, its tests and its go.mod around
// it. It is detached, so it does not collide with the same branch checked out
// elsewhere.
//
// The cleanup runs on the way out of splitSurvivors. A killed process leaves
// the registration behind; `git worktree prune` clears it, and the directory is
// under the system temp dir, where it is nobody's working copy.
func baseWorktree(base string) (string, func(), error) {
	parent, err := os.MkdirTemp("", "guard-base-")
	if err != nil {
		return "", nil, err
	}
	root := filepath.Join(parent, "tree")
	if out, err := exec.Command("git", "worktree", "add", "--detach", root, base).CombinedOutput(); err != nil {
		_ = os.RemoveAll(parent)
		return "", nil, fmt.Errorf("checking %s out into a worktree: %v: %s",
			base, err, strings.TrimSpace(string(out)))
	}
	return root, func() {
		_ = exec.Command("git", "worktree", "remove", "--force", root).Run()
		_ = os.RemoveAll(parent)
	}, nil
}

// lastLines trims output to its tail, which is where a Go test failure says
// what went wrong.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return "  …\n" + strings.Join(lines[len(lines)-n:], "\n")
}

// reportPreExisting prints the survivors the base tree already had.
//
// They are printed rather than dropped. They are still holes, and the verdict
// each one earned is still the thing that says what would fix it; what changed
// is only whose hole it is.
func reportPreExisting(gs []guardcheck.Guard, verdicts map[guardcheck.Guard]string, base string) {
	if len(gs) == 0 {
		return
	}
	fmt.Printf("\nThese survived here AND in %s, where the same condition — same package,\n"+
		"same function, same text — was neutralised and survived too. They are not this\n"+
		"change's doing and do not fail it. Read them anyway: where one function holds\n"+
		"several identical conditions the pairing is by count, so this says one of them\n"+
		"was already unobserved, not that this one was; and a condition whose text is\n"+
		"unchanged but whose meaning is not reads as pre-existing here:\n", base)
	for _, g := range gs {
		fmt.Printf("  %-10s %s:%d  %s\n", verdicts[g], g.File, g.Line, short(g.Src))
	}
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
//
// dir is the tree to run in, empty for this one. It is not empty when the
// reviewer is asking the base tree whether it had a survivor already: the same
// neutralisation, the same bound, run against the checkout the change started
// from.
func runTests(dir string, pkgs []string, limit time.Duration, neutralising bool) (string, bool) {
	args := append([]string{"test", "-count=1", "-timeout", limit.String()}, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	// Declare the neutralisation to the suite being run. A tree with a
	// `false &&` in it is exactly what guardcheck.GC6 refuses, and without
	// this the reviewer fails that test on every guard — which makes every
	// mutant look caught and the whole verdict worthless.
	//
	// It is set only while a guard is neutralised. Neither baseline sets it —
	// not this tree's and not the base tree's — so GC6 still guards both trees
	// the reviewer reasons from, and a `false &&` someone left in the base is a
	// red base baseline rather than a silent certification.
	if neutralising {
		cmd.Env = append(cmd.Env, guardcheck.NeutralisingEnv+"=1")
	}
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
