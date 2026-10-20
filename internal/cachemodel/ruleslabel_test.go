package cachemodel

import (
	"strings"
	"testing"
)

// RL: RulesLabel names the rules that classified a set of models' breaks
// (2026-10-16). Each case pins one branch: Anthropic alone keeps the old
// label, Astra and models without rules are named, and a model is named once.
func TestRL1_TheLabelNamesTheRulesThatApplied(t *testing.T) {
	v := RulesVersionInEffect()
	none := func(ms string) string {
		return "no published cache rules for " + ms + " (their break causes are not measured)"
	}
	for _, tc := range []struct {
		models []string
		want   string
	}{
		{nil, v},
		{[]string{""}, v},
		{[]string{"claude-opus-5", "claude-sonnet-5"}, v},
		{[]string{"gpt-6-astra"}, AstraRulesLabel},
		{[]string{"GPT-6-Astra-2026-09-01"}, AstraRulesLabel},
		{[]string{"o4-mini", "gpt-5", "gpt-5"}, none("gpt-5, o4-mini")},
		{[]string{"claude-opus-5", "gpt-6-astra"}, v + " and " + AstraRulesLabel},
		{[]string{"gpt-5", "claude-opus-5"}, v + " and " + none("gpt-5")},
		{[]string{"gpt-6-astra", "gpt-5"}, AstraRulesLabel + " and " + none("gpt-5")},
	} {
		if got := RulesLabel(tc.models); got != tc.want {
			t.Errorf("RulesLabel(%q) = %q, want %q", tc.models, got, tc.want)
		}
	}
	if strings.Contains(AstraRulesLabel, v) {
		t.Errorf("Astra's label %q carries Anthropic's version", AstraRulesLabel)
	}
}
