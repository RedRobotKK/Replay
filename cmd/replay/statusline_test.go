package main

import (
	"strings"
	"testing"
)

// The status line runs on a 300ms debounce during active work, so it does
// arithmetic on the JSON Claude Code already computed and never opens a
// transcript. Claude Code reports cache misses in tokens; Replay owns the price
// table, so it reports them in money. That conversion is the whole feature.

func TestStatuslinePricesWastedTokens(t *testing.T) {
	in := `{
	  "model": {"id": "claude-opus-5", "display_name": "Opus"},
	  "cost": {"total_cost_usd": 18.40},
	  "prompt_cache": {"warm": true, "hit_ratio": 0.91, "misses": 2, "ttl": "1h",
	                   "miss_recache_tokens": 310200,
	                   "last_miss_cause": {"causes": ["tools_changed"]}}
	}`
	got := statusLine(mustParseStatus(t, in), false)
	for _, want := range []string{"$18.40", "91%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status line %q is missing %q", got, want)
		}
	}
	// 310200 tokens at $5/MTok with the 1h write multiplier of 2.0 = $3.10,
	// which is credible against $18.40 of actual spend.
	if !strings.Contains(got, "$3.10") {
		t.Fatalf("the wasted tokens were never priced: %q", got)
	}
	if !strings.Contains(got, "tools changed") {
		t.Fatalf("the cause is the actionable part and must appear: %q", got)
	}
}

// Absent fields are the normal case on older clients and before the first API
// response. A status line that errors, or prints "undefined", is worse than one
// that prints less.
func TestStatuslineDegradesWhenFieldsAreMissing(t *testing.T) {
	for _, in := range []string{
		`{}`,
		`{"model":{"id":"claude-opus-5"}}`,
		`{"cost":{"total_cost_usd":0.5}}`,
		`{"prompt_cache":{"hit_ratio":0.5}}`,
	} {
		got := statusLine(mustParseStatus(t, in), false)
		if strings.Contains(got, "NaN") || strings.Contains(got, "undefined") || strings.Contains(got, "%!") {
			t.Fatalf("input %s produced %q", in, got)
		}
		if strings.Contains(got, "\n") {
			t.Fatalf("the status line must be one line: %q", got)
		}
	}
}

// A model outside the price table cannot be priced, and inventing a rate would
// put a fabricated dollar figure in front of somebody every 300ms.
func TestStatuslineWillNotPriceAnUnknownModel(t *testing.T) {
	in := `{"model":{"id":"some-model-we-do-not-know"},
	        "prompt_cache":{"hit_ratio":0.5,"miss_recache_tokens":1000000,"ttl":"1h"}}`
	got := statusLine(mustParseStatus(t, in), false)
	if strings.Contains(got, "avoidable") {
		t.Fatalf("priced an unknown model: %q", got)
	}
}

// Nothing wasted is worth saying plainly rather than showing a zero.
func TestStatuslineSaysNothingIsWrongWhenNothingIs(t *testing.T) {
	in := `{"model":{"id":"claude-opus-5"},"cost":{"total_cost_usd":0.2},
	        "prompt_cache":{"warm":true,"hit_ratio":1,"misses":0,"miss_recache_tokens":0}}`
	got := statusLine(mustParseStatus(t, in), false)
	if strings.Contains(got, "avoidable") {
		t.Fatalf("a clean session must not show a waste figure: %q", got)
	}
}

// Colour is opt-in on the writer, because the same string is asserted in tests
// and rendered in a terminal.
func TestStatuslineColourIsOptional(t *testing.T) {
	in := `{"model":{"id":"claude-opus-5"},"cost":{"total_cost_usd":9},
	        "prompt_cache":{"hit_ratio":0.2,"misses":9,"miss_recache_tokens":5000000,"ttl":"5m"}}`
	plain := statusLine(mustParseStatus(t, in), false)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain output must carry no escape codes: %q", plain)
	}
	coloured := statusLine(mustParseStatus(t, in), true)
	if !strings.Contains(coloured, "\x1b[") {
		t.Fatal("coloured output should carry escape codes")
	}
	if stripANSI(coloured) != plain {
		t.Fatalf("colour changed the text:\n %q\n %q", stripANSI(coloured), plain)
	}
}

func mustParseStatus(t *testing.T, in string) statusInput {
	t.Helper()
	s, err := parseStatusInput(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse %s: %v", in, err)
	}
	return s
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Avoidable spend is priced from token counts at list rates; total cost is what
// the session was actually charged. When the first exceeds the second, the two
// numbers are contradicting each other, and the one to doubt is ours. Showing
// "$57.50 avoidable" beside "$42.10 spent" is arithmetic a reader can see is
// wrong, on the tool whose entire pitch is that its numbers can be trusted.
func TestStatuslineWillNotClaimMoreWasteThanWasSpent(t *testing.T) {
	in := `{"model":{"id":"claude-opus-5"},"cost":{"total_cost_usd":42.10},
	        "prompt_cache":{"hit_ratio":0.42,"misses":11,"ttl":"5m",
	                        "miss_recache_tokens":9200000,
	                        "last_miss_cause":{"causes":["history_rewritten"]}}}`
	got := statusLine(mustParseStatus(t, in), false)
	if strings.Contains(got, "$57.50") {
		t.Fatalf("claimed more waste than the session cost: %q", got)
	}
	// The cache is still plainly unhealthy, and that must still be visible.
	if !strings.Contains(got, "cache 42%") {
		t.Fatalf("dropped the cache health along with the bad figure: %q", got)
	}
	if !strings.Contains(got, "history rewritten") {
		t.Fatalf("the cause is still actionable and must remain: %q", got)
	}
}
