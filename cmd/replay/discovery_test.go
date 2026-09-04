package main

import (
	"path/filepath"
	"testing"
)

// Claude Code can be relocated with CLAUDE_CONFIG_DIR. A user who has set it
// and is told "none found" concludes the tool is broken rather than that it
// looked in the wrong place.
func TestClaudeConfigDirHonoursTheEnvironment(t *testing.T) {
	home := "/home/u"
	if got, want := claudeConfigDir(home), filepath.Join(home, ".claude"); got != want {
		t.Fatalf("default: got %q want %q", got, want)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "/opt/claude-data")
	if got := claudeConfigDir(home); got != "/opt/claude-data" {
		t.Fatalf("override: got %q", got)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "   ")
	if got, want := claudeConfigDir(home), filepath.Join(home, ".claude"); got != want {
		t.Fatalf("blank override must fall back: got %q want %q", got, want)
	}
}
