package probe

import (
	"strings"
	"testing"
)

// The trap this file exists to refuse.
//
// Probe decides a model's caching floor by asking whether a prefix of size N
// caused a WRITE. On Anthropic the provider says so directly:
// cache_creation_input_tokens is the write, cache_read_input_tokens is the
// read, and the two are separate fields.
//
// OpenAI's Responses API reports only usage.input_tokens_details.cached_tokens,
// which is the READ. There is no write field. A single response with
// cached_tokens == 0 is consistent with three different worlds:
//
//	the provider declined to cache this prefix   (a real floor answer)
//	the provider wrote a new entry               (the normal first request)
//	the entry existed and routing missed it      (OpenAI documents best effort)
//
// Collapsing those to Wrote=false is the failure run.go already names: it
// pushes the lower bound UP, which is the direction that manufactures a
// confirmation of the vendor's published figure. Publishing "we measured
// OpenAI's floor and it is exactly the 1024 they documented" on evidence that
// can only ever agree with them is worse than not measuring.
func TestOpenAIUsage_SingleResponseCannotSettleWhetherAWriteHappened(t *testing.T) {
	// A well formed Astra response for a prefix that read nothing.
	body := []byte(`{
	  "model": "gpt-6-astra",
	  "usage": {
	    "input_tokens": 4096,
	    "input_tokens_details": {"cached_tokens": 0},
	    "output_tokens": 12
	  }
	}`)

	_, err := parseOpenAIUsage(body)
	if err == nil {
		t.Fatal("a lone OpenAI response was accepted as a floor answer; cached_tokens==0 cannot tell " +
			"a declined prefix from a first write, and treating it as 'did not cache' manufactures " +
			"agreement with the documented floor")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Errorf("refusal does not say the write signal is what is missing, so a reader cannot tell "+
			"this from a transport failure: %v", err)
	}
}

// The read is reportable, because OpenAI does report it. Refusing the whole
// payload would be the opposite error: absence of a write field is not absence
// of a measurement.
func TestOpenAIUsage_ReadIsReportedBecauseTheProviderReportsIt(t *testing.T) {
	body := []byte(`{
	  "model": "gpt-6-astra",
	  "usage": {
	    "input_tokens": 4096,
	    "input_tokens_details": {"cached_tokens": 3072},
	    "output_tokens": 12
	  }
	}`)

	u, err := parseOpenAIUsage(body)
	if err != nil {
		t.Fatalf("a response carrying a real cached_tokens read was refused: %v", err)
	}
	if u.Input != 4096 {
		t.Errorf("input_tokens = %d, want 4096", u.Input)
	}
	if u.CachedTokens != 3072 {
		t.Errorf("cached_tokens = %d, want 3072", u.CachedTokens)
	}
	if u.Model != "gpt-6-astra" {
		t.Errorf("model = %q, want gpt-6-astra", u.Model)
	}
}

// Absence is not zero, the same rule run.go already applies to Anthropic. A
// 200 with a reshaped or missing usage object says nothing, and reading it as
// "cached nothing" is how a stub once produced "floor above 61490" with no
// error and no caveat.
func TestOpenAIUsage_MissingUsageIsRefusedRatherThanReadAsZero(t *testing.T) {
	for name, body := range map[string]string{
		"no usage object": `{"model":"gpt-6-astra"}`,
		"usage empty":     `{"model":"gpt-6-astra","usage":{}}`,
		"input zero":      `{"model":"gpt-6-astra","usage":{"input_tokens":0,"input_tokens_details":{"cached_tokens":0}}}`,
	} {
		if _, err := parseOpenAIUsage([]byte(body)); err == nil {
			t.Errorf("%s: accepted as a measurement; absence read as zero is how a floor gets invented", name)
		}
	}
}

// The write a probe needs is settled by the PAIR, not by one response: send
// the same prefix twice and ask whether the SECOND one read. That is the only
// evidence available on this API, it costs one extra billable request per
// size, and saying so is the difference between a method and a guess.
func TestOpenAIWrite_IsDerivedFromTheSecondRequestReading(t *testing.T) {
	first := openAIUsage{Input: 4096, CachedTokens: 0}
	second := openAIUsage{Input: 4096, CachedTokens: 3072}

	wrote, ok := writeFromPair(first, second)
	if !ok {
		t.Fatal("a pair where the second request read is exactly the evidence that the first wrote")
	}
	if !wrote {
		t.Error("second request read 3072 tokens of the first request's prefix, so the first wrote")
	}

	// Second request also read nothing: the prefix was never cached. This is
	// a real negative, unlike the single-response case.
	if wrote, ok := writeFromPair(first, openAIUsage{Input: 4096, CachedTokens: 0}); !ok || wrote {
		t.Errorf("two requests at the same prefix with no read = (wrote %v, ok %v), want (false, true)", wrote, ok)
	}
}

// Both branches the mutation sweep found unguarded on 2026-09-15.
//
// The first is the one that bites in practice: a reply with a real
// input_tokens and NO input_tokens_details. That is what a reshaped response,
// an older API version, or a gateway that strips the details block produces,
// and it is the exact shape the earlier "usage empty" case does not reach,
// because that one is caught by the input_tokens check first. Reading the
// absent details as a zero read turns a transport change into a floor answer.
func TestOpenAIUsage_DetailsAbsentWithRealInputIsRefused(t *testing.T) {
	body := []byte(`{"model":"gpt-6-astra","usage":{"input_tokens":4096,"output_tokens":12}}`)

	_, err := parseOpenAIUsage(body)
	if err == nil {
		t.Fatal("a reply with real input_tokens and no input_tokens_details was accepted; " +
			"its cached share is unknown, not zero")
	}
	if strings.Contains(err.Error(), "no usage") {
		t.Errorf("refused as though usage were missing entirely, which misnames what is wrong: %v", err)
	}
}

// The second: a pair has to be the SAME prefix twice. Comparing a read at one
// size against a send at another is not a measurement of either, and silently
// answering it would let the caller believe a size was settled when two
// different questions were asked.
func TestOpenAIWrite_MismatchedPrefixSizesCannotBePaired(t *testing.T) {
	first := openAIUsage{Input: 4096, CachedTokens: 0}
	second := openAIUsage{Input: 2048, CachedTokens: 2048}

	if wrote, ok := writeFromPair(first, second); ok {
		t.Errorf("paired a 4096 send with a 2048 read and answered wrote=%v; "+
			"the two requests did not describe the same prefix", wrote)
	}
}

// Both branches guard-reachability found unentered on 2026-09-15.
//
// They were written and never driven. The mutation sweep on this file missed
// them because a sweep only tests the conditions its author already thought
// about, and these two were the ones I had not: every fixture in this file was
// valid JSON, and every pair was built from two real prefixes.
//
// Neither is decorative. A provider answering 200 with a truncated or
// HTML-wrapped body is what a gateway, a captive portal or a proxy error page
// produces, and that reply reaching the floor search as anything other than a
// refusal is the "absence read as a measurement" failure this file exists to
// stop.
func TestOpenAIUsage_AReplyThatIsNotJSONIsRefusedAndSaysSo(t *testing.T) {
	for name, body := range map[string]string{
		"html error page": `<html><body>502 Bad Gateway</body></html>`,
		"truncated json":  `{"model":"gpt-6-astra","usage":{"input_tokens":4096`,
		"empty":           ``,
	} {
		u, err := parseOpenAIUsage([]byte(body))
		if err == nil {
			t.Errorf("%s: accepted as usage (%+v); a body that is not JSON says nothing about caching", name, u)
			continue
		}
		// The refusal has to NAME this failure, not fall through to the one
		// below it. Deleting the unmarshal check leaves parsed.Usage nil and
		// the next branch refuses too, so a test asserting only err != nil
		// passes either way: guard-reachability reported exactly that on
		// 2026-09-15, "these branches run, and no test depends on whether
		// they did".
		//
		// The distinction is the operator's, not the compiler's. A truncated
		// or HTML-wrapped body is a gateway between them and the provider; a
		// 200 carrying a reshaped usage object is the provider answering in a
		// shape this build does not know. Those have different fixes, and a
		// refusal that calls the first one "carried no usage" sends the reader
		// to the wrong one.
		if strings.Contains(err.Error(), "carried no usage") {
			t.Errorf("%s: refused as though usage were missing, which points the reader at the "+
				"provider when the body never parsed: %v", name, err)
		}
		if !strings.Contains(err.Error(), "could not be read") {
			t.Errorf("%s: refusal does not say the body could not be read: %v", name, err)
		}
	}
}

// A pair needs two real requests. A zero Input is a request that was never
// sent or never measured, and pairing it would let the caller believe a size
// was settled by one request and a placeholder.
func TestOpenAIWrite_APairNeedsTwoRealRequests(t *testing.T) {
	measured := openAIUsage{Input: 4096, CachedTokens: 3072}

	for name, pair := range map[string][2]openAIUsage{
		"first never measured":  {{Input: 0}, measured},
		"second never measured": {measured, {Input: 0}},
		"neither measured":      {{Input: 0}, {Input: 0}},
	} {
		if wrote, ok := writeFromPair(pair[0], pair[1]); ok {
			t.Errorf("%s: answered wrote=%v from a pair with no measured prefix", name, wrote)
		}
	}
}
