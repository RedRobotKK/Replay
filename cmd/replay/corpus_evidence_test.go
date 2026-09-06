package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The corpus report counts transcript files and calls them sessions.
//
// A Claude Code session writes one transcript per lane: the main one, plus one
// per subagent it spawns. They all carry the SAME sessionId. So `len(rows)` is
// a file count, and on the real corpus the difference is not marginal — 1450
// files carry 78 distinct session ids, and a single session supplies 1020 of
// the 1363 rows in the published evidence document.
//
// It matters because independence is the whole point of the number. "1363
// sessions" is the sentence every downstream claim rests on, and it overstates
// the independent sample by roughly twenty times. Turns within one session
// share a client version, an account, a machine, a project and an operator's
// habits, so they are nothing like 1363 draws.

// lanesOfOneSession writes n transcripts that share a session id, which is what
// a session with n-1 subagents looks like on disk.
func lanesOfOneSession(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("lane%02d.jsonl", i)
	}
	for i, name := range names {
		// Timestamps must stay valid past i=9: a naive "10:0%d:00" produces
		// "10:010:00" and the transcript silently fails to parse, which showed
		// up as a fixture producing half the files it claimed.
		ts := func(sec int) string {
			return fmt.Sprintf("2026-09-06T%02d:%02d:%02dZ", 10+i/60, i%60, sec)
		}
		body := fmt.Sprintf(
			`{"type":"user","uuid":"u%d","sessionId":"shared-session","timestamp":"%s","message":{"role":"user","content":"hello"}}`+"\n"+
				`{"type":"assistant","uuid":"a%d","parentUuid":"u%d","sessionId":"shared-session","requestId":"req-%d-0","timestamp":"%s","apiBlockIndex":0,`+
				`"message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],`+
				`"usage":{"input_tokens":10,"cache_creation_input_tokens":100,"cache_read_input_tokens":0,"output_tokens":5}}}`+"\n"+
				`{"type":"assistant","uuid":"b%d","parentUuid":"a%d","sessionId":"shared-session","requestId":"req-%d-1","timestamp":"%s","apiBlockIndex":0,`+
				`"message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],`+
				`"usage":{"input_tokens":2,"cache_creation_input_tokens":20,"cache_read_input_tokens":110,"output_tokens":5}}}`+"\n",
			i, ts(0), i, i, i, ts(1), i, i, i, ts(2))
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// CR1: the report distinguishes transcripts from sessions.
//
// Both numbers are worth having — transcripts say how much material was read,
// sessions say how many independent draws it represents — but they must not
// share a name.
//
// PASS: the totals name both, and the session count is the smaller one.
// FAIL: one number labelled "Sessions" that is really a file count.
func TestCR1_CorpusSeparatesTranscriptsFromSessions(t *testing.T) {
	dir := lanesOfOneSession(t, 2)
	var out, errb bytes.Buffer
	if err := run([]string{"corpus", dir}, &out, &errb); err != nil {
		t.Fatalf("corpus: %v (stderr %s)", err, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "Transcripts: 2") {
		t.Errorf("the totals do not report how many transcripts were read:\n%s", totalsOf(got))
	}
	if !strings.Contains(got, "Sessions: 1") {
		t.Errorf("two transcripts of ONE session are not reported as one session. Independence is "+
			"the whole point of this number, and turns inside one session share a client, an "+
			"account, a machine and an operator:\n%s", totalsOf(got))
	}
}

// CR2: the roadmap gate counts independent sessions.
//
// The gate exists to say whether there is enough evidence. Counting files
// means a single session with many subagents can satisfy it alone, which is
// the opposite of what it is for.
//
// PASS: two transcripts of one session do not satisfy a gate of 20.
// FAIL: the gate satisfied by file count.
func TestCR2_TheRoadmapGateCountsSessionsNotFiles(t *testing.T) {
	// Exactly enough transcripts to satisfy the gate if it counts files, all
	// from ONE session. A smaller fixture cannot fail: with two files, "2 < 20"
	// trips the gate whichever quantity is counted, so the test would pass
	// against a version counting files and assert nothing. The frozen-mutant
	// harness caught precisely that.
	dir := lanesOfOneSession(t, corpusTarget)
	var out, errb bytes.Buffer
	if err := run([]string{"corpus", dir}, &out, &errb); err != nil {
		t.Fatalf("corpus: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, fmt.Sprintf("Transcripts: %d", corpusTarget)) {
		t.Fatalf("the fixture did not produce %d transcripts, so the gate is not under test:\n%s",
			corpusTarget, totalsOf(got))
	}
	if !strings.Contains(got, "Fewer than") {
		t.Errorf("%d transcripts of ONE session satisfied the %d-session roadmap gate. The gate "+
			"exists to say whether there is enough independent evidence, and counting files lets "+
			"a single session clear it alone:\n%s", corpusTarget, corpusTarget, totalsOf(got))
	}
}

// CR3: a row with nothing compared is not a perfect row.
//
// Same defect as analysis.MatchRate carried into the report: a transcript with
// one request offers no turn to check, and scoring it 100% both inflates the
// per-row column and counts it as above the calibration threshold in the
// "below threshold" tally.
//
// PASS: the empty rate is below the threshold.
// FAIL: 1, which reports an absent measurement as the best possible one.
func TestCR3_ARowWithNothingComparedIsNotPerfect(t *testing.T) {
	r := corpusRow{compared: 0, matched: 0}
	got := r.matchRate()
	if got == 1 {
		t.Error("matchRate() = 1 with nothing compared: an absent measurement reported as a perfect one")
	}
	if got >= 0.95 {
		t.Errorf("matchRate() = %.3f with nothing compared, at or above the calibration threshold, "+
			"so it is counted as a calibrated session", got)
	}
}

// totalsOf returns the Totals section, for readable failures.
func totalsOf(s string) string {
	i := strings.Index(s, "## Totals")
	if i < 0 {
		return s
	}
	return s[i:]
}
