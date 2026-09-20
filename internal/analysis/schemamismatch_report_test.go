package analysis

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// Counting the population apart is half the fix; the report is the other half.
//
// Session.Refusals is the precedent this package already records: local
// refusals were moved out of Skipped and nothing in production ever read the
// new field, so the misleading line stopped counting them and no line replaced
// it. A reader learned less than before.
//
// These tests run the whole chain rather than a counter, because the false
// statement lives at the end of it:
//
//	ledger file -> ledger.ReadFile -> transcript.Session -> LaneReport.header
//
// A unit test on the count cannot see the sentence, and the sentence is the
// defect. "transcript lines were not conversation content and were skipped" is
// false of a readable ledger record three times over: it is not a transcript
// line, it is conversation content, and it was not unreadable.

// smReportRecord is one well-formed ledger record at the given schema version.
func smReportRecord(schema int) string {
	return fmt.Sprintf(`{"schema":%d,"ts":"2026-09-12T00:00:00Z","session_id":"s",`+
		`"path":"/v1/messages","model":"claude-opus-5","stream":false,`+
		`"prompt":{"system_bytes":0,"tool_bytes":0,"tool_count":0,"cache_control":0,`+
		`"messages":[{"role":"user","blocks":[{"kind":"text","bytes":4}]}]},`+
		`"status":200,"latency_ms":1,`+
		`"response":{"usage":{"input_tokens":10,"cache_creation_input_tokens":0,`+
		`"cache_read_input_tokens":0,"output_tokens":1}}}`, schema)
}

// smHeader reads a ledger built from the given lines and returns its header.
//
// The real reader, not a hand-built Session: a Session assembled in the test
// would prove the printer works and say nothing about what the ledger path
// puts in front of it.
func smHeader(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sess-1.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sess, err := ledger.ReadFile(p)
	if err != nil {
		t.Fatalf("reading the ledger: %v", err)
	}
	if len(sess.Lanes) == 0 {
		t.Fatal("the ledger produced no lane, so there is no report to read")
	}
	r := &LaneReport{Session: sess, Lane: sess.Lanes[0], Calibration: &Calibration{}}
	var b bytes.Buffer
	pr := NewPrinter(&b)
	r.header(pr)
	return b.String()
}

// smNoteLines returns only the Note lines, so an assertion about what this
// change emits cannot fail on the pre-existing Rules or Tier lines.
func smNoteLines(out string) string {
	var keep []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Note:") {
			keep = append(keep, strings.TrimSpace(line))
		}
	}
	return strings.Join(keep, "\n")
}

// SM6: the schema-mismatch population is described as what it is.
//
// The wording must claim only what the reader established: the record was
// written under a different schema version and this build did not read it.
// Not that it was lost, not that it held nothing, not that it was a transcript.
func TestSM6_TheSchemaMismatchNoteIsTruthful(t *testing.T) {
	notes := smNoteLines(smHeader(t, smReportRecord(ledger.SchemaVersion), smReportRecord(ledger.SchemaVersion-1)))
	if notes == "" {
		t.Fatalf("no note was emitted for a ledger carrying a superseded record, so the " +
			"reader is told nothing at all about it")
	}
	low := strings.ToLower(notes)
	if !strings.Contains(low, "schema") {
		t.Errorf("the note does not say the record's schema is why it was not read:\n%s", notes)
	}
	// Each of these was true of the old sentence and is false of this record.
	for _, banned := range []string{
		"transcript line", "not conversation content", "could not be read",
		"could not be parsed", "unreadable", "lost",
	} {
		if strings.Contains(low, banned) {
			t.Errorf("the note claims %q, which is not true of a record that parsed:\n%s",
				banned, notes)
		}
	}
	// It carries usage this build did not read. That is not zero and not a
	// measurement, so the note must not put a number of tokens or dollars on it.
	for _, banned := range []string{"token", "$", "cost", "usage", "billed", "zero"} {
		if strings.Contains(low, banned) {
			t.Errorf("the note claims %q about a record whose contents were never read:\n%s",
				banned, notes)
		}
	}
}

// SM6b: the two populations are reported apart, each keeping its own sentence.
//
// A malformed line and a superseded record in one file must not produce one
// note covering both, which is the shape of the defect being fixed.
func TestSM6b_TheTwoPopulationsAreReportedApart(t *testing.T) {
	notes := smNoteLines(smHeader(t,
		smReportRecord(ledger.SchemaVersion),
		`this line is not JSON at all`,
		smReportRecord(ledger.SchemaVersion-1),
	))
	if !strings.Contains(notes, "not conversation content") {
		t.Errorf("the existing Skipped note is gone, so the malformed line is no longer "+
			"reported at all:\n%s", notes)
	}
	if !strings.Contains(strings.ToLower(notes), "schema") {
		t.Errorf("the superseded record is not reported:\n%s", notes)
	}
	// One of each. A "2" in either sentence means the populations are still
	// sharing a number.
	if strings.Contains(notes, "2 transcript lines") {
		t.Errorf("the superseded record is still being counted as a skipped line:\n%s", notes)
	}
}

// SM6c: a clean current-schema ledger says neither thing.
func TestSM6c_ACleanLedgerReportsNeitherNote(t *testing.T) {
	notes := smNoteLines(smHeader(t, smReportRecord(ledger.SchemaVersion), smReportRecord(ledger.SchemaVersion)))
	if strings.Contains(strings.ToLower(notes), "schema") {
		t.Errorf("a schema note appeared for a ledger that had no superseded record:\n%s", notes)
	}
	if strings.Contains(notes, "not conversation content") {
		t.Errorf("a skipped note appeared for a clean ledger:\n%s", notes)
	}
}
