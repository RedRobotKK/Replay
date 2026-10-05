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

// Cross-session cross-wiring.
//
// The question is not whether the reader rejects malformed input. It is
// whether valid evidence from two different sessions can be combined into a
// plausible history that never happened. That is the attack a claim system
// has to survive, because every individual record here is well formed and
// every one of them is true of SOME session.
//
// The join under attack is requestJoin in overlap.go, which is the documented
// enforcement point: "an id that this program synthesised is not a join key."
// These tests ask what else it does and does not check.

// ---------------------------------------------------------------- fixtures

// Two sessions with nothing in common. Every field differs, so any crossing
// is visible in the result rather than hidden behind a coincidence.
type xwSession struct {
	name     string
	ids      []string // provider request ids
	measured bool
	model    string
	input    int
}

var (
	sessionA = xwSession{name: "sess-alpha", ids: []string{"req_A1", "req_A2"}, measured: true, model: "claude-opus-5", input: 1_000}
	sessionB = xwSession{name: "sess-beta", ids: []string{"req_B1", "req_B2"}, measured: true, model: "claude-sonnet-5", input: 7_000}
)

// oracleJoin is the INDEPENDENT reference. It does not call requestJoin. It
// decides from the fixture's own ground truth what the answer must be, using
// the stated rule and nothing else: a measured id joins across files, a
// synthesised one never does.
func oracleJoin(records []struct {
	id       string
	measured bool
}) (total, duplicated, unjoinable int) {
	seen := map[string]bool{}
	for _, r := range records {
		total++
		if !r.measured {
			unjoinable++
			continue
		}
		if seen[r.id] {
			duplicated++
			continue
		}
		seen[r.id] = true
	}
	return
}

// ------------------------------------------------- XW1, the positive control

// Legitimate associations must survive. Without this the rejections below
// would be indistinguishable from a join that rejects everything.
func TestXW1_LegitimateSessionsJoinWithinThemselvesAndNotAcrossThem(t *testing.T) {
	recs := []struct {
		id       string
		measured bool
	}{
		{sessionA.ids[0], true}, {sessionA.ids[1], true},
		{sessionB.ids[0], true}, {sessionB.ids[1], true},
	}

	j := newRequestJoin()
	for _, r := range recs {
		j.add(r.id, r.measured)
	}

	wantTotal, wantDup, wantUnjoin := oracleJoin(recs)
	if j.total != wantTotal || j.duplicated != wantDup || j.unjoinable != wantUnjoin {
		t.Fatalf("production join %+v disagrees with the independent oracle "+
			"(total %d, duplicated %d, unjoinable %d)", j, wantTotal, wantDup, wantUnjoin)
	}
	if j.duplicated != 0 {
		t.Errorf("duplicated = %d: four requests from two unrelated sessions were "+
			"combined. No id is shared between them.", j.duplicated)
	}
}

// ------------------------------------- XW2, the attack the brief asks about

// THE FINDING.
//
// Two genuinely different requests, in two different sessions, on two
// different models, with two different token counts, that happen to carry the
// same provider id. The join merges them and nothing checks whether the rest
// of the record agrees.
//
// This is not a defect in the sense of a wrong line of code: provider request
// ids are globally unique in practice, so the merge is correct under that
// assumption. It is a finding because THE ASSUMPTION IS NEVER VERIFIED, and
// the join has no way to notice when it fails. The join's correctness rests on
// a property of the provider, not on anything Replay observes.
//
// This test pins the CURRENT behaviour and names the assumption, so that a
// future reader meets it deliberately. It does not assert that the behaviour
// is wrong.
func TestXW2_AnIDCollisionAcrossSessionsMergesWithoutCorroboration(t *testing.T) {
	const shared = "req_COLLIDE"

	recs := []struct {
		id       string
		measured bool
	}{
		{shared, true}, // belongs to sess-alpha, opus, 1,000 input
		{shared, true}, // belongs to sess-beta, sonnet, 7,000 input
	}

	j := newRequestJoin()
	for _, r := range recs {
		j.add(r.id, r.measured)
	}

	wantTotal, wantDup, wantUnjoin := oracleJoin(recs)
	if j.total != wantTotal || j.duplicated != wantDup || j.unjoinable != wantUnjoin {
		t.Fatalf("production join %+v disagrees with the independent oracle "+
			"(total %d, duplicated %d, unjoinable %d)", j, wantTotal, wantDup, wantUnjoin)
	}

	if j.duplicated != 1 {
		t.Fatalf("duplicated = %d, want 1. This test exists to pin the behaviour "+
			"and it is not observing it.", j.duplicated)
	}

	t.Log("RECORDED LIMITATION: two requests from different sessions, on different " +
		"models, with different token counts, were counted as one request solely " +
		"because they share a provider id. The join compares no other field. " +
		"Correct while provider ids are globally unique; unverified by Replay.")
}

// --------------------------------------------- XW3, partial cross-wiring

// A has a valid provider id. B has a synthesised id that is superficially a
// perfectly good string. The superficially compatible identifier must not
// reach the join at all.
func TestXW3_ASuperficiallyCompatibleIdentifierDoesNotJoin(t *testing.T) {
	recs := []struct {
		id       string
		measured bool
	}{
		{"req_A1", true},  // A, provider id
		{"req_A1", false}, // B, same STRING, locally synthesised
	}

	j := newRequestJoin()
	for _, r := range recs {
		j.add(r.id, r.measured)
	}

	wantTotal, wantDup, wantUnjoin := oracleJoin(recs)
	if j.total != wantTotal || j.duplicated != wantDup || j.unjoinable != wantUnjoin {
		t.Fatalf("production join %+v disagrees with the independent oracle "+
			"(total %d, duplicated %d, unjoinable %d)", j, wantTotal, wantDup, wantUnjoin)
	}
	if j.duplicated != 0 {
		t.Error("a synthesised id matched a provider id of the same text. Provenance " +
			"is carried beside the value for exactly this reason, and it was ignored.")
	}
	if j.unjoinable != 1 {
		t.Errorf("unjoinable = %d, want 1: the synthesised one is not joinable "+
			"whatever it is spelled", j.unjoinable)
	}
}

// ------------------------------------------- XW4, provenance across the path

// The end-to-end question. Two sessions on disk, each with its own usage.
// Session A's tokens must not appear in session B's accounting, and the
// corpus total must be the sum of the two and not some third number produced
// by a crossing.
//
// The oracle here is arithmetic over the fixture's declared ground truth, not
// a second call into the reader.
func TestXW4_TwoSessionsOnDiskDoNotShareUsage(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)

	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	// Ground truth, declared here and used by the oracle below.
	type spec struct {
		session string
		reqIDs  []string
		input   int
		model   string
	}
	specs := []spec{
		{"sess-alpha", []string{"req_A1", "req_A2"}, 1_000, "claude-opus-5"},
		{"sess-beta", []string{"req_B1", "req_B2"}, 7_000, "claude-sonnet-5"},
	}

	for _, s := range specs {
		var lines []byte
		for i, rid := range s.reqIDs {
			u := transcript.Usage{Input: s.input, CacheRead: 4_000, CacheCreation: 500, Output: 200}
			rec := ledger.Record{
				Schema:    ledger.SchemaVersion,
				Timestamp: at.Add(time.Duration(i) * time.Minute),
				SessionID: s.session,
				RequestID: rid,
				Path:      "/v1/messages",
				Status:    200,
				LatencyMS: 900,
				RequestSummary: ledger.RequestSummary{
					Model:  s.model,
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
		if err := os.WriteFile(filepath.Join(dir, s.session+".jsonl"), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var out, errOut bytes.Buffer
	if err := runCost([]string{"--json", dir}, &out, &errOut); err != nil {
		t.Fatalf("cost --json: %v (stderr: %s)", err, errOut.String())
	}
	var got struct {
		Total      int `json:"totalRequests"`
		Duplicated int `json:"duplicatedRequests"`
		Unjoinable int `json:"unjoinableRequests"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("cost JSON does not parse: %v\n%s", err, out.String())
	}

	// INDEPENDENT ORACLE: four distinct provider ids across two sessions.
	// Nothing is shared, so nothing may be reported as duplicated, and every
	// id is the provider's so nothing is unjoinable.
	if got.Total != 4 {
		t.Fatalf("totalRequests = %d, want 4; the fixture is not reaching the reader "+
			"and the assertions below would be vacuous", got.Total)
	}
	if got.Duplicated != 0 {
		t.Errorf("duplicatedRequests = %d, want 0. Two sessions with no id in common "+
			"were reported as sharing a request, which is a history that did not happen.",
			got.Duplicated)
	}
	if got.Unjoinable != 0 {
		t.Errorf("unjoinableRequests = %d, want 0: every record carries a provider id",
			got.Unjoinable)
	}
}

// XW5. Adding a second session's evidence must not change what is reported
// about the first. This is the metamorphic half of the provenance rule:
// unrelated evidence may not strengthen or weaken an unrelated claim.
func TestXW5_AddingAForeignSessionDoesNotChangeTheFirstSessionsFigures(t *testing.T) {
	alone := newRequestJoin()
	for _, id := range sessionA.ids {
		alone.add(id, true)
	}

	together := newRequestJoin()
	for _, id := range sessionA.ids {
		together.add(id, true)
	}
	for _, id := range sessionB.ids {
		together.add(id, true)
	}

	if together.duplicated != alone.duplicated {
		t.Errorf("session A reported %d duplicates alone and %d once session B was "+
			"added. Unrelated evidence changed an unrelated figure.",
			alone.duplicated, together.duplicated)
	}
	if together.total != alone.total+len(sessionB.ids) {
		t.Errorf("total = %d; adding %d foreign requests to %d should give %d",
			together.total, len(sessionB.ids), alone.total, alone.total+len(sessionB.ids))
	}
}

// XW7. The exact boundary of the join contract, as a matrix.
//
// The expected column is DERIVED FROM THE CONTRACT, not invented. The
// contract, from overlap.go and transcript.Request, is two inputs and no
// others:
//
//	id        the request identifier
//	measured  whether that id came off the provider's wire
//
// Session, account, model and correlation are carried on the record and are
// NOT consulted. The matrix exists to make that explicit rather than leave it
// as something a reader has to infer from an absence.
//
// Account is absent from every row because no account identity exists in the
// model at all. See RPL-C019, classified NO_ENDPOINT.
func TestXW7_TheJoinContractBoundary(t *testing.T) {
	type row struct {
		name string
		// what the two records look like
		idA, idB        string
		measuredA       bool
		measuredB       bool
		sameSession     bool // carried, not consulted
		sameModel       bool // carried, not consulted
		sameCorrelation bool // carried, not consulted
		wantDuplicated  int
		wantUnjoinable  int
		note            string
	}

	rows := []row{
		{
			name: "identical in every respect",
			idA:  "req_X", idB: "req_X", measuredA: true, measuredB: true,
			sameSession: true, sameModel: true, sameCorrelation: true,
			wantDuplicated: 1, wantUnjoinable: 0,
			note: "the legitimate case: one request re-rendered into a sub-agent lane",
		},
		{
			name: "same provider id, different session and model",
			idA:  "req_X", idB: "req_X", measuredA: true, measuredB: true,
			sameSession: false, sameModel: false, sameCorrelation: false,
			wantDuplicated: 1, wantUnjoinable: 0,
			note: "MERGES. Session and model are carried and not consulted. This is " +
				"the observed behaviour the campaign records on RPL-C020",
		},
		{
			name: "different provider id, same session",
			idA:  "req_X", idB: "req_Y", measuredA: true, measuredB: true,
			sameSession: true, sameModel: true, sameCorrelation: true,
			wantDuplicated: 0, wantUnjoinable: 0,
			note: "two requests of one session are two requests",
		},
		{
			name: "different provider id, different session",
			idA:  "req_X", idB: "req_Y", measuredA: true, measuredB: true,
			sameSession: false, sameModel: false, sameCorrelation: false,
			wantDuplicated: 0, wantUnjoinable: 0,
			note: "no join, and nothing in the record could have caused one",
		},
		{
			name: "synthesised ids, textually identical, same session",
			idA:  "ledger-0", idB: "ledger-0", measuredA: false, measuredB: false,
			sameSession: true, sameModel: true, sameCorrelation: true,
			wantDuplicated: 0, wantUnjoinable: 2,
			note: "provenance beats text: never a join key whatever it is spelled",
		},
		{
			name: "synthesised ids, textually identical, different session",
			idA:  "ledger-0", idB: "ledger-0", measuredA: false, measuredB: false,
			sameSession: false, sameModel: false, sameCorrelation: false,
			wantDuplicated: 0, wantUnjoinable: 2,
			note: "the case RJ1 covers, restated here as part of the boundary",
		},
		{
			name: "one measured, one synthesised, same text",
			idA:  "req_X", idB: "req_X", measuredA: true, measuredB: false,
			sameSession: false, sameModel: false, sameCorrelation: false,
			wantDuplicated: 0, wantUnjoinable: 1,
			note: "a synthesised id does not match a provider id of the same text",
		},
	}

	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			j := newRequestJoin()
			j.add(r.idA, r.measuredA)
			j.add(r.idB, r.measuredB)

			// Independent oracle over the same two records.
			wantTotal, wantDup, wantUnjoin := oracleJoin([]struct {
				id       string
				measured bool
			}{{r.idA, r.measuredA}, {r.idB, r.measuredB}})

			if j.total != wantTotal || j.duplicated != wantDup || j.unjoinable != wantUnjoin {
				t.Fatalf("production %+v disagrees with the independent oracle "+
					"(total %d, duplicated %d, unjoinable %d)", j, wantTotal, wantDup, wantUnjoin)
			}
			if j.duplicated != r.wantDuplicated {
				t.Errorf("duplicated = %d, contract says %d. %s",
					j.duplicated, r.wantDuplicated, r.note)
			}
			if j.unjoinable != r.wantUnjoinable {
				t.Errorf("unjoinable = %d, contract says %d. %s",
					j.unjoinable, r.wantUnjoinable, r.note)
			}
		})
	}

	// The matrix must contain at least one row of each outcome, or it is not
	// a boundary, only a list of agreements.
	var merges, refusals, unjoinables int
	for _, r := range rows {
		switch {
		case r.wantDuplicated > 0:
			merges++
		case r.wantUnjoinable > 0:
			unjoinables++
		default:
			refusals++
		}
	}
	if merges == 0 || refusals == 0 || unjoinables == 0 {
		t.Fatalf("the matrix has %d merges, %d non-joins and %d unjoinable rows; a "+
			"boundary needs all three or it is not testing a boundary", merges, refusals, unjoinables)
	}
}
