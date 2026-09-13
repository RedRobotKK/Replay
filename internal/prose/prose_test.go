package prose

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

// No em-dash in anything the binary prints.
//
// WHY THIS IS A TEST AND NOT A PREFERENCE.
//
// The site repository has carried this rule since 2026-09-14 and enforces it in
// tests/prose.test.mjs. The binary did not, and the binary is the surface most
// readers meet first: a person who installs Replay reads its output long before
// they read a web page. A rule enforced on one surface and trusted on the other
// is a rule that holds until someone is in a hurry.
//
// The character is the most reliable tell of machine-written prose, and this
// project's entire argument is that a person measured these things. Losing that
// argument to a punctuation mark would be a cheap way to lose it.
//
// SCOPE, AND WHAT IS DELIBERATELY OUT OF IT.
//
// String literals only, in non-test files. Comments are not checked: they are
// read by people working on the code, not printed to anyone, and sweeping
// several hundred of them would be a large diff that changes nothing a user
// sees. Test files are not checked either, for the same reason plus one more:
// a test that asserts on the old text needs to be able to quote it.
//
// The en-dash is included. It is the same tell wearing a narrower glyph, and a
// rule that banned one character would be satisfied by typing the other.

// dashes are the characters that may not appear in printed output. ASCII
// hyphen-minus is fine and is the intended replacement, along with rewriting
// the sentence, which is usually better.
var dashes = map[rune]string{
	'—': "em-dash",
	'–': "en-dash",
}

// skipDirs are trees that are not this repository's own source.
//
// .claude/worktrees holds checkouts other agents are working in. Scanning them
// reported 260 findings across 186 files, which is this repository's 15 seen
// seventeen times over, and a count that moves when somebody else's branch
// moves is not a measurement of anything here.
var skipDirs = map[string]bool{
	".claude":  true,
	".git":     true,
	"vendor":   true,
	"testdata": true,
}

func TestNoEmDashInAnythingTheBinaryPrints(t *testing.T) {
	root := repoRoot(t)
	var found []string

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			// A file this repository cannot parse is a separate problem and the
			// compiler will say so. Reporting it here as a prose finding would
			// name the wrong defect.
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				v = lit.Value
			}
			for r, name := range dashes {
				if strings.ContainsRune(v, r) {
					found = append(found, rel+":"+
						strconv.Itoa(fset.Position(lit.Pos()).Line)+"  "+name+"  "+
						excerpt(v, r))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	sort.Strings(found)
	if len(found) > 0 {
		t.Errorf("%d printed string(s) contain a dash character that reads as machine prose.\n"+
			"Rewrite the sentence, or use a comma, a colon or a plain hyphen:\n  %s",
			len(found), strings.Join(found, "\n  "))
	}
}

// excerpt shows the offending text around the character, on one line, so a
// failure names the sentence rather than a file and a number.
func excerpt(s string, r rune) string {
	s = strings.ReplaceAll(s, "\n", " ")
	i := strings.IndexRune(s, r)
	lo, hi := i-40, i+40
	if lo < 0 {
		lo = 0
	}
	if hi > len(s) {
		hi = len(s)
	}
	return strings.TrimSpace(s[lo:hi])
}

// repoRoot walks up from this package to the directory holding go.mod.
//
// Derived rather than hardcoded as "../..", so moving this package does not
// silently narrow the scan to a subtree and turn the check green by accident.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory; the scan would cover nothing")
		}
		dir = parent
	}
}
