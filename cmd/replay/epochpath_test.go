package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The epoch reaches the report through the real walk, not through a hand-built
// unit.
//
// The first version of this test constructed a costUnit with MixedEpochs
// already set and checked the renderer. That left the one line carrying the
// fact out of AsRunSession untested — mutating it to `false` kept the test
// green — which is the same shape as the freeze wiring a review had just
// caught: both pure ends covered, the glue between them not. So this drives a
// real ledger through runCost instead, and the mutation dies.
func TestCostCarriesTheEpochFromTheLedgerToTheReport(t *testing.T) {
	dir := t.TempDir()
	rec := func(epoch, id string) string {
		r := map[string]any{
			"schema": ledger.SchemaVersion, "ts": "2026-09-12T05:00:00Z",
			"session_id": "mixed", "path": "/v1/messages", "request_id": id,
			"model": "claude-opus-5", "stream": false, "epoch": epoch,
			"status": 200, "latency_ms": 1,
			"prompt": map[string]any{
				"system_bytes": 0, "tool_bytes": 0, "tool_count": 0, "cache_control": 0,
				"messages": []any{map[string]any{"role": "user",
					"blocks": []any{map[string]any{"kind": "text", "bytes": 400}}}},
			},
			"response": map[string]any{"usage": map[string]any{
				"input_tokens": 5000, "cache_creation_input_tokens": 2000,
				"cache_read_input_tokens": 8000, "output_tokens": 900}},
		}
		b, _ := json.Marshal(r)
		return string(b)
	}
	body := rec("aaaaaaaaaaaaaaaa", "req_a") + "\n" + rec("bbbbbbbbbbbbbbbb", "req_b") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "mixed.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := runCost([]string{dir}, &out, &errOut); err != nil {
		t.Fatalf("runCost: %v (stderr: %s)", err, errOut.String())
	}
	if !strings.Contains(out.String(), "changed tool set mid-run") {
		t.Fatalf("two labelled epochs in one ledger session did not reach the report:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "1 session(s)") {
		t.Fatalf("the session count is wrong:\n%s", out.String())
	}
}
