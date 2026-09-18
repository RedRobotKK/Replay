package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Grok CLI keeps one directory per session under
// ~/.grok/sessions/<urlencoded-cwd>/<session-id>/, and writes the usage
// object onto a `turn_completed` update in updates.jsonl. Every fixture here
// is constructed; nothing in this package may read the developer's ~/.grok,
// because a test whose result depends on the machine it runs on measures the
// machine.
func grokTurnLine(ts int64, sessionID string, input, output, cached, created, reasoning int) string {
	total := input + output
	return fmt.Sprintf(`{"timestamp":%d,"method":"_x.ai/session/update","params":{"sessionId":%q,`+
		`"update":{"sessionUpdate":"turn_completed","stop_reason":"end_turn","usage":`+
		`{"inputTokens":%d,"outputTokens":%d,"totalTokens":%d,"cachedReadTokens":%d,`+
		`"cacheCreationTokens":%d,"reasoningTokens":%d,"modelCalls":1,"apiDurationMs":4927,`+
		`"costUsdTicks":43574400,"modelUsage":{"grok-4.6-build":{"inputTokens":%d,"outputTokens":%d,`+
		`"totalTokens":%d,"cachedReadTokens":%d,"cacheCreationTokens":%d,"reasoningTokens":%d,`+
		`"modelCalls":1,"costUsdTicks":43574400}}}}}}`,
		ts, sessionID, input, output, total, cached, created, reasoning,
		input, output, total, cached, created, reasoning)
}

// grokUsageRecord is one usage object with every field stated, so a test can
// break exactly one rule at a time.
func grokUsageLine(usage string) string {
	return `{"timestamp":1789191644,"method":"_x.ai/session/update","params":{"sessionId":"s",` +
		`"update":{"sessionUpdate":"turn_completed","usage":` + usage + `}}}`
}

func writeGrokSession(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// GR1: every turn_completed usage object is read, and nothing else is.
//
// The file interleaves tool calls, message chunks and thought chunks with the
// turn records — 2,731 tool_call_update lines against 133 turn_completed in
// the largest session measured. A reader that counted lines, or that took the
// usage off any update that carried one, would report a different number of
// provider calls than were made.
func TestGR1_EveryCompletedTurnIsRead(t *testing.T) {
	dir := writeGrokSession(t, filepath.Join(t.TempDir(), "01a09421-sess"), map[string]string{
		"updates.jsonl": strings.Join([]string{
			`{"timestamp":1,"method":"_x.ai/session/update","params":{"sessionId":"01a09421-sess","update":{"sessionUpdate":"tool_call","title":"read"}}}`,
			grokTurnLine(1789191644, "01a09421-sess", 17415, 35, 6272, 0, 30),
			`{"timestamp":3,"method":"_x.ai/session/update","params":{"sessionId":"01a09421-sess","update":{"sessionUpdate":"agent_thought_chunk"}}}`,
			grokTurnLine(1789191676, "01a09421-sess", 1000, 20, 800, 0, 5),
			``,
			`not json at all`,
		}, "\n") + "\n",
	})

	s, err := ParseGrokSession(dir)
	if err != nil {
		t.Fatalf("ParseGrokSession: %v", err)
	}
	if got, want := len(s.Turns), 2; got != want {
		t.Fatalf("read %d turn(s), want %d: a turn is a provider call, and this "+
			"count is what every per-request figure downstream divides by", got, want)
	}
	if s.ID != "01a09421-sess" {
		t.Errorf("session id %q, want the id the records carry", s.ID)
	}
	if s.Source != SourceGrok {
		t.Errorf("source %q, want %q", s.Source, SourceGrok)
	}
	if got := s.Turns[0].Usage.InputTokens; got != 17415 {
		t.Errorf("first turn input %d, want 17415", got)
	}
	// The provider's own counts are carried verbatim at this layer. The
	// subtraction that turns an inclusive prompt into a fresh count belongs to
	// usage.FromInclusive, which this package cannot import, and doing it in
	// both places is how it gets done twice.
	if got := s.Turns[0].Usage.CachedReadTokens; got != 6272 {
		t.Errorf("first turn cached read %d, want 6272 carried verbatim", got)
	}
	if got := s.Turns[0].Model; got != "grok-4.6-build" {
		t.Errorf("model %q, want the single key modelUsage names", got)
	}
	if s.Turns[0].At.Unix() != 1789191644 {
		t.Errorf("turn timestamp %v, want the record's own", s.Turns[0].At)
	}
	if len(s.Turns[0].Raw) == 0 {
		t.Errorf("the provider's usage object was not kept; a field nobody knew " +
			"mattered is exactly what a later calibration needs")
	}
	// A line that is not JSON is refused and counted, not silently dropped.
	if len(s.Refused) != 1 {
		t.Errorf("refused %d record(s), want 1 for the unparseable line: %v", len(s.Refused), s.Refused)
	}
}

// GR2: a record this reader cannot trust is refused with its reason, and is
// not added to a total.
//
// Absent is not zero. Each row below states one thing that cannot be true of a
// usage object, and each must keep its tokens out of the sum rather than
// contributing a silent nothing.
func TestGR2_RecordsThatCannotBeTrustedAreRefused(t *testing.T) {
	cases := []struct {
		name   string
		usage  string
		reason string
	}{
		{
			// The reason is pinned to the phrase, not to the field name. Both
			// of the next two rows name cachedReadTokens, so a check on the
			// name alone cannot tell which rule fired and the rule that reads
			// the cache against the input on its own could be deleted without
			// a test noticing.
			"cached read exceeds the input it is part of",
			`{"inputTokens":100,"outputTokens":10,"totalTokens":110,"cachedReadTokens":200,"cacheCreationTokens":0,"reasoningTokens":0}`,
			"cachedReadTokens 200 exceeds inputTokens 100",
		},
		{
			"cached read plus write exceeds the input",
			`{"inputTokens":100,"outputTokens":10,"totalTokens":110,"cachedReadTokens":60,"cacheCreationTokens":60,"reasoningTokens":0}`,
			"plus cacheCreationTokens 60",
		},
		{
			"reasoning exceeds the output it is part of",
			`{"inputTokens":100,"outputTokens":10,"totalTokens":110,"cachedReadTokens":0,"cacheCreationTokens":0,"reasoningTokens":40}`,
			"reasoningTokens 40 exceeds outputTokens 10",
		},
		{
			"a negative counter",
			`{"inputTokens":-1,"outputTokens":10,"totalTokens":9,"cachedReadTokens":0,"cacheCreationTokens":0,"reasoningTokens":0}`,
			"negative",
		},
		{
			"a total with no breakdown at all",
			`{"inputTokens":0,"outputTokens":0,"totalTokens":5000,"cachedReadTokens":0,"cacheCreationTokens":0,"reasoningTokens":0}`,
			"totalTokens 5000 arrived with no breakdown",
		},
		{
			"the parts do not make the total",
			`{"inputTokens":100,"outputTokens":10,"totalTokens":999,"cachedReadTokens":50,"cacheCreationTokens":0,"reasoningTokens":0}`,
			"is not totalTokens 999",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeGrokSession(t, filepath.Join(t.TempDir(), "s"), map[string]string{
				"updates.jsonl": grokUsageLine(c.usage) + "\n",
			})
			s, err := ParseGrokSession(dir)
			if err != nil {
				t.Fatalf("ParseGrokSession: %v", err)
			}
			if len(s.Turns) != 0 {
				t.Errorf("a record that cannot be true was added to the total: %+v", s.Turns)
			}
			if len(s.Refused) != 1 {
				t.Fatalf("refused %d record(s), want 1: a format this reader does not "+
					"understand must be counted, not passed over", len(s.Refused))
			}
			if !strings.Contains(s.Refused[0].Reason, c.reason) {
				t.Errorf("refusal reads %q, want it to name %q: a bare count says "+
					"something was dropped without saying what", s.Refused[0].Reason, c.reason)
			}
			if s.Refused[0].Line != 1 {
				t.Errorf("refusal on line %d, want 1", s.Refused[0].Line)
			}
		})
	}
}

// GR3: a turn spanning more than one model names none of them.
//
// The model decides the price, and a session billed at two rates cannot be
// attributed to one of them. Empty means unknown here, which is what keeps a
// guessed model out of a figure that would look measured.
func TestGR3_AModelIsNamedOnlyWhenTheRecordNamesOne(t *testing.T) {
	two := `{"inputTokens":100,"outputTokens":10,"totalTokens":110,"cachedReadTokens":0,` +
		`"cacheCreationTokens":0,"reasoningTokens":0,"modelUsage":{"grok-4.6-build":{},"grok-4.6-fast":{}}}`
	dir := writeGrokSession(t, filepath.Join(t.TempDir(), "s"), map[string]string{
		"updates.jsonl": grokUsageLine(two) + "\n",
	})
	s, err := ParseGrokSession(dir)
	if err != nil {
		t.Fatalf("ParseGrokSession: %v", err)
	}
	if len(s.Turns) != 1 {
		t.Fatalf("read %d turn(s), want 1: two models is not a reason to refuse the tokens", len(s.Turns))
	}
	if s.Turns[0].Model != "" {
		t.Errorf("model %q, want empty: one of two models is a guess", s.Turns[0].Model)
	}
}

// GR4: the session's own rollup is read, and is kept apart from the sum of
// the turns.
//
// usage.json carries a `session` object that is NOT another turn. On the
// corpus this was written against, a parent session's rollup absorbs the usage
// of sub-agent sessions that are also on disk in their own right, so adding it
// to the per-turn sum counts those tokens twice. Measured: 33 of 34 rollups
// equal their own per-turn sum, and the one that does not is a session that
// spawned sub-agents.
func TestGR4_TheRollupIsReadAndKeptApart(t *testing.T) {
	dir := writeGrokSession(t, filepath.Join(t.TempDir(), "s"), map[string]string{
		"updates.jsonl": grokTurnLine(1789191644, "s", 1000, 20, 800, 0, 5) + "\n",
		"usage.json": `{"sessionId":"s","session":{"inputTokens":5000,"outputTokens":40,` +
			`"totalTokens":5040,"cachedReadTokens":4000,"cacheCreationTokens":0,` +
			`"reasoningTokens":9,"modelCalls":7,"costUsdTicks":1,"turnCount":2}}` + "\n",
	})
	s, err := ParseGrokSession(dir)
	if err != nil {
		t.Fatalf("ParseGrokSession: %v", err)
	}
	if len(s.Turns) != 1 {
		t.Fatalf("read %d turn(s), want 1: the rollup is not a turn", len(s.Turns))
	}
	if s.Rollup == nil {
		t.Fatalf("no rollup read, and usage.json is on disk beside the updates")
	}
	if got := s.Rollup.InputTokens; got != 5000 {
		t.Errorf("rollup input %d, want 5000", got)
	}
}

// GR5: a session directory with nothing to read is not an error.
//
// 2 of the 106 session directories measured carry no completed turn at all.
// A session that was opened and never used spent nothing, and a reader that
// errors there reports a broken machine.
func TestGR5_AnEmptySessionIsNotAnError(t *testing.T) {
	dir := writeGrokSession(t, filepath.Join(t.TempDir(), "s"), map[string]string{
		"updates.jsonl": "",
	})
	s, err := ParseGrokSession(dir)
	if err != nil {
		t.Fatalf("ParseGrokSession on a session with no turns: %v", err)
	}
	if len(s.Turns) != 0 || len(s.Refused) != 0 {
		t.Errorf("an empty session produced %d turn(s) and %d refusal(s)", len(s.Turns), len(s.Refused))
	}
}
