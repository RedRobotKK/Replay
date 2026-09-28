package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	// Read the server's own printed rate from the same log rather than
	// comparing against a band typed into this test. grok's review (F3)
	// observed that the first version checked arithmetic against a hardcoded
	// 19.7-19.9 and so was not an independent control: if the fixture changed,
	// the band would silently go stale.
	body, err := os.ReadFile("burndata/ollama/server-1.log")
	if err != nil {
		t.Fatal(err)
	}
	// "prompt eval time" also ends in "eval time", and \b matches after the
	// space, so the first version of this regex captured the PROMPT rate
	// (182.12) and reported the derivation as wrong. ParseOllamaLog guards the
	// same collision with !strings.Contains(line, "prompt eval").
	m := regexp.MustCompile(`(?m)^.*[^t] eval time =.*?,\s*([\d.]+) tokens per second`).FindSubmatch(body)
	if m == nil {
		t.Fatal("the fixture no longer prints a tokens-per-second figure; this test would assert nothing")
	}
	printed, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	if diff := got.Value - printed; diff > 0.05 || diff < -0.05 {
		t.Errorf("derived throughput %.2f tok/s disagrees with the server's own "+
			"%.2f tok/s for the same block; the derivation is wrong", got.Value, printed)
	}
}

// The printed block count must describe the population the rate averages.
//
// Found in review by grok (research/agents/grok/2026-09-28-perf-contract-review.md,
// F4): the first implementation set tokensPerSecBlocks from measured+unmeasured,
// which partitions every emitted request by n_past presence. The rate is
// accumulated only over blocks whose generation time the server printed.
//
// The two counts coincide today, because ParseOllamaLog drops a block whose
// eval line is missing ("a total with no eval line is a truncated record"), so
// every emitted request carries one. Measured on this repository's fixtures:
// 2 emitted, 2 with an observed generation time, 2 with n_past. The defect was
// therefore latent rather than active, and the count was right by accident of
// the parser rather than by construction.
//
// This test pins the count to the rate's own population, so the two cannot
// drift apart if the parser's drop rule ever changes.
func TestThroughputBlockCountMatchesTheRatesPopulation(t *testing.T) {
	s := burnOllama("", "burndata")
	if s.tokensPerSec <= 0 {
		t.Fatal("no rate was derived; this test would assert nothing")
	}
	// Recount independently, from the same parser, using the rate's criterion.
	want := 0
	ms, _ := filepath.Glob("burndata/ollama/*.log")
	for _, p := range ms {
		rs, err := transcript.ParseOllamaLogFile(p)
		if err != nil {
			continue
		}
		for _, r := range rs {
			if r.Perf().GenerateMS.Provenance == transcript.Observed {
				want++
			}
		}
	}
	if want == 0 {
		t.Fatal("no fixture block reports a generation time; this test would assert nothing")
	}
	if s.tokensPerSecBlocks != want {
		t.Errorf("the rate is labelled as covering %d block(s); %d block(s) "+
			"reported a generation time and therefore entered it",
			s.tokensPerSecBlocks, want)
	}
}
