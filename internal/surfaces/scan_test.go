package surfaces

import "strings"

import "testing"

// Field shapes below are the real ones, copied from transcripts on one machine
// 2026-09-15. Each surface names its counters differently and nests them
// differently, and one of them names two different quantities the same thing.

func TestScan_AnthropicShapeCountsBothHalves(t *testing.T) {
	in := `{"message":{"role":"assistant","usage":{"cache_read_input_tokens":22078,"cache_creation_input_tokens":14644}}}
{"message":{"role":"assistant","usage":{"cache_read_input_tokens":30000,"cache_creation_input_tokens":500}}}`
	got := Scan(strings.NewReader(in), FieldSpec{
		Name: "Claude Code", ReadKey: "cache_read_input_tokens", WriteKey: "cache_creation_input_tokens"})
	if got.Records != 2 || got.Reads != 52078 || got.Writes != 15144 {
		t.Fatalf("got %+v, want 2 records / 52078 read / 15144 written", got)
	}
	if !got.WriteFieldPresent {
		t.Error("write key was present in both records and was not recorded as present")
	}
}

func TestScan_AWriteKeyThatNeverAppearsIsNotPresent(t *testing.T) {
	// Codex: cached_input_tokens with no write counterpart anywhere in the shape.
	in := `{"payload":{"info":{"last_token_usage":{"input_tokens":12792,"cached_input_tokens":9344}}}}`
	got := Scan(strings.NewReader(in), FieldSpec{
		Name: "Codex", ReadKey: "cached_input_tokens", WriteKey: "cache_creation_input_tokens"})
	if got.Reads != 9344 {
		t.Fatalf("reads %d, want 9344", got.Reads)
	}
	if got.WriteFieldPresent {
		t.Fatal("a write key that appears in no record was recorded as present; that turns an " +
			"absent field into a reported zero, which is the distinction this package exists for")
	}
	if Classify(got) != StateWriteFieldAbsent {
		t.Fatalf("classified %q, want %q", Classify(got), StateWriteFieldAbsent)
	}
}

func TestScan_AWriteKeyPresentAndZeroIsPresent(t *testing.T) {
	// Grok: cacheCreationTokens exists on every record and is zero on every record.
	in := `{"usage":{"cachedReadTokens":20656640,"cacheCreationTokens":0}}
{"usage":{"cachedReadTokens":12,"cacheCreationTokens":0}}`
	got := Scan(strings.NewReader(in), FieldSpec{
		Name: "Grok", ReadKey: "cachedReadTokens", WriteKey: "cacheCreationTokens"})
	if !got.WriteFieldPresent {
		t.Fatal("a write key present on every record was not recorded as present")
	}
	if got.Writes != 0 {
		t.Fatalf("writes %d, want 0", got.Writes)
	}
	if Classify(got) != StateWriteReportedZero {
		t.Fatalf("classified %q, want %q", Classify(got), StateWriteReportedZero)
	}
}

// The experimenter made exactly this mistake on 2026-09-15 and produced a token
// count with a decimal point. OpenClaw carries `cacheRead` twice per record:
// once under /message/usage in TOKENS and once under /message/usage/cost in
// DOLLARS. Summing both silently mixes units.
func TestScan_TheDollarTwinOfATokenKeyIsNotSummed(t *testing.T) {
	in := `{"message":{"usage":{"cacheRead":117928,"cacheWrite":0,"cost":{"cacheRead":0.0177,"cacheWrite":0}}}}`
	got := Scan(strings.NewReader(in), FieldSpec{
		Name: "OpenClaw", ReadKey: "cacheRead", WriteKey: "cacheWrite", ExcludeUnder: "cost"})
	if got.Reads != 117928 {
		t.Fatalf("reads %d, want 117928: the 0.0177 under /cost is dollars and must not be added "+
			"to a token count merely because it shares a key name", got.Reads)
	}
	if got.Records != 1 {
		t.Fatalf("records %d, want 1: the dollar twin must not be counted as a second record", got.Records)
	}
}

func TestScan_MalformedLinesAreSkippedNotCounted(t *testing.T) {
	in := "{not json\n" + `{"usage":{"cachedReadTokens":5,"cacheCreationTokens":1}}` + "\n\n"
	got := Scan(strings.NewReader(in), FieldSpec{ReadKey: "cachedReadTokens", WriteKey: "cacheCreationTokens"})
	if got.Records != 1 || got.Reads != 5 {
		t.Fatalf("got %+v, want 1 record / 5 read", got)
	}
}

func TestScan_NonNumericCountersDoNotBecomeZero(t *testing.T) {
	// A string where a number belongs is a broken reader, not a zero.
	in := `{"usage":{"cachedReadTokens":"lots","cacheCreationTokens":0}}`
	got := Scan(strings.NewReader(in), FieldSpec{ReadKey: "cachedReadTokens", WriteKey: "cacheCreationTokens"})
	if got.Records != 0 {
		t.Fatalf("records %d, want 0: a record whose read counter is not a number establishes nothing", got.Records)
	}
}
