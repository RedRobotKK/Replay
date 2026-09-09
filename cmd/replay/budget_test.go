package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The artefact the gate compares against.
//
// MONEY-PATH section 5 makes this step 4 of 8, and its whole job is to be the
// file a repository commits: what this configuration costs on every request
// before anybody types anything. Step 5's gate then re-measures and fails the
// build when the standing cost has grown.
//
// The design constraint that shapes everything here: **it records a
// measurement, never a configuration.** The gate must be able to answer "did
// the standing cost grow" without ever opening .mcp.json, because that file can
// hold credentials and the gate runs in CI. Comparing two artefacts crosses
// nothing; parsing agent config in CI crosses the one boundary
// WHAT-YOU-GET.md draws.
//
// "Signed by nothing" in MONEY-PATH is deliberate and stays true here. This is
// free, it is plain JSON, and a repository can read it with jq. Entitlement
// (step 6) gates the gate, never this.

// ledgerCorpus writes a small ledger whose sessions carry tool definitions.
//
// Transcripts do not record what tools were OFFERED, only what was called, so
// standing cost is only knowable from a ledger. That is a real limit on who
// this command can serve and it is asserted rather than hidden: BG4 covers the
// transcript-only case.
func ledgerCorpus(t *testing.T, tools map[string]int) string {
	t.Helper()
	dir := t.TempDir()
	var defs []map[string]any
	toolBytes := 0
	for name, b := range tools {
		defs = append(defs, map[string]any{"name": name, "bytes": b})
		toolBytes += b
	}
	var lines []string
	prev := 0
	for turn := 0; turn < 8; turn++ {
		ctx := []map[string]any{
			{"uuid": "prefix", "role": "system", "blocks": []map[string]any{
				{"kind": "text", "label": "system prompt", "bytes": 4200},
				{"kind": "other", "label": "tool definitions", "bytes": toolBytes}}},
			{"uuid": "a", "role": "assistant", "blocks": []map[string]any{
				{"kind": "tool_use", "label": "tool call Bash", "tool_name": "Bash",
					"bytes": 120, "tool_use_id": "1"}}},
		}
		total := (4200+toolBytes+120)/4 + turn*40
		read := 0
		if prev > 0 {
			read = prev - 20
		}
		rec := map[string]any{
			"schema": 2, "ts": "2026-09-0" + string(rune('1'+turn%3)) + "T12:00:0" +
				string(rune('0'+turn%10)) + "Z",
			"session_id": "s1", "request_id": "r" + string(rune('0'+turn)),
			"path": "/v1/messages", "model": "claude-opus-5",
			"prompt": map[string]any{"system_bytes": 4200, "tool_bytes": toolBytes,
				"tool_count": len(defs), "tools": defs, "cache_control": 1, "messages": ctx},
			"prefix_hash": "p", "status": 200, "latency_ms": 900,
			"response": map[string]any{"usage": map[string]any{
				"input_tokens": 20, "cache_read_input_tokens": read,
				"cache_creation_input_tokens": max(0, total-read-20), "output_tokens": 30}},
		}
		b, _ := json.Marshal(rec)
		lines = append(lines, string(b))
		prev = total
	}
	if err := os.WriteFile(filepath.Join(dir, "l.jsonl"),
		[]byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// BG1: it emits a machine-readable artefact carrying the standing cost.
func TestBG1_EmitsTheStandingCost(t *testing.T) {
	dir := ledgerCorpus(t, map[string]int{
		"mcp__playwright__click": 3400, "mcp__playwright__nav": 3100,
		"mcp__jira__search": 2900, "Bash": 1800})

	var stdout, stderr bytes.Buffer
	if err := run([]string{"budget", dir, "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("budget failed: %v\n%s", err, stderr.String())
	}
	var got struct {
		Schema   int `json:"schema"`
		Standing struct {
			TokensPerRequest int `json:"tokens_per_request"`
			SystemTokens     int `json:"system_tokens"`
			ToolTokens       int `json:"tool_tokens"`
			ToolCount        int `json:"tool_count"`
		} `json:"standing"`
		Servers  map[string]int `json:"servers"`
		Measured struct {
			Sessions int    `json:"sessions"`
			Requests int    `json:"requests"`
			Source   string `json:"source"`
		} `json:"measured"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
	if got.Schema == 0 {
		t.Error("no schema version: a committed artefact that cannot say what shape it is " +
			"cannot be read safely by a later gate")
	}
	if got.Standing.TokensPerRequest <= 0 {
		t.Fatalf("standing cost is %d", got.Standing.TokensPerRequest)
	}
	// The parts must account for the whole, or the headline is decoration.
	if sum := got.Standing.SystemTokens + got.Standing.ToolTokens; sum != got.Standing.TokensPerRequest {
		t.Errorf("system %d + tools %d = %d, but the standing figure is %d",
			got.Standing.SystemTokens, got.Standing.ToolTokens, sum, got.Standing.TokensPerRequest)
	}
	if got.Standing.ToolCount != 4 {
		t.Errorf("tool count %d, want 4", got.Standing.ToolCount)
	}
	// Per-server, so the gate can say WHICH server grew — the same attribution
	// advise gained, in the artefact rather than the prose.
	if got.Servers["playwright"] <= got.Servers["jira"] {
		t.Errorf("playwright (2 tools, 6500 bytes) must cost more than jira (1 tool, 2900): %v",
			got.Servers)
	}
	if got.Measured.Requests == 0 || got.Measured.Source == "" {
		t.Errorf("the artefact does not say what it was measured from: %+v", got.Measured)
	}
}

// BG2: a corpus that measured nothing must not emit a zero budget.
//
// The defect this repository keeps finding. A budget file saying "0 tokens" is
// not an empty configuration, it is an absent measurement — and committing it
// would make step 5's gate pass forever on a ceiling nobody set.
func TestBG2_NothingMeasuredIsNotAZeroBudget(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := run([]string{"budget", dir, "--json"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("an empty corpus produced a budget; a gate built on it would pass forever")
	}
	out := stdout.String() + stderr.String() + err.Error()
	if !strings.Contains(strings.ToUpper(out), "NOT MEASURED") &&
		!strings.Contains(strings.ToLower(out), "no ledger") {
		t.Errorf("the refusal does not say nothing was measured:\n%s", out)
	}
}

// BG3: it reads no configuration.
//
// The boundary the whole design exists to keep. A .mcp.json beside the corpus
// must be neither read nor needed; if the standing cost changed because that
// file was consulted, the gate could not run in CI without exposing whatever it
// holds.
func TestBG3_NeverReadsAgentConfiguration(t *testing.T) {
	dir := ledgerCorpus(t, map[string]int{"mcp__jira__search": 2900, "Bash": 1800})
	poison := filepath.Join(dir, ".mcp.json")
	if err := os.WriteFile(poison, []byte(`{"mcpServers":{"secret":{"env":{"TOKEN":"sk-live-do-not-read"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"budget", dir, "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("budget failed: %v", err)
	}
	all := stdout.String() + stderr.String()
	if strings.Contains(all, "sk-live-do-not-read") || strings.Contains(all, "secret") {
		t.Fatal("the budget artefact carries content from .mcp.json. It must be derivable " +
			"from the ledger alone, because the gate that reads it runs in CI")
	}
}

// BG4: transcripts alone cannot answer this, and it says so.
//
// A transcript records what was CALLED, never what was OFFERED, so the standing
// cost of a configuration is not in it. Guessing from called tools would report
// a budget that falls when somebody uses fewer tools, which is backwards.
func TestBG4_TranscriptsCannotSupplyStandingCost(t *testing.T) {
	corpus(t)
	root := os.Getenv("REPLAY_TRANSCRIPTS")
	var stdout, stderr bytes.Buffer
	err := run([]string{"budget", root, "--json"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a transcript-only corpus produced a standing cost. Transcripts record " +
			"what was called, not what was offered")
	}
	out := stdout.String() + stderr.String() + err.Error()
	if !strings.Contains(strings.ToLower(out), "ledger") {
		t.Errorf("the refusal does not tell the reader they need a ledger:\n%s", out)
	}
}

// BG5: the printed parts add up to the printed total.
//
// The first version used formatCount, which renders 11,550 as "12k". That is
// right for a blame table, where the reader wants a sense of scale, and wrong
// for a figure that is committed and compared: 6,400 + 6,400 prints as
// "6k + 6k = 13k", three numbers that do not add up, in a tool whose whole
// pitch is that its figures do.
//
// Caught by reading the output rather than by a test, which is why this exists
// now — the JSON was exact throughout, so nothing downstream was wrong and
// nothing would have failed.
func TestBG5_ThePrintedPartsAddUp(t *testing.T) {
	dir := ledgerCorpus(t, map[string]int{
		"mcp__a__one": 12800, "mcp__b__two": 12800, "Bash": 1800})

	var human, stderr bytes.Buffer
	if err := run([]string{"budget", dir}, &human, &stderr); err != nil {
		t.Fatalf("budget failed: %v\n%s", err, stderr.String())
	}
	var jsonOut bytes.Buffer
	stderr.Reset()
	if err := run([]string{"budget", dir, "--json"}, &jsonOut, &stderr); err != nil {
		t.Fatalf("budget --json failed: %v", err)
	}
	var got struct {
		Standing struct {
			// Tagged. Without these the fields never matched
			// tokens_per_request and friends, every wanted value was 0,
			// comma(0) is "0", and "0" appears inside "8,950" — so the first
			// version of this test passed against the rounded output it was
			// written to catch. It asserted nothing, and said so in green.
			TokensPerRequest int `json:"tokens_per_request"`
			SystemTokens     int `json:"system_tokens"`
			ToolTokens       int `json:"tool_tokens"`
		} `json:"standing"`
	}
	if err := json.Unmarshal(jsonOut.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	// Every figure the human sees must be the exact one, so the sum on screen
	// is checkable by the reader who is about to commit it.
	text := human.String()
	if got.Standing.TokensPerRequest == 0 {
		t.Fatal("the JSON decoded to zero, so the comparison below would pass against " +
			"anything. This is how the first version of this test passed against the " +
			"rounded output it exists to catch.")
	}
	for _, n := range []int{got.Standing.SystemTokens, got.Standing.ToolTokens, got.Standing.TokensPerRequest} {
		if !strings.Contains(text, comma(n)) {
			t.Errorf("the report does not carry the exact figure %s:\n%s", comma(n), text)
		}
	}
}
