package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// replay simulate --policy <file> <ledger-dir...>
//
// The claim under test, and the only one: given a recorded ledger and an
// explicit alternative spend-cap policy, the command deterministically says
// which recorded requests would have been refused under that policy, using the
// same guard the proxy uses, and alters nothing. Every result is SIMULATED: a
// replay of what was recorded, never a statement about requests that have not
// happened.

type simResult struct {
	Schema    string `json:"schema"`
	Simulated bool   `json:"simulated"`
	Policy    struct {
		Hash          string  `json:"hash"`
		MaxSessionUSD float64 `json:"maxSessionUsd"`
		MaxDayUSD     float64 `json:"maxDayUsd"`
	} `json:"policy"`
	Population struct {
		Requests            int `json:"requests"`
		Sessions            int `json:"sessions"`
		HistoricallyRefused int `json:"historicallyRefused"`
		WithoutUsage        int `json:"withoutUsage"`
		Unpriced            int `json:"unpriced"`
	} `json:"population"`
	Summary struct {
		Admitted         int     `json:"admitted"`
		Refused          int     `json:"refused"`
		SessionsAffected int     `json:"sessionsAffected"`
		RefusedListUSD   float64 `json:"refusedListUsd"`
	} `json:"summary"`
	Decisions []struct {
		Session    string  `json:"session"`
		RequestID  string  `json:"requestId"`
		TS         string  `json:"ts"`
		Model      string  `json:"model"`
		ListUSD    float64 `json:"listUsd"`
		UpperBound bool    `json:"upperBound"`
		Simulated  string  `json:"simulated"`
		Reason     string  `json:"reason,omitempty"`
	} `json:"decisions"`
}

func writePolicy(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func simulateJSON(t *testing.T, policy, dir string) (simResult, string, error) {
	t.Helper()
	out, errb, err := e2e(t, "simulate", "--policy", policy, "--json", dir)
	var r simResult
	if err == nil {
		if jerr := json.Unmarshal([]byte(out), &r); jerr != nil {
			t.Fatalf("simulate --json did not print a JSON object: %v\n%s\n%s", jerr, out, errb)
		}
	}
	return r, out, err
}

// a ledger with a priced session (two requests, each a cache break), an
// unpriced session, and one request the proxy refused at the time
func simLedger(t *testing.T) (home, dir string) {
	t.Helper()
	home = t.TempDir()
	isolateHome(t, home)
	dir = filepath.Join(home, "ledger")
	e2eLedger(t, dir)
	rec := ledger.Record{Schema: ledger.SchemaVersion, Timestamp: time.Date(2026, 10, 2, 12, 5, 0, 0, time.UTC),
		SessionID: "priced", Path: "/v1/messages", Status: 400, Refusal: "spend_cap", RefusalReason: "session spend cap reached: $1.00 of $1.00 at list price",
		RequestSummary: ledger.RequestSummary{Model: "claude-opus-5"}}
	b, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, "refused.jsonl"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return home, dir
}

func dirDigest(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		h.Write([]byte(e.Name()))
		h.Write(b)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Test 1: under a cap nothing reaches, every recorded request stays admitted.
func TestSimulate_RequestsUnderTheCapRemainAdmitted(t *testing.T) {
	_, dir := simLedger(t)
	r, _, err := simulateJSON(t, writePolicy(t, dir, `{"maxSessionUsd": 1000}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Population.Requests != 4 || r.Summary.Admitted != 4 || r.Summary.Refused != 0 {
		t.Errorf("population %d, admitted %d, refused %d; want 4, 4, 0 under a $1000 cap", r.Population.Requests, r.Summary.Admitted, r.Summary.Refused)
	}
	for _, d := range r.Decisions {
		if d.Simulated != "admitted" {
			t.Errorf("%s/%s simulated %q under a $1000 cap", d.Session, d.RequestID, d.Simulated)
		}
	}
}

// Test 2: a request past the cap is refused with the guard's own reason.
func TestSimulate_ARequestPastTheCapIsRefused(t *testing.T) {
	_, dir := simLedger(t)
	r, _, err := simulateJSON(t, writePolicy(t, dir, `{"maxSessionUsd": 0.001}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary.Refused == 0 {
		t.Fatalf("nothing refused under a $0.001 cap over requests that cost more than that:\n%+v", r.Summary)
	}
	var refused int
	for _, d := range r.Decisions {
		if d.Session == "priced" && d.Simulated == "refused" {
			refused++
			if !strings.Contains(d.Reason, "session spend cap reached") {
				t.Errorf("refusal reason %q is not the guard's", d.Reason)
			}
		}
	}
	if refused == 0 {
		t.Error("the priced session had no refused request")
	}
}

// Test 3: the population is replayed with the proxy's rule: a request is
// refused exactly when the session's admitted list spend before it has reached
// the cap. Recomputed here independently of the command.
func TestSimulate_ThePopulationFollowsTheGuardsRule(t *testing.T) {
	_, dir := simLedger(t)
	const sessionCap = 0.05
	r, _, err := simulateJSON(t, writePolicy(t, dir, `{"maxSessionUsd": 0.05}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	spent := map[string]float64{}
	for i, d := range r.Decisions {
		want := "admitted"
		if spent[d.Session] >= sessionCap {
			want = "refused"
		}
		if d.Simulated != want {
			t.Errorf("decision %d (%s/%s): simulated %q, want %q with $%.4f spent before it", i, d.Session, d.RequestID, d.Simulated, want, spent[d.Session])
		}
		if d.Simulated == "admitted" {
			spent[d.Session] += d.ListUSD
		}
	}
	if r.Population.HistoricallyRefused != 1 {
		t.Errorf("historically refused = %d, want 1 (counted, not replayed: its usage is unknown)", r.Population.HistoricallyRefused)
	}
	if r.Population.Requests != len(r.Decisions) {
		t.Errorf("population %d but %d decisions", r.Population.Requests, len(r.Decisions))
	}
}

// Test 4: the supplied policy is the one evaluated.
func TestSimulate_EvaluatesTheSuppliedPolicyNotAnotherOne(t *testing.T) {
	_, dir := simLedger(t)
	loose, _, err := simulateJSON(t, writePolicy(t, dir, `{"maxSessionUsd": 1000}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	tight, _, err := simulateJSON(t, writePolicy(t, dir, `{"maxSessionUsd": 0.001}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	if loose.Policy.Hash == tight.Policy.Hash {
		t.Error("two different policies carry the same hash")
	}
	if loose.Summary.Refused >= tight.Summary.Refused {
		t.Errorf("a $1000 cap refused %d and a $0.001 cap refused %d; the supplied policy was not evaluated", loose.Summary.Refused, tight.Summary.Refused)
	}
	if tight.Policy.MaxSessionUSD != 0.001 {
		t.Errorf("the output names cap %v, want 0.001", tight.Policy.MaxSessionUSD)
	}
}

// Test 5: malformed policy input is refused.
func TestSimulate_RefusesAMalformedPolicy(t *testing.T) {
	_, dir := simLedger(t)
	for name, body := range map[string]string{
		"not json":       `{maxSessionUsd: 1`,
		"wrong type":     `{"maxSessionUsd": "five"}`,
		"unknown key":    `{"maxSesionUsd": 1}`,
		"no cap enabled": `{"maxSessionUsd": 0}`,
		"negative cap":   `{"maxSessionUsd": -1}`,
	} {
		_, _, err := simulateJSON(t, writePolicy(t, dir, body), dir)
		if err == nil {
			t.Errorf("%s: accepted %s", name, body)
		} else if !errors.Is(err, errUsage) {
			t.Errorf("%s: refused with %v, want errUsage", name, err)
		}
	}
	if _, _, err := e2e(t, "simulate", dir); err == nil {
		t.Error("simulate without --policy ran")
	}
	if _, _, err := e2e(t, "simulate", "--policy", filepath.Join(dir, "missing.json"), dir); err == nil {
		t.Error("a missing policy file was accepted")
	}
}

// Test 6: a missing ledger fails clearly.
func TestSimulate_AMissingLedgerFails(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	policy := writePolicy(t, home, `{"maxSessionUsd": 1}`)
	_, errb, err := e2e(t, "simulate", "--policy", policy, filepath.Join(home, "no-such-ledger"))
	if err == nil {
		t.Fatal("a nonexistent ledger directory produced a simulation")
	}
	if !strings.Contains(err.Error()+errb, "no-such-ledger") {
		t.Errorf("the error does not name the missing directory: %v %s", err, errb)
	}
	if _, _, err := e2e(t, "simulate", "--policy", policy); err == nil {
		t.Error("simulate with no ledger argument ran")
	}
}

// Test 7: an empty population is a valid zero result.
func TestSimulate_AnEmptyPopulationIsAValidZeroResult(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	empty := filepath.Join(home, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	r, out, err := simulateJSON(t, writePolicy(t, home, `{"maxSessionUsd": 1}`), empty)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if r.Population.Requests != 0 || len(r.Decisions) != 0 || r.Summary.Refused != 0 {
		t.Errorf("empty ledger: %+v", r)
	}
	human, _, err := e2e(t, "simulate", "--policy", filepath.Join(home, "policy.json"), empty)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "simulate empty", human, "0 requests")
}

// Test 8: the JSON is the documented shape and labelled SIMULATED.
func TestSimulate_JSONIsLabelledSimulated(t *testing.T) {
	_, dir := simLedger(t)
	r, out, err := simulateJSON(t, writePolicy(t, dir, `{"maxSessionUsd": 0.05}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Schema != "replay.simulate.v1" || !r.Simulated {
		t.Errorf("schema %q simulated %v", r.Schema, r.Simulated)
	}
	if r.Policy.Hash == "" {
		t.Error("no policy hash")
	}
	for _, d := range r.Decisions {
		if d.TS == "" || d.Session == "" || d.Model == "" || (d.Simulated != "admitted" && d.Simulated != "refused") {
			t.Errorf("decision missing a field: %+v", d)
		}
	}
	for _, banned := range []string{"saving", "forecast", "will save", "expected future"} {
		if strings.Contains(strings.ToLower(out), banned) {
			t.Errorf("output uses %q", banned)
		}
	}
	human, _, err := e2e(t, "simulate", "--policy", filepath.Join(dir, "policy.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "simulate human", human, "SIMULATED", r.Policy.Hash, "refused")
	for _, banned := range []string{"saving", "forecast", "will save", "expected future"} {
		if strings.Contains(strings.ToLower(human), banned) {
			t.Errorf("human output uses %q", banned)
		}
	}
}

// Test 9: same ledger, same policy, byte-identical JSON.
func TestSimulate_IsDeterministic(t *testing.T) {
	_, dir := simLedger(t)
	policy := writePolicy(t, dir, `{"maxSessionUsd": 0.05}`)
	_, a, err := simulateJSON(t, policy, dir)
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := simulateJSON(t, policy, dir)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("two runs differ:\n%s\n---\n%s", a, b)
	}
}

// Test 10: the recorded ledger is not touched.
func TestSimulate_DoesNotAlterTheLedger(t *testing.T) {
	_, dir := simLedger(t)
	policy := writePolicy(t, dir, `{"maxSessionUsd": 0.05}`)
	before := dirDigest(t, dir)
	if _, _, err := simulateJSON(t, policy, dir); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e2e(t, "simulate", "--policy", policy, dir); err != nil {
		t.Fatal(err)
	}
	if after := dirDigest(t, dir); after != before {
		t.Error("the ledger directory changed during simulation")
	}
}

// Test 11: the replay order is the records' timestamps, with (session, request
// id) breaking ties, whatever order the files or the lines inside them are in.
// A ledger directory lists files alphabetically and a session file is written
// in arrival order, so an implementation that walked files would replay a
// later session's requests before an earlier one's and reach a day cap in the
// wrong place; this fixture puts the earliest request in the last file, last
// line, and gives two requests the same timestamp.
func TestSimulate_ReplaysInTimestampOrderRegardlessOfFileOrder(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := func(sec int) time.Time { return time.Date(2026, 10, 2, 12, 0, sec, 0, time.UTC) }
	write := func(name string, recs ...ledger.Record) {
		var lines []byte
		for _, r := range recs {
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		if err := os.WriteFile(filepath.Join(dir, name), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rec := func(sid, rid string, ts time.Time) ledger.Record {
		u := transcript.Usage{Input: 1000, Output: 500}
		return ledger.Record{Schema: ledger.SchemaVersion, Timestamp: ts, SessionID: sid, RequestID: rid, Path: "/v1/messages", Status: 200,
			RequestSummary: ledger.RequestSummary{Model: "claude-opus-5"}, Response: ledger.Response{Usage: &u}}
	}
	// file "a" holds the LATER requests, out of order inside the file too
	write("a.jsonl", rec("s-late", "r4", at(40)), rec("s-late", "r3", at(30)))
	// file "z" holds the EARLIEST request, and two with the same timestamp
	write("z.jsonl", rec("s-early", "r2", at(20)), rec("s-early", "r1", at(20)), rec("s-early", "r0", at(0)))

	r, _, err := simulateJSON(t, writePolicy(t, home, `{"maxDayUsd": 1000}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range r.Decisions {
		got = append(got, d.Session+"/"+d.RequestID+"@"+d.TS)
	}
	want := []string{
		"s-early/r0@2026-10-02T12:00:00Z",
		"s-early/r1@2026-10-02T12:00:20Z",
		"s-early/r2@2026-10-02T12:00:20Z",
		"s-late/r3@2026-10-02T12:00:30Z",
		"s-late/r4@2026-10-02T12:00:40Z",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("replay order:\n got %v\nwant %v", got, want)
	}

	// The order decides the result under a day cap: the earliest request must
	// be the one admitted and the rest refused, not whichever file came first.
	// Each request costs $0.0175 at list; a $0.02 day cap admits exactly one.
	r2, _, err := simulateJSON(t, writePolicy(t, home, `{"maxDayUsd": 0.02}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Decisions) != 5 || r2.Decisions[0].RequestID != "r0" || r2.Decisions[0].Simulated != "admitted" || r2.Summary.Refused != 3 && r2.Summary.Refused != 4 {
		t.Errorf("under a $0.02 day cap the earliest request (r0) must be the admitted one; got %+v", r2.Decisions)
	}
	for _, d := range r2.Decisions[2:] {
		if d.Simulated != "refused" {
			t.Errorf("%s after the day cap was reached is %q", d.RequestID, d.Simulated)
		}
	}
}
