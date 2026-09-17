package cachemodel

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The shipped OpenAI rows are pinned against the vendor page they came from.
//
// Read at developers.openai.com/api/docs/pricing on 2026-09-17:
//
//	gpt-6-astra    input $10.00  output $50.00  cached input $1.00
//	gpt-5.6-terra  input  $2.00  output $12.00  cached input $0.20
//
// Cached input divided by input is the read multiplier: 0.1 for both. This
// test does not fetch anything; it holds the document to the figures somebody
// actually read, so a later edit has to disagree with a dated reading rather
// than with nothing.
func TestTheOpenAIRowsMatchTheVendorPageTheyCameFrom(t *testing.T) {
	rows := openAIDoc(t)
	for _, want := range []struct {
		match             string
		in, out, readMult float64
	}{
		{"gpt-6-astra", 10.0, 50.0, 0.1},
		{"gpt-5.6-terra", 2.0, 12.0, 0.1},
	} {
		got, ok := rows[want.match]
		if !ok {
			t.Errorf("row %q is gone from the document", want.match)
			continue
		}
		if got.InputPerMTok != want.in || got.OutputPerMTok != want.out || got.ReadMult != want.readMult {
			t.Errorf("row %q = in %v out %v read x%v, want in %v out %v read x%v (vendor page, 2026-09-17)",
				want.match, got.InputPerMTok, got.OutputPerMTok, got.ReadMult,
				want.in, want.out, want.readMult)
		}
	}
}

// A row claiming a price must carry one.
func TestEveryPricedOpenAIRowCarriesRates(t *testing.T) {
	for match, r := range openAIDoc(t) {
		if !r.Priced {
			continue
		}
		if r.InputPerMTok <= 0 || r.OutputPerMTok <= 0 || r.ReadMult <= 0 {
			t.Errorf("row %q claims priced with in %v out %v read x%v",
				match, r.InputPerMTok, r.OutputPerMTok, r.ReadMult)
		}
	}
}

// No row may shadow a longer one that contains it.
//
// Rows match by substring in order, so a row named "astra" placed before
// "gpt-6-astra" would swallow the flagship and price it at the shorter row's
// rates. That is a silent mis-price, which is worse than the unpriced model it
// would be trying to fix.
//
// This is a standing guard rather than a reaction to a defect: no such row
// exists today. It exists because adding one is the obvious way to cover the
// bare "astra" and "astra-medium" ids seen once each in a real corpus, and the
// obvious way is wrong unless the shorter row goes last.
func TestNoOpenAIRowShadowsALongerOne(t *testing.T) {
	var doc struct {
		Models []struct {
			Match string `json:"match"`
		} `json:"models"`
	}
	b, err := os.ReadFile("../../docs/rules/openai-2026-09-15.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for i, short := range doc.Models {
		for j, long := range doc.Models {
			if i < j && len(short.Match) < len(long.Match) &&
				strings.Contains(long.Match, short.Match) {
				t.Errorf("row %q at index %d precedes %q at index %d and shadows it",
					short.Match, i, long.Match, j)
			}
		}
	}
}

type oaRow struct {
	InputPerMTok  float64 `json:"inputPerMTok"`
	OutputPerMTok float64 `json:"outputPerMTok"`
	ReadMult      float64 `json:"readMult"`
	Priced        bool    `json:"priced"`
	Match         string  `json:"match"`
}

func openAIDoc(t *testing.T) map[string]oaRow {
	t.Helper()
	b, err := os.ReadFile("../../docs/rules/openai-2026-09-15.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Models []oaRow `json:"models"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	m := map[string]oaRow{}
	for _, r := range doc.Models {
		m[r.Match] = r
	}
	return m
}
