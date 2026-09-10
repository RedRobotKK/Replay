package regression

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Packages that exist but are not in the shipped binary.
//
// Nine capabilities have now been found built, tested, and unreachable by any
// user: LiveScreen, five TUI screens, PoolFits' second half, forecast.go,
// internal/quota, saveQuota, preflight.go and internal/usage. That is not a run
// of bad luck. It is one gap, appearing repeatedly, between "written and tested"
// and "reachable".
//
// The reason it survived so long is worth stating, because it is the thing this
// test exists to break: **a passing test proves the function works. It does not
// prove anything calls it.** internal/proxy/preflight.go has eight tests and
// cannot fire in production, because Config.PreFlight is never assigned.
// internal/usage had tests and zero importers, while the double-count it guards
// against sat live in transcript/codex.go:151. That one is now fixed in the
// reader, and UNWIRED-LOG.md #8 records why wiring the package would not have
// caught it: Validate's oracle was derived from the value it checked.
//
// So this compares the package list against the binary's dependency closure. A
// package absent from that closure ships no behaviour, whatever its coverage
// says.
//
// It is a baseline rather than a blanket ban, following
// frozen_defects_test.go: the four known-absent packages are listed with the
// reason each is out, and the test fails when a NEW one appears or when a
// listed one is quietly deleted rather than fixed. Removing an entry requires
// wiring the package, which is the point.
func TestNoNewlyUnwiredPackages(t *testing.T) {
	// Every entry needs a reason, and "not got round to it" is a valid one as
	// long as it is written down and linked. UNWIRED-LOG.md tracks the fix.
	known := map[string]string{
		"internal/regression":        "this package. Test-only by design, and correctly absent.",
		"scripts/x402-e2e":           "an end-to-end tool built separately, not part of the binary.",
		"scripts/guard-reachability": "a developer tool carrying //go:build ignore, run by CI against a pull request diff. Correctly absent: it neutralises the binary's conditionals, so being part of the binary would be the defect.",
		"internal/quota":             "OPEN. The only consumer of ledger Record.Quota, with forecast.go inside it. docs/design/UNWIRED-LOG.md #5.",
		"internal/usage":             "OPEN. Still zero importers, but the double-count it was written for is fixed at source and asserted across the package boundary by internal/usage/codex_normalises_test.go. Wiring it would NOT have closed #8: Validate compares a sum against the same sum, so it returned nil for the very defect it is named the guard against. UNWIRED-LOG.md #8.",
		"internal/otlp":              "OPEN, may be by design — it writes spans to a file and nothing yet asks it to. UNWIRED-LOG.md #9.",
		"internal/feed":              "OPEN, may be by design — the rules feed is served from the site, not the binary. UNWIRED-LOG.md #9.",
	}

	all, inBinary := reachable(t)

	// The graph must not be empty, or this test passes by measuring nothing —
	// exactly the failure it exists to catch, wearing its own hat.
	if len(all) < 10 || len(inBinary) < 5 {
		t.Fatalf("found %d packages and %d reachable from cmd/replay; the walk is "+
			"not working and this test proves nothing", len(all), len(inBinary))
	}
	if !inBinary["cmd/replay"] || !inBinary["internal/proxy"] {
		t.Fatalf("cmd/replay does not reach internal/proxy, which it certainly does; " +
			"the import graph is wrong and every result below is meaningless")
	}

	var unwired []string
	for _, p := range all {
		if !inBinary[p] {
			unwired = append(unwired, p)
		}
	}
	sort.Strings(unwired)

	for _, p := range unwired {
		if _, ok := known[p]; !ok {
			t.Errorf("%s is not reachable from cmd/replay, and is not a known case.\n"+
				"      A package absent from the binary's dependency closure ships no\n"+
				"      behaviour to anyone, whatever its tests report. Either wire it,\n"+
				"      or add it to `known` with the reason and a line in\n"+
				"      docs/design/UNWIRED-LOG.md.", p)
		}
	}

	// The other direction. An entry that is no longer unwired should be removed
	// rather than left to rot: a stale allowlist is how the next one hides.
	have := map[string]bool{}
	for _, p := range unwired {
		have[p] = true
	}
	for p := range known {
		if !have[p] {
			t.Errorf("%s is listed as unwired but is now in the binary. Delete the entry "+
				"and mark it WIRED in docs/design/UNWIRED-LOG.md — a stale allowlist is "+
				"where the next unwired package will hide.", p)
		}
	}
}

// reachable returns every repository package the binary's entry point can
// reach, and every package that exists, by parsing imports.
//
// It parses rather than shelling out to `go list`, and that is not a
// preference. TestX402_ExecIsConfinedToTheMutationHarness requires any file
// importing os/exec to carry the `mutation` build tag, because a binary people
// pipe from curl onto machines holding provider credentials must not be able to
// start processes. Tagging this file would keep it out of the normal CI run,
// which is the one place it needs to be. So the graph is built here instead.
//
// Test files are excluded deliberately. A package reached only from a _test.go
// file ships nothing to a user, which is the entire distinction this check
// exists to draw.
func reachable(t *testing.T) (all []string, inBinary map[string]bool) {
	t.Helper()
	root := repoRoot(t)
	const mod = "github.com/RedRobotKK/Replay"

	imports := map[string][]string{}
	seen := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, filepath.Dir(path))
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		seen[rel] = true

		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, im := range f.Imports {
			ip, uerr := strconv.Unquote(im.Path.Value)
			if uerr != nil || !strings.HasPrefix(ip, mod) {
				continue
			}
			dep := strings.TrimPrefix(strings.TrimPrefix(ip, mod), "/")
			if dep != "" {
				imports[rel] = append(imports[rel], dep)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	inBinary = map[string]bool{}
	var walk func(string)
	walk = func(p string) {
		if inBinary[p] {
			return
		}
		inBinary[p] = true
		for _, dep := range imports[p] {
			walk(dep)
		}
	}
	walk("cmd/replay")

	for p := range seen {
		all = append(all, p)
	}
	sort.Strings(all)
	return all, inBinary
}
