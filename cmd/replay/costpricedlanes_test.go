package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// summary.lanes is "the transcript files actually read and priced", and on a
// lane-unit report it equals tasks. It was len(units), every lane read, so a
// session on a model no table prices added a lane to the priced count: one
// priced single-lane session beside one unpriced one reported lanes 2 and a
// headline of "1 sessions (2 agent lanes)", a fan-out that did not happen
// (2026-10-14). A lane of a priced session still counts on the session
// report when its own model is unpriced: TestCPL4, and TestQ01's mixed lane.

// oneRecordLane writes one transcript file holding a single request.
func oneRecordLane(t *testing.T, path, session, request, model string) {
	t.Helper()
	line := map[string]any{
		"type": "assistant", "requestId": request, "sessionId": session,
		"uuid": request + "-u", "parentUuid": nil,
		"timestamp": "2026-09-01T00:00:00.000Z", "version": "2.1.233",
		"message": map[string]any{
			"role": "assistant", "type": "message", "model": model,
			"content": []any{map[string]any{"type": "text", "text": "x"}},
			"usage": map[string]any{
				"input_tokens": 1000, "output_tokens": 100,
				"cache_read_input_tokens": 4000, "cache_creation_input_tokens": 500,
			},
		},
	}
	b, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

type lanesReport struct {
	unit         string
	tasks, lanes int
}

func runLanes(t *testing.T, proj string) (session, lane lanesReport, headline string) {
	t.Helper()
	read := func(args ...string) lanesReport {
		var out, errOut bytes.Buffer
		if err := runCost(append(args, proj), &out, &errOut); err != nil {
			t.Fatalf("cost %v: %v (%s)", args, err, errOut.String())
		}
		var doc struct {
			Summary struct {
				Unit  string `json:"unit"`
				Tasks int    `json:"tasks"`
				Lanes int    `json:"lanes"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("cost %v: not JSON: %v", args, err)
		}
		return lanesReport{doc.Summary.Unit, doc.Summary.Tasks, doc.Summary.Lanes}
	}
	session, lane = read("--json"), read("--per-lane", "--json")
	var out bytes.Buffer
	if err := runCost([]string{proj}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.Contains(l, "Cost per ") {
			headline = l
		}
	}
	if headline == "" {
		t.Fatalf("no headline in:\n%s", out.String())
	}
	return session, lane, headline
}

func lanesProject(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REPLAY_TRANSCRIPTS", "")
	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	return proj
}

func TestCPL1_AnUnpricedSessionIsNotAPricedLane(t *testing.T) {
	proj := lanesProject(t)
	oneRecordLane(t, filepath.Join(proj, "aaaa1111-priced.jsonl"), "aaaa1111-priced", "a-req", "claude-opus-5")
	oneRecordLane(t, filepath.Join(proj, "bbbb2222-unpriced.jsonl"), "bbbb2222-unpriced", "b-req", "no-such-model-anywhere")
	session, lane, headline := runLanes(t, proj)
	if session != (lanesReport{"session", 1, 1}) {
		t.Errorf("session report %+v, want 1 priced session over 1 priced lane", session)
	}
	if lane != (lanesReport{"lane", 1, 1}) {
		t.Errorf("lane report %+v, want tasks and lanes equal at 1", lane)
	}
	if strings.Contains(headline, "agent lanes") {
		t.Errorf("headline claims a fan-out that did not happen: %s", headline)
	}
}

// The control: an unpriced session must not hide a real fan-out either.
func TestCPL2_APricedFanOutStillShowsItsLanes(t *testing.T) {
	proj := lanesProject(t)
	oneRecordLane(t, filepath.Join(proj, "cccc3333-lane-a.jsonl"), "cccc3333-fanout", "c1-req", "claude-opus-5")
	oneRecordLane(t, filepath.Join(proj, "cccc3333-lane-b.jsonl"), "cccc3333-fanout", "c2-req", "claude-opus-5")
	oneRecordLane(t, filepath.Join(proj, "dddd4444-unpriced.jsonl"), "dddd4444-unpriced", "d-req", "no-such-model-anywhere")
	session, lane, headline := runLanes(t, proj)
	if session != (lanesReport{"session", 1, 2}) {
		t.Errorf("session report %+v, want 1 priced session over its 2 lanes", session)
	}
	if lane != (lanesReport{"lane", 2, 2}) {
		t.Errorf("lane report %+v, want 2 priced lanes", lane)
	}
	if !strings.Contains(headline, "1 sessions (2 agent lanes)") {
		t.Errorf("headline lost the real fan-out: %s", headline)
	}
}

// Only unpriced sessions: nothing is priced, so no lane is.
func TestCPL3_OnlyUnpricedSessionsHaveNoPricedLanes(t *testing.T) {
	proj := lanesProject(t)
	oneRecordLane(t, filepath.Join(proj, "eeee5555-unpriced.jsonl"), "eeee5555-unpriced", "e-req", "no-such-model-anywhere")
	oneRecordLane(t, filepath.Join(proj, "ffff6666-unpriced.jsonl"), "ffff6666-unpriced", "f-req", "no-such-model-anywhere")
	read := func(args ...string) lanesReport {
		var out bytes.Buffer
		if err := runCost(append(args, proj), &out, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Summary struct {
				Unit  string `json:"unit"`
				Tasks int    `json:"tasks"`
				Lanes int    `json:"lanes"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("not JSON: %v", err)
		}
		return lanesReport{doc.Summary.Unit, doc.Summary.Tasks, doc.Summary.Lanes}
	}
	if got := read("--json"); got != (lanesReport{"session", 0, 0}) {
		t.Errorf("session report %+v, want no priced sessions and no priced lanes", got)
	}
	if got := read("--per-lane", "--json"); got != (lanesReport{"lane", 0, 0}) {
		t.Errorf("lane report %+v, want 0 and 0", got)
	}
}

// A priced session with one lane on an unpriced model: the session is priced,
// so on the session report both of its lanes are behind that row; on the lane
// report only the priced lane is a task, and lanes equals tasks.
func TestCPL4_APricedSessionKeepsItsUnpricedLane(t *testing.T) {
	proj := lanesProject(t)
	oneRecordLane(t, filepath.Join(proj, "gggg7777-lane-a.jsonl"), "gggg7777-mixed", "g1-req", "claude-opus-5")
	oneRecordLane(t, filepath.Join(proj, "gggg7777-lane-b.jsonl"), "gggg7777-mixed", "g2-req", "no-such-model-anywhere")
	session, lane, headline := runLanes(t, proj)
	if session != (lanesReport{"session", 1, 2}) {
		t.Errorf("session report %+v, want 1 priced session over both of its lanes", session)
	}
	if lane != (lanesReport{"lane", 1, 1}) {
		t.Errorf("lane report %+v, want the one priced lane", lane)
	}
	if !strings.Contains(headline, "1 sessions (2 agent lanes)") {
		t.Errorf("headline: %s", headline)
	}
}
