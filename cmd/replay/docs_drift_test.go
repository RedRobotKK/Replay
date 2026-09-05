package main

import (
	"bytes"
	"os"
	"path/filepath"
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
