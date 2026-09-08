package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What a commit can do to a cached prefix.
//
// The break-cause corpus says a prefix change is rare and expensive: 5 events,
// 1,807,000 tokens, a mean of 361,400 per break — the highest mean of any cause
// in the table, higher than a TTL expiry. Rare and enormous is exactly the shape
// a gate is for, because nobody catches it by watching.
//
// The design is constrained by two things this repository already established,
// and neither was my idea.
//
// The first is in internal/proxy/causedetail.go: "Across the whole 30-lane
// trial of 2026-09-06, system_bytes never moved once: every real prefix change
// was the tool SET changing." So a gate built to watch system prompts would be
// watching the half that did not move. The tool set is the signal.
//
// The second is the boundary in docs/WHAT-YOU-GET.md: on every default
// invocation Replay does not read settings.json, CLAUDE.md, .mcp.json or any
// other configuration, because a tool that reads your config to advise you on
// your config has to be trusted with it. This command therefore reads only what
// it is handed by name, discovers nothing, and refuses settings.json outright.
// That is the same shape as `advise --apply`: a typed request to read one named
// thing, not a licence to go looking.

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const mcpTwo = `{"mcpServers":{"github":{"command":"gh-mcp"},"sentry":{"command":"sentry-mcp"}}}`
const mcpThree = `{"mcpServers":{"github":{"command":"gh-mcp"},"sentry":{"command":"sentry-mcp"},"linear":{"command":"linear-mcp"}}}`

// PX1: adding an MCP server invalidates the cached prefix, and the gate says so.
//
// This is the case the corpus measured. An MCP connector appearing appends its
// whole tool block to the prefix, so every cached prefix in the team is void on
// the next request.
func TestPX1_AddingAServerInvalidatesThePrefix(t *testing.T) {
	dir := t.TempDir()
	before := writeFile(t, dir, "before.json", mcpTwo)
	after := writeFile(t, dir, "after.json", mcpThree)

	var stdout, stderr bytes.Buffer
	err := run([]string{"prefix", "--before", before, "--after", after}, &stdout, &stderr)
	if err == nil {
		t.Error("a prefix-invalidating change exited 0, so a CI gate built on it would pass the " +
			"one diff it exists to catch")
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "linear") {
		t.Errorf("the added server is not named, so a reviewer cannot tell what did it:\n%s", out)
	}
}

// PX2: an unchanged tool set is not an invalidation.
//
// The half that makes the gate usable. A gate that fires on every diff is a
// gate somebody disables, and it would take PX1 with it.
func TestPX2_AnUnchangedToolSetPasses(t *testing.T) {
	dir := t.TempDir()
	before := writeFile(t, dir, "before.json", mcpTwo)
	after := writeFile(t, dir, "after.json", mcpTwo)

	var stdout, stderr bytes.Buffer
	if err := run([]string{"prefix", "--before", before, "--after", after}, &stdout, &stderr); err != nil {
		t.Errorf("an unchanged tool set was reported as invalidating: %v\n%s", err, stdout.String()+stderr.String())
	}
}

// PX3: reordering the same servers is not a change.
//
// JSON object order is not semantic, and a formatter or a merge can reorder
// keys. A gate that fires on that is reporting its own parser, not the prefix.
//
// Note for anyone mutating this file to check the test bites: it survives
// either single mutation, because two independent mechanisms each guarantee it
// — the names are sorted, and diffNames compares by set membership. Removing
// both fails it, which is how this was verified. A test that needs a double
// mutation is still evidence; a test nobody tried to break is not.
func TestPX3_ReorderingIsNotAChange(t *testing.T) {
	dir := t.TempDir()
	before := writeFile(t, dir, "before.json", mcpTwo)
	after := writeFile(t, dir, "after.json",
		`{"mcpServers":{"sentry":{"command":"sentry-mcp"},"github":{"command":"gh-mcp"}}}`)

	var stdout, stderr bytes.Buffer
	if err := run([]string{"prefix", "--before", before, "--after", after}, &stdout, &stderr); err != nil {
		t.Errorf("reordering the same two servers was called an invalidation: %v", err)
	}
}

// PX4: settings.json is refused rather than read.
//
// The boundary in WHAT-YOU-GET.md, enforced rather than described.
// settings.json can hold environment variables and credentials. A gate that
// quietly widened its own read scope would be the thing this project warns
// about, shipped by this project.
func TestPX4_SettingsJSONIsRefused(t *testing.T) {
	dir := t.TempDir()
	s := writeFile(t, dir, "settings.json", `{"env":{"ANTHROPIC_API_KEY":"sk-do-not-read-me"}}`)
	other := writeFile(t, dir, "after.json", mcpTwo)

	var stdout, stderr bytes.Buffer
	err := run([]string{"prefix", "--before", s, "--after", other}, &stdout, &stderr)
	// The error counts as output: main prints it with `fmt.Fprintln(os.Stderr,
	// "replay:", err)`, so it is what a person and a CI log actually see.
	out := stdout.String() + stderr.String()
	if err != nil {
		out += err.Error()
	}
	if err == nil {
		t.Error("settings.json was accepted as an input")
	}
	if strings.Contains(out, "sk-do-not-read-me") {
		t.Fatal("the refusal printed the file's contents, which is worse than reading it quietly")
	}
	if !strings.Contains(strings.ToLower(out), "settings.json") {
		t.Errorf("the refusal does not say which file it refused:\n%s", out)
	}
}

// PX5: how many sessions this costs is never printed as a number.
//
// The gate knows the size of one cold write. It does not know how many caches
// are warm across a team, and there is no way for it to find out from a diff.
// Multiplying by a guessed headcount would produce exactly the defect this
// repository keeps finding: a figure standing in for one nobody measured.
func TestPX5_TeamCostIsNotInvented(t *testing.T) {
	dir := t.TempDir()
	before := writeFile(t, dir, "before.json", mcpTwo)
	after := writeFile(t, dir, "after.json", mcpThree)

	var stdout, stderr bytes.Buffer
	_ = run([]string{"prefix", "--before", before, "--after", after}, &stdout, &stderr)
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "NOT MEASURED") {
		t.Errorf("the number of affected sessions is not marked NOT MEASURED, so a reader will "+
			"take the per-session figure for a team total:\n%s", out)
	}
}

// PX6: a file that carries no tool definitions is not a prefix input.
//
// Source files, tests and documentation do not enter the cached prefix. Saying
// so is the difference between a gate and an alarm.
func TestPX6_AnOrdinaryFileIsNotAPrefixInput(t *testing.T) {
	dir := t.TempDir()
	before := writeFile(t, dir, "README.md", "# hello\n")
	after := writeFile(t, dir, "README2.md", "# hello, changed\n")

	var stdout, stderr bytes.Buffer
	err := run([]string{"prefix", "--before", before, "--after", after}, &stdout, &stderr)
	if err != nil {
		t.Errorf("an ordinary file pair was treated as a prefix change: %v", err)
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(out), "not a prefix input") {
		t.Errorf("the report does not say the files do not participate:\n%s", out)
	}
}
