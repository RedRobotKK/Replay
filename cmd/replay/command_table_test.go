package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// aliases are spellings of a command that are not themselves commands, so
// they are not expected to appear in help as their own line or in the guide.
var aliases = map[string]bool{
	"--version": true, "-v": true,
	"--help": true, "-h": true, "help": true,
}

// dispatchedCommands reads the case labels of dispatch() out of the source.
//
// It parses the switch rather than reading a hand-written list, and that is
// the whole point. The guard this replaces derived its command list by
// PARSING replay --help, so a command missing from the help text was invisible
// to it by construction: the one defect it existed to catch was the one shape
// it could not see. Three commands (burn, codex, tui) sat unlisted behind it.
//
// A hand-typed list here would have the same flaw one step further out, since
// nothing would make it track the switch. The source is the only description
// of what commands exist that cannot drift from what commands exist.
func dispatchedCommands(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	var names []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "dispatch" {
			return true
		}
		ast.Inspect(fn, func(m ast.Node) bool {
			cc, ok := m.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, e := range cc.List {
				lit, ok := e.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				s, err := strconv.Unquote(lit.Value)
				if err == nil && !aliases[s] {
					names = append(names, s)
				}
			}
			return true
		})
		return false
	})
	if len(names) < 15 {
		t.Fatalf("found only %d dispatch cases (%v); the parser is wrong, not the code", len(names), names)
	}
	return names
}

// TestCT1: every command the binary dispatches is advertised by --help.
//
// printUsage claims "replay --help  every command". This is that claim,
// stated as a check that fails when it stops being true.
func TestCT1(t *testing.T) {
	var out, errb bytes.Buffer
	_ = run([]string{"--help"}, &out, &errb)
	help := out.String()

	var missing []string
	for _, name := range dispatchedCommands(t) {
		if !strings.Contains(help, "replay "+name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("dispatched but absent from --help, which promises every command: %v", missing)
	}
}

// TestCT2: every dispatched command is in the user guide.
func TestCT2(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "guide", "commands.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Headings only, and stripped of HTML comments first.
	//
	// This used to look for the bare string "replay <name>" anywhere in the
	// guide. Two things satisfied that without documenting anything: a passing
	// mention in prose, and an HTML comment. Verified by neutralising every
	// real occurrence of `replay budget` and leaving one inside
	// <!-- replay budget -->, at which point the command was undocumented and
	// this test still passed.
	//
	// A command is documented when it has a section, not when its name appears.
	// The guide's own convention is `### ` + backticked usage, which is what a
	// reader scrolls to and what the table of contents links.
	guide := stripHTMLComments(string(b))

	var missing []string
	for _, name := range dispatchedCommands(t) {
		// The tool is named for its own first command, so that one is
		// documented under the bare heading `replay` rather than as
		// "replay replay". Both spellings dispatch to it.
		want := "### `replay " + name
		if name == "replay" {
			want = "### `replay`"
		}
		if !strings.Contains(guide, want) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("dispatched but with no section in docs/guide/commands.md: %v\n"+
			"A mention is not documentation: each needs a `### ` heading a reader can "+
			"scroll to.", missing)
	}
}

// stripHTMLComments removes <!-- ... --> so a commented-out heading cannot
// stand in for a live one.
//
// Markdown has no comment syntax of its own, so this is the only way to hide
// text in a .md file — which makes it the only way to satisfy a substring
// check without publishing anything.
func stripHTMLComments(s string) string {
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+j+3:]
	}
}
