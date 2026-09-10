package transcript

import "testing"

// Codex states its cache WRITE, and this reader was dropping it.
//
// `TokenUsage` in codex-rs/protocol/src/protocol.rs declares six counters:
//
//	input_tokens, cached_input_tokens, cache_write_input_tokens,
//	output_tokens, reasoning_output_tokens, total_tokens
//
// codexUsage carried five of them. cache_write_input_tokens was never in the
// struct, so it never reached Usage.CacheCreation and every Codex session
// reported a write of zero.
//
// That one field is the difference between a hit-rate surface and a re-billing
// surface. Replay exists to name what a cache write cost that a different
// layout would have read; on Anthropic the write has to be inferred by hashing
// the prefix, and Codex simply states it. The data was already on disk in every
// rollout file this tool has ever read.
func TestCodexReadsTheCacheWriteItIsGiven(t *testing.T) {
	s, err := ParseCodexFile("codexdata/cachewrite.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	// Two billed deltas: 900 written on the first turn, 100 on the second.
	if got, want := s.Billed.CacheCreation, 1000; got != want {
		t.Errorf("billed cache creation = %d, want %d: cache_write_input_tokens is "+
			"stated by the provider on every turn and was being discarded", got, want)
	}
	if got, want := s.Billed.CacheRead, 900; got != want {
		t.Errorf("billed cache read = %d, want %d", got, want)
	}
	if got, want := s.Reported.CacheCreation, 100; got != want {
		t.Errorf("reported cache creation = %d, want %d: the cumulative snapshot "+
			"carries the write too", got, want)
	}
}

// A write larger than the prompt it was written from cannot occur.
//
// The existing guards refuse cached_input_tokens > input_tokens and
// reasoning_output_tokens > output_tokens for the same reason: a subset that
// exceeds its parent describes a state the provider cannot be in, and a record
// like that must be refused rather than added to a total someone is billed
// against. cache_write_input_tokens is a share of the same prompt and needs the
// same refusal, or the one counter with no ceiling becomes the way a bad record
// gets in.
func TestCodexRefusesAWriteLargerThanItsPrompt(t *testing.T) {
	c := &codexUsage{Input: 100, Cached: 0, CacheWrite: 101, Output: 10}
	if _, ok := c.usage(); ok {
		t.Error("a cache write of 101 against a prompt of 100 was accepted; " +
			"a subset larger than its parent must be refused like the others")
	}
	c = &codexUsage{Input: 100, Cached: 0, CacheWrite: -1, Output: 10}
	if _, ok := c.usage(); ok {
		t.Error("a negative cache write was accepted")
	}
	c = &codexUsage{Input: 100, Cached: 0, CacheWrite: 100, Output: 10}
	if _, ok := c.usage(); !ok {
		t.Error("a write exactly equal to the prompt was refused; the boundary is legal")
	}
}

// Both refusals in usage() must be reachable as whole guards.
//
// The clause tests above pin the clauses this change added; neither pins the
// conditional they live in. guard-reachability neutralises the whole `if` and
// found both removable with no test noticing — which is the difference between
// mutating a clause and mutating a guard, and the reason a clause-level pass is
// not the coverage it appears to be.
func TestCodexUsageRefusalsAreReachableAsGuards(t *testing.T) {
	// The negative-counter guard, one field at a time.
	for name, c := range map[string]*codexUsage{
		"negative input":     {Input: -1, Output: 10},
		"negative output":    {Input: 10, Output: -1},
		"negative cached":    {Input: 10, Cached: -1, Output: 10},
		"negative reasoning": {Input: 10, Output: 10, Reasoning: -1},
		"negative write":     {Input: 10, Output: 10, CacheWrite: -1},
	} {
		if _, ok := c.usage(); ok {
			t.Errorf("%s was accepted; a counter below zero describes a state that cannot occur", name)
		}
	}
	// The subset-ceiling guard, one relation at a time.
	for name, c := range map[string]*codexUsage{
		"cached over input":       {Input: 10, Cached: 11, Output: 10},
		"reasoning over output":   {Input: 10, Output: 10, Reasoning: 11},
		"cached+write over input": {Input: 10, Cached: 6, CacheWrite: 5, Output: 10},
	} {
		if _, ok := c.usage(); ok {
			t.Errorf("%s was accepted; a subset larger than its parent cannot occur", name)
		}
	}
	// And the boundary is legal, so the guard is a ceiling rather than a ban.
	if _, ok := (&codexUsage{Input: 10, Cached: 6, CacheWrite: 4, Output: 10}).usage(); !ok {
		t.Error("cached+write exactly equal to input was refused; the boundary is legal")
	}
}
