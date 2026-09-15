package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A read with no write is impossible, and Codex reports it on this machine.
//
// Measured 2026-09-15 across 158 rollouts: cacheRead 571,720,960,
// cacheWrite 0. One of those 158 files contains the string
// `cache_write_input_tokens` at all. The reader has looked for that field the
// whole time; the client does not send it.
//
// That is not a curiosity, it is a correction to this command's own output.
// burn prices Codex from the same usage, so a cost printed from a corpus whose
// write half is missing understates the bill by exactly the expensive half.
// The figure shipped before anybody noticed, which is the argument for saying
// it on the report rather than in an evidence file nobody has open.
func TestBurnSaysWhenASurfaceReportsReadsWithNoWrites(t *testing.T) {
	dir := t.TempDir()
	cx := filepath.Join(dir, "codex")
	if err := os.MkdirAll(cx, 0o755); err != nil {
		t.Fatal(err)
	}
	// Two turns that read a cached prefix and never report writing one.
	rollout := `{"type":"session_meta","payload":{"id":"s1"}}
{"type":"turn_context","payload":{"model":"gpt-5.4"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":50000,"cached_input_tokens":40000},"total_token_usage":{"input_tokens":50000,"cached_input_tokens":40000}}}}
`
	if err := os.WriteFile(filepath.Join(cx, "rollout-a.jsonl"), []byte(rollout), 0o600); err != nil {
		t.Fatal(err)
	}

	s := burnCodex("", dir)

	joined := strings.Join(s.problems, " | ")
	if !strings.Contains(strings.ToLower(joined), "write") {
		t.Fatalf("a surface reporting %d cached reads and no writes said nothing about it.\n"+
			"      notes: %q\n"+
			"      Something wrote the prefix being read, so the zero is a missing field, and "+
			"every cost computed from it is short by the expensive half", 40000, joined)
	}
}

// The note must NOT fire on a surface that reports both halves. A warning that
// appears on healthy data is one an operator learns to scroll past, and this
// one needs to still mean something the day it is right.
func TestBurnStaysQuietWhenBothCountersAreReported(t *testing.T) {
	dir := t.TempDir()
	cx := filepath.Join(dir, "codex")
	if err := os.MkdirAll(cx, 0o755); err != nil {
		t.Fatal(err)
	}
	rollout := `{"type":"session_meta","payload":{"id":"s2"}}
{"type":"turn_context","payload":{"model":"gpt-5.4"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":50000,"cached_input_tokens":30000,"cache_write_input_tokens":10000},"total_token_usage":{"input_tokens":50000,"cached_input_tokens":30000,"cache_write_input_tokens":10000}}}}
`
	if err := os.WriteFile(filepath.Join(cx, "rollout-b.jsonl"), []byte(rollout), 0o600); err != nil {
		t.Fatal(err)
	}

	s := burnCodex("", dir)

	// Asserts the note does not FIRE, not that its text is reassuring. The
	// first version matched on "no write", which is absent from the healthy
	// verdict ("read and write both reported"), so a build that emitted the
	// note unconditionally passed it. A mutation sweep reddened nothing when
	// the condition was replaced with true, which is how that was found.
	for _, p := range s.problems {
		if strings.Contains(p, "cached read(s) reported") {
			t.Errorf("the counter note fired on a surface that reported BOTH halves: %q", p)
		}
	}
}
