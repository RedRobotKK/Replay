package ledger_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/advisor"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The ledger is where unused-tools gets its input, and nothing tested the join.
//
// Every test of KindUnusedTools hand-builds a transcript.Request with Tools
// already populated. That covers the advisor and skips the only path that
// fills the field on a real corpus: Store.Append writes Prompt.Tools to disk,
// ReadFile builds a session from it, and store.go copies it onto the request.
//
// A regression anywhere in that chain would leave the whole advisor suite
// green while the feature stopped firing for every user, because the suite
// starts after the break. That is ADR-0018's join, from the other side: the
// tests sat too close to the thing they checked.
//
// Written after a probe confirmed the chain works end to end. It does; this is
// what notices when it stops.

// AP1: a tool written to the ledger and read back produces the suggestion.
func TestAP1_ToolsSurviveTheLedgerAndReachTheAdvisor(t *testing.T) {
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatalf("opening the ledger: %v", err)
	}

	// A session that is offered three tools and calls one of them. The two it
	// never calls are the finding.
	tools := []transcript.ToolDef{
		{Name: "Bash", Bytes: 1200},
		{Name: "mcp__playwright__click", Bytes: 4000},
		{Name: "mcp__playwright__screenshot", Bytes: 4200},
	}
	base := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		rec := ledger.Record{
			Schema:    ledger.SchemaVersion,
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			SessionID: "s1",
			RequestSummary: ledger.RequestSummary{
				Model: "claude-opus-4",
				Prompt: ledger.Prompt{
					SystemBytes: 2000,
					ToolBytes:   9400,
					ToolCount:   len(tools),
					Tools:       tools,
					Messages: []ledger.Message{{
						Role:   "user",
						Blocks: []ledger.Block{{Kind: transcript.KindText, Bytes: 400}},
					}},
				},
			},
			Response: ledger.Response{
				Blocks: []ledger.Block{{
					Kind: transcript.KindToolUse, ToolName: "Bash", Bytes: 60,
				}},
				Usage: &transcript.Usage{Input: 500, Output: 60},
			},
		}
		if err := store.Append(rec); err != nil {
			t.Fatalf("appending record %d: %v", i, err)
		}
	}

	// Back off disk, the way `replay advise` reads it.
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no ledger file was written under %s (%v)", dir, err)
	}
	session, err := ledger.ReadFile(files[0])
	if err != nil {
		t.Fatalf("reading the ledger back: %v", err)
	}

	// The join itself: the field the advisor needs must have survived.
	lane := session.Lane("", false)
	if len(lane.Requests) == 0 {
		t.Fatal("the session read back holds no requests")
	}
	if got := len(lane.Requests[0].Tools); got != len(tools) {
		t.Fatalf("the ledger round trip lost the tool definitions: %d of %d survived. "+
			"unused-tools reads Request.Tools and nothing else fills it on a real "+
			"corpus, so the advisor would go quiet on every session while its own "+
			"tests stayed green", got, len(tools))
	}

	obs, ok := advisor.Observe(session)
	if !ok {
		t.Fatal("the session read back from the ledger produced no observation")
	}
	var named []string
	for _, s := range advisor.Suggest([]advisor.Observation{obs}, nil) {
		if s.Kind == advisor.KindUnusedTools {
			named = append(named, s.Target)
		}
	}
	if len(named) == 0 {
		t.Error("three tools were offered, one was called, and the advisor named no " +
			"unused-tools target. The chain from Store.Append through ReadFile to " +
			"Request.Tools is the only thing that fills this on a real corpus")
	}
}
