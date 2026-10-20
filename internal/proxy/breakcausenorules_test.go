package proxy

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// NRP: the live proxy does not explain a break by a model with no published
// cache rules here with Anthropic's TTL (2026-10-16). Before the fix,
// breakCause called a gpt-5 cold read ten minutes after the last request
// "cache expired", which is Anthropic's five-minute TTL applied to OpenAI.
// These call (*stats).breakCause, the method the running proxy calls, as
// breakcauseprovider_test.go does and for the same reason.
func TestNRP1_TheProxyDoesNotUseAnthropicTTLForAModelWithoutRules(t *testing.T) {
	now := time.Now()
	for _, m := range []string{"gpt-5", "o4-mini"} {
		ln := &laneState{
			last:     transcript.Usage{Input: 100, CacheRead: 50_000},
			lastSeen: now,
			model:    m,
			seen:     true,
		}
		rec := &ledger.Record{Timestamp: now.Add(10 * time.Minute)}
		rec.Model = m
		cold := transcript.Usage{Input: 50_100}
		rec.Response.Usage = &cold

		cause, _ := (&stats{}).breakCause(ln, rec, false)
		if cause == cachemodel.CauseTTLExpired {
			t.Errorf("%s: a cold read was explained by Anthropic's TTL: %q", m, cause)
		}
		if !strings.HasPrefix(string(cause), "NOT MEASURED") || !strings.Contains(string(cause), "no published cache rules") {
			t.Errorf("%s: cause %q, want the not-measured cause that names the missing rules", m, cause)
		}
	}
}

// A model change away from a model without rules is still reported as a model
// change, which needs no provider's rules.
func TestNRP2_AModelChangeWithoutRulesIsStillAModelChange(t *testing.T) {
	now := time.Now()
	ln := &laneState{
		last:     transcript.Usage{Input: 100, CacheRead: 50_000},
		lastSeen: now,
		model:    "gpt-5",
		seen:     true,
	}
	rec := &ledger.Record{Timestamp: now.Add(time.Second)}
	rec.Model = "claude-opus-5"
	cold := transcript.Usage{Input: 50_100}
	rec.Response.Usage = &cold

	if cause, _ := (&stats{}).breakCause(ln, rec, false); cause != cachemodel.CauseModelChanged {
		t.Errorf("gpt-5 -> claude-opus-5: cause %q, want %q", cause, cachemodel.CauseModelChanged)
	}
}
