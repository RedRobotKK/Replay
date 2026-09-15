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
// OpenAI reports usage.input_tokens_details.cached_tokens and nothing else.
// That is the READ. There is no write field on this API, so a single response
// cannot answer the question the search asks, and the shape of the mistake is
// specific: cached_tokens == 0 would be read as "this prefix did not cache",
// which pushes the floor's lower bound UP. Every such error moves the answer
// toward the vendor's published figure, so a run built on it can only ever
// agree with the documentation it was meant to test.
//
// The evidence that does exist is the PAIR. Send the same prefix twice: if the
// second request reads, the first one wrote. That costs one extra billable
// request per size and it is the only honest signal on this API.

// openAIUsage is the part of a Responses reply that says anything about
// caching. Deliberately not a mirror of Anthropic's struct: the fields that do
// not exist here are absent rather than zero.
type openAIUsage struct {
	Model string
	Input int
	// CachedTokens is the READ. There is no write counterpart on this API.
	CachedTokens int
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
				Cached int `json:"cached_tokens"`
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
	// A read of zero is the case that cannot be interpreted alone. Returning
	// it as a usable measurement is what lets a caller write Wrote=false and
	// manufacture a floor.
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
