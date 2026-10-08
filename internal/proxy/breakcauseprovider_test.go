package proxy

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The production defect this file closes: (*stats).breakCause is the
// function the live proxy calls on every response to decide why a cache
// read came back short, and until this change it called the
// Anthropic-pinned cachemodel.ClassifyBreak unconditionally. An OpenAI
// Astra-tier request's break was therefore always read against Anthropic's
// 5-minute TTL default, never against Astra's own published 30 minutes.
//
// These tests call (*stats).breakCause directly -- the actual method the
// running proxy calls, not a reimplementation of its logic -- with a
// constructed laneState and ledger.Record, because driving a live HTTP
// request through a 20+ minute real clock gap is not practical in a test
// and this project's standing TTL study (docs/evidence/ttl-*) is a separate,
// protected, non-contact instrument that this change must not touch.

func TestBreakCause_AstraTierUsesAstraTTLInProduction(t *testing.T) {
	s := &stats{}
	now := time.Now()
	ln := &laneState{
		last:     transcript.Usage{Input: 4000, CacheCreation: 4000},
		lastSeen: now,
		model:    "gpt-6-astra",
		seen:     true,
	}
	rec := &ledger.Record{
		Timestamp: now.Add(20 * time.Minute),
	}
	rec.Model = "gpt-6-astra"
	usage := transcript.Usage{Input: 100, CacheRead: 4000}
	rec.Response.Usage = &usage

	cause, _ := s.breakCause(ln, rec, false)
	if cause == cachemodel.CauseTTLExpired {
		t.Fatal("the live proxy's breakCause reported a 20m Astra gap as TTL expiry; " +
			"it is still classifying an Astra-tier request under Anthropic's 5m rule")
	}
}

// Regression: the same method, same 20 minute gap, an Anthropic model --
// must still report the TTL expiry it always has. The production call site
// was switched to a provider-aware function; Anthropic traffic must not move.
func TestBreakCause_AnthropicTierUnchangedInProduction(t *testing.T) {
	s := &stats{}
	now := time.Now()
	ln := &laneState{
		last:     transcript.Usage{Input: 4000, CacheCreation: 4000},
		lastSeen: now,
		model:    "claude-opus-5",
		seen:     true,
	}
	rec := &ledger.Record{
		Timestamp: now.Add(20 * time.Minute),
	}
	rec.Model = "claude-opus-5"
	usage := transcript.Usage{Input: 100, CacheRead: 4000}
	rec.Response.Usage = &usage

	cause, _ := s.breakCause(ln, rec, false)
	if cause != cachemodel.CauseTTLExpired {
		t.Fatalf("breakCause(claude-opus-5, 20m gap) = %q, want TTL expiry unchanged", cause)
	}
}

// A 40 minute gap on an Astra entry DOES exceed Astra's own 30 minute TTL,
// through the same production method -- the dispatch is wired, not merely
// permissive.
func TestBreakCause_AstraTierStillExpiresPastItsOwnTTL(t *testing.T) {
	s := &stats{}
	now := time.Now()
	ln := &laneState{
		last:     transcript.Usage{Input: 4000, CacheCreation: 4000},
		lastSeen: now,
		model:    "gpt-6-astra",
		seen:     true,
	}
	rec := &ledger.Record{
		Timestamp: now.Add(40 * time.Minute),
	}
	rec.Model = "gpt-6-astra"
	usage := transcript.Usage{Input: 4000}
	rec.Response.Usage = &usage

	cause, _ := s.breakCause(ln, rec, false)
	if cause != cachemodel.CauseTTLExpired {
		t.Fatalf("breakCause(gpt-6-astra, 40m gap) = %q, want TTL expiry under Astra's own 30m TTL", cause)
	}
}
