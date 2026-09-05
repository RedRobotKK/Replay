package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Documentation drifts; a binary cannot. This asserts that every command the
// help text lists has a section in the guide, and that the guide's own count
// matches reality.
//
// Written after a documentation pass that added four user-visible surfaces —
// `rules --check-prices`, `cost --share`, the tip line and working per-command
// help — none of which appeared in any document until they were audited for.
func TestGuideCoversEveryCommand(t *testing.T) {
	var out, errb bytes.Buffer
	_ = run([]string{"--help"}, &out, &errb)

	var commands []string
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "replay ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[1]
		// The bare `replay <path>` form, and any continuation line that
		// happens to start with the word.
		if strings.HasPrefix(name, "<") || strings.HasPrefix(name, "-") || name == "replay" {
			continue
		}
		commands = append(commands, name)
	}
	if len(commands) < 10 {
		t.Fatalf("only found %d commands in the help text; the parser is wrong, not the docs", len(commands))
	}

	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "guide", "commands.md"))
	if err != nil {
		t.Skipf("guide not readable from here: %v", err)
	}
	text := string(guide)

	var missing []string
	for _, c := range commands {
		if !strings.Contains(text, "`replay "+c+"`") && !strings.Contains(text, "`replay "+c+" ") {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		t.Errorf("commands the binary offers and the guide never mentions: %v.\n"+
			"The guide is advertised as covering every subcommand, so a gap here is a "+
			"promise broken rather than a nicety missed", missing)
	}
}

// Every flag a subcommand accepts must appear in the command guide.
//
// The sibling test above covers commands, and that is why two flags added on
// 2026-09-05 — `rules --export` and `rules --x402-json` — reached a green suite
// undocumented: a user-visible surface with no documentation is invisible, and
// nothing failed. A flag is as user-visible as a command.
//
// PASS: every flag printed by `replay <cmd> --help` appears somewhere in
// docs/guide/commands.md.
// FAIL: any flag missing, which is a surface a reader cannot discover.
func TestGuideCoversEveryFlag(t *testing.T) {
	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "guide", "commands.md"))
	if err != nil {
		t.Fatalf("the command guide must exist for this to mean anything: %v", err)
	}
	text := string(guide)

	var out, errb bytes.Buffer
	_ = run([]string{"--help"}, &out, &errb)
	var commands []string
	for _, line := range strings.Split(out.String(), "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 2 || f[0] != "replay" || strings.HasPrefix(f[1], "<") || strings.HasPrefix(f[1], "-") {
			continue
		}
		commands = append(commands, f[1])
	}
	if len(commands) < 10 {
		t.Fatalf("parsed only %d commands from --help; the parser has drifted, not the docs", len(commands))
	}

	var missing []string
	seen := map[string]bool{}
	for _, cmd := range commands {
		var h, he bytes.Buffer
		_ = run([]string{cmd, "--help"}, &h, &he)
		for _, line := range strings.Split(h.String()+he.String(), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "-") {
				continue
			}
			flag := strings.Fields(line)[0]
			flag = strings.TrimSuffix(strings.TrimSuffix(flag, ","), "=")
			if len(flag) < 3 || seen[cmd+flag] {
				continue
			}
			seen[cmd+flag] = true
			// Anchored, because the obvious form is not a check.
			// `strings.Contains(text, "--"+bare) || strings.Contains(text,
			// "-"+bare)` reduces to the second clause — "--x" contains "-x" —
			// so it passed for any flag that is a substring of another flag or
			// of any hyphenated word in the prose. A new `--json` would have
			// been considered documented because `--x402-json` appears.
			//
			// A flag is documented when the guide names it as a token: at a
			// word boundary, and not immediately followed by more flag
			// characters.
			bare := strings.TrimLeft(flag, "-")
			documented := regexp.MustCompile(`(^|[^-\w])--?` + regexp.QuoteMeta(bare) + `($|[^-\w])`).MatchString(text)
			if !documented {
				missing = append(missing, cmd+" "+flag)
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("flags with no entry in docs/guide/commands.md:\n  %s",
			strings.Join(missing, "\n  "))
	}
}

// Every document under docs/ must be linked from somewhere.
//
// `docs/architecture/mcp-server.md` was written on 2026-09-05 and linked from
// nothing: not the architecture index, not the docs index, not the README, and
// there was no ADR. 229 lines of reasoning that only the person who wrote it
// could find, which is the same as not having written it — and worse than
// nothing if someone later implements the convenient version of a decision
// that was already thought through and rejected here.
//
// PASS: every .md under docs/ is referenced by at least one other Markdown
// file in the repository.
// FAIL: an orphan, which is a document nobody will read.
func TestNoOrphanedDocuments(t *testing.T) {
	root := filepath.Join("..", "..")
	docs := filepath.Join(root, "docs")

	// Every Markdown file in the repository, as one haystack. Link syntax
	// varies — relative paths, ../ prefixes, anchors — so this looks for the
	// filename as a token rather than trying to resolve each link.
	var corpus strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		// A file cannot vouch for itself: a README that mentions its own name
		// would otherwise make every index self-linking.
		corpus.WriteString("\x00" + path + "\x00")
		corpus.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := corpus.String()

	var orphans []string
	err = filepath.Walk(docs, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		name := filepath.Base(path)
		// An index is reached by its directory, and the ADR template is a
		// stencil rather than a document.
		if name == "README.md" || name == "template.md" {
			return nil
		}
		// Count LINKS, not mentions.
		//
		// The first version of this counted the filename anywhere in the
		// corpus, and passed because a sentence in the index happened to name
		// the file in prose. Prose is not navigation: a reader cannot click
		// it, and a filename in a paragraph is exactly what an orphan looks
		// like on the way to being forgotten. So this looks for Markdown link
		// syntax pointing at the file.
		own, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		link := "](" + name
		elsewhere := strings.Count(text, link) - strings.Count(string(own), link)
		if elsewhere < 1 {
			rel, _ := filepath.Rel(root, path)
			orphans = append(orphans, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) > 0 {
		t.Errorf("documents nothing links to, so nobody will find them:\n  %s\n"+
			"Link each from its section index, or delete it.",
			strings.Join(orphans, "\n  "))
	}
}
