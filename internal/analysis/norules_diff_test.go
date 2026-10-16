package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// NRD: replay diff does not explain a non-Anthropic, non-Astra model's break
// with Anthropic's TTL (2026-10-15). Ten minutes is past Anthropic's 5m TTL.
func TestNRD1_DiffDoesNotUseAnthropicTTLForAModelWithoutRules(t *testing.T) {
	at := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	cold := transcript.Usage{Input: 20_500, CacheRead: 0, CacheCreation: 1_000}
	for _, tc := range []struct {
		model string
		next  transcript.Usage
		want  cachemodel.BreakCause
	}{
		{"gpt-5", cold, cachemodel.CauseNoRules},
		{"o4-mini", cold, cachemodel.CauseNoRules},
		{"claude-opus-5", acBroken, cachemodel.CauseTTLExpired}, // the control keeps its rules
	} {
		lane := &transcript.Lane{ID: "main", Requests: []*transcript.Request{
			corrRequest("req_1", at, 2*time.Second, tc.model, transcript.CorrelationUnmeasured, acSeed),
			corrRequest("req_2", at.Add(10*time.Minute), 2*time.Second, tc.model, transcript.CorrelationUnmeasured, tc.next),
		}}
		breaks := FindBreaks(Calibrate(lane), TokenFit{})
		if len(breaks) != 1 {
			t.Fatalf("%s: breaks = %d, want 1", tc.model, len(breaks))
		}
		if breaks[0].Cause != tc.want {
			t.Errorf("%s: cause %q, want %q", tc.model, breaks[0].Cause, tc.want)
		}
	}
}

// A partial read by a model without rules is located by the history diff, not
// called an expiry.
func TestNRD2_APartialReadWithoutRulesIsNotCalledAnExpiry(t *testing.T) {
	at := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{ID: "main", Requests: []*transcript.Request{
		corrRequest("req_1", at, 2*time.Second, "gpt-5", transcript.CorrelationUnmeasured, acSeed),
		corrRequest("req_2", at.Add(10*time.Minute), 2*time.Second, "gpt-5", transcript.CorrelationUnmeasured, acBroken),
	}}
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1", len(breaks))
	}
	if breaks[0].Cause == cachemodel.CauseTTLExpired {
		t.Errorf("a gpt-5 partial read was explained by Anthropic's TTL: %q", breaks[0].Cause)
	}
}

// The detail beside a not-measured cause does not claim a prefix change: with
// no rules, a cold read is not known to have diverged before the first message.
func TestNRD3_TheDetailOfANoRulesBreakDoesNotNameAPrefixChange(t *testing.T) {
	at := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	cold := transcript.Usage{Input: 20_500, CacheRead: 0, CacheCreation: 1_000}
	lane := &transcript.Lane{ID: "main", Requests: []*transcript.Request{
		corrRequest("req_1", at, 2*time.Second, "gpt-5", transcript.CorrelationUnmeasured, acSeed),
		corrRequest("req_2", at.Add(10*time.Minute), 2*time.Second, "gpt-5", transcript.CorrelationUnmeasured, cold),
	}}
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1", len(breaks))
	}
	d := breaks[0].Detail
	if strings.Contains(d, "TTL") || strings.Contains(d, "before the first message") {
		t.Errorf("detail %q names a cause the usage cannot settle for gpt-5", d)
	}
	if !strings.Contains(d, "gpt-5") {
		t.Errorf("detail %q does not name the model whose rules are missing", d)
	}
}
