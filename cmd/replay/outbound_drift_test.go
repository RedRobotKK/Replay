package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
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
	}

	// Call expressions that construct or perform an outbound request.
	senders := map[string]bool{
		"http.NewRequest": true, "http.NewRequestWithContext": true,
		"http.Get": true, "http.Post": true, "http.PostForm": true, "http.Head": true,
	}

	found := map[string]bool{}
	fset := token.NewFileSet()
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", "dist", "bin", "testdata":
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
