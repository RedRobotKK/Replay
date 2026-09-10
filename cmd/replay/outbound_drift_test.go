package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every package that can originate an outbound request must be named in the
// surface inventory.
//
// This exists because the claim drifted without anyone editing it. The README
// said the binary makes no network request except the proxy and one you type,
// and docs/SURFACES.md called itself the exhaustive outbound inventory. Both
// were true when written. Then `replay probe` shipped — deliberately, with its
// own tests saying in plain words that it originates billable requests on the
// operator's credential — and neither document was touched. The sentence became
// false by addition elsewhere, which is the one kind of documentation error no
// amount of care in the edit catches.
//
// So the list is derived from the code rather than typed. A new package that
// can reach the network fails this test until somebody writes down what it
// reaches and when.
//
// PASS: every outbound-capable package is accounted for, and each is named in
// the inventory.
// FAIL: a package that can send and is not written down.
func TestOutboundSurfacesAreAllDocumented(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	// Packages known to originate outbound requests, each with the reason it
	// is allowed to. Adding an entry here is a deliberate act that should come
	// with a row in SURFACES.md and a line in the README's telemetry cell.
	known := map[string]string{
		"internal/proxy":  "the proxy itself, forwarding your own traffic to the provider you configured",
		"internal/probe":  "probe --execute, which originates synthetic billable requests on your own key",
		"cmd/replay":      "rules --check-prices fetches a public price table; doctor probes loopback for a running proxy",
		"internal/ledger": "streaming passthrough helpers on the proxy path",
		// `replay upgrade`, and nothing else in this package, reaches
		// github.com. It runs only when the user types the command, which is
		// what the README's promise actually says; a background version check
		// would be the drift this test exists to catch.
		"internal/selfupdate": "replay upgrade, which resolves and downloads a release you asked for",
		// internal/feed fetches a signed rules bundle and its detached
		// signature (feed.go:172). It has no non-test caller yet — the
		// capability is compiled in and nothing types a command that reaches
		// it — and it is written down anyway, because "unwired" is a fact
		// about today's call graph and this list is about what the binary can
		// do.
		//
		// It was NOT in this list until 2026-09-10, and the reason is the
		// whole point of the second check below: it sends with `c.Get(u)` on
		// an *http.Client parameter, which the call-expression matcher cannot
		// see. A package with a live outbound sender sat outside an inventory
		// whose job is to be complete, and the test guarding that inventory
		// reported success.
		"internal/feed": "the signed rules feed: fetches a bundle and its signature, verifies, never signs",
	}

	// Packages that may import the network stack at all. This is the check
	// that does not depend on spelling.
	//
	// Added 2026-09-10, after the matcher below was shown to miss a live
	// sender. Every one of these gets through it:
	//
	//	http.DefaultClient.Get(u)      // receiver is a selector, not an ident
	//	c := &http.Client{}; c.Do(req) // "c.Do" is not in the name list
	//	import h "net/http"; h.Get(u)  // one-word alias defeats the list
	//	(&net.Dialer{}).DialContext()  // not a name in the list either
	//
	// Each is one line, none is exotic, and none is caught by any other test
	// in this repository: cmd/replay/x402_test.go's allowlist permits `net`
	// and `net/http` repo-wide on purpose — its subject is signing, not
	// egress — and internal/observation's and internal/otlp's import bans are
	// scoped to those two packages.
	//
	// So the rule is capability, not spelling, and it is the same argument
	// x402_test.go makes for its own list: a call shape is something you can
	// rename, and an import is a line somebody has to add. You cannot reach
	// the network without one of these imports, so a package holding one is
	// outbound-capable until argued otherwise, and the argument goes here.
	networkCapable := map[string]string{
		"cmd/replay":          "the commands that fetch: rules, doctor, upgrade's driver, burn's Ollama probe",
		"internal/proxy":      "the proxy itself",
		"internal/probe":      "probe --execute",
		"internal/selfupdate": "replay upgrade",
		"internal/feed":       "the signed rules feed",
		// A build-tagged fixture, excluded from every build of the module: it
		// is the mock x402 seller the end-to-end harness runs against.
		"scripts/x402-e2e": "a mock seller for the x402 end-to-end harness; not part of any build of the binary",
	}
	netImports := map[string]bool{"net": true, "net/http": true, "net/url": true}

	// Call expressions that construct or perform an outbound request.
	senders := map[string]bool{
		"http.NewRequest": true, "http.NewRequestWithContext": true,
		"http.Get": true, "http.Post": true, "http.PostForm": true, "http.Head": true,
	}

	found := map[string]bool{}
	capable := map[string]bool{}
	fset := token.NewFileSet()
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			// .claude/worktrees holds git worktrees for parallel agents: full
			// copies of this repo. Walking into one makes every package appear
			// twice and fails this test with paths that are not source at all.
			case ".git", ".claude", "node_modules", "dist", "bin", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		pkg := filepath.ToSlash(filepath.Dir(rel))
		for _, imp := range f.Imports {
			// The PATH, not the local name: an alias changes the name and
			// cannot change the path, which is why this reads the path.
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr == nil && netImports[p] {
				capable[pkg] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if senders[ident.Name+"."+sel.Sel.Name] {
				found[pkg] = true
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}

	if len(found) == 0 {
		t.Fatal("no outbound call sites were found at all, so this test asserts nothing. " +
			"Either the walk is broken or the sender list no longer matches the code.")
	}
	if len(capable) == 0 {
		t.Fatal("no package imports the network stack at all, so the capability check asserts nothing. " +
			"The walk is broken.")
	}

	var unaccounted []string
	for pkg := range capable {
		if _, ok := networkCapable[pkg]; !ok {
			unaccounted = append(unaccounted, pkg)
		}
	}
	sort.Strings(unaccounted)
	if len(unaccounted) > 0 {
		t.Errorf("these packages import the network stack and are not accounted for:\n  %s\n\n"+
			"Importing net, net/http or net/url is the capability; the call that uses it can be spelled "+
			"in ways no matcher enumerates. If the package genuinely cannot send — it parses URLs, or it "+
			"only reads an inbound request — say so in networkCapable above and say why.",
			strings.Join(unaccounted, "\n  "))
	}
	// Stale entries matter as much as missing ones: an inventory that keeps
	// naming a capability the code no longer has trains a reader to discount
	// it. The same reasoning as x402_test.go's stale-exemption check.
	var gone []string
	for pkg := range networkCapable {
		if !capable[pkg] {
			gone = append(gone, pkg)
		}
	}
	sort.Strings(gone)
	if len(gone) > 0 {
		t.Errorf("these packages are listed as network-capable and no longer import the network stack:\n  %s\n\n"+
			"Remove the entry rather than leaving the list describing a capability that is gone.",
			strings.Join(gone, "\n  "))
	}

	var undocumented []string
	for pkg := range found {
		if _, ok := known[pkg]; !ok {
			undocumented = append(undocumented, pkg)
		}
	}
	sort.Strings(undocumented)
	if len(undocumented) > 0 {
		t.Errorf("these packages can originate an outbound request and are not accounted for:\n  %s\n\n"+
			"The README's telemetry cell and docs/SURFACES.md both state what this binary can reach. "+
			"Adding a sender without updating them is how that claim became false once already: "+
			"`probe` shipped on purpose and the sentence saying it could not exist was left in place.",
			strings.Join(undocumented, "\n  "))
	}

	// The inventory must actually name each one, or the README's link to it is
	// a link to a document that does not cover the case.
	surfaces, err := os.ReadFile(filepath.Join(root, "docs", "SURFACES.md"))
	if err != nil {
		t.Fatalf("the surface inventory the README links to is unreadable: %v", err)
	}
	for pkg := range found {
		name := path0(pkg)
		if !strings.Contains(string(surfaces), name) {
			t.Errorf("docs/SURFACES.md never mentions %q, which can originate an outbound request", name)
		}
	}
}

// path0 reduces a package path to the word a document would use for it.
func path0(pkg string) string {
	parts := strings.Split(pkg, "/")
	return parts[len(parts)-1]
}
