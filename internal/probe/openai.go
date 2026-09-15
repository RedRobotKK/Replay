package probe

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Reading OpenAI's Responses usage, and refusing what it cannot say.
//
// The floor search in probe.go turns on Result.Wrote: did a prefix of size N
// cause the provider to create a cache entry. Anthropic answers that directly,
// reporting cache_creation_input_tokens beside cache_read_input_tokens.
//
// CORRECTED 2026-09-15, same day, before any run: OpenAI DOES report the
// write. usage.input_tokens_details.cache_write_tokens sits beside
// cached_tokens and is documented in the prompt-caching guide. The first
// version of this file said otherwise on the strength of a search summary
// that was never checked against the guide it summarised.
//
// Where the field is present, one response answers the question and the pair
// is not needed. Where it is absent, which is older models and the
// OpenAI-compatible third parties that normalise it away, the old reasoning
// still holds exactly: cached_tokens == 0 would be read as "this prefix did
// not cache", which pushes the floor's lower bound UP, and every such error
// moves the answer toward the vendor's published figure, so a run built on it
// can only ever agree with the documentation it was meant to test. The PAIR is
// the fallback for that case: send the same prefix twice, and if the second
// reads, the first wrote.

// openAIUsage is the part of a Responses reply that says anything about
// caching. Deliberately not a mirror of Anthropic's struct: the fields that do
// not exist here are absent rather than zero.
type openAIUsage struct {
	Model string
	Input int
	// CachedTokens is the READ.
	CachedTokens int
	// CacheWriteTokens is the WRITE, from
	// usage.input_tokens_details.cache_write_tokens.
	//
	// This file first shipped saying no such field existed, which was taken
	// from a search summary rather than from the guide. It exists, and the
	// error was expensive in the only currency that matters here: without it
	// a write had to be inferred from a second billable request at every
	// prefix size, so a search capped at 16 probes covered half the sizes it
	// could have, at cache-write rates.
	CacheWriteTokens int
	// WriteObserved distinguishes a write of zero from no write field at all.
	// With the field present, a zero read is decided; without it, the zero is
	// the ambiguity writeFromPair exists for.
	WriteObserved bool
}

// errNoWriteSignal is returned when a caller asks a lone response whether a
// write happened. It is a refusal, not a failure.
var errNoWriteSignal = errors.New(
	"this API reports a cached_tokens read and no write field, so one response cannot tell a prefix " +
		"the provider declined to cache from one it cached for the first time: send the same prefix " +
		"twice and read the second")

// parseOpenAIUsage reads a Responses reply.
//
// It succeeds when the provider reported a usable read, and refuses in the two
// cases that would otherwise be silently converted into a floor answer: a
// reply carrying no usage at all, and a reply whose zero read is being asked
// to stand in for a write.
func parseOpenAIUsage(raw []byte) (openAIUsage, error) {
	var parsed struct {
		Model string `json:"model"`
		Usage *struct {
			Input   int `json:"input_tokens"`
			Details *struct {
				Cached     int  `json:"cached_tokens"`
				CacheWrite *int `json:"cache_write_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return openAIUsage{}, fmt.Errorf("the provider's answer could not be read as usage")
	}
	// Absence is not a measurement. Same rule run.go applies to the other
	// provider, and for the same reason: a 200 with a reshaped usage object
	// once produced "floor above 61490" with no error and no caveat.
	if parsed.Usage == nil || parsed.Usage.Input <= 0 {
		return openAIUsage{}, fmt.Errorf("the provider's answer carried no usage, so it says nothing about caching")
	}
	if parsed.Usage.Details == nil {
		return openAIUsage{}, fmt.Errorf("the provider's answer carried no input_tokens_details, so its cached share is unknown rather than zero")
	}
	u := openAIUsage{
		Model:        parsed.Model,
		Input:        parsed.Usage.Input,
		CachedTokens: parsed.Usage.Details.Cached,
	}
	// A pointer, so a reported zero is told from a field that is not there.
	// That distinction is the whole of this function: one is the provider
	// saying it wrote nothing, the other is the provider saying nothing.
	if w := parsed.Usage.Details.CacheWrite; w != nil {
		u.CacheWriteTokens, u.WriteObserved = *w, true
		return u, nil
	}
	// No write field. A read of zero cannot be interpreted alone, and
	// returning it as a usable measurement is what lets a caller write
	// Wrote=false and manufacture a floor.
	if u.CachedTokens == 0 {
		return u, fmt.Errorf("prefix of %d tokens read nothing: %w", u.Input, errNoWriteSignal)
	}
	return u, nil
}

// writeFromPair answers the question a lone response cannot: did the first
// request write an entry.
//
// The evidence is the second request reading it. ok is false when the pair
// cannot settle it, which is when the two requests did not describe the same
// prefix: comparing a read at one size against a write at another measures
// nothing.
func writeFromPair(first, second openAIUsage) (wrote bool, ok bool) {
	if first.Input <= 0 || second.Input <= 0 {
		return false, false
	}
	if first.Input != second.Input {
		return false, false
	}
	// The second request reading is the write. The second request reading
	// nothing, at a prefix the first request already sent, is a real negative
	// on this API: the provider had its chance to serve an entry and did not.
	return second.CachedTokens > 0, true
}
