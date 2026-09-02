package analysis

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

const fixture = "../transcript/testdata/session-redacted.jsonl"

func loadFixture(t *testing.T) (*transcript.Session, *transcript.Lane) {
	t.Helper()
	s, err := transcript.ParseClaudeCodeFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return s, MainLane(s)
}

func TestCalibrationOnRealSession(t *testing.T) {
	_, lane := loadFixture(t)
	cal := Calibrate(lane)
	if cal.Compared() != len(lane.Requests)-1 {
		t.Fatalf("compared %d turns of %d requests", cal.Compared(), len(lane.Requests))
	}
	if !cal.Passes() {
		t.Fatalf("calibration failed on a real session: %+v", cal)
	}
	if cal.Broken != 1 || cal.Exceeded == 0 {
		t.Fatalf("expected the fixture's one known break and some sibling reads, got broken=%d exceeded=%d", cal.Broken, cal.Exceeded)
	}
}

func TestBreakIsClassifiedAsRerender(t *testing.T) {
	_, lane := loadFixture(t)
	cal := Calibrate(lane)
	fit := Fit(cal)
	breaks := FindBreaks(cal, fit)
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1", len(breaks))
	}
	b := breaks[0]
	if b.Cause != CauseRerendered {
		t.Fatalf("cause = %q, want %q (%s)", b.Cause, CauseRerendered, b.Detail)
	}
	if b.Deficit <= 0 || b.MessageIndex != 0 {
		t.Fatalf("break detail wrong: %+v", b)
	}
}

func TestFitUsesMeasuredPrefix(t *testing.T) {
	_, lane := loadFixture(t)
	fit := Fit(Calibrate(lane))
	if !fit.UnseenPrefixMeasured || fit.UnseenPrefixTokens != lane.Requests[0].Usage.CacheRead {
		t.Fatalf("system prefix should come from the first cache read: %+v", fit)
	}
	if fit.TokensPerByte <= 0 || fit.TokensPerByte > 2 {
		t.Fatalf("implausible fit: %+v", fit)
	}
	if fit.RelativeError > 2 {
		t.Fatalf("fit uncertainty too large to be useful: %+v", fit)
	}
}

// Attribution must sum to exactly what the provider reported: the first
// prompt plus every turn's new content.
func TestBlameSumsToReportedUsage(t *testing.T) {
	_, lane := loadFixture(t)
	cal := Calibrate(lane)
	fit := Fit(cal)
	entries := Blame(cal, fit)

	got := 0
	for _, e := range entries {
		got += e.Tokens.Value
	}
	want := lane.Requests[0].Usage.PromptTotal()
	for _, turn := range cal.Turns {
		if turn.Outcome == cachemodel.ReadFirst {
			continue
		}
		tc := splitTurn(turn)
		want += tc.outputTokens + tc.userTokens
	}
	if got != want {
		t.Fatalf("attributed %d tokens, provider reported %d", got, want)
	}
	if entries[0].PromptTokens.Value < entries[len(entries)-1].PromptTokens.Value {
		t.Fatal("blame is not sorted by prompt contribution")
	}
}

// Replaying the TTL a session actually used must reproduce as-run exactly,
// on the synthetic lane and on the real session, including the turns where
// the client's own behavior (a break, a sibling request) shaped the read.
func TestAsRunTTLReplayMatchesAsRun(t *testing.T) {
	_, session := loadFixture(t)
	for name, lane := range map[string]*transcript.Lane{"synthetic": syntheticLane(10, -1, 0, true), "fixture": session} {
		cal := Calibrate(lane)
		asRun := AsRun(lane)
		replayed := WithTTL(cal, cachemodel.TTLOf(lane.Requests[0].Usage))
		if math.Abs(replayed.EffectiveTokens-asRun.EffectiveTokens) > 0.5 || replayed.PromptTokens != asRun.PromptTokens {
			t.Fatalf("%s: replaying the as-run TTL must reproduce as-run: %.0f vs %.0f effective, %d vs %d prompt", name, replayed.EffectiveTokens, asRun.EffectiveTokens, replayed.PromptTokens, asRun.PromptTokens)
		}
	}
}

// syntheticLane builds a lane whose usage follows the invariant exactly, with
// a configurable gap before one turn.
func syntheticLane(turns int, gapAt int, gap time.Duration, ttl1h bool) *transcript.Lane {
	lane := &transcript.Lane{ID: "synthetic"}
	start := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	const (
		prefix   = 5000
		perTurn  = 800
		tail     = 20
		output   = 300
		thinking = 100
	)
	var ctx []*transcript.Message
	at := start
	for i := 0; i < turns; i++ {
		if i == gapAt {
			at = at.Add(gap)
		} else if i > 0 {
			at = at.Add(30 * time.Second)
		}
		user := &transcript.Message{UUID: "u" + string(rune('a'+i)), Role: transcript.RoleUser, Timestamp: at, Blocks: []transcript.Block{{Kind: transcript.KindToolResult, Label: "tool result: Read file.go", Bytes: 3000, Text: strings.Repeat("x", 3000), ToolName: "Read"}}}
		ctx = append(ctx, user)
		req := &transcript.Request{ID: "r" + string(rune('a'+i)), Model: "claude-fable-5-1", Effort: "high", Timestamp: at, Context: append([]*transcript.Message(nil), ctx...)}
		req.Output = &transcript.Message{UUID: "o" + string(rune('a'+i)), Role: transcript.RoleAssistant, Timestamp: at, Blocks: []transcript.Block{{Kind: transcript.KindThinking, Label: "assistant thinking"}, {Kind: transcript.KindText, Label: "assistant text", Bytes: 800}}}
		total := prefix + perTurn*(i+1)
		if i == 0 {
			req.Usage = transcript.Usage{Input: tail, CacheCreation: total - tail, Output: output, ThinkingTokens: thinking}
		} else {
			prev := lane.Requests[i-1].Usage
			read := prev.PromptTotal() - prev.Input
			req.Usage = transcript.Usage{Input: tail, CacheRead: read, CacheCreation: total - tail - read, Output: output, ThinkingTokens: thinking}
		}
		if ttl1h {
			req.Usage.Create1h = req.Usage.CacheCreation
		} else {
			req.Usage.Create5m = req.Usage.CacheCreation
		}
		ctx = append(ctx, req.Output)
		lane.Requests = append(lane.Requests, req)
	}
	return lane
}

func TestSyntheticLaneCalibratesPerfectly(t *testing.T) {
	lane := syntheticLane(10, -1, 0, true)
	cal := Calibrate(lane)
	if cal.Reproduced != 9 || cal.Broken != 0 || cal.Exceeded != 0 {
		t.Fatalf("synthetic lane should reproduce every turn: %+v", cal)
	}
}

func TestShortTTLMissesAcrossLongGap(t *testing.T) {
	lane := syntheticLane(10, 5, 8*time.Minute, true)
	cal := Calibrate(lane)
	short := WithTTL(cal, cachemodel.TTLShort)
	long := WithTTL(cal, cachemodel.TTLLong)
	if short.Misses != 1 || long.Misses != 0 {
		t.Fatalf("misses short=%d long=%d, want 1 and 0", short.Misses, long.Misses)
	}
	if short.CachedShare >= long.CachedShare {
		t.Fatalf("a miss must lower the cached share: %.3f vs %.3f", short.CachedShare, long.CachedShare)
	}
}

func TestContextEditShrinksPromptAndInvalidates(t *testing.T) {
	lane := syntheticLane(12, -1, 0, false)
	cal := Calibrate(lane)
	fit := Fit(cal)
	asRun := AsRun(lane)
	edited := WithContextEdit(cal, ContextEditPolicy{KeepLast: 2, TriggerTokens: 9000}, fit)
	if edited.PromptTokens >= asRun.PromptTokens {
		t.Fatalf("clearing tool results must shrink prompts: %d vs %d", edited.PromptTokens, asRun.PromptTokens)
	}
	if !edited.Estimated || !strings.HasPrefix(edited.ReachableLive, "yes") {
		t.Fatalf("context edit must be marked estimated and live-reachable: %+v", edited)
	}
}

func TestReportCarriesMandatoryLines(t *testing.T) {
	s, lane := loadFixture(t)
	rep := AnalyzeLane(s, lane)
	var buf bytes.Buffer
	if err := rep.WriteReplay(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Tier: estimated", "Calibration: reproduced provider cache reads on", "Assumption: " + AssumptionNote, "Rules: " + cachemodel.RulesVersion, "as-run", "provider retries: not visible"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q", want)
		}
	}
	buf.Reset()
	if err := rep.WriteDiff(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), string(CauseRerendered)) {
		t.Errorf("diff report does not name the break cause")
	}
}

func TestReportSurfacesWriteErrors(t *testing.T) {
	s, lane := loadFixture(t)
	rep := AnalyzeLane(s, lane)
	if err := rep.WriteReplay(failingWriter{}); err == nil {
		t.Fatal("expected the write error to be returned")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

var errWrite = &writeError{}

type writeError struct{}

func (*writeError) Error() string { return "write failed" }
