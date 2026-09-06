package proxy

import (
	"net/http"
	"testing"
)

// Quota capture: what a flat-seat user actually spends.
//
// Every dollar figure Replay prints is denominated in a currency a Claude Max,
// Copilot or Cursor subscriber does not spend. docs/WHAT-YOU-GET.md says so
// plainly: a broken cache costs them nothing. But no flat seat is unlimited -
// each one meters something (a rolling usage window, fast requests, premium
// requests), and none of them tell the user what they are spending it on.
//
// Providers already answer that question on every response and the proxy has
// been throwing it away. These headers are the only place the subscriber's real
// budget is visible, and the delta in "remaining" between two requests is the
// quota a request actually consumed - which is what makes it possible to ask
// whether a cache break burns quota the way it burns dollars.
//
// Captured by allowlisted prefix, because a response header is provider-
// controlled input and a passthrough would be a channel for anything the
// provider chose to put there.

// Q1: the provider's own quota headers are captured.
//
// PASS: limits, remaining and reset all land.
// FAIL: any dropped - "remaining" alone cannot be turned into consumption
// without knowing the limit it counts against.
func TestQ1_AnthropicQuotaHeadersAreCaptured(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "999")
	h.Set("anthropic-ratelimit-tokens-limit", "80000")
	h.Set("anthropic-ratelimit-tokens-remaining", "42500")
	h.Set("anthropic-ratelimit-tokens-reset", "2026-09-06T12:00:00Z")

	got := quotaFrom(h)
	for _, k := range []string{
		"anthropic-ratelimit-requests-limit",
		"anthropic-ratelimit-requests-remaining",
		"anthropic-ratelimit-tokens-limit",
		"anthropic-ratelimit-tokens-remaining",
		"anthropic-ratelimit-tokens-reset",
	} {
		if got[k] == "" {
			t.Errorf("quota header %q was not captured: %v", k, got)
		}
	}
}

// Q2: the OpenAI-family spelling is captured too.
//
// The two vendors order the words differently - tokens-remaining against
// remaining-tokens - so this is stored under the header's own name rather than
// mapped onto a common schema, which would be a guess about equivalence.
//
// PASS: both spellings survive.
// FAIL: an Anthropic-only prefix, which silently drops the whole OpenAI path
// the proxy already supports.
func TestQ2_OpenAIQuotaHeadersAreCaptured(t *testing.T) {
	h := http.Header{}
	h.Set("x-ratelimit-limit-requests", "500")
	h.Set("x-ratelimit-remaining-requests", "499")
	h.Set("x-ratelimit-remaining-tokens", "12000")
	h.Set("x-ratelimit-reset-tokens", "6ms")

	got := quotaFrom(h)
	if len(got) != 4 {
		t.Errorf("expected 4 OpenAI quota headers, got %d: %v", len(got), got)
	}
}

// Q3: it is an allowlist, not a passthrough.
//
// The response header block is provider-controlled input. Copying it wholesale
// onto an append-only ledger would put whatever the provider sent - a cookie, a
// token echo, a tracking id - into a file the user later publishes findings
// from.
//
// PASS: nothing outside the quota prefixes is stored.
// FAIL: any credential-shaped or identifying header captured.
func TestQ3_OnlyQuotaHeadersAreCaptured(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-tokens-remaining", "42500")
	h.Set("authorization", "Bearer sk-ant-secret")
	h.Set("x-api-key", "sk-ant-also-secret")
	h.Set("set-cookie", "session=abc123")
	h.Set("x-request-id", "req_identifying")
	h.Set("content-type", "application/json")

	got := quotaFrom(h)
	if len(got) != 1 {
		t.Fatalf("expected only the quota header, got %d: %v", len(got), got)
	}
	for k, v := range got {
		for _, bad := range []string{"sk-ant", "Bearer", "session=", "req_"} {
			if contains(k, bad) || contains(v, bad) {
				t.Errorf("captured %q=%q which carries %q", k, v, bad)
			}
		}
	}
}

// Q4: no quota headers means no field, not an empty one.
//
// PASS: nil, so the record omits it.
// FAIL: an empty map serialized on every record, which reads as "the provider
// reported no quota" rather than "this response carried none".
func TestQ4_NoQuotaHeadersYieldsNothing(t *testing.T) {
	if got := quotaFrom(http.Header{}); got != nil {
		t.Errorf("expected nil for a response with no quota headers, got %v", got)
	}
	h := http.Header{}
	h.Set("content-type", "application/json")
	if got := quotaFrom(h); got != nil {
		t.Errorf("expected nil when only non-quota headers are present, got %v", got)
	}
}

// Q5: retry-after is captured, because it is the lockout itself.
//
// Every other header describes budget remaining. This one is the provider
// saying the budget is gone and naming the wait - the exact event a flat-seat
// user feels, and the one the dollar figures never show.
//
// PASS: captured.
// FAIL: dropped, leaving the lockout the only part of the story with no record.
func TestQ5_RetryAfterIsCaptured(t *testing.T) {
	h := http.Header{}
	h.Set("retry-after", "42")
	got := quotaFrom(h)
	if got["retry-after"] != "42" {
		t.Errorf("retry-after was not captured: %v", got)
	}
}

func contains(s, sub string) bool {
	return len(sub) <= len(s) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// Q6: the wiring. A response carrying quota headers writes them to the ledger.
//
// Q1-Q5 test a pure function, and a pure function nothing calls is the defect
// ADR-0014 was written for: this project has already shipped a payment gate no
// test imported, where deleting it left the suite green. So this drives a real
// request through the proxy and reads the file that results.
//
// PASS: the record carries the provider's quota figures.
// FAIL: green unit tests over a field that is never populated in production.
func TestQ6_QuotaReachesTheLedger(t *testing.T) {
	up := &quotaUpstream{}
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	_ = resp.Body.Close()

	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record was written")
	}
	got := recs[0].Quota
	if got == nil {
		t.Fatal("the record carries no quota at all, so quotaFrom is not wired to the proxy")
	}
	if got["anthropic-ratelimit-tokens-remaining"] != "42500" {
		t.Errorf("tokens-remaining did not reach the ledger: %v", got)
	}
	if got["anthropic-ratelimit-tokens-limit"] != "80000" {
		t.Errorf("tokens-limit did not reach the ledger: %v", got)
	}
	// And the allowlist still holds on the real path, not only in the unit test.
	for k, v := range got {
		for _, bad := range []string{"sk-ant", "Bearer", "session="} {
			if contains(k, bad) || contains(v, bad) {
				t.Errorf("the ledger captured %q=%q carrying %q", k, v, bad)
			}
		}
	}
}

// quotaUpstream answers like a provider reporting rate-limit state, and sets a
// credential-shaped header beside it so the allowlist is exercised end to end.
type quotaUpstream struct{}

func (q *quotaUpstream) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("request-id", "req_quota_1")
	w.Header().Set("anthropic-ratelimit-tokens-limit", "80000")
	w.Header().Set("anthropic-ratelimit-tokens-remaining", "42500")
	w.Header().Set("anthropic-ratelimit-tokens-reset", "2026-09-06T12:00:00Z")
	w.Header().Set("set-cookie", "session=must-not-be-captured")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant",` +
		`"content":[{"type":"text","text":"ok"}],` +
		`"usage":{"input_tokens":4,"cache_creation_input_tokens":30,` +
		`"cache_read_input_tokens":300,"output_tokens":1}}`))
}
