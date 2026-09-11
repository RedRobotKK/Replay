package cachemodel

import "testing"

// A version this table has never heard of must not be priced as the version
// before it.
//
// Rows are matched by substring, so `claude-opus-4-9` contains `opus-4` and
// took the Opus 4 row: $15/$75, `priced` true, printed with the price table's
// date next to it as a documented list price. Every model in that family since
// 4.5 is $5/$25, so the next release of a family the table already knows is
// billed at three times its rate by an id nobody typed wrong.
//
// The direction is what makes this a money-path defect rather than an
// approximation. A bare `opus-4` row is the OLDEST and dearest member of its
// family, so the fallback overstates; `fable-5-2` falling back to `fable-5`
// takes ReadMult 0.10 where the 5.1 tier reads at 0.025, which overstates every
// cached token fourfold and is the exact bias unknownModel's comment says the
// unrecognised-model row exists to avoid.
//
// What it does not fix: a suffix that is not a version number. `opus-5-preview`
// still prices as Opus 5, and an id from another provider that happens to carry
// an Anthropic family name still matches. Those are substring matching's other
// failure modes and neither is a version-number collision.
func TestUnknownVersionIsNotPricedAsTheVersionBeforeIt(t *testing.T) {
	for _, id := range []string{
		"claude-opus-4-9",   // no such row; contains `opus-4` at $15/$75
		"claude-opus-4-10",  // contains `opus-4-1` as a prefix of its own number
		"claude-sonnet-4-7", // contains `sonnet-4` at $3/$15
		"claude-fable-5-2",  // contains `fable-5`, whose ReadMult is the older tier
		"claude-opus-4-50",  // contains `opus-4-5`, same shape one family over
	} {
		p, priced := PriceFor(id)
		if priced {
			t.Errorf("PriceFor(%q) = $%.2f/$%.2f, priced: this table has no row for that "+
				"version, and pricing it from a shorter row's substring publishes a dollar "+
				"figure for a model nobody has read a price for", id, p.InputPerMTok, p.OutputPerMTok)
		}
	}
}

// The guard must not disqualify the ids the table is actually for.
//
// Every first-party id carries a build date, which is a digit run after the
// family name exactly where a version component would be. Rejecting those would
// unprice the whole corpus, so the rule turns on length: a version component is
// short, and YYYYMMDD is eight digits.
func TestDatedModelIDsStillPrice(t *testing.T) {
	cases := []struct {
		id    string
		input float64
	}{
		{"claude-opus-4-8", 5},
		{"claude-opus-4-20250514", 15},
		{"claude-sonnet-4-5-20250929", 3},
		{"claude-haiku-4-5-20251001", 1},
		{"claude-3-5-haiku-20241022", 0.80},
		{"claude-opus-5", 5},
		{"claude-fable-5-1", 10},
		// Opus 4.1 is the id this guard nearly unpriced. It had no row of its
		// own and reached $15/$75 through `opus-4` containing it, which is the
		// accidental half of the same mechanism. It has a row now.
		{"claude-opus-4-1", 15},
		{"claude-opus-4-1-20250805", 15},
	}
	for _, c := range cases {
		p, priced := PriceFor(c.id)
		if !priced || p.InputPerMTok != c.input {
			t.Errorf("PriceFor(%q) = $%.2f priced=%v, want $%.2f priced: this is an id the "+
				"table is written for", c.id, p.InputPerMTok, priced, c.input)
		}
	}
}

// A rules document is matched the same way, so it inherits the same defect.
//
// The document is the path that exists precisely so a price correction does not
// need a binary release. A feed row reading `opus-4` pricing the unreleased
// `opus-4-9` would spread one wrong figure to every machine that installed it.
func TestLoadedRulesDoNotPriceAnUnknownVersion(t *testing.T) {
	r := &Rules{
		Schema:  RulesSchema,
		Version: "test-1",
		Models: []ModelRule{
			{Match: "opus-4", MinPrefix: 1024, InputPerMTok: 15, OutputPerMTok: 75, ReadMult: 0.1, Priced: true},
		},
	}
	if err := r.validate(); err != nil {
		t.Fatalf("fixture is not a valid document: %v", err)
	}
	defer Override(r)()

	if p, priced := PriceFor("claude-opus-4-9"); priced {
		t.Errorf("a loaded row for %q priced %q at $%.2f: a feed exists to correct prices, "+
			"not to extend one to a model it never mentioned", "opus-4", "claude-opus-4-9", p.InputPerMTok)
	}
	if p, priced := PriceFor("claude-opus-4-20250514"); !priced || p.InputPerMTok != 15 {
		t.Errorf("the loaded row stopped pricing the dated id it is for: $%.2f priced=%v", p.InputPerMTok, priced)
	}
}
