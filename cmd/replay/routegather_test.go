package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// gatherByModel is the only producer of a modelCorpus outside tests, and the
// RT tests never reach it: they build the corpus by hand. Two things it got
// wrong were therefore invisible.
//
// It summed Input, CacheCreation, CacheRead and Output and dropped Create5m
// and Create1h, so buildRoute's "at the TTL this session actually wrote"
// branch saw a zero split on every real transcript and priced the switch
// write at the five-minute multiplier. RT2 covers that branch only because it
// sets Create1h itself.
//
// And it counted every model's requests into one total, while the prefix and
// the payback divide the source model's usage by that total. Adding another
// model's sessions to the corpus shrank the source model's per-turn prefix and
// moved the switch figure, and WithinObserved compared the payback against
// turns the source model never ran.
//
// These tests drive the real parser over the redacted fixture and take their
// expectations from the raw JSON and the published price row, not from the
// code under test.

const routeFixture = "../../internal/transcript/testdata/session-redacted.jsonl"

// rawTally reads the fixture as plain JSON: one entry per requestId, because a
// response split into several content blocks repeats the same usage on each.
type rawTally struct {
	requests                    int
	cacheRead, create, c5m, c1h int
}

func tallyFixture(t *testing.T) rawTally {
	t.Helper()
	f, err := os.Open(routeFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	seen := map[string]bool{}
	var r rawTally
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	for sc.Scan() {
		var line struct {
			Type      string `json:"type"`
			RequestID string `json:"requestId"`
			Message   struct {
				Usage *struct {
					CacheRead     int `json:"cache_read_input_tokens"`
					CacheCreation int `json:"cache_creation_input_tokens"`
					Nested        struct {
						Short int `json:"ephemeral_5m_input_tokens"`
						Long  int `json:"ephemeral_1h_input_tokens"`
					} `json:"cache_creation"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil || line.Type != "assistant" || line.Message.Usage == nil {
			continue
		}
		if seen[line.RequestID] {
			continue
		}
		seen[line.RequestID] = true
		u := line.Message.Usage
		r.requests++
		r.cacheRead += u.CacheRead
		r.create += u.CacheCreation
		r.c5m += u.Nested.Short
		r.c1h += u.Nested.Long
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return r
}

// copyAs writes the fixture under dir as a session of another model, with its
// own session id so the two are not one session.
func copyAs(t *testing.T, dir, model, session string) string {
	t.Helper()
	b, err := os.ReadFile(routeFixture)
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.ReplaceAll(b, []byte(`"model":"claude-fable-5-1"`), []byte(`"model":"`+model+`"`))
	b = bytes.ReplaceAll(b, []byte(`"sessionId":"redacted"`), []byte(`"sessionId":"`+session+`"`))
	if !bytes.Contains(b, []byte(`"model":"`+model+`"`)) {
		t.Fatalf("the fixture no longer names claude-fable-5-1, so the copy is not %s", model)
	}
	p := filepath.Join(dir, session+".jsonl")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const routeFrom, routeTo = "claude-fable-5-1", "claude-sonnet-5"

func TestRG1_GatherKeepsTheTTLSplit(t *testing.T) {
	raw := tallyFixture(t)
	if raw.create == 0 || raw.c5m != 0 || raw.c1h != raw.create {
		t.Fatalf("fixture precondition: every write is 1h (create %d, 5m %d, 1h %d); "+
			"without that this test cannot tell the split was kept", raw.create, raw.c5m, raw.c1h)
	}
	c, err := gatherByModel([]string{routeFixture})
	if err != nil {
		t.Fatal(err)
	}
	u := c.usage[routeFrom]
	if u.CacheCreation != raw.create {
		t.Fatalf("parser total %d, raw total %d: the oracle below would be wrong", u.CacheCreation, raw.create)
	}
	if u.Create1h != raw.c1h || u.Create5m != raw.c5m {
		t.Errorf("gathered split 5m=%d 1h=%d, the transcript holds 5m=%d 1h=%d. "+
			"Dropping it prices every switch write at 1.25x on a session that wrote at 2x.",
			u.Create5m, u.Create1h, raw.c5m, raw.c1h)
	}
}

// The switch cost from a real corpus, priced by hand: the source's per-turn
// warm prefix, written once on the destination at the one-hour multiplier
// because the source wrote one-hour entries only.
func TestRG2_TheSwitchCostFromARealCorpusUsesTheTTLAndTheSourcesTurns(t *testing.T) {
	raw := tallyFixture(t)
	dir := t.TempDir()
	files := []string{routeFixture, copyAs(t, dir, routeTo, "dest")}
	c, err := gatherByModel(files)
	if err != nil {
		t.Fatal(err)
	}
	r := buildRoute(routeFrom, routeTo, c)
	if !r.Dilation.Measured || r.Dilation.Sigma != 1 {
		t.Fatalf("the copy is the same bytes and tokens, so sigma must be measured at exactly 1: "+
			"measured=%v sigma=%v (%s)", r.Dilation.Measured, r.Dilation.Sigma, r.Dilation.Why)
	}
	if r.Switch == nil {
		t.Fatal("no switch figure, so this asserts nothing")
	}
	pt, ok := cachemodel.PriceFor(routeTo)
	if !ok || pt.InputPerMTok != 2 {
		t.Fatalf("expected the published sonnet-5 row ($2/MTok input), got %+v", pt)
	}
	// Anthropic's published cache-write multipliers: 1.25x base input for a
	// five-minute write, 2x for a one-hour write. Literals, so a change to the
	// package constants cannot move the oracle with them.
	const short5m, long1h = 1.25, 2.0
	if cachemodel.WriteMultiplierShort != short5m || cachemodel.WriteMultiplierLong != long1h {
		t.Fatalf("the package multipliers are %v/%v, not the published %v/%v",
			cachemodel.WriteMultiplierShort, cachemodel.WriteMultiplierLong, short5m, long1h)
	}
	prefix := raw.cacheRead / raw.requests
	want := float64(prefix) * pt.InputPerMTok / 1e6 * long1h
	if d := r.Switch.CostUSD - want; d > 1e-9 || d < -1e-9 {
		short := float64(prefix) * pt.InputPerMTok / 1e6 * short5m
		t.Errorf("switch cost $%.6f, want $%.6f (%d-token prefix over %d source turns at 2.00x). "+
			"At 1.25x it would be $%.6f.", r.Switch.CostUSD, want, prefix, raw.requests, short)
	}
	if r.Switch.PaysBack && r.Switch.WithinObserved != (r.Switch.PaybackTurns <= raw.requests) {
		t.Errorf("payback %d turns judged within_observed=%v, but the source ran %d turns",
			r.Switch.PaybackTurns, r.Switch.WithinObserved, raw.requests)
	}
}

// A third model's sessions are not evidence about this switch.
func TestRG3_AnotherModelsSessionsDoNotMoveTheSwitch(t *testing.T) {
	dir := t.TempDir()
	base := []string{routeFixture, copyAs(t, dir, routeTo, "dest")}
	c1, err := gatherByModel(base)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := gatherByModel(append(base, copyAs(t, dir, "claude-haiku-4-5", "other")))
	if err != nil {
		t.Fatal(err)
	}
	r1, r2 := buildRoute(routeFrom, routeTo, c1), buildRoute(routeFrom, routeTo, c2)
	if r1.Switch == nil || r2.Switch == nil {
		t.Fatal("no switch figure, so this asserts nothing")
	}
	if *r1.Switch != *r2.Switch {
		t.Errorf("adding claude-haiku-4-5 sessions moved the %s -> %s switch:\n %+v\n %+v",
			routeFrom, routeTo, *r1.Switch, *r2.Switch)
	}
}

// The sentence names the turn count the payback was judged against. It
// printed the corpus total, every model's requests, beside a verdict reached
// on the source model's own.
func TestRG4_TheSentenceNamesTheTurnsThePaybackWasJudgedOn(t *testing.T) {
	raw := tallyFixture(t)
	dir := t.TempDir()
	copyAs(t, dir, routeTo, "dest")
	b, err := os.ReadFile(routeFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source.jsonl"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	isolateHome(t, t.TempDir())
	var out, errb bytes.Buffer
	if err := runRoute([]string{"--to", routeTo, dir}, &out, &errb); err != nil {
		t.Fatalf("%v\n%s", err, errb.String())
	}
	text := strings.Join(strings.Fields(out.String()), " ")
	for _, w := range []string{
		fmt.Sprintf("the %d turns measured", raw.requests),
	} {
		if !strings.Contains(text, w) {
			t.Errorf("route output lacks %q; the payback was judged on %s's %d turns:\n%s",
				w, routeFrom, raw.requests, out.String())
		}
	}
}
