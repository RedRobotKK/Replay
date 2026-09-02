package cachemodel

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/transcript"
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
	if got := EffectiveTokens(u); got != want {
		t.Fatalf("EffectiveTokens = %v, want %v", got, want)
	}
	noBreakdown := transcript.Usage{CacheCreation: 1000}
	if got := EffectiveTokens(noBreakdown); got != 1000*WriteMultiplierShort {
		t.Fatalf("EffectiveTokens without breakdown = %v", got)
	}
}

func TestMinCacheablePrefixByFamily(t *testing.T) {
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
}

func TestWriteMultiplier(t *testing.T) {
	if WriteMultiplier(5*time.Minute) != WriteMultiplierShort || WriteMultiplier(time.Hour) != WriteMultiplierLong {
		t.Fatal("write multiplier does not follow TTL")
	}
}
