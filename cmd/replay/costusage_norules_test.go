package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/usage"
)

// UONR: `replay cost --usage` does not name Anthropic's TTL as the cause of a
// break by a model with no published cache rules here (2026-10-16).
//
// s1 opens on a priced Claude model so the session is a row. Minute 1 moves
// to gpt-5 and reads nothing: a model change. Minute 11 is gpt-5 again, ten
// minutes later, and reads nothing: before the fix this was "cache expired"
// by Anthropic's five-minute TTL; the cause is not measured.
func TestUONR1_TheUsageExportDoesNotUseAnthropicTTLForAModelWithoutRules(t *testing.T) {
	recs := []fixtureRecord{
		{0, 9000, 1000, 0, 8000, 100, usageFixtureModel, "s1"},
		{1, 9000, 1000, 0, 8000, 100, "gpt-5", "s1"},
		{11, 9000, 1000, 0, 8000, 100, "gpt-5", "s1"},
	}
	e, err := usage.ParseExport([]byte(exportDoc(true, true, recs)))
	if err != nil {
		t.Fatalf("fixture is not a valid export: %v", err)
	}
	rep, rows := priceUsage(e)

	if rep.Breaks == nil || *rep.Breaks != 2 {
		t.Fatalf("breaks = %v, want 2", rep.Breaks)
	}
	if n := rep.Causes[string(cachemodel.CauseTTLExpired)]; n != 0 {
		t.Errorf("%d gpt-5 break(s) explained by Anthropic's TTL", n)
	}
	if n := rep.Causes[string(cachemodel.CauseModelChanged)]; n != 1 {
		t.Errorf("model changes = %d, want 1", n)
	}
	notMeasured := 0
	for c, n := range rep.Causes {
		if strings.HasPrefix(c, "NOT MEASURED") && strings.Contains(c, "no published cache rules") {
			notMeasured += n
		}
	}
	if notMeasured != 1 {
		t.Errorf("not-measured causes naming the missing rules = %d, want 1; causes %v", notMeasured, rep.Causes)
	}
	out := renderUsageCost(rep, rows, false)
	if strings.Contains(out, string(cachemodel.CauseTTLExpired)) {
		t.Errorf("the report shows a TTL expiry for gpt-5:\n%s", out)
	}
	if !strings.Contains(out, "no published cache rules") {
		t.Errorf("the not-measured cause is not on screen:\n%s", out)
	}
}
