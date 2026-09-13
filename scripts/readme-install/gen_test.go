package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The block sits between two markers in README.md, and the only thing that
// may change it is this generator reading distribution/channels.json. A hand
// edit inside the markers is drift, and TestReadmeBlockMatchesGenerator is
// what CI runs to notice it.

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("locating go.mod: %v", err)
	}
	return root
}

func readManifest(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "distribution", "channels.json"))
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	return b
}

func readReadme(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("reading README: %v", err)
	}
	return string(b)
}

// Running the generator against its own output changes nothing.
func TestIdempotent(t *testing.T) {
	block, err := render(readManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	once, err := splice(readReadme(t), block)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := splice(once, block)
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Error("a second run rewrote the README; the generator is not idempotent")
	}
}

// Every live route with a command is printed once; nothing still building is
// printed as a command a reader could type.
func TestLiveCommandsOnceBuildingNever(t *testing.T) {
	raw := readManifest(t)
	block, err := render(raw)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var live int
	for _, c := range m.Channels {
		if c.Command == "" {
			continue
		}
		line := "\n" + c.Command + "\n"
		switch c.Status {
		case "live":
			live++
			if n := strings.Count(block, line); n != 1 {
				t.Errorf("live command %q appears %d times, want 1", c.Command, n)
			}
		case "building":
			if strings.Contains(block, line) {
				t.Errorf("building command %q is printed as if a reader could run it", c.Command)
			}
		}
	}
	if live == 0 {
		t.Fatal("the manifest has no live command; this test checked nothing")
	}
}

// A check that cannot fail is not evidence: change one live command in a
// copy of the manifest and the block must change with it.
func TestMutatedManifestChangesBlock(t *testing.T) {
	raw := readManifest(t)
	before, err := render(raw)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	const mutant = "curl -fsSL https://example.invalid/mutant.sh | sh"
	mutated := false
	for i := range m.Channels {
		if m.Channels[i].Status == "live" && m.Channels[i].Command != "" {
			m.Channels[i].Command = mutant
			mutated = true
			break
		}
	}
	if !mutated {
		t.Fatal("no live command to mutate")
	}
	altered, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	after, err := render(altered)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("the generated block did not change when a live command did")
	}
	if !strings.Contains(after, "\n"+mutant+"\n") {
		t.Error("the mutated command is not in the regenerated block")
	}
}

// The vocabulary rules for the block. The rest of the README is outside this
// generator's remit and carries words it may not print.
func TestGeneratedVocabulary(t *testing.T) {
	block, err := render(readManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(block, "\u2014") {
		t.Error("the generated block contains an em-dash")
	}
	lower := strings.ToLower(block)
	for _, w := range []string{"save", "optimise", "optimize", "unlimited", "typical", "you should"} {
		if strings.Contains(lower, w) {
			t.Errorf("the generated block contains %q", w)
		}
	}
	const opening = "Every route below ends at the same signed release; verify any download with cosign as shown in the Releases footer."
	if !strings.Contains(block, opening) {
		t.Error("the generated block does not open with the sentence that ties every route to one signed release")
	}
}

// Blocked rows name their gate; rows without a command are not routes and do
// not print.
func TestBlockedRowsNameTheirGate(t *testing.T) {
	block, err := render([]byte(`{"channels":[
		{"name":"homebrew-core","command":"brew install replay","status":"blocked","gate":"OSI; notability thresholds"},
		{"name":"a listing site","command":"","status":"blocked","gate":"OSI"},
		{"name":"a tap","command":"brew install x/tap/replay","verify":"brew info x/tap/replay","status":"building"},
		{"name":"a plan","command":"apt install replay","status":"planned"},
		{"name":"a skip","command":"snap install replay","status":"skipped","gate":"ruled out"},
		{"name":"a live one","command":"go install example/replay@latest","verify":"replay version","status":"live"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Not offered: homebrew-core (OSI; notability thresholds)",
		"Coming: a tap",
		"**a live one**",
		"Check: `replay version`",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
	for _, absent := range []string{"a listing site", "a plan", "a skip", "apt install", "snap install"} {
		if strings.Contains(block, absent) {
			t.Errorf("block prints %q, which is not a route a reader can take:\n%s", absent, block)
		}
	}
}

func TestSpliceRefusesReadmeWithoutMarkers(t *testing.T) {
	if _, err := splice("# Replay\n\nno markers here\n", "body\n"); err == nil {
		t.Error("splice accepted a README with no markers; it would have nowhere to write")
	}
}

// What CI runs: the block in README.md is the generator's output byte for
// byte. A hand edit inside the markers, or a manifest change without a
// regeneration, fails here.
func TestReadmeBlockMatchesGenerator(t *testing.T) {
	block, err := render(readManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	readme := readReadme(t)
	current, err := extract(readme)
	if err != nil {
		t.Fatal(err)
	}
	if current != block {
		t.Errorf("README.md's install block differs from the generator's output.\n"+
			"Run: go run ./scripts/readme-install\n--- README ---\n%s\n--- generator ---\n%s",
			current, block)
	}
}
