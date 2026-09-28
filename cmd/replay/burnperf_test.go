package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The Ollama surface reports throughput, because the log establishes it.
//
// `replay burn` already reads these logs for tokens and cache counts. The
// timings sat in OllamaRequest unread: at 2974d192 `git grep PromptMS` returned
// the parser and no consumer, so a figure Replay had already extracted about a
// surface it already supports reached no user.
//
// This is the wiring test. Without it the Perf contract is a type with a test
// and no caller, which is the defect UNWIRED-LOG.md records nine times.
func TestBurnOllamaReportsObservedThroughput(t *testing.T) {
	s := burnOllama("", "burndata")
	if s.requests == 0 {
		t.Fatal("the fixture produced no requests; this test would assert nothing")
	}
	if s.tokensPerSec <= 0 {
		t.Errorf("burn derived no throughput on a surface whose log prints it")
	}
	if s.tokensPerSecBlocks == 0 {
		t.Error("a rate was reported with no count of what it averages")
	}
	// It must NOT land in the problems channel: a measurement is not a problem.
	if strings.Contains(strings.Join(s.problems, " "), "tokens per second") {
		t.Error("throughput was written into the problems channel, where it reads as a fault")
	}
}

// The derivation is checked against Ollama's own printed rate.
//
// The fixture's eval line reads "4699.52 ms / 93 tokens ... 19.79 tokens per
// second". Replay derives tokens/second independently from the count and the
// duration, so the server's own figure is a positive control: if the two
// disagree, the derivation is wrong, not merely unverified.
func TestOllamaThroughputMatchesTheServersOwnFigure(t *testing.T) {
	rs, err := transcript.ParseOllamaLogFile("burndata/ollama/server-1.log")
	if err != nil {
		t.Fatal(err)
	}
	var got transcript.Measure
	for _, r := range rs {
		if m := r.Perf().TokensPerSec; m.Provenance != "" {
			got = m
			break
		}
	}
	if got.Provenance != "derived" {
		t.Fatalf("throughput provenance = %q, want derived", got.Provenance)
	}
	// Ollama printed 19.79 for this block.
	if got.Value < 19.7 || got.Value > 19.9 {
		t.Errorf("derived throughput %.2f tok/s disagrees with the server's own "+
			"19.79 tok/s for the same block; the derivation is wrong", got.Value)
	}
}
