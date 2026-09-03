package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// synthLane builds a session of n turns whose cache reads follow the
// rules (each turn reads the prefix the last one wrote) or, when broken,
// a changed provider that never serves a read.
func synthLane(model string, start time.Time, turns int, broken bool) *LaneReport {
	lane := &transcript.Lane{ID: "main"}
	prefix := 0
	for i := 0; i < turns; i++ {
		u := transcript.Usage{Input: 200, CacheCreation: 3000, CacheRead: prefix, Output: 50}
		if broken {
			u.CacheRead = 0
		}
		lane.Requests = append(lane.Requests, &transcript.Request{Model: model, Timestamp: start.Add(time.Duration(i) * time.Minute), Usage: u})
		prefix = u.CacheCreation + u.CacheRead
	}
	return &LaneReport{Lane: lane, Calibration: Calibrate(lane)}
}

// A simulated rule change: a model whose sessions calibrated until the
// newest ones stopped is reported stale with the reason; a model whose
// sessions always calibrate is not; one that never calibrated is left to
// the per-session gate.
func TestModelCalibrationsDetectARuleChange(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var reports []*LaneReport
	for i := 0; i < 6; i++ {
		reports = append(reports, synthLane("claude-opus-5", t0.Add(time.Duration(i)*time.Hour), 12, false))
		reports = append(reports, synthLane("claude-sonnet-5", t0.Add(time.Duration(i)*time.Hour), 12, false))
		reports = append(reports, synthLane("claude-haiku-4-5", t0.Add(time.Duration(i)*time.Hour), 12, true))
	}
	// The newest three opus sessions run on a changed provider.
	for i := 6; i < 9; i++ {
		reports = append(reports, synthLane("claude-opus-5", t0.Add(time.Duration(i)*time.Hour), 12, true))
	}
	cals := ModelCalibrations(reports)
	if len(cals) != 3 {
		t.Fatalf("models: %+v", cals)
	}
	byModel := map[string]ModelCalibration{}
	for _, c := range cals {
		byModel[c.Model] = c
	}
	opus := byModel["claude-opus-5"]
	if !opus.Stale || opus.Sessions != 9 || opus.RecentSessions != StalenessRecentSessions || opus.RecentFailing != 3 || opus.RecentMatchRate() >= CalibrationThreshold {
		t.Fatalf("opus must be stale: %+v", opus)
	}
	if !strings.Contains(opus.Reason, "provider behavior changed") || !strings.Contains(opus.Reason, "not scored") {
		t.Fatalf("reason: %s", opus.Reason)
	}
	if sonnet := byModel["claude-sonnet-5"]; sonnet.Stale || sonnet.MatchRate() != 1 || !strings.Contains(sonnet.MinPrefix.String(), "at most 3000 tokens") {
		t.Fatalf("sonnet must be healthy: %+v", sonnet)
	}
	if haiku := byModel["claude-haiku-4-5"]; haiku.Stale || haiku.MatchRate() != 0 {
		t.Fatalf("a model that never calibrated is not stale: %+v", haiku)
	}
	if stale := StaleModels(cals); len(stale) != 1 || !stale["claude-opus-5"] {
		t.Fatalf("stale set: %v", stale)
	}
	// Too little recent evidence is not a verdict.
	few := append([]*LaneReport(nil), reports[:6*3]...)
	few = append(few, synthLane("claude-opus-5", t0.Add(10*time.Hour), 12, true))
	for _, c := range ModelCalibrations(few) {
		if c.Stale {
			t.Fatalf("one broken session is not a rule change: %+v", c)
		}
	}
}

func TestMinPrefixFitBoundsTheRule(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{ID: "main"}
	add := func(input, created, read int) {
		lane.Requests = append(lane.Requests, &transcript.Request{Model: "claude-opus-5", Timestamp: t0.Add(time.Duration(len(lane.Requests)) * time.Minute), Usage: transcript.Usage{Input: input, CacheCreation: created, CacheRead: read}})
	}
	add(400, 0, 0)    // too small to cache
	add(100, 600, 0)  // the first cached prefix
	add(100, 50, 600) // reads it back
	cals := ModelCalibrations([]*LaneReport{{Lane: lane, Calibration: Calibrate(lane)}})
	fit := cals[0].MinPrefix
	if fit.Rule != 512 || fit.LargestUncached != 400 || fit.SmallestCached != 600 || !fit.Conclusive() || fit.Disagrees() {
		t.Fatalf("fit: %+v", fit)
	}
	if fit.String() != "minimum cacheable prefix: between 401 and 600 tokens (rules say 512)" {
		t.Fatal(fit.String())
	}
	// A prompt the rule says should cache but did not: the rule disagrees.
	add(900, 0, 0)
	fit = ModelCalibrations([]*LaneReport{{Lane: lane, Calibration: Calibrate(lane)}})[0].MinPrefix
	if !fit.Disagrees() || fit.Conclusive() || !strings.Contains(fit.String(), "inconclusive") {
		t.Fatalf("fit: %+v %s", fit, fit)
	}
	empty := MinPrefixFit{Rule: 1024}
	if empty.Conclusive() || empty.Disagrees() || !strings.Contains(empty.String(), "no evidence") {
		t.Fatal(empty.String())
	}
}
