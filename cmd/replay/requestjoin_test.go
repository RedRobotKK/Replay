package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The cost report's overlap figure is a JOIN, and a join is only as good as
// its key.
//
// `replay cost` counts a request as "seen in more than one transcript" when its
// id has been seen before. That is a measurement while the id is the
// provider's. It is not one for a ledger record whose provider sent no id: the
// reader names those for their position in the file, so every ledger file has
// a `ledger-0`, and two unrelated sessions' first requests match.

// RJ1: a synthesised id is never a match.
//
// PASS: two files' `ledger-0` are two requests, and both are disclosed as
// unjoinable.
// FAIL: they collide, and the report says half the corpus is duplicated - a
// wrong figure derived from a right one, arriving with no hedge.
func TestRJ1_SynthesisedIDsAreNotJoinedAcrossFiles(t *testing.T) {
	j := newRequestJoin()
	j.add("ledger-0", false)
	j.add("ledger-0", false)
	if j.duplicated != 0 {
		t.Errorf("duplicated = %d; two files each have a first record and they are not the same request", j.duplicated)
	}
	if j.unjoinable != 2 {
		t.Errorf("unjoinable = %d, want 2", j.unjoinable)
	}
	if j.total != 2 {
		t.Errorf("total = %d, want 2", j.total)
	}
}

// RJ2: a provider id still joins, so RJ1 is not the join being switched off.
func TestRJ2_ProviderIDsStillJoin(t *testing.T) {
	j := newRequestJoin()
	j.add("req_011CenqQ", true)
	j.add("req_011CenqQ", true)
	j.add("req_other", true)
	if j.duplicated != 1 {
		t.Errorf("duplicated = %d, want 1: the same provider id in two files is one request counted twice", j.duplicated)
	}
	if j.unjoinable != 0 {
		t.Errorf("unjoinable = %d, want 0", j.unjoinable)
	}
}

// RJ3: the warm path counts what the cold path counted.
//
// The index stores a file's provider ids and its unjoinable count separately,
// because the unjoinable ones are deliberately absent from the id list. A warm
// run that forgot the count would report a fully joinable corpus, which is the
// disclosure dropped by the cache rather than by the code.
func TestRJ3_CachedFilesCarryTheirUnjoinableCount(t *testing.T) {
	cold := newRequestJoin()
	cold.add("req_a", true)
	cold.add("ledger-1", false)

	warm := newRequestJoin()
	warm.addCached([]string{"req_a"}, 1)

	if warm.total != cold.total || warm.unjoinable != cold.unjoinable || warm.duplicated != cold.duplicated {
		t.Fatalf("warm run %+v does not match cold run %+v", warm, cold)
	}
}

// RJ4: the count is stated, not left to the reader.
//
// PASS: a line naming how many requests the overlap figure could not consider.
// FAIL: silence, which lets an overlap rate of "0 of 2" read as a clean corpus.
func TestRJ4_UnjoinableRequestsAreDisclosed(t *testing.T) {
	got := unjoinableNote(2, 8)
	if got == "" {
		t.Fatal("2 of 8 requests with no provider id must be disclosed")
	}
	for _, want := range []string{"2", "8", "25.0"} {
		if !contains(got, want) {
			t.Errorf("the note omits %q: %q", want, got)
		}
	}
	if unjoinableNote(0, 8) != "" {
		t.Error("a corpus whose every request carries a provider id must print nothing")
	}
	if unjoinableNote(2, 0) != "" {
		t.Error("no requests means no percentage to state")
	}
}

// RJ5 crosses the join, end to end.
//
// RJ1-RJ4 exercise the counter directly, which is the shape ADR-0018 warns
// about: the rule can hold in the type and be missing at the one call site
// that matters. This runs the real command over two real ledger files whose
// provider sent no request id, and reads the figures back off its own JSON.
//
// PASS: four requests, none joined, all disclosed.
// FAIL: `ledger-0` and `ledger-1` match across the two files and the report
// says half the corpus is a duplicate.
func TestRJ5_CostReportsLedgerRequestsAsUnjoinable(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)

	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	for _, session := range []string{"sess-one", "sess-two"} {
		var lines []byte
		for i := 0; i < 2; i++ {
			u := transcript.Usage{Input: 1_000, CacheRead: 4_000, CacheCreation: 500, Output: 200}
			rec := ledger.Record{
				Schema:    ledger.SchemaVersion,
				Timestamp: at.Add(time.Duration(i) * time.Minute),
				SessionID: session,
				Path:      "/v1/messages",
				Status:    200,
				LatencyMS: 900,
				// No RequestID: this provider sent none, which is the whole
				// case. The reader will name the record for its position.
				RequestSummary: ledger.RequestSummary{
					Model:  "claude-opus-5",
					Prompt: ledger.Prompt{SystemBytes: 40, Messages: []ledger.Message{{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 120}}}}},
				},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 80}}},
			}
			b, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		if err := os.WriteFile(filepath.Join(dir, session+".jsonl"), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	type figures struct {
		Total      int `json:"totalRequests"`
		Duplicated int `json:"duplicatedRequests"`
		Unjoinable int `json:"unjoinableRequests"`
	}
	run := func(what string) figures {
		t.Helper()
		var out, errOut bytes.Buffer
		if err := runCost([]string{"--json", dir}, &out, &errOut); err != nil {
			t.Fatalf("%s cost --json: %v (stderr: %s)", what, err, errOut.String())
		}
		var got figures
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("%s cost JSON does not parse: %v\n%s", what, err, out.String())
		}
		return got
	}

	cold := run("cold")
	if cold.Total != 4 {
		t.Fatalf("totalRequests = %d, want 4; this test cannot observe what it was written for", cold.Total)
	}
	if cold.Duplicated != 0 {
		t.Errorf("duplicatedRequests = %d, want 0: two ledger files each have a first record and they are different requests", cold.Duplicated)
	}
	if cold.Unjoinable != 4 {
		t.Errorf("unjoinableRequests = %d, want 4: no provider sent an id for any of them", cold.Unjoinable)
	}

	// The second run reads the index rather than the files. A disclosure that
	// survives only while the cache is cold is a disclosure that disappears
	// the moment anyone uses the tool twice.
	if warm := run("warm"); warm != cold {
		t.Errorf("warm run reported %+v and the cold run %+v; the index dropped what the files said", warm, cold)
	}
}
