package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `--out -` means "do not write a file". It must not also mean "and skip the
// work I asked for".
//
// The early return for `--out -` sat above the `--apply` dispatch, so
// `advise --apply --json --out -` — the obvious agent-safe invocation, don't
// touch my disk and give me JSON — printed prose, emitted no JSON, and exited
// 0. To get the JSON you had to let it write a file, which is the opposite of
// what the flag combination asks for.
func TestAdviseOutDashStillApplies(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone
	// leaves the command pointed at the real home.
	t.Setenv("USERPROFILE", home)

	// A minimal transcript so advise has something to read.
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	_ = run([]string{"advise", dir, "--apply", "--json", "--out", "-"}, &out, &errb)

	got := out.String()
	if got == "" {
		t.Fatalf("nothing on stdout; stderr was %q", errb.String())
	}
	// The contract: --json means machine-readable output, whatever else is set.
	trimmed := strings.TrimSpace(got)
	start := strings.IndexAny(trimmed, "{[")
	if start < 0 {
		t.Fatalf("--json produced no JSON. This is the bug: --out - returned "+
			"before --apply ran.\n%s", truncate(got))
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed[start:]), &v); err != nil {
		t.Errorf("--json output does not parse: %v\n%s", err, truncate(trimmed[start:]))
	}

	// And it must genuinely not have written the advice file.
	if _, err := os.Stat(filepath.Join(home, ".replay", adviceFileName)); err == nil {
		t.Error("--out - wrote the advice file anyway; the flag means do not write")
	}
}

// With --json, stdout is the machine's. A human report interleaved with the
// document means `| jq` fails, which defeats the flag.
func TestAdviseJSONStdoutIsPureJSON(t *testing.T) {
	dir, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone
	// leaves the command pointed at the real home.
	t.Setenv("USERPROFILE", home)
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	_ = run([]string{"advise", dir, "--apply", "--json", "--out", "-"}, &out, &errb)

	trimmed := strings.TrimSpace(out.String())
	if trimmed == "" {
		t.Fatal("no stdout at all")
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		t.Errorf("stdout is not parseable as a single JSON document, so `| jq` "+
			"fails: %v\nfirst 160 bytes: %s", err, truncate(trimmed))
	}
}
