package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contribution is opt-in, and these tests are what makes that a property rather
// than a habit.
//
// internal/consent already refuses everything except the exact affirmative. The
// risk this file addresses is different and it is the one that actually
// happened once in this repository: the consent package existed, was correct,
// and NOTHING CALLED IT. A gate enforced by nobody is documentation.
//
// So there are two kinds of test here. OI1 and OI2 drive the real command
// through every state the consent file can be in. OI3 is structural, and it is
// the one that survives a change nobody thought to add to the table: it parses
// contribute.go and asserts that every function which can write a submission
// also consults the gate.

// noConsentHome is a HOME with a corpus to price and no decision on file.
func noConsentHome(t *testing.T) string {
	t.Helper()
	corpus(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone would leave
	// the command pointed at the developer's real home.
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

// OI1: nothing but the exact affirmative builds a submission.
//
// PASS: every state below refuses, and leaves the output directory empty.
// FAIL: any arrangement of the filesystem that contributes spend on behalf of
// somebody who never said yes.
func TestOI1_OnlyTheExactAffirmativeContributes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		write bool
	}{
		{name: "no file at all", write: false},
		{name: "explicitly declined", body: "corpus_opt_in = false\n", write: true},
		{name: "empty file", body: "", write: true},
		{name: "comments only", body: "# I have not decided yet\n", write: true},
		{name: "contradicts itself", body: "corpus_opt_in = true\ncorpus_opt_in = false\n", write: true},
		{name: "a different key", body: "update_checks = true\n", write: true},
		{name: "truthy but not the sentence", body: "corpus_opt_in = yes\n", write: true},
		{name: "true with a stray suffix", body: "corpus_opt_in = true # sure\n", write: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := noConsentHome(t)
			if tc.write {
				writeConsent(t, home, tc.body)
			}
			out := t.TempDir()
			var stdout, stderr bytes.Buffer
			err := run([]string{"cost", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr)
			if err == nil {
				t.Errorf("a submission was built with consent in state %q", tc.name)
			}
			if entries, _ := os.ReadDir(out); len(entries) != 0 {
				t.Errorf("state %q left %d file(s) behind", tc.name, len(entries))
			}
		})
	}
}

// OI2: the affirmative does permit it.
//
// Without this, OI1 passes just as well against a command that refuses
// everything, which would be a gate that is also a wall. A refusal test with no
// permission test beside it cannot tell the two apart.
func TestOI2_TheAffirmativePermitsIt(t *testing.T) {
	home := noConsentHome(t)
	writeConsent(t, home, "corpus_opt_in = true\n")
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr); err != nil {
		t.Fatalf("an opted-in contributor was refused: %v\n%s", err, stderr.String())
	}
	if entries, _ := os.ReadDir(out); len(entries) != 1 {
		t.Fatalf("want one submission, found %d", len(entries))
	}
}

// OI3: every path that can write a submission consults the gate.
//
// Mechanical rather than enumerated, because the defect this guards against is
// a path that does not exist yet. A third contribution path added next year
// would not be in OI1's table and would not fail it; it will fail this.
//
// The check is deliberately crude — does this function body mention the consent
// reader at all — because a precise one would need to model control flow, and a
// gate that is called but ignored is a different defect with a different test
// (OI1 catches that one, for the paths it knows about).
func TestOI3_EveryWriterConsultsTheGate(t *testing.T) {
	const src = "contribute.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", src, err)
	}

	// The writers in internal/observation. Any function reaching one of these
	// has produced a file a human may go on to send.
	writers := map[string]bool{"WriteCorpus": true, "WriteObservation": true}

	var checked int
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var writes, gates bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				if id, ok := f.X.(*ast.Ident); ok && id.Name == "observation" && writers[f.Sel.Name] {
					writes = true
				}
			case *ast.Ident:
				if f.Name == "readCorpusConsent" {
					gates = true
				}
			}
			return true
		})
		if !writes {
			continue
		}
		checked++
		if !gates {
			t.Errorf("%s writes a submission without reading the corpus consent file; "+
				"contribution must be opt-in on every path, not just the ones with a test",
				fn.Name.Name)
		}
	}
	// The test must be able to fail. If the AST walk matched nothing — a rename
	// in internal/observation, a moved file — it would pass silently while
	// checking nothing at all, which is the exact shape of vacuous test this
	// repository keeps finding.
	if checked < 2 {
		t.Fatalf("found %d submission-writing functions in %s, expected at least 2 "+
			"(contribute and contributeCorpus); this test is not looking at what it thinks "+
			"it is", checked, src)
	}
}

// OI4: the flag is off by default.
//
// The gate only matters if something asks. Nothing should ask unless the
// operator typed the flag, so a contributor who upgrades does not start
// producing submissions because a default moved.
func TestOI4_ContributionIsOffWithoutTheFlag(t *testing.T) {
	home := noConsentHome(t)
	writeConsent(t, home, "corpus_opt_in = true\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost"}, &stdout, &stderr); err != nil {
		t.Fatalf("plain cost failed: %v", err)
	}
	if strings.Contains(stdout.String(), "carries SPEND") {
		t.Error("cost contributed without being asked, on a machine that had merely opted in")
	}
}
