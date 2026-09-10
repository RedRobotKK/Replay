package usage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The guard, pointed at the reader it was written for.
//
// docs/design/UNWIRED-LOG.md #8 records this package as "the guard against a
// live ~1.94x double-count in codex.go:151", and internal/regression's
// TestNoNewlyUnwiredPackages repeats it. Both were right, and neither made the
// double-count observable: FromInclusive and Validate had eleven tests between
// them, every one of them built from a literal InclusiveCounts written by the
// test. Nothing ever ran a real Codex rollout through them, so the guard and
// the defect never met.
//
// ADR-0018: "Two-part fixes need a test that crosses the join." This is that
// test. The counts come out of transcript.ParseCodex; the assertion is this
// package's own Validate.
//
// It does not wire the package into the binary. cmd/replay still does not
// import internal/usage, and UNWIRED-LOG #8 stays open for that reason — what
// is closed is the arithmetic it was guarding, and the silence about it.

// codexRollout is one turn as Codex writes it: input_tokens INCLUSIVE of
// cached_input_tokens.
func codexRollout(input, cached, output, reasoning int) string {
	return `{"timestamp":"t","type":"session_meta","payload":{"id":"s","cli_version":"1"}}` + "\n" +
		fmt.Sprintf(`{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":`+
			`{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"reasoning_output_tokens":%d,"total_tokens":%d}}}}`,
			input, cached, output, reasoning, input+output) + "\n"
}

// UC-1: what the Codex reader produces satisfies this package's own guard.
//
// A Record built from the reader's Usage must have Fresh + read + write equal
// to Prompt. Validate is the function that says so, and it is the function
// UNWIRED-LOG #8 names.
//
// PASS: Validate returns nil for every shape, including a fully cached turn.
// FAIL: "usage does not add up: fresh 1000 + read 800 + write 0 = 1800, but
// prompt is 1800" — the shape Validate's own error message calls out as "a
// provider counting inclusively must be converted, not copied". Reintroduce it
// by writing `Input: c.Input` in transcript's codexUsage.usage().
func TestUC1_TheCodexReaderSatisfiesValidate(t *testing.T) {
	cases := []struct {
		name                              string
		input, cached, output, reasoning  int
		wantPrompt, wantFresh, wantCached int
	}{
		{"mostly cached", 1000, 800, 10, 0, 1000, 200, 800},
		{"nothing cached", 1000, 0, 10, 0, 1000, 1000, 0},
		{"entirely cached", 2000, 2000, 100, 40, 2000, 0, 2000},
		{"one token", 1, 1, 1, 1, 1, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := transcript.ParseCodex(strings.NewReader(
				codexRollout(c.input, c.cached, c.output, c.reasoning)))
			if err != nil {
				t.Fatal(err)
			}
			if s.Turns != 1 {
				t.Fatalf("the fixture is wrong: %d turn(s) parsed", s.Turns)
			}
			rec := Record{
				Provider:    "openai-codex",
				Mechanism:   MechanismImplicitPrefix,
				At:          time.Unix(0, 0),
				Prompt:      s.Billed.PromptTotal(),
				Fresh:       s.Billed.Input,
				CachedRead:  s.Billed.CacheRead,
				CachedWrite: s.Billed.CacheCreation,
				Output:      s.Billed.Output,
				Reasoning:   s.Billed.ThinkingTokens,
			}
			if err := rec.Validate(); err != nil {
				t.Errorf("a real Codex turn fails this package's own guard: %v", err)
			}
			if rec.Prompt != c.wantPrompt {
				t.Errorf("Prompt = %d, want %d: the provider's prompt figure, counted once",
					rec.Prompt, c.wantPrompt)
			}
			if rec.Fresh != c.wantFresh {
				t.Errorf("Fresh = %d, want %d", rec.Fresh, c.wantFresh)
			}
			if rec.CachedRead != c.wantCached {
				t.Errorf("CachedRead = %d, want %d", rec.CachedRead, c.wantCached)
			}
		})
	}
}

// UC-2: the reader's output equals what FromInclusive would have produced.
//
// FromInclusive is the conversion this package holds. transcript cannot call
// it — internal/usage imports internal/transcript, so the dependency only runs
// one way — and the reader therefore does the subtraction itself. The two must
// not drift.
//
// PASS: the record built from the parser and the record built by FromInclusive
// from the same raw counts are equal in every token field.
// FAIL: any divergence, which is the reader and the guard disagreeing about
// what a prompt is.
func TestUC2_TheReaderAgreesWithFromInclusive(t *testing.T) {
	const input, cached, output, reasoning = 12345, 9000, 700, 250
	s, err := transcript.ParseCodex(strings.NewReader(codexRollout(input, cached, output, reasoning)))
	if err != nil {
		t.Fatal(err)
	}
	want := FromInclusive("openai-codex", MechanismImplicitPrefix, "", time.Unix(0, 0),
		InclusiveCounts{Prompt: input, Cached: cached, Output: output, Reasoning: reasoning}, nil)
	if err := want.Validate(); err != nil {
		t.Fatalf("FromInclusive itself does not validate: %v", err)
	}
	got := struct{ prompt, fresh, read, write, out, reason int }{
		s.Billed.PromptTotal(), s.Billed.Input, s.Billed.CacheRead,
		s.Billed.CacheCreation, s.Billed.Output, s.Billed.ThinkingTokens,
	}
	if got.prompt != want.Prompt || got.fresh != want.Fresh || got.read != want.CachedRead ||
		got.write != want.CachedWrite || got.out != want.Output || got.reason != want.Reasoning {
		t.Errorf("the Codex reader and FromInclusive disagree:\n  reader %+v\n  guard  "+
			"prompt:%d fresh:%d read:%d write:%d out:%d reason:%d",
			got, want.Prompt, want.Fresh, want.CachedRead, want.CachedWrite, want.Output, want.Reasoning)
	}
}

// UC-3: what Validate can and cannot catch, stated rather than assumed.
//
// UC-1 is evidence only if Validate distinguishes the two worlds, so the copied
// shape is built here by hand and must be refused. But the second half is the
// finding, and it is against this package:
//
// Validate compares Fresh + read + write against Prompt. The Codex reader's
// Prompt came from PromptTotal(), which is that same sum. Both sides moved
// together, so Validate would have returned nil for the very defect
// UNWIRED-LOG.md #8 calls it "the guard against". Wiring this package as the
// document proposed would have closed nothing, and the entry would have been
// marked WIRED.
//
// That is ADR-0018's "an oracle may not derive from the thing it checks", one
// level up. The fix Validate needs is an independent Prompt — the provider's
// own figure — which is why UC-1 asserts Prompt against a literal as well.
//
// PASS: refused when Prompt is the provider's figure; accepted when Prompt is
// the inflated sum, and this test says so out loud.
// FAIL: either behaviour changing without this comment changing with it.
func TestUC3_WhatValidateCatchesAndWhatItCannot(t *testing.T) {
	// Prompt is the provider's own 1,000; the parts claim 1,800. Refused.
	copied := Record{Prompt: 1000, Fresh: 1000, CachedRead: 800}
	if err := copied.Validate(); err == nil {
		t.Error("Validate accepted prompt 1000 with fresh 1000 and read 800; the " +
			"guard cannot distinguish the defect it exists for, so UC-1 proves nothing")
	}
	// Prompt derived from the same inflated sum, which is exactly what the
	// unfixed reader handed downstream. Accepted, and wrong.
	inflated := Record{Prompt: 1800, Fresh: 1000, CachedRead: 800}
	if err := inflated.Validate(); err != nil {
		t.Errorf("Validate now refuses the self-consistent inflated shape (%v). That "+
			"is an improvement, and this test's comment — and UNWIRED-LOG.md #8 — "+
			"describe a weaker guard than the one that now exists. Update both.", err)
	}
}
