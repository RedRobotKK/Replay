package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// BB. The artefact threw away the only exact number it had.
//
// A design panel attacked `replay gate` on 2026-09-13 and found something
// worse than the problem it was asked about. The standing cost in this
// artefact is `fit.EstimateTokens(bytes)`: a byte count multiplied by a
// tokens-per-byte coefficient. Three facts about that coefficient, all of them
// already written down in this tree:
//
//   - It is fitted on ONE session. budget.go:242 says so and gives the reason,
//     which is a good reason: "the question is what the configuration costs
//     NOW, and a mean over a fortnight of edits describes a setup nobody has."
//   - It is fitted on PROSE. internal/analysis/fit.go:219 deliberately EXCLUDES
//     every turn that re-laid the shared prefix, because "its write covers tool
//     definitions, which are denser than prose and would drag the fit."
//   - It is then applied to TOOL DEFINITIONS, which are schema JSON. That is
//     the population the fit was built by excluding.
//
// The tree already knew. `TokenFit.EstimateOutsideFit` exists for exactly this
// call and its comment says "Nobody has measured tokens-per-byte on schema JSON
// for this provider ... Stating no uncertainty is honest; stating the prose
// fit's was not." budget.go called the other one.
//
// WHY THIS DECIDES WHETHER A GATE CAN EXIST. Measured across the 1,751 rows of
// docs/evidence/calibration-corpus-2026-09-10.md the coefficient has a median
// of about 0.71 tokens per byte and an interquartile range of roughly 0.58 to
// 1.00. Two regenerations of a BYTE-IDENTICAL configuration can therefore
// differ by more than a newly added MCP server would move the real figure. A
// gate comparing those two numbers fires on noise, and a gate that fires on
// noise is deleted in a week.
//
// The bytes, meanwhile, are exact. transcript.Block.Bytes and ToolDef.Bytes are
// decoded textual sizes, deterministic, and identical across two sessions of an
// unchanged configuration. budget.go computed them at lines 269 and 274 and
// then discarded them in favour of the estimate.
//
// So the artefact now carries both: the bytes, which a gate can compare, and
// the tokens, which a human can read, each labelled as what it is.

func budgetKeys(t *testing.T) map[string]any {
	t.Helper()
	b, err := json.Marshal(newBudgetFile(
		standing{TokensPerRequest: 10, SystemTokens: 4, ToolTokens: 6, ToolCount: 2,
			SystemBytes: 40, ToolBytes: 60},
		map[string]int{"srv": 6},
		measured{Sessions: 3, Requests: 30, Source: "ledger"},
	))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// BB1: the exact bytes are in the artefact.
//
// PASS: system_bytes and tool_bytes are present.
// FAIL: the only deterministic quantity available is discarded, and anything
// reading this file has nothing to compare that does not move on its own.
func TestBB1_TheArtefactCarriesTheExactBytes(t *testing.T) {
	m := budgetKeys(t)
	st, _ := m["standing"].(map[string]any)
	if st == nil {
		t.Fatal("no standing block")
	}
	for _, k := range []string{"system_bytes", "tool_bytes"} {
		if _, ok := st[k]; !ok {
			t.Errorf("standing has no %q.\nThe bytes are exact and were already computed; "+
				"the tokens beside them are a prose-fitted coefficient applied to schema "+
				"JSON, whose spread exceeds the change a gate would look for.", k)
		}
	}
}

// BB2: the token figures say they are estimates, in the file.
//
// A consumer of this artefact months from now has the JSON and nothing else.
// If the tokens are not labelled, they read as measured, and this repository's
// whole claim is that a figure says how it was obtained.
func TestBB2_TheTokenFiguresAreLabelledAsEstimates(t *testing.T) {
	m := budgetKeys(t)
	raw, _ := json.Marshal(m)
	if !strings.Contains(string(raw), "tokens_are_estimated") {
		t.Error("the artefact does not mark its token figures as estimated.\n" +
			"They are a coefficient fitted on prose, applied to tool schemas, from one " +
			"session. A reader with only this file would take them for measurements.")
	}
}

// BB3: the provenance names the sample the figure rests on, not the corpus walked.
//
// `sessions` and `requests` count everything read. The standing figure comes
// from the newest session's fit and one request's byte counts. Printing the
// larger pair beside the figure overstates the evidence behind it, which is the
// defect class this project hunts in other people's output.
func TestBB3_TheProvenanceDoesNotOverstateTheSample(t *testing.T) {
	m := budgetKeys(t)
	meas, _ := m["measured"].(map[string]any)
	if meas == nil {
		t.Fatal("no measured block")
	}
	if _, ok := meas["fit_sessions"]; !ok {
		t.Error("measured does not say how many sessions the FIT used.\n" +
			"sessions and requests describe the corpus walked. The standing figure rests " +
			"on one session's fit, and a reader cannot tell those apart.")
	}
}
