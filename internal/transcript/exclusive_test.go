package transcript

import (
	"fmt"
	"strings"
	"testing"
)

// The counting convention, on the surface that had it backwards.
//
// openai.go states the rule for the whole package: "This package's Usage is
// exclusive, so the adapter subtracts. Copying the provider's prompt figure
// into Input instead would count the cache twice, and the error grows with the
// cache hit rate." Codex counts inclusively for the same reason OpenAI does —
// cached_input_tokens is a SHARE of input_tokens, as codexUsage.usage()'s own
// refusal rule says — and the Codex reader copied instead of subtracting.
//
// Nothing on the Codex path called PromptTotal(), so the defect was live and
// unobservable at once, which is why it was filed as UNWIRED-LOG.md #8 with a
// measured 1.94x and left open: the guard against it (usage.FromInclusive,
// usage.Validate) sits in a package with zero importers.
//
// These tests make it observable from inside this package, which is where the
// conversion has to happen: internal/usage imports internal/transcript, so
// internal/transcript cannot import internal/usage back.

// codexTurn is one rollout carrying a single turn with the given counts.
func codexTurn(input, cached, output, reasoning int) string {
	return `{"timestamp":"t","type":"session_meta","payload":{"id":"s","cli_version":"1"}}` + "\n" +
		fmt.Sprintf(`{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":`+
			`{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"reasoning_output_tokens":%d,"total_tokens":%d}}}}`,
			input, cached, output, reasoning, input+output) + "\n"
}

// CX-1: the whole prompt is the prompt, counted once.
//
// A turn Codex reports as input_tokens 1000 with cached_input_tokens 800 is a
// 1,000-token prompt of which 800 came from cache. PromptTotal is
// Input+CacheCreation+CacheRead, so it must be 1000.
//
// PASS: PromptTotal() == 1000 and Input == 200, the uncached remainder.
// FAIL: PromptTotal() == 1800, which is the 1.94x on this fixture's shape and
// what shipped. Restore it by writing `Input: c.Input` in codexUsage.usage().
func TestCX1_CodexPromptIsCountedOnceNotTwice(t *testing.T) {
	s, err := ParseCodex(strings.NewReader(codexTurn(1000, 800, 10, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Billed.PromptTotal(); got != 1000 {
		t.Errorf("PromptTotal = %d, want 1000: a 1,000-token prompt with 800 of it "+
			"cached is 1,000 tokens. %d counts the cached share twice, and the "+
			"overstatement grows with the hit rate, so it is largest on exactly "+
			"the sessions anyone runs this tool for.", got, got)
	}
	if got := s.Billed.Input; got != 200 {
		t.Errorf("Input = %d, want 200. Usage.Input is the UNCACHED remainder on "+
			"every surface this package reads; openai.go says so for the sibling "+
			"adapter and this one must agree.", got)
	}
	if got := s.Billed.CacheRead; got != 800 {
		t.Errorf("CacheRead = %d, want 800", got)
	}
}

// CX-2: the headline figure did not move.
//
// CX-1 alone is satisfied by a reader that simply loses the cached tokens, and
// this project has shipped that shape before. Total() is what `replay codex`
// and `replay burn` print, and it must still be every token the provider
// counted: 1000 prompt + 10 output.
//
// PASS: Total() == 1010, unchanged by the conversion.
// FAIL: 210, which is what Total() returns if it stays `Input + Output` after
// Input becomes exclusive — the cache dropped out of the bill entirely.
func TestCX2_TheBilledTotalIsUnchangedByTheConversion(t *testing.T) {
	s, err := ParseCodex(strings.NewReader(codexTurn(1000, 800, 10, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Billed.Total(); got != 1010 {
		t.Errorf("Total = %d, want 1010 (1,000 prompt + 10 output). Total()'s own "+
			"comment says it is every token the provider counted; a figure that "+
			"omits the 800 cached tokens is not that.", got)
	}
}

// CX-3: the parts add up, which is the invariant usage.Validate enforces.
//
// Stated here as arithmetic rather than by calling usage.Validate, because the
// import direction forbids it. internal/usage carries the same assertion
// against a real parse in codex_normalises_test.go, which is the test that
// crosses the join.
//
// PASS: Fresh + read + write == Prompt on every turn.
// FAIL: any mapping that copies an inclusive figure into Input.
func TestCX3_TheCodexPartsAddUpToTheWhole(t *testing.T) {
	cases := []struct{ input, cached, output, reasoning int }{
		{1000, 800, 10, 0},
		{1000, 0, 10, 0},
		{2000, 2000, 100, 40},
		{1, 1, 1, 1},
	}
	for _, c := range cases {
		s, err := ParseCodex(strings.NewReader(codexTurn(c.input, c.cached, c.output, c.reasoning)))
		if err != nil {
			t.Fatal(err)
		}
		u := s.Billed
		if sum := u.Input + u.CacheRead + u.CacheCreation; sum != c.input {
			t.Errorf("input %d cached %d: parts sum to %d, prompt is %d",
				c.input, c.cached, sum, c.input)
		}
		if u.Input < 0 {
			t.Errorf("input %d cached %d: Input went negative (%d)", c.input, c.cached, u.Input)
		}
	}
}

// CX-4: a cache break is still seen, and still measured against the prompt.
//
// The share is CacheRead over the WHOLE prompt. Dividing by Input, which the
// break detector used to do and which is now the uncached remainder, gives
// 800/200 = 4.0 where the answer is 0.8 — so every warm turn would clear the
// warm threshold and no turn could ever look cold. The detector would go
// silent without the count changing, which is the shape this project calls a
// guard no test can observe.
//
// PASS: the collapse from a warm turn to a cold one is reported once, with
// ColdTokens equal to the uncached remainder of the cold turn.
// FAIL: no break, which is what dividing by Input produces.
func TestCX4_ACacheBreakSurvivesTheConversion(t *testing.T) {
	warm := `{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10000,"cached_input_tokens":9000,"output_tokens":10,"total_tokens":10010}}}}`
	cold := `{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10000,"cached_input_tokens":100,"output_tokens":10,"total_tokens":10010}}}}`
	in := `{"timestamp":"t","type":"session_meta","payload":{"id":"s","cli_version":"1"}}` + "\n" + warm + "\n" + cold + "\n"
	s, err := ParseCodex(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Breaks) != 1 {
		t.Fatalf("got %d break(s), want 1: a turn falling from 90%% cached to 1%% "+
			"is the collapse this detector exists for", len(s.Breaks))
	}
	b := s.Breaks[0]
	if b.ColdTokens != 9900 {
		t.Errorf("ColdTokens = %d, want 9900 (10,000 prompt less 100 cached)", b.ColdTokens)
	}
	if b.BeforeShare < 0.89 || b.BeforeShare > 0.91 {
		t.Errorf("BeforeShare = %v, want ~0.90 — the share is of the prompt, not of "+
			"the uncached remainder", b.BeforeShare)
	}
}

// CX-5: a rollout's turns are counted, and a file is not a turn.
//
// A Codex rollout FILE is one session and a session is many provider calls.
// `replay burn` incremented its "requests" column once per file, so a corpus of
// 148 rollouts covering thousands of turns reported 148 requests in the same
// column where the Ollama row reported real requests. Turns is what that column
// needs, and it has to exist here before the command can stop lying.
//
// PASS: three accepted turns count as three.
// FAIL: any count that follows the number of files, or that counts refused
// records as turns.
func TestCX5_TurnsCountProviderCallsNotFiles(t *testing.T) {
	in := `{"timestamp":"t","type":"session_meta","payload":{"id":"s","cli_version":"1"}}
{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"c","primary":{"used_percent":4,"window_minutes":300}}}}
{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":10,"output_tokens":5,"total_tokens":105}}}}
{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":200,"cached_input_tokens":20,"output_tokens":5,"total_tokens":205}}}}
{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":300,"cached_input_tokens":30,"output_tokens":5,"total_tokens":305}}}}
{"timestamp":"t","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":9999,"output_tokens":5,"total_tokens":15}}}}
`
	s, err := ParseCodex(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if s.Turns != 3 {
		t.Errorf("Turns = %d, want 3. The opening event carries quota and no usage, "+
			"and the last record is refused for reporting more cache than prompt; "+
			"neither is a provider call.", s.Turns)
	}
	if s.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", s.Skipped)
	}
}
