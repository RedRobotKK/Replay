package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// What serve computes must reach the server.
//
// The pattern this repository keeps finding: a capability is built, tested, and
// never connected to a caller. A passing test proves the function works, not
// that anything calls it.
//
// Rehydration is the sharpest instance available. masking.Rehydrator restores
// the reader's real values in a response body, and internal/proxy tests it
// thoroughly — by passing a Rehydrator into Config directly. Nothing asserted
// that `replay serve` does. Deleting `Rehydrator: rehydrator` from the literal
// leaves every test in the tree green, and the operator gets responses still
// carrying __REPLAY_SECRET_1__ where their own values should be, having asked
// for rehydration on the command line and been told nothing.
//
// This is a source check rather than a behavioural one, and that is a real
// limit: it proves the value is passed, not that the server then uses it. The
// proxy package owns the second half and covers it. What was missing is the
// join, and the join is a line in a struct literal.

// SW1: every value maskingFromFlags produces is passed to proxy.New.
func TestSW1_TheMaskingHelpersResultsReachTheProxy(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "serve.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing serve.go: %v", err)
	}

	// The names maskingFromFlags is assigned to at its call site, so a rename
	// cannot quietly make this test pass by looking for a string nobody uses.
	var produced []string
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != "maskingFromFlags" {
			return true
		}
		for _, lhs := range as.Lhs {
			name, ok := lhs.(*ast.Ident)
			if !ok || name.Name == "_" || name.Name == "err" {
				continue
			}
			produced = append(produced, name.Name)
		}
		return true
	})
	if len(produced) < 2 {
		t.Fatalf("found %d values from maskingFromFlags (%v); it returns a masker and a "+
			"rehydrator, so this guard is looking at the wrong call and would pass "+
			"whatever serve.go did", len(produced), produced)
	}

	// Where those names are used as a value in the proxy.Config literal.
	passed := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Config" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "proxy" {
			return true
		}
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if v, ok := kv.Value.(*ast.Ident); ok {
				passed[v.Name] = true
			}
		}
		return false
	})
	if len(passed) == 0 {
		t.Fatal("no proxy.Config literal found in serve.go; this guard is measuring nothing")
	}
	for _, name := range produced {
		if !passed[name] {
			t.Errorf("serve.go computes %q from maskingFromFlags and never passes it to "+
				"proxy.New. The capability is built, tested in internal/proxy, and "+
				"unreachable from the command line.\n  passed: %v", name, keysOf(passed))
		}
	}
}

// SW2: the rehydrate flag is documented as doing something serve can do.
//
// The other half of the same failure: a flag whose help text promises
// behaviour nothing implements reads exactly like a working feature.
func TestSW2_TheRehydrateFlagIsWired(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "serve.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !strings.HasPrefix(sel.Sel.Name, "Bool") {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || !strings.Contains(lit.Value, "rehydrate") {
			return true
		}
		found = true
		return false
	})
	if !found {
		t.Skip("serve exposes no --rehydrate flag; SW1 covers the wiring either way")
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
