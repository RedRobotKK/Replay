package cachemodel

import (
	"strings"
	"testing"
)

// Replay reads four providers and could price one.
//
// internal/transcript ships readers for Claude Code, Codex, OpenAI and Ollama.
// `replay burn` counts all of them — on the corpus this was written against,
// 610,551,532 Codex tokens beside 16,468,741,252 Claude Code tokens. The rules
// document could carry a price for exactly one of those, because Provider was a
// single string on the document: one document, one provider, and nowhere to put
// a second.
//
// That is a schema limit, not an unfilled table. These tests are about the
// schema.

func multi() *Rules {
	return &Rules{
		Schema: RulesSchema, Version: "multi-test", Provider: "anthropic",
		Models: []ModelRule{
			{Match: "opus-5", MinPrefix: 1024, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true},
			{Match: "gpt-5-codex", Provider: "openai", MinPrefix: 1024,
				InputPerMTok: 1.25, OutputPerMTok: 10, ReadMult: 0.1, Priced: true},
		},
	}
}

// MP1: a row may name its own provider, and the document's is the default.
func TestMP1_ARowCarriesItsOwnProvider(t *testing.T) {
	r := multi()
	if err := r.validate(); err != nil {
		t.Fatalf("a two-provider document is refused: %v", err)
	}
	if got := r.Models[0].ProviderOr(r); got != "anthropic" {
		t.Errorf("a row with no provider reports %q, want the document's", got)
	}
	if got := r.Models[1].ProviderOr(r); got != "openai" {
		t.Errorf("a row naming openai reports %q", got)
	}
}

// MP2: and the document says which providers it covers, so a report can name
// them rather than claim one.
//
// Every figure Replay prints carries the rules version. On a multi-provider
// document "anthropic-2026-09-01" beside a Codex dollar figure is a provenance
// line that names the wrong publisher.
func TestMP2_TheDocumentNamesEveryProviderItPrices(t *testing.T) {
	got := multi().Providers()
	if len(got) != 2 {
		t.Fatalf("a two-provider document reports %d providers: %v", len(got), got)
	}
	if got[0] != "anthropic" || got[1] != "openai" {
		t.Errorf("providers are %v, want them sorted and complete", got)
	}
	// One provider stays one provider, so every existing document is unchanged.
	single := &Rules{Schema: RulesSchema, Version: "v", Provider: "anthropic",
		Models: []ModelRule{{Match: "opus-5", Priced: true, InputPerMTok: 5, ReadMult: 0.1}}}
	if p := single.Providers(); len(p) != 1 || p[0] != "anthropic" {
		t.Errorf("a single-provider document reports %v", p)
	}
}

// MP3: mixed attribution with no default is refused; the legacy shape is not.
//
// The failure this prevents is silent. In a document where one row says
// "openai" and another says nothing, with no document-level provider, the bare
// row belongs to nobody — and a report carrying its dollar figures cannot say
// whose published numbers produced them.
//
// What must NOT be refused is the shape every document had before rows could
// carry a provider at all: no document provider, no row providers. Requiring
// attribution there would reject every rules file already installed, which is
// a format break dressed as a validation improvement.
func TestMP3_MixedAttributionWithNoDefaultIsRefused(t *testing.T) {
	mixed := &Rules{Schema: RulesSchema, Version: "v",
		Models: []ModelRule{
			{Match: "gpt-5-codex", Provider: "openai", Priced: true, InputPerMTok: 1.25, ReadMult: 0.1},
			{Match: "opus-5", Priced: true, InputPerMTok: 5, ReadMult: 0.1},
		}}
	err := mixed.validate()
	if err == nil {
		t.Fatal("a row belonging to nobody was accepted beside one that names its publisher")
	}
	if !strings.Contains(err.Error(), "provider") {
		t.Errorf("the error does not mention the provider: %v", err)
	}

	legacy := &Rules{Schema: RulesSchema, Version: "v",
		Models: []ModelRule{{Match: "opus-5", Priced: true, InputPerMTok: 5, ReadMult: 0.1}}}
	if err := legacy.validate(); err != nil {
		t.Errorf("the shape every existing rules document has was refused: %v", err)
	}
}

// MP4: pricing is by row, so a Codex model gets Codex's read multiple.
//
// The read multiple is the term that decides which layout the comparison calls
// cheaper. Falling back to another provider's is the defect the rules-staleness
// notice exists to warn about, with the provider swapped for the date.
func TestMP4_EachProviderKeepsItsOwnReadMultiple(t *testing.T) {
	r := multi()
	r.Models[1].ReadMult = 0.5
	restore := Override(r)
	defer restore()

	if got := ReadMultiplierFor("gpt-5-codex"); got != 0.5 {
		t.Errorf("a Codex model reads at %v, want its own row's 0.5", got)
	}
	if got := ReadMultiplierFor("claude-opus-5"); got != 0.1 {
		t.Errorf("an Anthropic model reads at %v, want 0.1", got)
	}
}

// MP5: a priced foreign model is priced, and an unpriced one still is not.
//
// EffectiveTokens dropped the cache terms for any model whose id did not carry
// an Anthropic family name, because the table could not know which counting
// convention applied. It is known now: transcript.Usage is defined as
// exclusive (PromptTotal = Input + CacheCreation + CacheRead) and every reader
// normalises into it — internal/transcript/codex.go:186 and openai.go:63 both
// subtract the cached share at parse time. So the guard's remaining job is to
// refuse models nothing prices, not models nobody named claude.
func TestMP5_APricedForeignModelGetsItsCacheArithmetic(t *testing.T) {
	restore := Override(multi())
	defer restore()

	u := usage(1000, 0, 4000, 0)
	got := EffectiveTokens(u, "gpt-5-codex")
	want := 1000 + 4000*0.1
	if got != want {
		t.Errorf("a priced Codex request is %v effective tokens, want %v. The cache "+
			"read was dropped, which is the whole reason the figure exists", got, want)
	}
	// And a model nothing prices still gets input alone: no row, no convention,
	// no arithmetic.
	if got := EffectiveTokens(u, "some-unknown-model-9"); got != 1000 {
		t.Errorf("an unpriced model is %v effective tokens, want its input alone", got)
	}
}

// MP6: the accessors survive a nil document.
//
// Reported by guard-reachability. Both are called from report paths that run
// before a rules file has been loaded — Provenance already returns the compiled
// version for a nil receiver — so a nil here is the ordinary first-run state,
// not a defensive flourish.
func TestMP6_TheAccessorsSurviveNoDocument(t *testing.T) {
	var none *Rules
	if got := none.Providers(); got != nil {
		t.Errorf("a nil document reports providers %v", got)
	}
	if got := (ModelRule{}).ProviderOr(nil); got != "" {
		t.Errorf("a bare row against no document reports provider %q", got)
	}
	if got := (ModelRule{Provider: "openai"}).ProviderOr(nil); got != "openai" {
		t.Errorf("a row naming its own provider loses it when the document is nil: %q", got)
	}
}

// MP7: Providers deduplicates and skips rows attributable to nobody.
//
// A document may legitimately carry many rows per publisher, and the list is
// what a report prints; repeating "anthropic" four times in a provenance line
// is noise, and printing an empty name is worse than printing nothing.
func TestMP7_ProvidersDeduplicatesAndSkipsTheUnattributable(t *testing.T) {
	r := &Rules{Schema: RulesSchema, Version: "v", Provider: "anthropic",
		Models: []ModelRule{
			{Match: "opus-5", Priced: true, InputPerMTok: 5, ReadMult: 0.1},
			{Match: "sonnet-5", Priced: true, InputPerMTok: 2, ReadMult: 0.1},
			{Match: "gpt-5-codex", Provider: "openai", Priced: true, InputPerMTok: 1.25, ReadMult: 0.1},
			{Match: "o4", Provider: "openai", Priced: true, InputPerMTok: 1, ReadMult: 0.1},
		}}
	got := r.Providers()
	if len(got) != 2 || got[0] != "anthropic" || got[1] != "openai" {
		t.Errorf("four rows over two publishers report %v", got)
	}

	// A row attributable to nobody contributes no name. The document-level
	// provider is empty here, which validate() permits only when no row names
	// one — so this is the legacy shape, and it reports nothing rather than "".
	bare := &Rules{Schema: RulesSchema, Version: "v",
		Models: []ModelRule{{Match: "opus-5", Priced: true, InputPerMTok: 5, ReadMult: 0.1}}}
	if got := bare.Providers(); len(got) != 0 {
		t.Errorf("an unattributed document reports providers %v, want none", got)
	}
}
