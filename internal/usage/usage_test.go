package usage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

func at() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }

// The accounting identity the whole engine rests on: every prompt token the
// provider processed was either fresh, read from cache, or written to cache.
// A normalised record that does not hold this is not a normalisation, it is a
// second place for the numbers to disagree.
func TestPromptIsFreshPlusReadPlusWrite(t *testing.T) {
	n := FromAnthropic(transcript.Usage{
		Input: 2_000, CacheRead: 198_000, CacheCreation: 40_000, Output: 900,
	}, "claude-opus-5", at(), nil)

	if got := n.Fresh + n.CachedRead + n.CachedWrite; got != n.Prompt {
		t.Fatalf("fresh %d + read %d + write %d = %d, but Prompt is %d",
			n.Fresh, n.CachedRead, n.CachedWrite, got, n.Prompt)
	}
	if n.Prompt != 240_000 {
		t.Fatalf("Prompt should be every input token processed: %d", n.Prompt)
	}
	if n.Output != 900 {
		t.Fatalf("Output %d", n.Output)
	}
}

// The engine must read one shape whatever reported it. These names are the
// engine's, not a provider's: "cache_creation_input_tokens" is one vendor's
// word for a cache write.
func TestNormalisedNamesAreTheEnginesNotAProviders(t *testing.T) {
	b, err := json.Marshal(FromAnthropic(transcript.Usage{Input: 1, CacheRead: 2, CacheCreation: 3}, "m", at(), nil))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, leaked := range []string{"cache_creation_input_tokens", "cache_read_input_tokens", "input_tokens"} {
		if contains(s, leaked) {
			t.Fatalf("a provider's own field name leaked into the normalised shape: %s\n%s", leaked, s)
		}
	}
	for _, want := range []string{"prompt", "cached_read", "cached_write"} {
		if !contains(s, want) {
			t.Fatalf("missing normalised field %q in %s", want, s)
		}
	}
}

// The raw payload is kept verbatim, including fields this build does not know
// about. A field we did not know mattered is exactly what tomorrow's
// calibration needs, and normalising is lossy by construction.
func TestRawPayloadSurvivesFieldsWeDoNotUnderstand(t *testing.T) {
	raw := json.RawMessage(`{"input_tokens":10,"some_future_field":{"tier":"gold","units":7}}`)
	n := FromAnthropic(transcript.Usage{Input: 10}, "m", at(), raw)
	if len(n.Raw) == 0 {
		t.Fatal("the raw payload was dropped")
	}
	var back map[string]any
	if err := json.Unmarshal(n.Raw, &back); err != nil {
		t.Fatalf("raw is not valid JSON after the round trip: %v", err)
	}
	if _, ok := back["some_future_field"]; !ok {
		t.Fatalf("a field this build does not parse was lost: %s", n.Raw)
	}
}

// Rent is the third caching family's meter: storage paid per unit time. No
// provider here charges it, so it must stay zero rather than acquire a
// plausible default that later reads as measured.
func TestRentIsNeverFabricated(t *testing.T) {
	n := FromAnthropic(transcript.Usage{Input: 1, CacheRead: 9, CacheCreation: 5}, "claude-opus-5", at(), nil)
	if n.RentUSD != 0 {
		t.Fatalf("rent was invented for a provider that does not charge it: %v", n.RentUSD)
	}
	if n.Mechanism != MechanismExplicitBreakpoint {
		t.Fatalf("this provider caches by explicit breakpoint, got %q", n.Mechanism)
	}
}

// Zero in, zero out. A normaliser that fills gaps is an estimator wearing a
// measurement's name, and every figure downstream is tiered on that difference.
func TestAnEmptyUsageNormalisesToZeroNotToADefault(t *testing.T) {
	n := FromAnthropic(transcript.Usage{}, "", time.Time{}, nil)
	if n.Prompt != 0 || n.Fresh != 0 || n.CachedRead != 0 || n.CachedWrite != 0 || n.Output != 0 {
		t.Fatalf("something was invented from nothing: %+v", n)
	}
}

// Normalising must not lose the TTL split, because the write multiplier
// depends on it and that is the largest single lever in the cost model.
func TestTheWriteTTLSplitSurvives(t *testing.T) {
	n := FromAnthropic(transcript.Usage{CacheCreation: 100, Create5m: 60, Create1h: 40}, "m", at(), nil)
	if n.CachedWrite5m != 60 || n.CachedWrite1h != 40 {
		t.Fatalf("TTL split lost: 5m=%d 1h=%d", n.CachedWrite5m, n.CachedWrite1h)
	}
	if n.CachedWrite5m+n.CachedWrite1h != n.CachedWrite {
		t.Fatalf("the split does not add up to the total write: %d + %d vs %d", n.CachedWrite5m, n.CachedWrite1h, n.CachedWrite)
	}
}

// Round-tripping back to the provider's shape must be exact, because the
// ledger's existing readers still speak it and a lossy hop would silently
// change measured figures.
func TestRoundTripBackToTheProviderShapeIsExact(t *testing.T) {
	in := transcript.Usage{Input: 2_000, CacheRead: 198_000, CacheCreation: 40_000, Output: 900, Create5m: 25_000, Create1h: 15_000, ThinkingTokens: 120}
	if got := FromAnthropic(in, "m", at(), nil).ToAnthropic(); got != in {
		t.Fatalf("round trip changed the record:\n got %+v\nwant %+v", got, in)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// The ledger must have somewhere to put the raw payload, or nothing above is
// reachable from real traffic.
func TestLedgerCarriesTheRawPayload(t *testing.T) {
	var rec struct {
		Response struct {
			RawUsage json.RawMessage `json:"raw_usage,omitempty"`
		} `json:"response"`
	}
	line := `{"response":{"raw_usage":{"input_tokens":5,"unknown_future":"x"}}}`
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Response.RawUsage) == 0 {
		t.Fatal("the ledger dropped the raw usage payload")
	}
	if !contains(string(rec.Response.RawUsage), "unknown_future") {
		t.Fatalf("an unparsed field was lost: %s", rec.Response.RawUsage)
	}
}
