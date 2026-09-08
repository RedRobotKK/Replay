package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The block an agent reads at boot has to be regenerable in place, which means
// it needs edges. Everything outside them is the human's and must survive.
func TestAG1(t *testing.T) {
	dir := t.TempDir()
	for p, body := range map[string]string{
		"funding/DEAL-LEDGER.md":             "x",
		"funding/notes.md":                   "x",
		"funding/pitch.md":                   "x",
		"assets/applications/SENT-LEDGER.md": "x",
	} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errb bytes.Buffer
	if err := run([]string{"agents", dir}, &out, &errb); err != nil {
		t.Fatalf("run: %v (%s)", err, errb.String())
	}
	got := out.String()
	for _, want := range []string{agentsBegin, agentsEnd, "assets/applications", "funding"} {
		if !strings.Contains(got, want) {
			t.Errorf("block is missing %q:\n%s", want, got)
		}
	}
}

// Splicing must not touch a line the person wrote. This is the whole contract:
// a generated block inside a file whose other content is not ours.
func TestAG2(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "funding"), 0o755)
	for _, n := range []string{"A-LEDGER.md", "b.md", "c.md"} {
		_ = os.WriteFile(filepath.Join(dir, "funding", n), []byte("x"), 0o644)
	}
	target := filepath.Join(dir, "AGENTS.md")
	human := "# Agent rules\n\nDo not delete this line.\n\n## Team\n\nWriter owns the write.\n"
	if err := os.WriteFile(target, []byte(human), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := run([]string{"agents", dir, "--write", target}, &out, &errb); err != nil {
		t.Fatalf("run: %v (%s)", err, errb.String())
	}
	after, _ := os.ReadFile(target)
	for _, line := range strings.Split(human, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.Contains(string(after), line) {
			t.Errorf("splice destroyed a human line: %q", line)
		}
	}
	if !strings.Contains(string(after), agentsBegin) {
		t.Error("no generated block was written")
	}
}

// Regenerating twice must produce the same file, or every run shows as a diff
// and people stop running it.
func TestAG3(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "funding"), 0o755)
	for _, n := range []string{"A-LEDGER.md", "b.md", "c.md"} {
		_ = os.WriteFile(filepath.Join(dir, "funding", n), []byte("x"), 0o644)
	}
	target := filepath.Join(dir, "AGENTS.md")
	_ = os.WriteFile(target, []byte("# Rules\n"), 0o644)
	var o1, o2, e bytes.Buffer
	_ = run([]string{"agents", dir, "--write", target}, &o1, &e)
	first, _ := os.ReadFile(target)
	_ = run([]string{"agents", dir, "--write", target}, &o2, &e)
	second, _ := os.ReadFile(target)
	if string(first) != string(second) {
		t.Errorf("not idempotent; second write differs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if strings.Count(string(second), agentsBegin) != 1 {
		t.Errorf("block was duplicated, not replaced: %d begin markers", strings.Count(string(second), agentsBegin))
	}
}

// The block states how it was produced. A fact an agent reads at boot and
// cannot check is worse than no fact.
func TestAG4(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "funding"), 0o755)
	for _, n := range []string{"A-LEDGER.md", "b.md", "c.md"} {
		_ = os.WriteFile(filepath.Join(dir, "funding", n), []byte("x"), 0o644)
	}
	var out, errb bytes.Buffer
	_ = run([]string{"agents", dir}, &out, &errb)
	got := out.String()
	for _, want := range []string{"replay agents", "not a complete list"} {
		if !strings.Contains(got, want) {
			t.Errorf("block must say how it was made and what it does not cover; missing %q", want)
		}
	}
}

// A hand-edited file whose end marker is the final bytes must not crash.
//
// splice indexed past the end marker assuming a newline followed it, so a file
// saved without a trailing newline panicked. The block is written INTO a file
// people are told to hand-edit, which is the one place a panic is least
// acceptable and most likely.
func TestAG5(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "funding"), 0o755)
	for _, n := range []string{"A-LEDGER.md", "b.md", "c.md"} {
		_ = os.WriteFile(filepath.Join(dir, "funding", n), []byte("x"), 0o644)
	}
	for _, tail := range []string{
		"# Rules\n\n" + agentsBegin + "\nSTALE-BLOCK-SENTINEL\n" + agentsEnd,        // no trailing newline
		"# Rules\n\n" + agentsBegin + "\nSTALE-BLOCK-SENTINEL\n" + agentsEnd + "\n", // with one
		"# Rules\n\n" + agentsBegin + "\nSTALE-BLOCK-SENTINEL\n" + agentsEnd + "\n\nkeep me\n",
	} {
		target := filepath.Join(dir, "AGENTS.md")
		if err := os.WriteFile(target, []byte(tail), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		if err := run([]string{"agents", dir, "--write", target}, &out, &errb); err != nil {
			t.Fatalf("run: %v (%s)", err, errb.String())
		}
		after, _ := os.ReadFile(target)
		if strings.Count(string(after), agentsBegin) != 1 {
			t.Errorf("marker count %d for input %q", strings.Count(string(after), agentsBegin), tail)
		}
		if strings.Contains(string(after), "STALE-BLOCK-SENTINEL") {
			t.Errorf("stale block survived for input %q", tail)
		}
		if strings.Contains(tail, "keep me") && !strings.Contains(string(after), "keep me") {
			t.Error("content after the end marker was eaten")
		}
	}
}
