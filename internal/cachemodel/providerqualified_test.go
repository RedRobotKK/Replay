package cachemodel

import "testing"

// A provider-qualified id prices from a foreign vendor's family substring.
//
// PIN, NOT A CONTRACT. This test records what the compiled table does today. It
// does not assert that the behaviour is correct, because the repository does not
// establish that it is wrong.
//
// What the repository does say, at match.go:27-30, is that substring matching
// leaves known failure modes standing: "`opus-5-preview` still prices as Opus 5,
// another provider's id carrying an Anthropic family name still matches ... The
// end state is ids matched whole, which is a rules-document format change and
// not this." unknownversion_test.go says the same. Those are admissions of a
// known limitation, deliberately deferred, not a requirement that the behaviour
// differ.
//
// The structural reason is in the types rather than the matcher: modelRow
// (anthropic.go:154) carries match, minPrefix, price and priced, and no provider
// dimension at all. There is nowhere for a vendor prefix to be represented, so
// `lookup` cannot consult one. foreignModel (anthropic.go:274) is not that
// guard either: it asks whether ANY Anthropic family name appears anywhere in
// the id, so "openai-claude-opus-5" contains "claude-opus" and is reported as
// NOT foreign.
//
// If a future change makes provider-qualified ids unpriced, this test is
// expected to fail. Rewrite it then, with the contract that justified the
// change. Do not weaken it to keep it green.
func TestProviderQualifiedIDPricesFromForeignFamilySubstring(t *testing.T) {
	// The positive case first. If this stops holding, the table moved and the
	// negative cases below say nothing about provider qualification.
	anthropic, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("claude-opus-5 is unpriced; the compiled table no longer holds the row " +
			"these cases are compared against, so this test proves nothing")
	}

	for _, id := range []string{
		"openai-claude-opus-5",
		"deepseek-claude-opus-5",
		"claude-opus-5-preview",
	} {
		got, gotOK := PriceFor(id)
		if !gotOK {
			t.Errorf("%s is now unpriced. That may well be an improvement, but it is a "+
				"behaviour change: this test pinned it as priced, and the contract that "+
				"justifies rejecting it belongs in the change that made it so", id)
			continue
		}
		if got.InputPerMTok != anthropic.InputPerMTok || got.OutputPerMTok != anthropic.OutputPerMTok {
			t.Errorf("%s priced at input=%v output=%v, want the Anthropic Opus 5 row "+
				"input=%v output=%v: the pin is that it takes the FOREIGN family's price, "+
				"not merely that it is priced at all",
				id, got.InputPerMTok, got.OutputPerMTok, anthropic.InputPerMTok, anthropic.OutputPerMTok)
		}
	}

	// An id with no family substring is unpriced, which is what makes the cases
	// above about the substring rather than about a permissive default.
	if _, ok := PriceFor("totally-unknown-model"); ok {
		t.Error("an id carrying no family name is priced, so the cases above are not " +
			"evidence about substring matching")
	}
}

// foreignModel does not distinguish a vendor prefix.
//
// Recorded because it is the function whose name most invites the assumption
// that it would. It reports whether the COMPILED table prices the family at all,
// and it is documented as the fallback for a machine with no rules document.
func TestForeignModelDoesNotSeeAVendorPrefix(t *testing.T) {
	if foreignModel("openai-claude-opus-5") {
		t.Error("openai-claude-opus-5 is reported foreign; this test records that it is " +
			"NOT, because the id contains an Anthropic family name")
	}
	if !foreignModel("deepseek-flash") {
		t.Error("deepseek-flash carries no Anthropic family name and should be foreign")
	}
}
