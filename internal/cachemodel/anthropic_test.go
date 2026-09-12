package cachemodel

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

func TestExpectedReadExcludesUncachedTail(t *testing.T) {
	prev := transcript.Usage{Input: 56, CacheCreation: 508, CacheRead: 73128}
	if got := ExpectedRead(prev); got != 73636 {
		t.Fatalf("ExpectedRead = %d, want 73636", got)
	}
}

func TestClassifyRead(t *testing.T) {
	prev := transcript.Usage{Input: 2, CacheCreation: 100, CacheRead: 900}
	cases := []struct {
		read int
		want ReadOutcome
	}{
		{1000, ReadReproduced},
		{1200, ReadExceeded},
		{0, ReadBroken},
		{500, ReadBroken},
	}
	for _, c := range cases {
		got, expected := ClassifyRead(prev, transcript.Usage{CacheRead: c.read})
		if got != c.want || expected != 1000 {
			t.Errorf("ClassifyRead(read=%d) = %s/%d, want %s/1000", c.read, got, expected, c.want)
		}
	}
}

func TestClassifyBreak(t *testing.T) {
	prev := transcript.Usage{Input: 2, CacheCreation: 100, Create1h: 100, CacheRead: 900}
	if c, ok := ClassifyBreak(prev, transcript.Usage{CacheRead: 500}, "m", "m", 2*time.Hour); !ok || c != CauseTTLExpired {
		t.Fatalf("long gap -> %q %v", c, ok)
	}
	if c, ok := ClassifyBreak(prev, transcript.Usage{CacheRead: 500}, "m", "n", time.Minute); !ok || c != CauseModelChanged {
		t.Fatalf("model change -> %q %v", c, ok)
	}
	if c, ok := ClassifyBreak(prev, transcript.Usage{CacheRead: 0}, "m", "m", time.Minute); !ok || c != CausePrefixChange {
		t.Fatalf("nothing read -> %q %v", c, ok)
	}
	if _, ok := ClassifyBreak(prev, transcript.Usage{CacheRead: 500}, "m", "m", time.Minute); ok {
		t.Fatal("a mid-history divergence needs the history and must not be decided here")
	}
}

func TestCostUSD_Nested1hWriteIsNotPricedAtTheShortRate(t *testing.T) {
	// Claude Code on this machine always sends cache_creation as a nested
	// object with ephemeral_1h_input_tokens. ccusage #899 priced every write
	// at the 5m rate (1.25x) because it never read that object. A million
	// 1h-write tokens on opus-5 is $10 at 2x and $6.25 at 1.25x.
	raw := []byte(`{"input_tokens":0,"cache_creation_input_tokens":1000000,"cache_creation":{"ephemeral_1h_input_tokens":1000000},"cache_read_input_tokens":0,"output_tokens":0}`)
	var w transcript.WireUsage
	if err := json.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	u := w.Usage()
	if u.Create1h != 1_000_000 || u.Create5m != 0 {
		t.Fatalf("wire split not parsed: %+v", u)
	}
	p, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("opus-5 must be priced")
	}
	got := CostUSD(u, p)
	const want1h, want5m = 10.0, 6.25
	if got != want1h {
		t.Fatalf("CostUSD = %v, want %v (1h at 2x). %v is the 5m rate: the nested split was ignored", got, want1h, want5m)
	}
}

func TestTTLOf(t *testing.T) {
	if got := TTLOf(transcript.Usage{Create1h: 10}); got != TTLLong {
		t.Errorf("1h breakdown -> %s", got)
	}
	if got := TTLOf(transcript.Usage{Create5m: 10}); got != TTLShort {
		t.Errorf("5m breakdown -> %s", got)
	}
	if got := TTLOf(transcript.Usage{CacheCreation: 10}); got != TTLShort {
		t.Errorf("no breakdown -> %s, want provider default", got)
	}
}

func TestEffectiveTokensUsesMultipliers(t *testing.T) {
	u := transcript.Usage{Input: 100, CacheCreation: 1000, Create1h: 1000, CacheRead: 10000}
	want := 100 + 1000*WriteMultiplierLong + 10000*ReadMultiplier
	if got := EffectiveTokens(u, "claude-opus-5"); got != want {
		t.Fatalf("EffectiveTokens = %v, want %v", got, want)
	}
	noBreakdown := transcript.Usage{CacheCreation: 1000}
	if got := EffectiveTokens(noBreakdown, "claude-opus-5"); got != 1000*WriteMultiplierShort {
		t.Fatalf("EffectiveTokens without breakdown = %v", got)
	}
	newest := EffectiveTokens(transcript.Usage{CacheRead: 10000}, "claude-fable-5-1")
	if newest != 10000*readMultiplierNewest {
		t.Fatalf("newest tier must use its lower read multiple: %v", newest)
	}
}

func TestModelTable(t *testing.T) {
	cases := map[string]int{
		"claude-fable-5-1":  512,
		"claude-opus-5":     512,
		"claude-sonnet-5":   1024,
		"claude-opus-4-7":   2048,
		"claude-opus-4-6":   4096,
		"claude-haiku-4-5":  4096,
		"something-unknown": 1024,
	}
	for model, want := range cases {
		if got := MinCacheablePrefix(model); got != want {
			t.Errorf("%s: %d, want %d", model, got, want)
		}
	}
	p, ok := PriceFor("claude-opus-5")
	if !ok || p.InputPerMTok != 5 || p.OutputPerMTok != 25 || p.ReadMult != ReadMultiplier {
		t.Fatalf("opus 5 price = %+v ok=%v", p, ok)
	}
	if _, ok := PriceFor("some-unknown-model"); ok {
		t.Fatal("unknown models must have no price")
	}
	// A family row with a caching floor and no list price. opus-4-5 stood here
	// until 2026-09-07, when the provider's page was read and it gained one;
	// the invariant still needs an example, so it uses a row that genuinely
	// has no price rather than one that used to.
	//
	// The example was `claude-haiku-9-9` until 2026-09-11. That is not a model
	// id anyone has shipped, and it only reached the bare `haiku` row because
	// matching ignored the version it invented; under matchesModel an invented
	// version is an unrecognised model, which is the point of that change. A
	// real id exercises the same row and cannot be wrong about the table.
	if _, ok := PriceFor("claude-3-haiku-20240307"); ok {
		t.Fatal("a model with a caching floor but no list price must not be priced")
	}
	if ReadMultiplierFor("claude-3-haiku-20240307") != ReadMultiplier || ReadMultiplierFor("claude-fable-5-1") != readMultiplierNewest {
		t.Fatal("read multiples wrong")
	}
}

func TestCostAndSimulatedUsage(t *testing.T) {
	p, _ := PriceFor("claude-opus-5")
	// 1M uncached input at $5 = $5; 1M 1h-writes at 2x = $10; 1M reads at 0.1x = $0.50; 1M output = $25.
	u := transcript.Usage{Input: 1_000_000, CacheCreation: 1_000_000, Create1h: 1_000_000, CacheRead: 1_000_000, Output: 1_000_000}
	if got := CostUSD(u, p); got != 40.5 {
		t.Fatalf("CostUSD = %v, want 40.5", got)
	}
	sim := SimulatedUsage(10, 1000, 5000, 300, TTLLong)
	if sim.Create1h != 1000 || sim.Create5m != 0 || sim.PromptTotal() != 6010 || sim.Output != 300 {
		t.Fatalf("SimulatedUsage = %+v", sim)
	}
	if SimulatedUsage(0, 5, 0, 0, TTLShort).Create5m != 5 {
		t.Fatal("short TTL must fill the 5m bucket")
	}
	if CachedShare(50, 200) != 0.25 || CachedShare(0, 0) != 0 {
		t.Fatal("CachedShare wrong")
	}
	if WriteMultiplier(5*time.Minute) != WriteMultiplierShort || WriteMultiplier(time.Hour) != WriteMultiplierLong {
		t.Fatal("write multiplier does not follow TTL")
	}
}

// An unknown model's read multiple is a number this table actually holds, not
// one invented for the occasion.
//
// This test used to assert the opposite of what it asserts now, and the reason
// is worth keeping rather than quietly rewriting. It read:
//
//	overstating what a cache read costs systematically inflates the apparent
//	value of cache-preserving policies against cache-clearing ones
//
// That is backwards, and ADR-0022 works it through. EffectiveTokens adds
// CacheRead * ReadMult, so a cache-preserving policy — many reads, little
// input — gets MORE expensive as the multiple rises. A high multiple makes
// cache preservation look worse, not better. The cheap number is the one that
// flatters this tool's own advice, by a factor of about three over ten turns.
//
// The test shared one inverted premise with ADR-0021, which is unsurprising:
// they were written from the same belief, and neither could catch the other.
//
// What it got RIGHT, and what this keeps: the fallback must not be a number
// somebody made up. It has to be a multiple the table actually publishes for
// some model, so a reader can find where it came from. Which of the published
// multiples it should be is ADR-0022's question, and
// TestUnknownModelReadMultipleIsTheDearestInTheTable owns that answer — one
// fact, one owner, so a future change cannot satisfy the nearer test.
func TestUnknownModelReadMultipleIsNotInvented(t *testing.T) {
	unknown := ReadMultiplierFor("a-model-nobody-has-heard-of")
	published := map[float64]string{}
	for _, m := range modelTable {
		if m.priced {
			published[m.price.ReadMult] = m.match
		}
	}
	if len(published) == 0 {
		t.Fatal("no priced row carries a read multiple; this test compares against nothing")
	}
	if from, ok := published[unknown]; !ok {
		t.Fatalf("an unknown model reads at %.4f, which no row in this table publishes. A "+
			"fallback nobody can trace to a real model is a number invented for the occasion, "+
			"whatever direction it errs in", unknown)
	} else if from == "" {
		t.Fatal("the matched row has no name")
	}
}

// ADR-0021: the read multiple and the dollar column are not the same question.
//
// EffectiveTokens / policy comparison uses 0.025 (the newest tier) so the
// instrument cannot inflate cache-preserving policies. Dollars refuse: PriceFor
// returns !ok, so a cap cannot treat an unknown model as cheap. Putting 0.10 on
// the row, pricing it, or inventing a third multiple each collapses one of
// those failure directions.
func TestUnknownModelReadMultiplePinsTheSplit(t *testing.T) {
	// The multiple half of this pin moved to
	// TestUnknownModelReadMultipleIsTheDearestInTheTable, and the number it
	// used to assert is gone rather than updated.
	//
	// It pinned 0.025 and readMultiplierNewest, which is exactly what ADR-0021
	// decided and ADR-0022 supersedes. Two tests asserting opposite numbers for
	// one field is how a superseded decision comes back: whoever changes the
	// code next satisfies the nearer test. Keeping this one pointed at the
	// SPLIT — which is 0021's real contribution and still correct — and letting
	// the other own the multiple leaves one owner per fact.
	if unknownModel.price.ReadMult != readMultiplierUnknown {
		t.Fatalf("unknownModel.ReadMult = %g, want readMultiplierUnknown (%g); the multiple is "+
			"a rule about the table, not a constant to retype here",
			unknownModel.price.ReadMult, readMultiplierUnknown)
	}
	if unknownModel.priced {
		t.Fatal("unknownModel.priced: a dollar figure for a model nobody has a price for")
	}
	if _, ok := PriceFor("a-model-nobody-has-heard-of"); ok {
		t.Fatal("PriceFor(unknown) must be !ok; dollars do not guess")
	}
}

// The cache floor is per model family, and the fallback is not a floor.
//
// claude-3-5-haiku fell through to the bare "haiku" row and took the 1,024
// default. Its published minimum is 2,048, and a prefix below the minimum does
// not cache at all: no error, cache_creation_input_tokens zero, and advice that
// recommends caching a prefix which cannot be cached.
func TestHaiku35CarriesItsOwnCacheFloor(t *testing.T) {
	if got := lookup("claude-3-5-haiku").minPrefix; got != 2048 {
		t.Errorf("claude-3-5-haiku minPrefix = %d, want 2048", got)
	}
	// The newer Haiku is a different floor again, and must not be disturbed.
	if got := lookup("claude-haiku-4-5").minPrefix; got != 4096 {
		t.Errorf("claude-haiku-4-5 minPrefix = %d, want 4096", got)
	}
}

// This table prices one provider, and must not answer for another.
//
// lookup matches by substring with no provider dimension, so an OpenAI id
// reaches unknownModel and comes back with a read multiple. That is worse than
// no answer: EffectiveTokens adds CacheRead on top of Input, which is the
// Anthropic arithmetic, and OpenAI reports cached tokens INSIDE input. Pricing
// an OpenAI usage record through this path counts every cached token twice.
//
// Measured on 148 local Codex rollouts, 6,751 usage records: total equals
// input + output on all of them, and input + cached + output on none.
func TestForeignModelIDsAreNotPricedByTheAnthropicTable(t *testing.T) {
	// Usage as an OpenAI surface reports it: 1,000 tokens of prompt, 900 of
	// which were served from cache. The cached 900 are INSIDE the 1,000.
	openAIShaped := transcript.Usage{Input: 1000, CacheRead: 900}
	for _, id := range []string{"gpt-5.4", "gpt-5.1-codex-mini", "gemini-3-pro", "deepseek-v4"} {
		if lookup(id).priced {
			t.Errorf("%q is priced by the Anthropic table", id)
		}
		// The real hazard is not the dollar column, which is already withheld.
		// It is EffectiveTokens, which adds CacheRead to Input because that is
		// what the Anthropic wire means, and which reports its label as
		// measured. Fed a subset-shaped record it counts the cached share
		// twice and says it measured it.
		if got := EffectiveTokens(openAIShaped, id); got > 1000 {
			t.Errorf("EffectiveTokens(%q) = %.0f from a 1,000 token prompt: the cached "+
				"share was added to a total that already contained it", id, got)
		}
	}
}

// Models the table used to decline to price, read from the provider's own page
// on 2026-09-07 rather than inferred from a second observer.
func TestPricesReadFromTheProviderPage20260907(t *testing.T) {
	for _, c := range []struct {
		id            string
		input, output float64
	}{
		{"claude-opus-4-5", 5, 25},
		{"claude-sonnet-4-5", 3, 15},
		{"claude-sonnet-4", 3, 15},
		{"claude-opus-4-1", 15, 75},
		{"claude-3-5-haiku", 0.80, 4},
	} {
		r := lookup(c.id)
		if !r.priced {
			t.Errorf("%s is unpriced; the page lists it", c.id)
			continue
		}
		if r.price.InputPerMTok != c.input || r.price.OutputPerMTok != c.output {
			t.Errorf("%s = $%g/$%g, page says $%g/$%g",
				c.id, r.price.InputPerMTok, r.price.OutputPerMTok, c.input, c.output)
		}
	}
	// The read multiple is 0.1x everywhere except the Fable and Mythos 5.1
	// tier, which the page footnotes at 0.025x.
	for _, id := range []string{"claude-opus-5", "claude-sonnet-5", "claude-haiku-4-5", "claude-3-5-haiku"} {
		if got := ReadMultiplierFor(id); got != 0.10 {
			t.Errorf("%s read multiple = %g, want 0.10", id, got)
		}
	}
	for _, id := range []string{"claude-fable-5-1", "claude-mythos-5-1"} {
		if got := ReadMultiplierFor(id); got != 0.025 {
			t.Errorf("%s read multiple = %g, want 0.025", id, got)
		}
	}
}

// The unknown-model read multiple is the DEAREST the table holds, and this
// checks the rule rather than the number.
//
// ADR-0021 pinned it to readMultiplierNewest, which named the Fable/Mythos
// tier and happened to be the cheapest number in the table. Two things were
// wrong with that and ADR-0022 records both.
//
// The symbol named a TIER while being used as a RULE, so the day a newer tier
// reads dearer than 0.10 the constant would still say Newest, the record would
// still say Accepted, and the decision would invert without anyone touching it.
//
// And the direction was backwards. EffectiveTokens adds CacheRead * ReadMult,
// so a lower multiple makes a cached read cheaper, which makes the
// cache-preserving layouts this tool recommends score better against
// cache-clearing ones. The cheap number is the self-flattering one. An
// instrument that must not puff itself takes the dear one.
//
// A test asserting `== 0.10` would pass while meaning nothing: it would pin
// today's arithmetic and say nothing about why. This derives the answer from
// the table, so a new tier moves it and a failure here names the reason.
func TestUnknownModelReadMultipleIsTheDearestInTheTable(t *testing.T) {
	dearest := 0.0
	for _, m := range modelTable {
		if m.priced && m.price.ReadMult > dearest {
			dearest = m.price.ReadMult
		}
	}
	if dearest == 0 {
		t.Fatal("no priced row carries a read multiple; this test derived its answer from " +
			"nothing and would pass on an empty table")
	}
	if unknownModel.price.ReadMult != dearest {
		t.Fatalf("an unknown model reads at %v and the dearest row in the table reads at %v.\n"+
			"      A model this table does not know is usually a NEW one, and the conservative\n"+
			"      choice for an instrument is the multiple that makes its own advice look\n"+
			"      WORST, not best: EffectiveTokens adds CacheRead*ReadMult, so a cheap read\n"+
			"      inflates the apparent value of the cache-preserving layouts Replay\n"+
			"      recommends. See ADR-0022.",
			unknownModel.price.ReadMult, dearest)
	}
	// The premise, asserted rather than assumed: the table really does hold
	// more than one multiple. If it ever holds only one, dearest is trivially
	// that one and this test stops distinguishing anything.
	distinct := map[float64]bool{}
	for _, m := range modelTable {
		if m.priced {
			distinct[m.price.ReadMult] = true
		}
	}
	if len(distinct) < 2 {
		t.Fatalf("the table holds %d distinct read multiple(s); with one, this test cannot "+
			"tell the dearest from the only", len(distinct))
	}
}

// DearestPrice's two guards, tested where they live.
//
// The behaviour is covered end-to-end in internal/proxy, which is where it
// matters — but guard-reachability is per-package, so a conditional in
// cachemodel needs a cachemodel test. A guard exercised only from another
// package reads as unobserved here, and the reviewer is right to say so: the
// package that owns the code owns the claim about it.

// withTable swaps the compiled table for one test and puts it back.
func withTable(t *testing.T, rows []modelRow) {
	t.Helper()
	prev := modelTable
	modelTable = rows
	t.Cleanup(func() { modelTable = prev })
}

// A table with no priced row reports that it has none.
//
// This is the guard that matters. Without the !priced skip, the first row
// taken is an unpriced one carrying a zero Price, `found` goes true, and
// DearestPrice returns (zero, true) — so a caller bounding an unpriced model
// computes zero and the spend cap is back to never firing, which is the defect
// #261 exists to fix, reintroduced one layer down.
func TestDearestPriceReportsWhenNothingIsPriced(t *testing.T) {
	withTable(t, []modelRow{
		{match: "a", minPrefix: 1024, price: Price{}, priced: false},
		{match: "b", minPrefix: 1024, price: Price{}, priced: false},
	})
	if p, ok := DearestPrice(); ok {
		t.Fatalf("a table with no priced row reported a bound of %+v. A zero bound is not a "+
			"bound: the caller multiplies by it and the cap never fires", p)
	}
}

// An unpriced row never beats a priced one, wherever it sits.
//
// Ordering matters and this checks both: an unpriced row FIRST would otherwise
// seed the search with a zero price, and an unpriced row LAST must not
// displace a real one.
func TestDearestPriceSkipsUnpricedRowsInAnyOrder(t *testing.T) {
	for _, c := range []struct {
		name string
		rows []modelRow
	}{
		{"unpriced first", []modelRow{
			{match: "free", price: Price{}, priced: false},
			{match: "cheap", price: Price{InputPerMTok: 1, OutputPerMTok: 5, ReadMult: 0.1}, priced: true},
			{match: "dear", price: Price{InputPerMTok: 15, OutputPerMTok: 75, ReadMult: 0.1}, priced: true},
		}},
		{"unpriced last", []modelRow{
			{match: "cheap", price: Price{InputPerMTok: 1, OutputPerMTok: 5, ReadMult: 0.1}, priced: true},
			{match: "dear", price: Price{InputPerMTok: 15, OutputPerMTok: 75, ReadMult: 0.1}, priced: true},
			{match: "free", price: Price{}, priced: false},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			withTable(t, c.rows)
			got, ok := DearestPrice()
			if !ok {
				t.Fatal("a table with priced rows reported none")
			}
			if got.InputPerMTok != 15 {
				t.Fatalf("dearest is %v per MTok, want 15. An unpriced row carries a zero "+
					"price, and a zero that wins the comparison is the spend cap failing open",
					got.InputPerMTok)
			}
		})
	}
}

// The dearest is the dearest, not the first or the last priced row.
func TestDearestPriceTakesTheMaximumNotAnEdge(t *testing.T) {
	withTable(t, []modelRow{
		{match: "a", price: Price{InputPerMTok: 3, OutputPerMTok: 15, ReadMult: 0.1}, priced: true},
		{match: "b", price: Price{InputPerMTok: 15, OutputPerMTok: 75, ReadMult: 0.1}, priced: true},
		{match: "c", price: Price{InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1}, priced: true},
	})
	got, ok := DearestPrice()
	if !ok {
		t.Fatal("no priced row found in a table of three")
	}
	if got.InputPerMTok != 15 {
		t.Fatalf("dearest is %v, want 15 — the maximum sits in the middle on purpose, so "+
			"taking the first or the last row passes for the wrong reason", got.InputPerMTok)
	}
	// The whole Price travels, not just the field compared on. A bound that
	// carried one row's input rate and another's output rate would be a price
	// no model has.
	if got.OutputPerMTok != 75 {
		t.Fatalf("the returned Price mixes rows: input %v with output %v", got.InputPerMTok, got.OutputPerMTok)
	}
}
