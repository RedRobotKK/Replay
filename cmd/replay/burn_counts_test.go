package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A session is not a transcript is not a request.
//
// The burn table's second column is headed "requests". Two of its three rows
// held the number of FILES read: burnCodex and burnClaudeCode incremented once
// per transcript, and only burnOllama — whose log has no per-session file to
// confuse a request with — counted requests. So one column carried two
// different quantities, printed adjacently, with nothing on the screen to tell
// them apart. A corpus of 148 Codex rollouts covering thousands of turns
// reported "148".
//
// This project has retracted a published figure over this conflation before,
// which is why the fix is a count and the check is a comparison against the
// parser, not against the renderer.

// BN-1: the codex row reports provider calls, not files.
//
// The fixture is ONE rollout file holding several token_count turns. The two
// numbers must differ, or the test cannot tell a fixed reader from a broken
// one — which is the fixture failure mode ADR-0014 says to look at first.
//
// PASS: the requests column equals the turns the parser accepted across the
// fixture, and that is more than the file count.
// FAIL: the file count, which is what shipped. Restore it by writing
// `s.requests++` in burnCodex instead of `s.requests += r.Turns`.
func TestBN1_TheCodexRowCountsRequestsNotFiles(t *testing.T) {
	files, err := filepath.Glob("burndata/codex/*.jsonl")
	if err != nil || len(files) == 0 {
		t.Fatalf("no codex fixture: %v", err)
	}
	wantTurns := 0
	for _, f := range files {
		s, err := transcript.ParseCodexFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		wantTurns += s.Turns
	}
	if wantTurns <= len(files) {
		t.Fatalf("the fixture cannot distinguish the defect: %d turn(s) across %d "+
			"file(s). A fixture where files and requests agree makes both readings "+
			"pass, which is the shape ADR-0014 says to check the fixture for first.",
			wantTurns, len(files))
	}

	s := burnCodex("", "burndata")
	if s.requests != wantTurns {
		t.Errorf("burn reports %d request(s); the parser accepted %d turn(s) across "+
			"%d file(s). %d is the file count wearing the requests column's name.",
			s.requests, wantTurns, len(files), len(files))
	}
	if s.sessions != len(files) {
		t.Errorf("sessions = %d, want %d: the file count is still worth reporting, "+
			"under its own name", s.sessions, len(files))
	}
}

// BN-2: the printed table says both, and says which is which.
//
// A corrected internal count that never reaches the screen is the
// built-but-unwired shape, and ADR-0018 records it being introduced by the
// commit that fixed an instance of it. This reads the rendered output.
//
// PASS: the report names a request count and a session-transcript count, and
// the request count is the larger.
// FAIL: only one number, or the file count printed as requests.
func TestBN2_TheReportDistinguishesSessionsFromRequests(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	re := regexp.MustCompile(`([\d,]+) request\(s\) across ([\d,]+) session transcript\(s\)`)
	m := re.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("the report does not distinguish requests from session transcripts:\n%s", got)
	}
	reqs, err1 := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	sess, err2 := strconv.Atoi(strings.ReplaceAll(m[2], ",", ""))
	if err1 != nil || err2 != nil {
		t.Fatalf("unreadable counts %q / %q", m[1], m[2])
	}
	if sess != 1 {
		t.Errorf("session transcripts = %d, want 1: burndata/codex holds one rollout", sess)
	}
	if reqs <= sess {
		t.Errorf("requests = %d against %d session transcript(s). One rollout of "+
			"several turns must not report one request.", reqs, sess)
	}
}

// BN-3: the Claude Code row counts requests too, and a session's lanes are all
// of it.
//
// A Claude Code session writes sidechain lanes into the same transcript, so
// requests live across several lanes and one file is neither one request nor
// one lane. Session.RequestCount() is the sum over lanes and existed unused;
// burnClaudeCode incremented per file beside it.
//
// PASS: the surface's request count equals the parser's RequestCount across
// the transcripts, and the fixture has more than one lane so the sum is a real
// sum.
// FAIL: the file count, or a count that reads only the first lane.
func TestBN3_TheClaudeCodeRowCountsRequestsAcrossEveryLane(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	// One transcript: a main lane with two requests and a sidechain with one.
	tr := `{"uuid":"a1","type":"user","sessionId":"s","version":"2.0","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"hi"}}
{"uuid":"a2","parentUuid":"a1","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"one"}],"usage":{"input_tokens":10,"output_tokens":2}}}
{"uuid":"a3","parentUuid":"a2","type":"user","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":"more"}}
{"uuid":"a4","parentUuid":"a3","type":"assistant","requestId":"r2","apiBlockIndex":0,"timestamp":"2026-01-01T00:00:03Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"two"}],"usage":{"input_tokens":20,"output_tokens":2}}}
{"uuid":"b1","type":"user","isSidechain":true,"timestamp":"2026-01-01T00:00:04Z","message":{"role":"user","content":"sub"}}
{"uuid":"b2","parentUuid":"b1","type":"assistant","isSidechain":true,"requestId":"r3","apiBlockIndex":0,"timestamp":"2026-01-01T00:00:05Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"three"}],"usage":{"input_tokens":30,"output_tokens":2}}}
`
	path := filepath.Join(proj, "session.jsonl")
	if err := os.WriteFile(path, []byte(tr), 0o600); err != nil {
		t.Fatal(err)
	}
	sess, err := transcript.ParseClaudeCodeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Lanes) < 2 {
		t.Fatalf("the fixture has %d lane(s); with one lane this test cannot tell a "+
			"per-lane count from a whole-session one", len(sess.Lanes))
	}
	if sess.RequestCount() != 3 {
		t.Fatalf("the fixture is wrong: RequestCount = %d, want 3", sess.RequestCount())
	}

	s := burnClaudeCode(home, "")
	if s.sessions != 1 {
		t.Fatalf("read %d transcript(s), want 1 — the fixture is not being found", s.sessions)
	}
	if s.requests != 3 {
		t.Errorf("burn reports %d request(s) for a transcript holding 3 across 2 "+
			"lanes. 1 is the file count; 2 is the main lane alone.", s.requests)
	}
}
