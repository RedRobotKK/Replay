package cachemodel

import (
	"strings"
	"testing"
)

// The comparison must detect a price move, name it, and never quietly adopt it.
//
// PriceTableVersion dates the compiled table and nothing in the tool could say
// whether it was still right — only how old it was. "73 days old" is a prompt
// to worry; "73 days old and still agrees with an independent source on all
// ten models" is an answer.

const oneModelDB = `{
  "claude-opus-5": {
    "litellm_provider": "anthropic",
    "input_cost_per_token": 5e-06,
    "output_cost_per_token": 2.5e-05,
    "cache_read_input_token_cost": 5e-07
  }
}`

func TestAgreementIsReportedAsAgreement(t *testing.T) {
	obs, err := ParseLiteLLMPrices([]byte(oneModelDB))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := obs["opus-5"]
	if !ok {
		t.Fatalf("opus-5 not parsed; got keys %v", keysOf(obs))
	}
	// Per-token to per-million, which is where a factor-of-a-million slip
	// would hide and produce a disagreement on every model at once.
	if got.InputPerMTok != 5 || got.OutputPerMTok != 25 {
		t.Errorf("scaling is wrong: got $%.4f in / $%.4f out per Mtok, want 5 and 25",
			got.InputPerMTok, got.OutputPerMTok)
	}
	res := CheckPrices(obs)
	for _, d := range res.Disagreements {
		if d.Model == "opus-5" {
			t.Errorf("reported a disagreement on matching prices: %+v", d)
		}
	}
}

// The point of the whole thing: a price that moved must surface, with both
// numbers, so a human decides.
func TestAPriceMoveIsCaught(t *testing.T) {
	moved := strings.Replace(oneModelDB, "5e-06", "7.5e-06", 1)
	obs, err := ParseLiteLLMPrices([]byte(moved))
	if err != nil {
		t.Fatal(err)
	}
	res := CheckPrices(obs)

	var found *PriceDisagreement
	for i := range res.Disagreements {
		if res.Disagreements[i].Model == "opus-5" && res.Disagreements[i].Field == "input" {
			found = &res.Disagreements[i]
		}
	}
	if found == nil {
		t.Fatal("a 50% input price rise was not reported")
	}
	if found.Ours == found.Theirs {
		t.Error("the disagreement does not carry both numbers, so nobody can judge it")
	}
	if found.SourceKey == "" {
		t.Error("the disagreement does not say which row was compared, so it cannot be chased")
	}
}

// A reseller's rate for the same model is a different product. Comparing
// against Bedrock or Vertex would report a disagreement that is really a
// different SKU, and that is the kind of false alarm that gets a check muted.
func TestResellerKeysAreNotCompared(t *testing.T) {
	db := `{
      "anthropic.claude-opus-5-v1:0": {"litellm_provider":"anthropic","input_cost_per_token":9e-06,"output_cost_per_token":4e-05},
      "vertex_ai/claude-opus-5":      {"litellm_provider":"anthropic","input_cost_per_token":9e-06,"output_cost_per_token":4e-05},
      "claude-opus-5@20260101":       {"litellm_provider":"anthropic","input_cost_per_token":9e-06,"output_cost_per_token":4e-05}
    }`
	obs, err := ParseLiteLLMPrices([]byte(db))
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Errorf("reseller and dated keys were treated as first-party rates: %v", keysOf(obs))
	}
}

// A model we price and the source does not name is an absence of evidence. It
// must not be silently dropped, and it must not be called a disagreement.
func TestUnknownModelsAreReportedSeparately(t *testing.T) {
	obs, err := ParseLiteLLMPrices([]byte(oneModelDB))
	if err != nil {
		t.Fatal(err)
	}
	res := CheckPrices(obs)
	if len(res.Unmatched) == 0 {
		t.Fatal("every priced model matched a one-entry database; the check is not looking")
	}
	for _, u := range res.Unmatched {
		for _, d := range res.Disagreements {
			if d.Model == u {
				t.Errorf("%s is both unmatched and disagreeing; absence of evidence "+
					"is not evidence of a difference", u)
			}
		}
	}
}

// Non-Anthropic rows must not leak in. The compiled table is first-party
// Anthropic pricing; a row from another provider under a similar name would
// disagree for a reason that has nothing to do with staleness.
func TestOtherProvidersAreIgnored(t *testing.T) {
	db := `{"claude-opus-5":{"litellm_provider":"openrouter","input_cost_per_token":9e-06,"output_cost_per_token":4e-05}}`
	obs, err := ParseLiteLLMPrices([]byte(db))
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Errorf("a non-Anthropic row was compared against the first-party table: %v", keysOf(obs))
	}
}

func keysOf(m map[string]PriceObservation) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
