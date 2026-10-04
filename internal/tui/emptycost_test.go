package tui

import (
	"strings"
	"testing"
)

// The cost screen the installer opens first. On a machine with no Claude
// Code transcripts it is the only screen a new user sees, and until now it
// offered one note: an environment variable for transcripts kept elsewhere.
// A user who runs another agent has two real next steps, the live proxy and,
// when Codex rollouts are on the machine, the offline reader; the screen
// must name them. Written RED against the v0.7.0 screen.
func TestEmptyCostScreenHandsOffToServe(t *testing.T) {
	body := CostScreen(Machine{Found: false, ProjectsDir: "/home/x/.claude/projects"}, 0, Selection{}).String()
	if !strings.Contains(body, "No transcripts to add up") {
		t.Fatalf("fixture: this is not the empty screen:\n%s", body)
	}
	if !strings.Contains(body, "replay serve") {
		t.Errorf("the empty cost screen does not name replay serve as the live path:\n%s", body)
	}
}

// When another agent's records are on the machine, the empty screen names
// the command that reads them. RED against a renderer that ignores the field.
func TestEmptyCostScreenNamesTheOtherAgentReader(t *testing.T) {
	m := Machine{Found: false, ProjectsDir: "/home/x/.claude/projects", OtherAgentCommands: []string{"replay codex"}}
	body := CostScreen(m, 0, Selection{}).String()
	if !strings.Contains(body, "replay codex") {
		t.Errorf("Codex rollouts are on this machine and the empty cost screen does not name replay codex:\n%s", body)
	}
	if plain := CostScreen(Machine{Found: false, ProjectsDir: "/home/x/.claude/projects"}, 0, Selection{}).String(); strings.Contains(plain, "replay codex") {
		t.Errorf("replay codex is named on a machine with no Codex records:\n%s", plain)
	}
}
