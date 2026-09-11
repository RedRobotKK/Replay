package regression

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Replay tells people what to type next. This file checks that what it tells
// them to type exists.
//
// The defect that motivated it shipped and fired on a real machine: `replay
// cost` prints
//
//	One session was 29% of everything you spent: facfd32e, $1056.14 of $3688.82.
//	  replay why facfd32e   shows what filled it.
//
// whenever one session dominates spend — which is to say at the single moment a
// reader is most likely to follow the instruction. There is no `why` command.
// It is a TUI screen label that escaped into CLI output, and nothing noticed
// because nothing was looking.
//
// A second instance of the same class shipped beside it: `replay advise
// --guards` recommends `--spend-session-usd` and `--spend-session-tokens`,
// while `serve` defines `-max-session-usd` and `-max-session-tokens`. Copying
// the tool's own recommendation fails. Its unit test pinned the wrong names
// too, so the test agreed with the defect.
//
// Both are the same failure: a string that names part of the CLI, written by
// hand, never compared against the CLI. These two tests do the comparison. They
// are deliberately source-scanning rather than output-scanning, because a check
// that only reads the output of the commands it happens to run cannot see the
// branch that fires once a quarter — and both defects here live in exactly such
// a branch.

// commandRef finds "replay <word>" in a line of source.
var commandRef = regexp.MustCompile(`\breplay ([a-z][a-z0-9-]*)`)

// flagRef finds "--flag-name" in a line of source. Two characters minimum
// after the dashes, so it cannot match the "--" that ends a flag list.
var flagRef = regexp.MustCompile(`--([a-z][a-z0-9-]{2,})`)

// prose is every word that follows "replay" in an English sentence rather than
// in an instruction. Each is a real sentence in the source, and the list is
// short because the tool mostly says "replay" in one of two registers.
//
// A word that is neither a command nor listed here fails the test, which is the
// point: the failure mode being guarded is a NEW hand-written command name, and
// a new word arriving is exactly the signal.
var prose = map[string]string{
	"is":         `"Replay is answering there", "replay is free"`,
	"can":        `"what replay can see"`,
	"at":         `"replay at" in a sentence about paths`,
	"holds":      `"the rules replay holds"`,
	"never":      `"replay never fetches"`,
	"recommends": `"what replay recommends"`,
	"reports":    `"what replay reports"`,
	"engine":     `"the replay engine"`,
	"priced":     `"of everything replay priced", the outlier note's denominator`,
}

// notOurs is every "--flag" the tool prints that is deliberately not one of its
// own. Printing another program's flag is legitimate; printing your own flag
// under the wrong name is the defect.
var notOurs = map[string]string{
	"unix-socket":   "curl's flag, in the line that shows how to reach a unix socket",
	"corpus-opt-in": "install.sh's flag, not the binary's",
	"help":          "handled by wantsHelp before any FlagSet is built",
	"dry-run":       "defined on several subcommands; see the FlagSet scan",
}

// goSources returns every non-test Go file under the given directories.
func goSources(t *testing.T, root string, dirs ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, d := range dirs {
		err := filepath.WalkDir(filepath.Join(root, d), func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			out[filepath.ToSlash(rel)] = string(b)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", d, err)
		}
	}
	if len(out) == 0 {
		t.Fatal("found no Go sources; the walk is broken, not the tree")
	}
	return out
}

// dispatchCommands reads the subcommand names out of the dispatch switch
// itself, rather than repeating them here.
//
// A hardcoded list would have to be updated by the same person who adds a
// command, which is the person who has just proved they update one list at a
// time. Reading the switch means a new command is known to this test the moment
// it is dispatchable.
func dispatchCommands(t *testing.T, root string) map[string]bool {
	t.Helper()
	path := filepath.Join(root, "cmd", "replay", "main.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	out := map[string]bool{}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "dispatch" {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			cl, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, e := range cl.List {
				lit, ok := e.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				out[strings.Trim(strings.TrimLeft(lit.Value, `"-`), `"`)] = true
			}
			return true
		})
	}
	if len(out) < 20 {
		t.Fatalf("found %d dispatch cases, expected the full command set. The switch "+
			"moved or was renamed, and this test is now checking nothing", len(out))
	}
	return out
}

// definedFlags collects every flag name any FlagSet in the binary defines.
func definedFlags(t *testing.T, srcs map[string]string) map[string]bool {
	t.Helper()
	def := regexp.MustCompile(`\.(?:String|Bool|Int|Int64|Uint|Float64|Duration|Var|StringVar|BoolVar|IntVar|Int64Var|Float64Var|DurationVar)\(\s*(?:&[\w.]+\s*,\s*)?"([a-z][a-z0-9-]*)"`)
	out := map[string]bool{}
	for _, src := range srcs {
		for _, m := range def.FindAllStringSubmatch(src, -1) {
			out[m[1]] = true
		}
	}
	if len(out) < 50 {
		t.Fatalf("found %d defined flags, expected most of the flag surface. The "+
			"pattern stopped matching and this test is now checking nothing", len(out))
	}
	return out
}

// codeLines yields the lines of a source file that are not whole-line comments.
//
// A comment may legitimately name a command that does not exist — a record of a
// defect, a plan, a rejected option — and treating prose about the CLI as an
// instruction to the user would make this test fail for the wrong reason.
func codeLines(src string) []struct {
	n    int
	text string
} {
	var out []struct {
		n    int
		text string
	}
	for i, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		out = append(out, struct {
			n    int
			text string
		}{i + 1, l})
	}
	return out
}

// PC1: every command Replay tells a reader to run is a command Replay runs.
func TestEveryPrintedCommandDispatches(t *testing.T) {
	root := repoRoot(t)
	cmds := dispatchCommands(t, root)
	srcs := goSources(t, root, "cmd", "internal")

	var bad []string
	for _, file := range sortedKeys(srcs) {
		for _, l := range codeLines(srcs[file]) {
			for _, m := range commandRef.FindAllStringSubmatch(l.text, -1) {
				w := m[1]
				if cmds[w] || prose[w] != "" {
					continue
				}
				bad = append(bad, file+":"+itoa(l.n)+"  replay "+w)
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("Replay prints %d command(s) it cannot run:\n  %s\n\n"+
			"Either dispatch the command, print one that exists, or — if this is a "+
			"sentence rather than an instruction — add the word to the prose map "+
			"above with the sentence it appears in.",
			len(bad), strings.Join(bad, "\n  "))
	}
}

// PC2: and every flag it recommends is a flag something defines.
func TestEveryPrintedFlagIsDefined(t *testing.T) {
	root := repoRoot(t)
	srcs := goSources(t, root, "cmd")
	flags := definedFlags(t, srcs)

	var bad []string
	for _, file := range sortedKeys(srcs) {
		for _, l := range codeLines(srcs[file]) {
			if !strings.Contains(l.text, `"`) {
				continue
			}
			for _, m := range flagRef.FindAllStringSubmatch(l.text, -1) {
				f := m[1]
				if flags[f] || notOurs[f] != "" {
					continue
				}
				bad = append(bad, file+":"+itoa(l.n)+"  --"+f)
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("Replay prints %d flag(s) nothing defines:\n  %s\n\n"+
			"A reader who copies one of these gets an error from the tool that "+
			"recommended it. Correct the name, or — if the flag belongs to another "+
			"program — add it to the notOurs map with whose flag it is.",
			len(bad), strings.Join(bad, "\n  "))
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
