package cachemodel

import (
	"math"
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

// A cache-read price that disagrees must be reported.
//
// Reported by roy-tong in issue #54, and the diagnosis was exact:
// CacheReadPerMTo was parsed from LiteLLM and never compared. Its only two
// appearances in the repository were the field declaration and the line that
// filled it. The comparison loop built entries for input and output and stopped.
//
// So "No disagreement" was silent on the cache-read price, which the cost model
// leans on more than either of the two it did check: a cached read is the
// cheapest token in the system and the one whose multiplier decides whether a
// break is worth anything. A check that cannot fail on the field that matters
// most is worse than no check, because the output says the prices were verified.
//
// The reason it went unwritten is visible in the types and is not an excuse.
// LiteLLM states an absolute cache-read price per million tokens; this table
// states a multiplier of input. Comparing them needs a conversion, and the
// conversion is one line.
const cacheReadDisagreesDB = `{
  "claude-opus-5": {
    "litellm_provider": "anthropic",
    "input_cost_per_token": 5e-06,
    "output_cost_per_token": 2.5e-05,
    "cache_read_input_token_cost": 2e-06
  }
}`

func TestCacheReadDisagreementIsReported(t *testing.T) {
	obs, err := ParseLiteLLMPrices([]byte(cacheReadDisagreesDB))
	if err != nil {
		t.Fatal(err)
	}
	got := obs["opus-5"]
	// $2.00 per Mtok against our $5.00 input times the read multiplier. If this
	// fixture ever stops disagreeing the test below proves nothing.
	ours := got.InputPerMTok * ReadMultiplier
	if math.Abs(got.CacheReadPerMTo-ours) <= priceTolerance {
		t.Fatalf("the fixture agrees (%v vs %v), so this test cannot fail",
			got.CacheReadPerMTo, ours)
	}

	res := CheckPrices(obs)
	var found *PriceDisagreement
	for i := range res.Disagreements {
		if res.Disagreements[i].Model == "opus-5" && res.Disagreements[i].Field == "cache read" {
			found = &res.Disagreements[i]
		}
	}
	if found == nil {
		t.Fatalf("a cache-read price of $%.2f per Mtok against our $%.2f was not "+
			"reported. --check-prices would print no disagreement over the field "+
			"the cost model leans on hardest. Fields reported: %v",
			got.CacheReadPerMTo, ours, fieldsOf(res.Disagreements))
	}
	if found.Theirs != got.CacheReadPerMTo {
		t.Errorf("reported their price as %v, parsed %v", found.Theirs, got.CacheReadPerMTo)
	}
	if math.Abs(found.Ours-ours) > priceTolerance {
		t.Errorf("reported our price as %v, want %v (input times the read multiplier)",
			found.Ours, ours)
	}
}

// An agreeing cache-read price must not be reported.
func TestCacheReadAgreementIsNotReported(t *testing.T) {
	obs, err := ParseLiteLLMPrices([]byte(oneModelDB))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range CheckPrices(obs).Disagreements {
		if d.Model == "opus-5" && d.Field == "cache read" {
			t.Errorf("reported a disagreement on a cache-read price that matches: %+v", d)
		}
	}
}

func fieldsOf(ds []PriceDisagreement) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Field)
	}
	return out
}
