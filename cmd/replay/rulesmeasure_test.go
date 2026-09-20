package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `replay rules --measure` counts superseded records apart from skipped ones.
//
// 872d2cb separated the two populations in the ledger reader and gave this
// command a second line so an operator measuring their own ledgers is not told
// that evidence was lost when it was only unread. Nothing tested it.
// guard-reachability reported `if superseded > 0` UNREACHED on 2026-09-20, and
// it was right about more than the branch: measureRules had no test at all.
//
// The distinction is the same one the ledger reader draws. A line the reader
// could not parse is data gone. A record written under another schema is intact
// and simply not this build's to read. Folding the second into the first is
// what the separation exists to stop, and this command is the second surface
// that reports it.

// rulesLedgerRecord is one well-formed ledger record at the given schema
// version, carrying a cache write so the corpus has something to measure.
func rulesLedgerRecord(schema int, requestID string) string {
	return `{"schema":` + itoaTest(schema) + `,"ts":"2026-09-12T00:00:00Z","session_id":"measure",` +
		`"request_id":"` + requestID + `","path":"/v1/messages","model":"claude-opus-5","stream":false,` +
		`"prompt":{"system_bytes":3000,"tool_bytes":60000,"tool_count":30,"cache_control":1,` +
		`"messages":[{"role":"user","blocks":[{"kind":"text","label":"user text","bytes":900}]}]},` +
		`"prefix_hash":"measure-prefix","status":200,"latency_ms":900,` +
		`"response":{"blocks":[{"kind":"text","label":"assistant text","bytes":240}],` +
		`"usage":{"input_tokens":0,"cache_creation_input_tokens":16000,` +
		`"cache_read_input_tokens":0,"output_tokens":40}}}`
}

func itoaTest(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// rulesLedgerDir writes one ledger file of the given lines and returns its dir.
func rulesLedgerDir(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "measure-1.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runMeasure invokes the command through the real dispatch table and returns
// what each stream received.
func runMeasure(t *testing.T, dir string) (string, string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := run([]string{"rules", "--measure", dir}, &out, &errb)
	return out.String(), errb.String(), err
}

// RM1: a superseded record is reported, and reported as its own population.
func TestRM1_MeasureReportsSupersededRecordsApartFromSkipped(t *testing.T) {
	dir := rulesLedgerDir(t,
		rulesLedgerRecord(2, "cur-0"),
		rulesLedgerRecord(1, "old-0"),
	)
	out, errb, err := runMeasure(t, dir)
	if err != nil {
		t.Fatalf("rules --measure over a mixed-schema ledger failed: %v\nstdout: %s\nstderr: %s", err, out, errb)
	}
	if !strings.Contains(errb, "1 record(s) were written under a different ledger schema and were not read") {
		t.Errorf("the superseded record is not reported:\n%s", errb)
	}
	// It must not also be counted as skipped, which is the collapse the
	// ledger separation exists to undo.
	if !strings.Contains(errb, "0 record(s) skipped") {
		t.Errorf("the superseded record was counted as skipped as well:\n%s", errb)
	}
}

// RM2: a clean current-schema ledger says nothing about superseded records.
//
// The other side of the branch. Without this the guard could be neutralised to
// report unconditionally and RM1 would still pass.
func TestRM2_ACleanLedgerReportsNoSupersededLine(t *testing.T) {
	dir := rulesLedgerDir(t,
		rulesLedgerRecord(2, "cur-0"),
		rulesLedgerRecord(2, "cur-1"),
	)
	out, errb, err := runMeasure(t, dir)
	if err != nil {
		t.Fatalf("rules --measure over a clean ledger failed: %v\nstdout: %s\nstderr: %s", err, out, errb)
	}
	if strings.Contains(errb, "different ledger schema") {
		t.Errorf("a superseded line appeared for a ledger that had no superseded record:\n%s", errb)
	}
	if !strings.Contains(errb, "record(s) skipped") {
		t.Errorf("the existing summary line is gone:\n%s", errb)
	}
}

// RM3: an unreadable line is still skipped, and is not called superseded.
func TestRM3_AnUnreadableLineIsStillSkipped(t *testing.T) {
	dir := rulesLedgerDir(t,
		rulesLedgerRecord(2, "cur-0"),
		`this line is not JSON at all`,
	)
	_, errb, err := runMeasure(t, dir)
	if err != nil {
		t.Fatalf("rules --measure failed: %v\n%s", err, errb)
	}
	if !strings.Contains(errb, "1 record(s) skipped") {
		t.Errorf("the unreadable line is not counted as skipped:\n%s", errb)
	}
	if strings.Contains(errb, "different ledger schema") {
		t.Errorf("an unreadable line was described as merely out of date:\n%s", errb)
	}
}
