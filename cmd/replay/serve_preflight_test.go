package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// The pre-flight guard exists and could not be reached.
//
// internal/proxy/preflight.go has had the policy, the estimate band and the
// refusal kind since it was written, and internal/proxy/preflight_test.go
// exercises all of it by calling s.preFlight directly. What nothing supplied
// was consent: proxy.Config.PreFlight was never assigned in serve.go, so
// analysis.PolicyState arrived zero-valued, OptInActive was false, and
// EvaluatePreFlightPolicy returned on its first line in every shipped run.
// Eight passing tests and a guard that could not fire, which is the shape
// ADR-0014 exists to catch.
//
// The operator's ceiling is the consent. That is not a convenience: with
// OptInActive true and no ceiling, a positive deficit is compared against zero
// and every changed prefix is refused, at any magnitude, while the straddle
// band that would otherwise warn instead can never contain zero. Measured
// before this flag existed: a 1-token deficit refused against a 0-token
// ceiling. So one flag carries both facts, which is also how every other guard
// in this command is spelled.

// SPF1: no flag leaves the guard exactly as inert as it was.
//
// This is the safety invariant. A default that refuses anything would be an
// ADR-0011 breach, because refusing without consent is the thing consent is
// for.
func TestSPF1_NoFlagLeavesPreFlightInert(t *testing.T) {
	p, err := preFlightFromFlag(0)
	if err != nil {
		t.Fatalf("the default must not be an error: %v", err)
	}
	if p.OptInActive {
		t.Error("pre-flight is active with no --preflight flag")
	}
	if p.CeilingTokens != 0 {
		t.Errorf("CeilingTokens = %d with no flag, want 0", p.CeilingTokens)
	}
	// Stated through the policy rather than the struct, because the struct
	// being zero is only interesting if the policy then declines to refuse.
	d := analysis.NewPreFlightDeficit(5_000_000, true, false)
	if d.WouldRefuse(p) {
		t.Error("the default configuration refuses a diverged request; consent was never given")
	}
}

// SPF2: the operator's number becomes the ceiling and turns the guard on.
func TestSPF2_TheFlagSuppliesBothTheCeilingAndTheConsent(t *testing.T) {
	p, err := preFlightFromFlag(1000)
	if err != nil {
		t.Fatalf("--preflight 1000: %v", err)
	}
	if !p.OptInActive {
		t.Error("an explicit ceiling did not activate the policy")
	}
	if p.CeilingTokens != 1000 {
		t.Errorf("CeilingTokens = %d, want 1000", p.CeilingTokens)
	}
}

// SPF3: the number the operator chose is the number the policy compares
// against.
//
// Parsing a value and then comparing against a different one would satisfy
// SPF2 and still be wrong, so this drives the real policy at two points that
// straddle the supplied ceiling and nothing else.
func TestSPF3_TheOperatorsCeilingIsTheOneThePolicyUses(t *testing.T) {
	p, err := preFlightFromFlag(50_000)
	if err != nil {
		t.Fatal(err)
	}
	// Well clear of the ceiling and clear of the estimate's error band.
	over := analysis.NewPreFlightDeficit(200_000, true, false)
	if !over.WouldRefuse(p) {
		t.Errorf("a 200,000-token deficit passed a 50,000-token ceiling")
	}
	// Equally clear on the other side. A ceiling is the largest deficit the
	// operator accepts, so a smaller one is not refused.
	under := analysis.NewPreFlightDeficit(10_000, true, false)
	if under.WouldRefuse(p) {
		t.Errorf("a 10,000-token deficit was refused by a 50,000-token ceiling")
	}
}

// SPF4: a different ceiling decides differently, on the same input.
//
// Without this, an implementation that hard-coded any single ceiling would
// pass SPF3 for the one deficit it was written against.
func TestSPF4_ADifferentCeilingDecidesDifferently(t *testing.T) {
	low, err := preFlightFromFlag(10_000)
	if err != nil {
		t.Fatal(err)
	}
	high, err := preFlightFromFlag(500_000)
	if err != nil {
		t.Fatal(err)
	}
	d := analysis.NewPreFlightDeficit(200_000, true, false)
	if !d.WouldRefuse(low) {
		t.Error("200,000 tokens was not refused by a 10,000-token ceiling")
	}
	if d.WouldRefuse(high) {
		t.Error("200,000 tokens was refused by a 500,000-token ceiling")
	}
}

// SPF5: a negative ceiling is refused rather than coerced.
//
// A negative value can only be a mistake, and silently treating it as off
// would tell an operator the guard is on when it is not.
func TestSPF5_ANegativeCeilingIsRejected(t *testing.T) {
	p, err := preFlightFromFlag(-1)
	if err == nil {
		t.Fatalf("--preflight -1 was accepted, giving %+v", p)
	}
	if p.OptInActive {
		t.Error("a rejected value still activated the policy")
	}
	if !strings.Contains(err.Error(), "preflight") {
		t.Errorf("the error does not name the flag: %v", err)
	}
}

// SPF6: the flag exists, takes a value, and refuses a malformed one.
//
// The flag package owns this, so the assertion is that the flag is declared as
// a value-taking flag at all rather than that the parser works.
func TestSPF6_TheFlagIsDeclaredAndTakesAValue(t *testing.T) {
	var out, errb strings.Builder
	_ = runServe([]string{"--preflight"}, &out, &errb)
	if !strings.Contains(errb.String(), "preflight") {
		t.Errorf("--preflight with no value did not produce a flag error:\n%s", errb.String())
	}

	errb.Reset()
	_ = runServe([]string{"--preflight", "not-a-number"}, &out, &errb)
	if !strings.Contains(errb.String(), "preflight") {
		t.Errorf("a malformed --preflight value did not produce a flag error:\n%s", errb.String())
	}
}

// SPF7: what the helper computes reaches proxy.New.
//
// This is the join serve_wiring_test.go was written for, applied to the field
// that was missing. It is a source check and that is the known limit: it
// proves the value is passed, not that the server then uses it. The proxy
// package owns the second half and already covers it, in
// TestPreFlight_RefusesAChangedPrefixOverTheCeiling, which drives the real
// handler with the same PolicyState shape this helper produces and asserts the
// request is answered with refusalPreFlight.
func TestSPF7_ThePreFlightPolicyReachesTheProxy(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "serve.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing serve.go: %v", err)
	}

	// The name preFlightFromFlag's result is assigned to, so a rename cannot
	// make this pass by looking for a string nobody uses.
	var produced string
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
		if !ok || id.Name != "preFlightFromFlag" {
			return true
		}
		for _, lhs := range as.Lhs {
			if name, ok := lhs.(*ast.Ident); ok && name.Name != "_" && name.Name != "err" {
				produced = name.Name
			}
		}
		return true
	})
	if produced == "" {
		t.Fatal("serve.go never assigns the result of preFlightFromFlag; the guard cannot be " +
			"reached however well it is tested")
	}

	var passed bool
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Config" {
			return true
		}
		for _, e := range lit.Elts {
			kv, ok := e.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "PreFlight" {
				continue
			}
			if v, ok := kv.Value.(*ast.Ident); ok && v.Name == produced {
				passed = true
			}
		}
		return true
	})
	if !passed {
		t.Errorf("proxy.Config does not carry PreFlight: %s. The operator sets a ceiling on "+
			"the command line and the server never hears about it", produced)
	}
}

// SPF8: the policy itself is untouched.
//
// This package wires an existing decision into production. If the arithmetic
// moved, the wiring would be the wrong thing to be looking at.
func TestSPF8_PolicyArithmeticIsUnchanged(t *testing.T) {
	// Straight from analysis: refusal needs consent, divergence, and a
	// deficit strictly greater than the ceiling.
	cases := []struct {
		diverged   bool
		tokens     int64
		ceiling    int64
		optIn      bool
		wantRefuse bool
	}{
		{true, 100, 50, true, true},
		{true, 50, 50, true, false},   // exactly on the ceiling passes
		{false, 100, 50, true, false}, // an unchanged prefix costs the read rate
		{true, 100, 50, false, false}, // no consent, no refusal
	}
	for _, c := range cases {
		if got := analysis.EvaluatePreFlightPolicy(c.diverged, c.tokens, c.ceiling, c.optIn); got != c.wantRefuse {
			t.Errorf("EvaluatePreFlightPolicy(%v, %d, %d, %v) = %v, want %v",
				c.diverged, c.tokens, c.ceiling, c.optIn, got, c.wantRefuse)
		}
	}
}
