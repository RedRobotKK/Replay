package cachemodel

import (
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
	if _, ok := PriceFor("claude-opus-4-5"); ok {
		t.Fatal("a model with a caching floor but no list price must not be priced")
	}
	if ReadMultiplierFor("claude-opus-4-5") != ReadMultiplier || ReadMultiplierFor("claude-fable-5-1") != readMultiplierNewest {
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

// An unknown model fell back to a read multiple of 0.10 while the current tier
// is 0.025, a 4x overstatement that entered every effective-token figure and was
// labelled measured rather than estimated. The bias has a direction: overstating
// what a cache read costs systematically inflates the apparent value of
// cache-preserving policies against cache-clearing ones, which is exactly the
// comparison this tool exists to make.
func TestUnknownModelDoesNotGetAFabricatedReadMultiple(t *testing.T) {
	unknown := ReadMultiplierFor("a-model-nobody-has-heard-of")
	newest := ReadMultiplierFor("fable-5-1")
	if unknown > newest*2 {
		t.Fatalf("an unknown model is charged %.4f per cached token while the current tier is %.4f; "+
			"a fallback that overstates by %.1fx is a fabricated number, not a conservative one",
			unknown, newest, unknown/newest)
	}
}
