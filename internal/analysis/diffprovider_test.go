package analysis

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The offline diff is one of the three production callers RPL-C038 wires:
// FindBreaks -> classify -> cachemodel.ClassifyBreakForModel. Confirmed by
// the project's own guard-reachability sweep (2026-10-08) that no existing
// test in this package drove that call's "a cause was decided from usage and
// timing alone" branch at all -- every existing FindBreaks test in this
// package lands on CauseNotMeasured (lane overlap) or a message-history
// cause (CauseRerendered), never a usage/timing one. This file closes that
// gap for the change this pass makes: it exercises the live provider
// dispatch through FindBreaks itself, not through cachemodel directly.
func TestFindBreaks_AstraTierUsesAstraTTLNotAnthropics(t *testing.T) {
	at := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	lane := transcriptLane(t, "gpt-6-astra", at, 20*time.Minute)
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1: %+v", len(breaks), breaks)
	}
	if breaks[0].Cause == cachemodel.CauseTTLExpired {
		t.Fatalf("FindBreaks reported a 20m Astra gap as TTL expiry (%+v); diff.go is still "+
			"applying Anthropic's 5m rule to an Astra-tier request", breaks[0])
	}
}

// Regression: the same shape, the same 20 minute gap, Anthropic's own model
// -- must still report the TTL expiry it always has, through FindBreaks
// itself.
func TestFindBreaks_AnthropicTierUnchanged(t *testing.T) {
	at := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	lane := transcriptLane(t, "claude-opus-5", at, 20*time.Minute)
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1: %+v", len(breaks), breaks)
	}
	if breaks[0].Cause != cachemodel.CauseTTLExpired {
		t.Fatalf("FindBreaks(claude-opus-5, 20m gap) cause = %q, want TTL expiry unchanged: %+v", breaks[0].Cause, breaks[0])
	}
	// The detail line has to name Anthropic's own TTL, not Astra's, so a
	// caller reading it is not told the wrong deadline for the provider it
	// actually ran against.
	if !containsTTL5m(breaks[0].Detail) {
		t.Fatalf("detail %q does not name the 5m TTL the Anthropic default actually used", breaks[0].Detail)
	}
}

func containsTTL5m(detail string) bool {
	return len(detail) > 0 && (indexOfSub(detail, "5m0s") >= 0 || indexOfSub(detail, "5m") >= 0)
}

func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// transcriptLane builds a two-request lane whose second request's cache read
// came back short of the first request's write, which is what makes
// Calibrate classify the turn as ReadBroken -- the same acSeed/acBroken
// shape correlation_test.go's own AC1 uses, reused here under a model axis
// rather than a correlation axis.
func transcriptLane(t *testing.T, model string, at time.Time, gap time.Duration) *transcript.Lane {
	t.Helper()
	return &transcript.Lane{ID: "main", Requests: []*transcript.Request{
		corrRequest("req_1", at, 2*time.Second, model, "", acSeed),
		corrRequest("req_2", at.Add(gap), 2*time.Second, model, "", acBroken),
	}}
}
