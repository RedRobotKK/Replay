package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// FC-DC, frozen. Hand-written totals for commands and flags drifted in five
// documents at once, disagreeing with the binary and with each other:
//
//	docs/AGENT-SURFACE.md      75 flags across 13 commands
//	docs/TUI-FLAG-SURFACE.md   81 flags across 14 commands
//	docs/README.md             all 80 flags
//	docs/DASHBOARD-DESIGN.md   26 flags
//	docs/guide/commands.md     Twenty-nine commands
//	README.md                  lists all thirty
//
// docs/CLI.md is generated from the binary by scripts/cli-blueprint/gen.py and
// CI fails when it drifts, so a total can be read from exactly one place. Every
// figure above was written by hand somewhere else.
//
// The sharpest of them was docs/guide/commands.md, which stated its count and
// then claimed in the next paragraph that the count "is checked against the
// binary rather than kept by hand: docs/CLI.md ... states its own totals at the
// foot" — while disagreeing with that file. A page asserting it is verified
// against a source it contradicts is worse than one asserting nothing.
//
// This repository had already noticed. docs/design/unwired-3-branches-and-docs.md
// lists two of these exact sentences as findings, and both survived. Recording a
// defect is not fixing it, and nothing in the build was comparing them.
//
// SCOPE, stated because the next reader will otherwise overestimate this.
// It matches a number immediately followed by "flags" or "commands" in prose,
// outside docs/CLI.md and docs/design. It would NOT catch "the flag count is
// seventy-five" spelled out in words, it does not compare any figure against
// the binary, and it says nothing about whether docs/CLI.md itself is current
// — the cli-blueprint CI job owns that. It stops a hand-written total
// returning to the prose a reader is most likely to quote. That is all it
// does.
func TestFCDC_NoHandWrittenCommandOrFlagTotals(t *testing.T) {
	root := filepath.Join("..", "..")

	// docs/CLI.md is generated from the binary and the build fails when it
	// drifts. It is the one place a live total is supposed to live.
	const generated = "docs/CLI.md"

	// docs/design holds dated audit records: measurements taken on a day,
	// about the tree as it stood, and the subject of the document rather than
	// a claim about the binary today. Stripping their figures would delete
	// the evidence they exist to carry.
	//
	// This exemption is the guard's weakest point and it is written here
	// rather than discovered later. Widening an allowlist until a test passes
	// is how a guard stops being one, and a live total misfiled under
	// docs/design would go unchallenged. It is a whole directory because the
	// alternative — naming each audit as it is written — fails open: a new
	// audit that nobody adds to the list turns this test red for the wrong
	// reason, and the fix under time pressure is to widen it anyway.
	const auditDir = "docs/design/"

	pat := regexp.MustCompile(`\b\d{1,4}\s+(?:flags|commands)\b`)

	var found []string
	var scanned int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == generated || strings.HasPrefix(rel, auditDir) {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		scanned++
		for i, line := range strings.Split(string(b), "\n") {
			for _, m := range pat.FindAllString(line, -1) {
				found = append(found, rel+":"+strconv.Itoa(i+1)+"  "+m+"  in: "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// A walk that read nothing passes. These figures live in Markdown, so if
	// the walk stops finding Markdown this test is reporting a clean bill on
	// a tree it never opened.
	if scanned < 50 {
		t.Fatalf("only %d Markdown files were scanned, which is too few for this "+
			"repository: the walk is not reaching the docs and this test proved "+
			"nothing", scanned)
	}

	if len(found) > 0 {
		t.Errorf("a command or flag total is written by hand again:\n  %s\n\n"+
			"Five documents carried five different figures, none matching the "+
			"binary. docs/CLI.md is generated from it and fails the build when it "+
			"drifts — cite that instead of restating a number here.",
			strings.Join(found, "\n  "))
	}
}
