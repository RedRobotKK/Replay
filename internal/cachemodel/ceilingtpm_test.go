package cachemodel

import (
	"strings"
	"testing"
)

// MeteredNote says a cache-blind ceiling runs high, and on this machine it
// runs 8.11x high. A reader can carry that one step too far.
//
// The ratio is about MONEY. A cached read is billed at roughly a tenth of an
// uncached one, so counting it at full rate overstates the bill and halts an
// agent early. That is the defect this file names.
//
// It is NOT true of the other throttle. OpenAI's prompt-caching guide, read
// 2026-09-15: "Cached input tokens still count toward tokens-per-minute
// limits. Prompt caching does not change how rate limits are calculated."
// Against a rate limit a cached read costs full price, so cache-blind
// arithmetic is exactly right there and the 8.11x does not apply.
//
// Same token, two throttles, opposite arithmetic. A reader who takes the
// discount to their rate-limit planning has capacity figures an order of
// magnitude optimistic, and nothing on the current report warns them.
func TestMeteredNoteSaysTheRatioIsAboutMoneyNotRateLimits(t *testing.T) {
	e := CeilingEffect{Requests: 100, BlindUSD: 800, CorrectUSD: 100}

	note := e.MeteredNote(0)
	low := strings.ToLower(note)
	if !strings.Contains(low, "rate limit") && !strings.Contains(low, "tokens-per-minute") &&
		!strings.Contains(low, "tokens per minute") {
		t.Fatalf("the metered note states a ratio that holds for money and not for rate "+
			"limits, and never says which throttle it means:\n  %s", note)
	}
}

// The clause must travel with the figure that needs it, including the form
// that names a ceiling. That is the one an operator acts on.
func TestMeteredNoteWithACeilingAlsoCarriesTheDistinction(t *testing.T) {
	e := CeilingEffect{Requests: 100, BlindUSD: 800, CorrectUSD: 100}

	note := e.MeteredNote(500)
	if !strings.Contains(strings.ToLower(note), "rate limit") {
		t.Errorf("the ceiling form drops the distinction the no-ceiling form carries:\n  %s", note)
	}
}

// It must not appear where there is no ratio. A refusal that grows an
// unrelated caveat reads as though something was measured.
func TestUnmeasuredCeilingStaysARefusal(t *testing.T) {
	var e CeilingEffect

	note := e.MeteredNote(0)
	if !strings.Contains(note, "NOT MEASURED") {
		t.Fatalf("an unpriced corpus must still refuse:\n  %s", note)
	}
	if strings.Contains(strings.ToLower(note), "rate limit") {
		t.Errorf("a refusal carried a rate-limit caveat about a ratio it does not have:\n  %s", note)
	}
}
