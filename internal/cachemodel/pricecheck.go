package cachemodel

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// Checking the price table against somebody else's, and calling the result a
// disagreement rather than a correction.
//
// Every dollar Replay prints comes from a table compiled into the binary and
// dated by PriceTableVersion. Prices move faster than releases, and on
// 2026-09-05 that table was 73 days old with nothing in the tool able to say
// whether it was still right — only how old it was.
//
// This does not fix that by trusting a feed. A community price database is a
// second observer, not an authority: it can be stale, wrong, or describing a
// different SKU under a similar name. So the output is a comparison, never an
// update. `documented` is what we compiled, `observed` is what the other
// source says, and where they differ the answer is that they differ — the same
// shape as Claim, and for the same reason.
//
// The engine has no network code and is not getting any. The caller fetches
// the bytes; this only compares them.

// PriceObservation is one model's prices as another source reports them.
type PriceObservation struct {
	Model string
	// Source is the key the other database used, kept verbatim so a
	// disagreement can be chased without guessing which row was compared.
	SourceKey       string
	InputPerMTok    float64
	OutputPerMTok   float64
	CacheReadPerMTo float64
}

// PriceDisagreement is one model where the two sources do not match.
type PriceDisagreement struct {
	Model     string
	SourceKey string
	Field     string
	Ours      float64
	Theirs    float64
}

// PriceCheck is the whole comparison.
type PriceCheck struct {
	TableVersion string
	Compared     []string
	// Unmatched are models we price and the other source does not name. Not a
	// disagreement: an absence of evidence, reported as one.
	Unmatched     []string
	Disagreements []PriceDisagreement
}

// priceTolerance is how far two sources may differ before it is worth a
// human's attention. Floating point round-trips through JSON and per-token
// figures multiplied to per-million introduce noise well below a tenth of a
// cent, and reporting that as a price change would train the reader to ignore
// this command.
const priceTolerance = 0.005

// ParseLiteLLMPrices reads the community model_prices_and_context_window.json
// shape into observations keyed by the bare model name.
//
// It deliberately keeps only bare `claude-<model>` keys. The same model appears
// under Bedrock and Vertex names at different prices, and comparing our
// first-party table against a reseller's rate would report a disagreement that
// is really a different product.
func ParseLiteLLMPrices(raw []byte) (map[string]PriceObservation, error) {
	var db map[string]struct {
		Input     *float64 `json:"input_cost_per_token"`
		Output    *float64 `json:"output_cost_per_token"`
		CacheRead *float64 `json:"cache_read_input_token_cost"`
		Provider  string   `json:"litellm_provider"`
	}
	if err := json.Unmarshal(raw, &db); err != nil {
		return nil, fmt.Errorf("parse price database: %w", err)
	}
	out := map[string]PriceObservation{}
	for key, v := range db {
		if v.Input == nil || v.Output == nil {
			continue
		}
		if v.Provider != "anthropic" {
			continue
		}
		name, ok := bareClaudeName(key)
		if !ok {
			continue
		}
		// A dated variant and its alias both map to the same bare name. Keep
		// the shorter key: it is the alias, which is what a client sends.
		if prev, seen := out[name]; seen && len(prev.SourceKey) <= len(key) {
			continue
		}
		const perM = 1_000_000
		obs := PriceObservation{
			Model:         name,
			SourceKey:     key,
			InputPerMTok:  *v.Input * perM,
			OutputPerMTok: *v.Output * perM,
		}
		if v.CacheRead != nil {
			obs.CacheReadPerMTo = *v.CacheRead * perM
		}
		out[name] = obs
	}
	return out, nil
}

// bareClaudeName reduces "claude-opus-5" to "opus-5" and rejects reseller keys.
func bareClaudeName(key string) (string, bool) {
	for _, c := range key {
		if c == '/' || c == '.' || c == '@' || c == ':' {
			return "", false
		}
	}
	const prefix = "claude-"
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return "", false
	}
	return key[len(prefix):], true
}

// CheckPrices compares the compiled table against observations.
func CheckPrices(obs map[string]PriceObservation) PriceCheck {
	res := PriceCheck{TableVersion: PriceTableVersion}
	for _, m := range modelTable {
		if !m.priced {
			continue
		}
		o, ok := obs[m.match]
		if !ok {
			res.Unmatched = append(res.Unmatched, m.match)
			continue
		}
		res.Compared = append(res.Compared, m.match)
		for _, f := range []struct {
			name         string
			ours, theirs float64
		}{
			{"input", m.price.InputPerMTok, o.InputPerMTok},
			{"output", m.price.OutputPerMTok, o.OutputPerMTok},
		} {
			if math.Abs(f.ours-f.theirs) > priceTolerance {
				res.Disagreements = append(res.Disagreements, PriceDisagreement{
					Model: m.match, SourceKey: o.SourceKey, Field: f.name,
					Ours: f.ours, Theirs: f.theirs,
				})
			}
		}
	}
	sort.Strings(res.Compared)
	sort.Strings(res.Unmatched)
	sort.Slice(res.Disagreements, func(i, j int) bool {
		if res.Disagreements[i].Model != res.Disagreements[j].Model {
			return res.Disagreements[i].Model < res.Disagreements[j].Model
		}
		return res.Disagreements[i].Field < res.Disagreements[j].Field
	})
	return res
}
