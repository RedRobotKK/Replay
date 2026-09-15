package cachemodel

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The numbers pinned here are OpenAI's published ones for the Astra tier, read
// from the prompt-caching guide on 2026-09-15. They are a CLAIM, not a
// measurement: nothing in this package has yet replayed Astra traffic to bound
// them. When calibration lands, the measured bound goes in MinPrefixClaim and
// these stay as what the document said on that date.

func TestAstra_PublishedTTLIsThirtyMinutes(t *testing.T) {
	if got := AstraRules().TTL; got != 30*time.Minute {
		t.Fatalf("Astra TTL = %v, want 30m (OpenAI prompt-caching guide, read 2026-09-15)", got)
	}
}

func TestAstra_PublishedMinimumPrefixIs1024(t *testing.T) {
	if got := AstraRules().MinPrefix; got != 1024 {
		t.Fatalf("Astra MinPrefix = %d, want 1024 (OpenAI prompt-caching guide, read 2026-09-15)", got)
	}
}

// The gap that separates the two providers. Twenty minutes idle expires an
// Anthropic 5-minute entry and does not expire an Astra one. Reporting this
// request as a TTL expiry would be naming a break that did not happen, which
// is the failure this file exists to prevent.
func TestAstra_GapInsideTTLIsNotExpiry(t *testing.T) {
	prev := transcript.Usage{Input: 4000, CacheCreation: 4000}
	cur := transcript.Usage{Input: 100, CacheRead: 4000}

	cause, ok := ClassifyBreakWith(AstraRules(), prev, cur, "gpt-6-astra", "gpt-6-astra", 20*time.Minute, 4000)
	if ok && cause == CauseTTLExpired {
		t.Fatal("20m idle reported as TTL expiry on Astra; that is the Anthropic 5m rule applied to a 30m provider")
	}

	// Same gap, same numbers, Anthropic rules: this one IS an expiry.
	if c, ok := ClassifyBreak(prev, cur, "claude-opus-5", "claude-opus-5", 20*time.Minute); !ok || c != CauseTTLExpired {
		t.Fatalf("Anthropic 20m gap = (%q, %v), want TTL expiry; the two providers must not share one TTL", c, ok)
	}
}

func TestAstra_GapBeyondTTLIsExpiry(t *testing.T) {
	prev := transcript.Usage{Input: 4000, CacheCreation: 4000}
	cur := transcript.Usage{Input: 4000}

	cause, ok := ClassifyBreakWith(AstraRules(), prev, cur, "gpt-6-astra", "gpt-6-astra", 40*time.Minute, 4000)
	if !ok || cause != CauseTTLExpired {
		t.Fatalf("40m idle on Astra = (%q, %v), want TTL expiry", cause, ok)
	}
}

// ADR-0018 applied to the prefix floor: a prompt too small to be cacheable and
// a prompt whose cache broke are different states. Below the floor the
// provider never wrote an entry, so there was nothing to break, and calling it
// a prefix change invents a cause for a request that behaved exactly as
// documented.
func TestAstra_BelowMinimumPrefixIsNotABreak(t *testing.T) {
	prev := transcript.Usage{Input: 900}
	cur := transcript.Usage{Input: 900}

	cause, ok := ClassifyBreakWith(AstraRules(), prev, cur, "gpt-6-astra", "gpt-6-astra", time.Minute, 900)
	if !ok {
		t.Fatal("a sub-floor prompt is decidable from usage alone; want ok")
	}
	if cause != CauseBelowMinPrefix {
		t.Fatalf("900-token prompt on Astra = %q, want %q", cause, CauseBelowMinPrefix)
	}
}

// At the floor exactly, the prompt is cacheable, so a zero cache read is a
// real break rather than the floor.
func TestAstra_AtMinimumPrefixIsCacheable(t *testing.T) {
	prev := transcript.Usage{Input: 1024, CacheCreation: 1024}
	cur := transcript.Usage{Input: 1024}

	cause, _ := ClassifyBreakWith(AstraRules(), prev, cur, "gpt-6-astra", "gpt-6-astra", time.Minute, 1024)
	if cause == CauseBelowMinPrefix {
		t.Fatal("1024 tokens reported as below the floor; the documented minimum is inclusive")
	}
}

// The asymmetry worth publishing. On Anthropic, changing the effort or
// thinking setting invalidates the prefix. On Astra, OpenAI documents a
// configuration_update item that changes reasoning effort while PRESERVING the
// prefix for cache reuse. The same operator action has opposite cost
// consequences on the two providers, and a tool carrying one rule across both
// reports the wrong cause on one of them.
func TestAstra_EffortChangeDoesNotBreakPrefix(t *testing.T) {
	if AstraRules().EffortChangeBreaks {
		t.Fatal("Astra rules say an effort change breaks the prefix; OpenAI documents configuration_update as preserving it")
	}
	if !AnthropicRules().EffortChangeBreaks {
		t.Fatal("Anthropic rules say an effort change preserves the prefix; that is not what this repo measured")
	}
}

// A guard, not a behaviour: the point of CacheRules is that TTL stops being a
// package constant. If a second provider ever shares Anthropic's numbers by
// accident this test says nothing, but if someone deletes the seam and goes
// back to one global TTL, every table row collapses to the same value.
func TestCacheRules_ProvidersDoNotShareATTL(t *testing.T) {
	if AstraRules().TTL == AnthropicRules().TTL {
		t.Fatalf("Astra and Anthropic both report TTL %v; the per-provider seam is gone", AstraRules().TTL)
	}
}
