package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// RPL-C021, behavioural. Replay offers no cross-surface grand total.
//
// TestC021_NoCrossSurfaceTotalIsReachable is a name lint over surfaceBurn: a
// reducer called rollup, planted beside the type on 2026-10-02, survived it.
// This test holds the claim where the reader meets it, on the burn report
// itself, with two surfaces populated so a total would have something to sum:
// no line of the report names a total across surfaces, and the sum of the two
// per-surface token figures appears nowhere in it.
func TestC021_BurnPrintsNoCrossSurfaceTotal(t *testing.T) {
	home, _ := e2eTranscript(t)
	// A second surface: one Codex rollout under the home Codex uses.
	codexDir := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "internal", "transcript", "codexdata", "compacted.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "rollout-2026-09-01.jsonl"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	out, errb, err := e2e(t, "burn")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}

	// POSITIVE CONTROL: two surfaces carry a token figure, or there is nothing
	// a total could have been summed from and the absence below is vacuous.
	row := regexp.MustCompile(`^\s{2}(claude-code|codex|ollama|grok)\s+([0-9,]+)\s+([0-9,]+)`)
	tokens := map[string]int{}
	for _, l := range strings.Split(out, "\n") {
		if m := row.FindStringSubmatch(l); m != nil {
			n, _ := strconv.Atoi(strings.ReplaceAll(m[3], ",", ""))
			tokens[m[1]] = n
		}
	}
	if len(tokens) < 2 {
		t.Fatalf("fewer than two surfaces reported tokens (%v); the report is:\n%s", tokens, out)
	}
	sum := 0
	for _, n := range tokens {
		sum += n
	}
	if sum == 0 {
		t.Fatalf("both surfaces read zero tokens; the sum is not discriminating:\n%s", out)
	}

	total := regexp.MustCompile(`(?i)^\s*(total|all surfaces|combined|overall|grand)\b`)
	for _, l := range strings.Split(out, "\n") {
		if total.MatchString(l) {
			t.Errorf("RPL-C021: the burn report carries a cross-surface line: %q", strings.TrimSpace(l))
		}
	}
	if strings.Contains(out, comma(sum)) {
		t.Errorf("RPL-C021: the sum of the per-surface token figures, %s, appears in the report; "+
			"those figures have different units and their sum has none", comma(sum))
	}
}
