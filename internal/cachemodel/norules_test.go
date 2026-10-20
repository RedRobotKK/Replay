package cachemodel

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// NR: a model with no published cache rules here is not classified under
// Anthropic's.
//
// RulesForModel returned AnthropicRules for every id that was not Astra, so a
// gpt-5 request ten minutes after the last was "cache expired (gap longer
// than the TTL)" by Anthropic's five-minute TTL even when OpenAI had served
// the cache, and its breaks were reported as caused (2026-10-15). OpenAI's
// TTLs for those models are not in this repository. What stays true without
// them: a model change is a model change, and a request that read nothing
// from the cache is a cold read whose cause is not known.
func TestNR1_AModelWithNoRulesIsNotClassifiedUnderAnthropicRules(t *testing.T) {
	warm := transcript.Usage{Input: 100, CacheRead: 50_000}
	cold := transcript.Usage{Input: 50_100, CacheRead: 0}
	gap := 10 * time.Minute // past Anthropic's 5m TTL
	for _, m := range []string{"gpt-5", "gpt-5.4-mini", "o4-mini", "gpt-5-codex"} {
		if cause, ok := ClassifyBreakForModel(warm, warm, m, m, gap); ok {
			t.Errorf("%s: a request that read the cache was classified as a break: %q", m, cause)
		}
		if cause, ok := ClassifyBreakForModel(warm, cold, m, m, gap); !ok || cause != CauseNoRules {
			t.Errorf("%s: a cold read is (%q, %v), want (%q, true)", m, cause, ok, CauseNoRules)
		}
		if cause, ok := ClassifyBreakForModel(warm, warm, m, "claude-opus-5", time.Second); !ok || cause != CauseModelChanged {
			t.Errorf("%s -> claude-opus-5: (%q, %v), want a model change", m, cause, ok)
		}
	}
}

// The control: Anthropic and Astra ids keep their own rules.
func TestNR2_ModelsWithRulesAreUnchanged(t *testing.T) {
	warm := transcript.Usage{Input: 100, CacheRead: 50_000}
	if cause, ok := ClassifyBreakForModel(warm, warm, "claude-opus-5", "claude-opus-5", 10*time.Minute); !ok || cause != CauseTTLExpired {
		t.Errorf("claude-opus-5 past its TTL: (%q, %v), want TTL expired", cause, ok)
	}
	if _, ok := ClassifyBreakForModel(warm, warm, "claude-opus-5", "claude-opus-5", time.Minute); ok {
		t.Error("claude-opus-5 inside its TTL with a warm read was classified as a break")
	}
	if r := RulesForModel("gpt-6-astra-2026-09-01"); r.Provider != "openai" {
		t.Errorf("Astra rules provider %q, want openai", r.Provider)
	}
	if !HasRulesForModel("gpt-6-astra") || !HasRulesForModel("claude-sonnet-5") || !HasRulesForModel("") {
		t.Error("an Astra, Anthropic or empty id lost its rules")
	}
	if HasRulesForModel("gpt-5") || HasRulesForModel("o4-mini") {
		t.Error("an OpenAI id without published rules here reports rules")
	}
}
