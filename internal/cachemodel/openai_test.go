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

// The seam the mutation sweep found unguarded on 2026-09-15.
//
// Anthropic sells a five minute and a one hour entry, so the deadline a gap
// must be measured against is a property of the request that WROTE the entry,
// not of the vendor. CacheRules carries that as TTLFrom. Nothing tested it:
// stubbing TTLFrom out reddened no test, and a build that ignored it would
// have measured every Anthropic gap against five minutes, reporting a TTL
// expiry at thirty minutes on a session that had bought an hour and was still
// holding a live entry.
//
// Astra has one TTL and a nil TTLFrom, which is why the flat path also has to
// keep working here.
func TestAnthropic_HourEntryIsNotExpiredByAThirtyMinuteGap(t *testing.T) {
	// Create1h alone is how the breakdown reports an hour entry.
	prev := transcript.Usage{Input: 4000, CacheCreation: 4000, Create1h: 4000}
	cur := transcript.Usage{Input: 100, CacheRead: 4000}

	cause, ok := ClassifyBreak(prev, cur, "claude-opus-5", "claude-opus-5", 30*time.Minute)
	if ok && cause == CauseTTLExpired {
		t.Fatal("30m gap on a one hour entry reported as TTL expiry; the per-request TTL was ignored " +
			"and every gap measured against the five minute default")
	}

	// Same gap, same model, a five minute entry: this one IS expired. Without
	// this half the test above would pass on a build that never expires
	// anything.
	short := transcript.Usage{Input: 4000, CacheCreation: 4000, Create5m: 4000}
	if c, ok := ClassifyBreak(short, cur, "claude-opus-5", "claude-opus-5", 30*time.Minute); !ok || c != CauseTTLExpired {
		t.Fatalf("30m gap on a five minute entry = (%q, %v), want TTL expiry", c, ok)
	}
}

// --- Production wiring: AstraRules() reachable outside this test package. ---
//
// Confirmed on 2026-10-08, by a repository-wide search and independently by
// the unblock panel before it: AstraRules() had zero non-test callers
// anywhere in the module. Every production site that classifies a cache
// break (internal/proxy/state.go, internal/analysis/diff.go,
// cmd/replay/costusage.go) called the Anthropic-pinned ClassifyBreak
// unconditionally, so an Astra-tier request's break was always read against
// Anthropic's 5-minute default and 0-floor rather than Astra's own 30-minute,
// 1024-token published terms. These tests are RED until RulesForModel and
// ClassifyBreakForModel exist and the three call sites use them.

func TestRulesForModel_SelectsAstraForTheGPT6AstraTier(t *testing.T) {
	r := RulesForModel("gpt-6-astra")
	if r.Provider != "openai" {
		t.Fatalf(`RulesForModel("gpt-6-astra").Provider = %q, want "openai"`, r.Provider)
	}
	if r.TTL != TTLAstra {
		t.Fatalf(`RulesForModel("gpt-6-astra").TTL = %v, want the Astra TTL %v`, r.TTL, TTLAstra)
	}
	if r.MinPrefix != MinPrefixAstra {
		t.Fatalf(`RulesForModel("gpt-6-astra").MinPrefix = %d, want %d`, r.MinPrefix, MinPrefixAstra)
	}
}

// Only the Astra tier is wired. Every other id -- Anthropic's own, an older
// or different OpenAI tier this build has no published CacheRules for, and
// every other provider -- falls back to AnthropicRules() exactly as it does
// today. That is a pre-existing limitation (recorded in docs/ROADMAP.md), not
// a new claim this change makes for those models, and this test pins it so a
// future change cannot widen AstraRules() past the tier it was actually read
// for without this test naming the widening.
func TestRulesForModel_FallsBackToAnthropicForEverythingElse(t *testing.T) {
	for _, m := range []string{"claude-opus-5", "gpt-5.6-terra", "gpt-5.4", "deepseek-chat", "gemini-2.5-pro", ""} {
		if got, want := RulesForModel(m).Provider, "anthropic"; got != want {
			t.Errorf("RulesForModel(%q).Provider = %q, want %q (unchanged fallback)", m, got, want)
		}
	}
}

// The actual production defect, reproduced directly: a 20 minute idle gap
// expires an Anthropic 5-minute entry and must not expire a 30-minute Astra
// one, through the SAME function every production call site now uses.
func TestClassifyBreakForModel_AstraTierUsesAstraTTLNotAnthropics(t *testing.T) {
	prev := transcript.Usage{Input: 4000, CacheCreation: 4000}
	cur := transcript.Usage{Input: 100, CacheRead: 4000}

	cause, ok := ClassifyBreakForModel(prev, cur, "gpt-6-astra", "gpt-6-astra", 20*time.Minute)
	if ok && cause == CauseTTLExpired {
		t.Fatal("ClassifyBreakForModel reported a 20m Astra gap as TTL expiry; it is still applying Anthropic's 5m rule to an Astra request")
	}

	// 40 minutes: Astra's own 30 minute TTL is actually exceeded now.
	cur2 := transcript.Usage{Input: 4000}
	cause2, ok2 := ClassifyBreakForModel(prev, cur2, "gpt-6-astra", "gpt-6-astra", 40*time.Minute)
	if !ok2 || cause2 != CauseTTLExpired {
		t.Fatalf("ClassifyBreakForModel(gpt-6-astra, 40m) = (%q, %v), want TTL expiry under Astra's own 30m TTL", cause2, ok2)
	}
}

// Regression: an Anthropic request through the new function must classify
// exactly as it did through the old one. The production call sites are being
// switched over; Anthropic traffic, which is everything measured so far, must
// not move.
func TestClassifyBreakForModel_AnthropicTierIsUnchanged(t *testing.T) {
	prev := transcript.Usage{Input: 4000, CacheCreation: 4000}
	cur := transcript.Usage{Input: 100, CacheRead: 4000}

	got, gotOK := ClassifyBreakForModel(prev, cur, "claude-opus-5", "claude-opus-5", 20*time.Minute)
	want, wantOK := ClassifyBreak(prev, cur, "claude-opus-5", "claude-opus-5", 20*time.Minute)
	if got != want || gotOK != wantOK {
		t.Fatalf("ClassifyBreakForModel(claude-opus-5) = (%q, %v), want the same answer as ClassifyBreak: (%q, %v)", got, gotOK, want, wantOK)
	}
	if !gotOK || got != CauseTTLExpired {
		t.Fatalf("claude-opus-5 at a 20m gap = (%q, %v), want TTL expiry (Anthropic's 5m default)", got, gotOK)
	}
}

// The human-readable detail line a caller builds for CauseTTLExpired has to
// name the TTL that was actually exceeded, not Anthropic's, when the entry
// was Astra's.
func TestTTLForModel_ReportsAstraNotAnthropicTTL(t *testing.T) {
	if got := TTLForModel("gpt-6-astra", transcript.Usage{}); got != TTLAstra {
		t.Fatalf("TTLForModel(gpt-6-astra) = %v, want the Astra TTL %v", got, TTLAstra)
	}
	// Anthropic path unchanged: still reads the write's own TTL split.
	hourWrite := transcript.Usage{CacheCreation: 4000, Create1h: 4000}
	if got := TTLForModel("claude-opus-5", hourWrite); got != TTLLong {
		t.Fatalf("TTLForModel(claude-opus-5, hour-write) = %v, want %v", got, TTLLong)
	}
}
