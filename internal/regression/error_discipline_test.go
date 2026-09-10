package regression

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ED-1: an error turned into "nothing here" carries a stated reason.
//
// This repository discards errors deliberately and often, and that is correct:
// a file that will not parse is counted nowhere, a stat that fails is not the
// same as a file that is absent, a write error matters more than the close
// error that follows it. Every one of those is a decision, and the ones already
// in the tree say so in a comment beside them.
//
// The ones that do not say so are how a command comes to make a false claim. An
// audit on 2026-09-10 found the shape in the two places it costs most:
//
//   - cmd/replay/purge.go skipped a ledger file it could not read, then printed
//     "removed N record(s)" or "Nothing to remove". A subject asking for erasure
//     was told it completed while their records were still on disk.
//   - cmd/replay/stores.go returned nil when ~/.replay could not be READ, and
//     privacy.go then printed "Replay has written nothing to this machine" —
//     an unreadable store reported as a nonexistent one, in the command that
//     answers "what do you hold about me".
//
// Both are ADR-0018: absence, zero and unknown are three values. A swallow
// collapses the third into the first, and the comment is what proves somebody
// decided which one they meant.
//
// PASS: every `if err != nil { return nil }` and `{ continue }` in a walk, and
// every `_ = <call>`, has a comment within three lines above it.
// FAIL: one does not, and nothing in the tree says which of the three values
// the caller intended.
func TestED1_ADiscardedErrorSaysWhy(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	type finding struct{ pos, src string }
	var undocumented []finding

	for _, dir := range []string{"cmd/replay", "internal/analysis", "internal/proxy", "internal/ledger", "internal/observation", "internal/transcript"} {
		full := filepath.Join(root, filepath.FromSlash(dir))
		pkgs, err := parser.ParseDir(fset, full, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", dir, err)
		}
		for _, p := range pkgs {
			for name, f := range p.Files {
				body, rerr := os.ReadFile(name)
				if rerr != nil {
					t.Fatalf("reading %s: %v", name, rerr)
				}
				lines := strings.Split(string(body), "\n")
				rel, _ := filepath.Rel(root, name)
				rel = filepath.ToSlash(rel)

				ast.Inspect(f, func(n ast.Node) bool {
					ifs, ok := n.(*ast.IfStmt)
					if !ok || ifs.Body == nil || len(ifs.Body.List) != 1 {
						return true
					}
					// `if err != nil { return nil }` or `{ continue }`.
					if !strings.Contains(exprText(fset, ifs.Cond), "err != nil") {
						return true
					}
					swallow := false
					switch s := ifs.Body.List[0].(type) {
					case *ast.ReturnStmt:
						if len(s.Results) == 1 {
							if id, ok := s.Results[0].(*ast.Ident); ok && id.Name == "nil" {
								swallow = true
							}
						}
					case *ast.BranchStmt:
						if s.Tok == token.CONTINUE {
							swallow = true
						}
					}
					if !swallow {
						return true
					}
					line := fset.Position(ifs.Pos()).Line
					if commentedAbove(lines, line) {
						return true
					}
					undocumented = append(undocumented, finding{
						pos: rel + ":" + itoa(line),
						src: strings.TrimSpace(lines[line-1]),
					})
					return true
				})
			}
		}
	}

	// Anti-vacuity: the tree is full of these, documented. Finding none at all
	// means the walk stopped working, not that the code is clean.
	if len(undocumented) == 0 {
		t.Fatal("no discards found at all; the walk is not reading the tree and this " +
			"check asserts nothing")
	}

	// Enforced now: the commands that answer a question ABOUT the reader.
	//
	// A swallow here becomes a false claim rather than a lost diagnostic.
	// `purge` reported an erasure over files it could not read; `privacy`
	// reported an unreadable ~/.replay as an empty one; `cost` and the
	// contribution path turn a skipped transcript into a published figure with
	// no note that anything was skipped.
	enforced := []string{
		"cmd/replay/purge.go", "cmd/replay/privacy.go", "cmd/replay/stores.go",
		"cmd/replay/contribute.go", "internal/observation/",
	}
	// Everything else is a RATCHET rather than an exemption.
	//
	// Thirty-three were found on 2026-09-10 and the two that made false claims
	// were fixed the same day. The rest are real and unreviewed: each needs a
	// judgement about which of absence, zero and unknown its author meant, and
	// making thirty of those at speed is how a wrong reason gets written down
	// and believed. The ceiling is what stops them being forgotten — it may
	// fall and must never rise, so a new swallow cannot hide among them.
	const ceiling = 29

	outside := 0
	for _, f := range undocumented {
		inScope := false
		for _, e := range enforced {
			if strings.HasPrefix(f.pos, e) {
				inScope = true
			}
		}
		if !inScope {
			outside++
			continue
		}
		t.Errorf("%s discards an error into \"nothing here\" with no stated reason:\n"+
			"      %s\n"+
			"      Absence, zero and unknown are three values (ADR-0018). Say which one\n"+
			"      this means, or return the error. A swallow the tree does not explain\n"+
			"      is how `purge` came to report an erasure it had not performed.", f.pos, f.src)
	}

	if outside > ceiling {
		t.Errorf("%d undocumented discards outside the enforced paths, ceiling is %d.\n"+
			"      The ceiling only falls. A new one may not hide among the ones already\n"+
			"      here: document it, or fix it, or lower nothing and explain why.", outside, ceiling)
	}
	if outside < ceiling {
		t.Errorf("%d undocumented discards outside the enforced paths, ceiling is %d.\n"+
			"      Lower the ceiling to %d in the same commit that fixed one, so the\n"+
			"      ratchet keeps its grip.", outside, ceiling, outside)
	}
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

func exprText(fset *token.FileSet, e ast.Expr) string {
	var sb strings.Builder
	ast.Inspect(e, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			sb.WriteString(v.Name + " ")
		case *ast.BinaryExpr:
			sb.WriteString(v.Op.String() + " ")
		}
		return true
	})
	s := sb.String()
	if strings.Contains(s, "err ") && strings.Contains(s, "!= ") && strings.Contains(s, "nil ") {
		return "err != nil"
	}
	return s
}

// commentedAbove reports whether a comment sits within three lines above.
func commentedAbove(lines []string, line int) bool {
	for i := line - 2; i >= 0 && i >= line-4; i-- {
		if i >= len(lines) {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "//") {
			return true
		}
	}
	return false
}
