package cachemodel

import (
	"strings"
	"testing"
)

// The digest exists because of #284: two builds priced one corpus at $4,088.49
// and $11,969.37 on the same day, from the same directory, and both stamped
// rulesVersion "anthropic-2026-09-01". The label was not lying. The provider's
// rule document really had not changed; what changed was the code applying it,
// when six models the older build left unpriced became priced.
//
// So rulesVersion keeps its published meaning and this is a second, different
// fact: not "which rules did the provider publish" but "which numbers did this
// binary actually price with".
//
// EVERY TEST BELOW MUTATES A PRICING INPUT AND REQUIRES THE DIGEST TO MOVE.
// That is the whole point. A digest that cannot change is not evidence of
// anything, and a digest that covers only the fields somebody remembered to
// include is a guarantee with a hole in it shaped like the next defect. These
// tests are the proof that the hole is not there, and each one names the
// dollar consequence of the input it moves.

// withUnknown swaps the unknown-model fallback for one test and puts it back.
// The table itself already has withTable in anthropic_test.go; this is its
// other half, and the two are separate because the fallback is a separate
// input with its own defect history (#271, #253).
func withUnknown(t *testing.T, row modelRow) {
	t.Helper()
	prev := unknownModel
	unknownModel = row
	t.Cleanup(func() { unknownModel = prev })
}

// withPricing swaps both at once, for the tests that need to hold the table
// still while moving the fallback or the reverse.
func withPricing(t *testing.T, rows []modelRow, unknown modelRow) {
	t.Helper()
	withTable(t, rows)
	withUnknown(t, unknown)
}

// snapshot copies the live table so a test can mutate one field of it without
// aliasing the original.
func snapshot() ([]modelRow, modelRow) {
	rows := make([]modelRow, len(modelTable))
	copy(rows, modelTable)
	return rows, unknownModel
}

// PD1: the digest is a function of the inputs and nothing else.
//
// PASS: two calls with nothing changed agree.
// FAIL: a digest that moves on its own puts a different string on every
// submission, and a pool that grouped by it would report one build per row.
func TestPD1_TheDigestIsStableWhileTheInputsAre(t *testing.T) {
	first := PricingDigest()
	for i := 0; i < 32; i++ {
		if got := PricingDigest(); got != first {
			t.Fatalf("call %d returned %q, first call returned %q", i, got, first)
		}
	}
	if first == "" {
		t.Fatal("the digest is empty, which would stamp every submission with the same nothing")
	}
}

// PD2: a price change moves it.
//
// This is the ordinary case. A provider drops the price of a model, the table
// changes, every dollar figure computed after it differs from every one before.
//
// PASS: the digest differs.
// FAIL: two submissions priced at different rates are indistinguishable, which
// is the pooled-figure defect in its plainest form.
func TestPD2_APriceChangeMovesTheDigest(t *testing.T) {
	before := PricingDigest()

	rows, unknown := snapshot()
	rows[0].price.InputPerMTok += 1
	withPricing(t, rows, unknown)

	if after := PricingDigest(); after == before {
		t.Errorf("changing %s input price from %v did not move the digest (%q both times).\n"+
			"Every dollar figure this build produces would differ from the previous build's, "+
			"and a pool would add them together under one name.",
			modelTable[0].match, modelTable[0].price.InputPerMTok-1, before)
	}
}

// PD3: the output price moves it too.
//
// Input and output are separate multipliers on separate token counts. A digest
// that covered one and not the other would be silent on exactly half of a
// price change.
func TestPD3_AnOutputPriceChangeMovesTheDigest(t *testing.T) {
	before := PricingDigest()

	rows, unknown := snapshot()
	rows[0].price.OutputPerMTok += 1
	withPricing(t, rows, unknown)

	if after := PricingDigest(); after == before {
		t.Errorf("changing an output price did not move the digest (%q both times)", before)
	}
}

// PD4: the cache-read multiple moves it.
//
// This one is not decoration. ReadMultiplier is what the whole tool turns on:
// it decides EffectiveTokens, which decides the policy comparison, which is
// reported as measured rather than estimated. The 0.10 and 0.025 tiers differ
// fourfold, and the bias has a direction.
func TestPD4_TheCacheReadMultipleMovesTheDigest(t *testing.T) {
	before := PricingDigest()

	rows, unknown := snapshot()
	rows[0].price.ReadMult = rows[0].price.ReadMult / 4
	withPricing(t, rows, unknown)

	if after := PricingDigest(); after == before {
		t.Errorf("changing a cache-read multiple did not move the digest (%q both times).\n"+
			"This is the number the advice turns on; a build that priced reads "+
			"fourfold differently would pool as the same build.", before)
	}
}

// PD5: flipping a row from unpriced to priced moves it.
//
// THIS IS #284 EXACTLY. The 0.5.4 build left six models unpriced and totalled
// $4,088.49; the main build priced them and totalled $11,969.37. Not one price
// changed. The set of models this build is willing to price changed, and
// nothing in the submission said so.
func TestPD5_PricingAModelThatWasUnpricedMovesTheDigest(t *testing.T) {
	rows, unknown := snapshot()
	var idx = -1
	for i, r := range rows {
		if !r.priced {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Skip("no unpriced row in the table to flip; this test needs one to be meaningful")
	}

	withPricing(t, rows, unknown)
	before := PricingDigest()

	flipped, unknown2 := snapshot()
	flipped[idx].priced = true
	flipped[idx].price = Price{3, 15, ReadMultiplier}
	withPricing(t, flipped, unknown2)

	if after := PricingDigest(); after == before {
		t.Errorf("pricing the previously-unpriced %q did not move the digest (%q both times).\n"+
			"This is defect #284: the same corpus totalled $4,088.49 and $11,969.37 "+
			"under one rulesVersion because the priced set changed and nothing recorded it.",
			rows[idx].match, before)
	}
}

// PD6: the minimum cacheable prefix moves it.
//
// A floor decides whether a prefix caches at all. Get it wrong and the tool
// recommends caching something that cannot be cached, which changes the
// avoidable figure without changing any price.
func TestPD6_TheMinimumPrefixMovesTheDigest(t *testing.T) {
	before := PricingDigest()

	rows, unknown := snapshot()
	rows[0].minPrefix *= 2
	withPricing(t, rows, unknown)

	if after := PricingDigest(); after == before {
		t.Errorf("changing a minimum cacheable prefix did not move the digest (%q both times)", before)
	}
}

// PD7: the unknown-model fallback moves it.
//
// This is the other half of #284. #271 and #253 made the unknown-model multiple
// a named rule and moved it. Nothing in the table changed; what an unrecognised
// model costs changed, and unrecognised models are exactly what a contributed
// corpus is full of.
func TestPD7_TheUnknownModelFallbackMovesTheDigest(t *testing.T) {
	before := PricingDigest()

	rows, unknown := snapshot()
	unknown.price.ReadMult = unknown.price.ReadMult / 4
	withPricing(t, rows, unknown)

	if after := PricingDigest(); after == before {
		t.Errorf("changing the unknown-model read multiple did not move the digest (%q both times).\n"+
			"An unrecognised model is the common case in a contributed corpus, "+
			"and this is what it costs.", before)
	}
}

// PD8: adding a row moves it, and so does removing one.
//
// A model added to the table is a model that stops being unpriced, which is a
// total that stops excluding it.
func TestPD8_AddingOrRemovingARowMovesTheDigest(t *testing.T) {
	before := PricingDigest()

	rows, unknown := snapshot()
	grown := append(rows, modelRow{"a-model-that-does-not-exist", minPrefixStandard, Price{7, 21, ReadMultiplier}, true})
	withPricing(t, grown, unknown)
	grownDigest := PricingDigest()
	if grownDigest == before {
		t.Errorf("adding a priced row did not move the digest (%q both times)", before)
	}

	shrunk, unknown2 := snapshot()
	shrunk = shrunk[:len(shrunk)-1]
	withPricing(t, shrunk, unknown2)
	if after := PricingDigest(); after == grownDigest {
		t.Errorf("removing a row did not move the digest (%q both times)", grownDigest)
	}
}

// PD9: order is part of the answer.
//
// Rows are matched by substring in order, so "sonnet-4-5" must be tried before
// "sonnet". Swapping two rows changes which row a model id resolves to without
// changing any number in the table, and that reprices the corpus.
//
// PASS: the digest differs after a swap.
// FAIL: a digest built from a sorted or set-like view of the table, which
// would be blind to the one kind of edit that needs no number to change.
func TestPD9_ReorderingTheTableMovesTheDigest(t *testing.T) {
	rows, unknown := snapshot()
	if len(rows) < 2 {
		t.Skip("need two rows to swap")
	}
	withPricing(t, rows, unknown)
	before := PricingDigest()

	swapped, unknown2 := snapshot()
	swapped[0], swapped[1] = swapped[1], swapped[0]
	withPricing(t, swapped, unknown2)

	if after := PricingDigest(); after == before {
		t.Errorf("swapping two rows did not move the digest (%q both times).\n"+
			"Rows are matched by substring in order, so order decides which row "+
			"a model id resolves to and therefore what it costs.", before)
	}
}

// PD10: the shape is short, printable, and says what it is.
//
// It goes next to rulesVersion on a report and into a JSON submission, so it
// has to survive both without quoting, and a reader who has never seen one
// should be able to tell it apart from the rules label beside it.
func TestPD10_TheDigestIsShortAndPrintable(t *testing.T) {
	d := PricingDigest()
	if !strings.HasPrefix(d, "p") {
		t.Errorf("digest %q does not start with the p that marks it a pricing digest, "+
			"which is what stops a reader reading it as a rules version", d)
	}
	if len(d) < 8 || len(d) > 20 {
		t.Errorf("digest %q is %d characters; it sits beside rulesVersion on a "+
			"terminal report and in every submission", d, len(d))
	}
	for _, r := range d {
		if !strings.ContainsRune("0123456789abcdefp", r) {
			t.Errorf("digest %q contains %q, which is not lowercase hex", d, r)
		}
	}
}

// PD11: the digest does not leak into the rules label, and the rules label
// does not absorb the digest.
//
// They answer different questions and a reader has to be able to ask each one
// separately. Folding the digest into RulesVersion would redefine a published
// field underneath the people already quoting it, which is the same mistake
// the MatchRate entry in the changelog refuses to make.
func TestPD11_TheRulesVersionIsNotRewrittenByThis(t *testing.T) {
	if strings.Contains(RulesVersion, PricingDigest()) {
		t.Error("RulesVersion now contains the pricing digest. It names the provider's " +
			"published rule document, and that document did not change when our code did.")
	}
	if RulesVersion != "anthropic-2026-09-01" {
		t.Errorf("RulesVersion is %q; it is quoted on the website and in published "+
			"evidence files, so moving it is a decision with an audience, not a refactor",
			RulesVersion)
	}
}

// PD12: a loaded rules document moves the digest.
//
// THIS IS THE DEFECT THE FIRST ELEVEN TESTS DID NOT CATCH. Replay does not
// always price from the compiled table. `replay rules` loads a document that
// Override installs for the process, activeRow consults it before the table,
// and every dollar figure after that comes from the loaded numbers. A digest
// that read only the compiled table would name numbers the report did not use,
// which is a worse failure than the one it was built to fix: #284 left a reader
// unable to tell two builds apart, and this would actively tell them the wrong
// thing.
//
// PASS: installing a document changes the digest.
// FAIL: a submission priced from a fetched rules feed is stamped with the
// digest of the table it ignored.
func TestPD12_ALoadedRulesDocumentMovesTheDigest(t *testing.T) {
	compiled := PricingDigest()

	restore := Override(&Rules{
		Schema:  RulesSchema,
		Version: "loaded-2026-09-12",
		Models: []ModelRule{
			{Match: "sonnet-5", MinPrefix: 1024, InputPerMTok: 2, OutputPerMTok: 10, ReadMult: 0.1, Priced: true},
		},
	})
	defer restore()

	if loaded := PricingDigest(); loaded == compiled {
		t.Errorf("a loaded rules document did not move the digest (%q both times).\n"+
			"Every figure produced under it comes from the document, not the compiled "+
			"table, and the submission would name the table anyway.", compiled)
	}
}

// PD13: two different loaded documents differ from each other.
//
// PD12 only proves the digest notices that SOME document is loaded. If it
// stopped there, every fetched rules feed would share one digest and the
// pooling defect would simply move house.
func TestPD13_DifferentLoadedDocumentsDiffer(t *testing.T) {
	restore := Override(&Rules{
		Schema:  RulesSchema,
		Version: "loaded-a",
		Models: []ModelRule{
			{Match: "sonnet-5", MinPrefix: 1024, InputPerMTok: 2, OutputPerMTok: 10, ReadMult: 0.1, Priced: true},
		},
	})
	a := PricingDigest()
	restore()

	restore = Override(&Rules{
		Schema:  RulesSchema,
		Version: "loaded-b",
		Models: []ModelRule{
			{Match: "sonnet-5", MinPrefix: 1024, InputPerMTok: 3, OutputPerMTok: 15, ReadMult: 0.1, Priced: true},
		},
	})
	defer restore()
	b := PricingDigest()

	if a == b {
		t.Errorf("two rules documents pricing sonnet-5 at $2/$10 and $3/$15 produced "+
			"the same digest (%q), so a pool could not tell them apart", a)
	}
}

// PD14: the account discount moves it.
//
// AccountDiscount multiplies every input and output price on the way out. It is
// the single field in the whole system that can change every dollar figure at
// once, and it is the one Replay cannot observe: the operator declares it. A
// submission priced at a negotiated rate that claimed the list-price digest
// would be pooling a private rate into a public list-price aggregate.
func TestPD14_TheAccountDiscountMovesTheDigest(t *testing.T) {
	models := []ModelRule{
		{Match: "sonnet-5", MinPrefix: 1024, InputPerMTok: 2, OutputPerMTok: 10, ReadMult: 0.1, Priced: true},
	}

	restore := Override(&Rules{Schema: RulesSchema, Version: "v", Models: models})
	list := PricingDigest()
	restore()

	restore = Override(&Rules{Schema: RulesSchema, Version: "v", Models: models, AccountDiscount: 0.8})
	defer restore()
	discounted := PricingDigest()

	if list == discounted {
		t.Errorf("a 20%% negotiated discount did not move the digest (%q both times).\n"+
			"Every figure under it is 80%% of the list-price figure, and a pool would "+
			"add it to list-price submissions as though it were one.", list)
	}
}

// PD15: removing the document puts the digest back.
//
// Override returns a restore function, and `replay rules --dry-run` uses it to
// price under a candidate document and then go back. If the digest did not
// return to its compiled value, a dry run would leave the process claiming a
// pricing identity it no longer has.
func TestPD15_RemovingTheDocumentRestoresTheDigest(t *testing.T) {
	before := PricingDigest()

	restore := Override(&Rules{
		Schema:  RulesSchema,
		Version: "temporary",
		Models:  []ModelRule{{Match: "sonnet-5", MinPrefix: 1024, InputPerMTok: 9, OutputPerMTok: 9, ReadMult: 0.5, Priced: true}},
	})
	if during := PricingDigest(); during == before {
		t.Fatal("the document did not take effect, so this test proves nothing")
	}
	restore()

	if after := PricingDigest(); after != before {
		t.Errorf("after the document was removed the digest is %q, was %q", after, before)
	}
}
