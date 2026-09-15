package probe

import "testing"

// The correction. openai.go shipped on the claim that this API reports no
// cache-write field, so a write could only be inferred from a PAIR of
// requests. That claim was taken from a search summary and stated as though
// the guide had been read.
//
// OpenAI's prompt-caching guide documents
// usage.input_tokens_details.cache_write_tokens beside cached_tokens. The
// field exists. Every request that writes says how much it wrote, and the
// pair was never needed where this field is present.
//
// The cost of the error was not correctness, it was money: the pair method
// doubles the billable requests per prefix size, and a floor search capped at
// 16 probes therefore covered half the sizes it could have. On a search
// someone is paying for at $12.50 per million cache-write tokens, that is the
// whole error.
func TestOpenAIUsage_TheWriteFieldIsReadWhenTheProviderSendsIt(t *testing.T) {
	body := []byte(`{
	  "model": "gpt-6-astra",
	  "usage": {
	    "input_tokens": 4096,
	    "input_tokens_details": {"cached_tokens": 0, "cache_write_tokens": 4096},
	    "output_tokens": 9
	  }
	}`)

	u, err := parseOpenAIUsage(body)
	if err != nil {
		t.Fatalf("a reply reporting a real cache write was refused: %v", err)
	}
	if u.CacheWriteTokens != 4096 {
		t.Errorf("cache_write_tokens = %d, want 4096", u.CacheWriteTokens)
	}
	if !u.WriteObserved {
		t.Error("a reply carrying cache_write_tokens must report the write as OBSERVED; " +
			"inferring it from a second request would bill a probe nobody needed")
	}
}

// A zero read is only undecidable when the write field is absent. With the
// field present and zero, the provider has said it wrote nothing, and that is
// a measurement rather than a silence.
func TestOpenAIUsage_AZeroReadIsDecidableOnceTheWriteFieldIsPresent(t *testing.T) {
	body := []byte(`{
	  "model": "gpt-6-astra",
	  "usage": {
	    "input_tokens": 900,
	    "input_tokens_details": {"cached_tokens": 0, "cache_write_tokens": 0},
	    "output_tokens": 9
	  }
	}`)

	u, err := parseOpenAIUsage(body)
	if err != nil {
		t.Fatalf("a reply that explicitly reported a zero write was refused as undecidable: %v.\n"+
			"      cached_tokens==0 is ambiguous ONLY when nothing says whether a write happened", err)
	}
	if u.CacheWriteTokens != 0 || !u.WriteObserved {
		t.Errorf("write = %d observed = %v, want 0 and observed", u.CacheWriteTokens, u.WriteObserved)
	}
}

// The old path has to survive for replies that genuinely lack the field:
// older models, OpenAI-compatible third parties, and anything normalising
// through a layer that drops it. Absent is still not zero.
func TestOpenAIUsage_WithoutTheWriteFieldTheZeroReadStaysUndecidable(t *testing.T) {
	body := []byte(`{
	  "model": "some-openai-compatible",
	  "usage": {
	    "input_tokens": 4096,
	    "input_tokens_details": {"cached_tokens": 0},
	    "output_tokens": 9
	  }
	}`)

	u, err := parseOpenAIUsage(body)
	if err == nil {
		t.Fatal("a zero read with NO write field was accepted; that is the ambiguity the pair " +
			"method exists for and it has not gone away")
	}
	if u.WriteObserved {
		t.Error("write reported as observed when the field was absent")
	}
}
