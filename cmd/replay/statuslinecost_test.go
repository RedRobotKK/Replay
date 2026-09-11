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

// `replay statusline --install` told the user, in the install instructions:
//
//	It reads the JSON Claude Code sends on stdin and opens no files, so it
//	costs nothing to run on every render.
//
// Both halves are false, and it is the tool describing its own cost to the
// person deciding whether to run it on every keystroke.
//
// main() calls LoadInstalledRules before it dispatches anything, so every
// subcommand opens ~/.replay/rules.json when one is installed. A statusline
// is re-executed as a fresh process per render, so that is one process
// start and one file open per render, not zero.
//
// The three tests below are deliberately not one grep. A test that only
// scanned the help text for a banned phrase would pass the moment somebody
// reworded the lie, and would prove nothing about the behaviour. So:
//
//	SLC1  proves the file is actually opened, by opening it.
//	SLC2  proves a statusline render reaches that code, from the AST of main.
//	SLC3  freezes the corrected sentence, and is only worth anything because
//	     SLC1 and SLC2 establish the fact it describes.
//
// Fixing the sentence rather than the behaviour is deliberate. The comment
// on LoadInstalledRules says every command must report under the same rules,
// and a statusline prints dollars. Skipping the load to make the old claim
// true would make the statusline the one surface that quotes compiled
// defaults after the operator installed a price document.

// TestSC1 proves LoadInstalledRules opens the file, by making the file
// unreadable-as-rules and catching it saying so. A test that asserted "no
// error" would pass whether or not anything was opened.
func TestSLC1_LoadingInstalledRulesOpensTheFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows

	dir := filepath.Join(home, ".replay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, rulesFileName)
	if err := os.WriteFile(path, []byte("{ this is not a rules document"), 0o644); err != nil {
		t.Fatal(err)
	}

	var errb bytes.Buffer
	LoadInstalledRules(&errb)

	if !strings.Contains(errb.String(), path) {
		t.Fatalf("LoadInstalledRules said nothing about %s.\nIt was handed a "+
			"deliberately broken document at that path; silence means the file "+
			"was never opened, and the whole point of this test is that it is.\n"+
			"got: %q", path, errb.String())
	}
}

// TestSC2 proves a statusline render reaches SC1's code path. It reads the
// AST of main() rather than the text of main.go, so a rename or a reformat
// does not fool it, and it checks the call is UNCONDITIONAL — a call sitting
// inside `if cmd != "statusline"` would still satisfy a grep.
func TestSLC2_EveryCommandLoadsRulesBeforeDispatch(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.FuncDecl); ok && g.Name.Name == "main" && g.Recv == nil {
			fn = g
		}
	}
	if fn == nil {
		t.Fatal("no main() in main.go")
	}

	var loadAt, runAt int
	for i, st := range fn.Body.List {
		// Only top-level statements of main(). Anything nested is conditional
		// by definition and does not establish "on every render".
		ast.Inspect(st, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok {
				switch id.Name {
				case "LoadInstalledRules":
					if _, top := st.(*ast.ExprStmt); top {
						loadAt = i + 1
					}
				case "run":
					runAt = i + 1
				}
			}
			return true
		})
	}
	if loadAt == 0 {
		t.Fatal("main() does not call LoadInstalledRules as a plain top-level " +
			"statement. If the call became conditional, the statusline may no " +
			"longer open the rules file — which would make the OLD help text " +
			"true and this whole test file wrong. Read it before deleting it.")
	}
	if runAt == 0 {
		t.Fatal("main() does not call run()")
	}
	if loadAt > runAt {
		t.Errorf("LoadInstalledRules is called at statement %d, after run() at %d. "+
			"Commands would then print figures from compiled defaults.", loadAt, runAt)
	}
}

// TestSC3 freezes the corrected sentence. On its own this is a grep and
// would be worth little; SLC1 and SLC2 are what make the claim checkable, and
// this stops the old one coming back in a reword.
func TestSLC3_TheInstallHintDoesNotClaimItIsFree(t *testing.T) {
	var out, errb bytes.Buffer
	_ = run([]string{"statusline", "--install"}, &out, &errb)
	text := strings.ToLower(out.String() + errb.String())
	if text == "" {
		t.Fatal("statusline --install printed nothing, so this test read no help " +
			"text at all and cannot have checked it")
	}

	for _, bad := range []string{"opens no files", "costs nothing"} {
		if strings.Contains(text, bad) {
			t.Errorf("the install hint still claims %q.\nmain() calls "+
				"LoadInstalledRules before dispatch (SLC2) and that opens "+
				"~/.replay/rules.json (SC1), once per render, because a "+
				"statusline is a fresh process each time.", bad)
		}
	}
}
