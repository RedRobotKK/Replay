package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A record written by an older Replay is not a record this one could not read.
//
// ReadRecords counts both in `skipped`, and its own documentation names the
// ambiguity without resolving it: "complete lines that are not records of the
// current schema", which it then glossed as data loss, or an upgrade. Those
// are opposite facts. One says bytes are gone; the other says the file is fine
// and this build is newer. The count reaches Session.Skipped, and the report
// renders it as "transcript lines were not conversation content and were
// skipped", which is false of a readable ledger record in all three of its
// claims.
//
// The repository has made this call four times already. Refusals came out of
// Skipped because a working spend cap read as a broken ledger. ProviderFailures
// came out because a rate limit read as a broken ledger. Codex Unparsable came
// out because a line that never parsed was described as refused for a reason
// nobody established. ReEmitted came out because one counter held four facts.
// This is the fifth instance of the same defect and it gets the same answer:
// absence, zero and unknown are three values (ADR-0018), and so are read,
// refused, unreadable and superseded.
//
// What the new population does NOT claim is as load-bearing as what it does.
// A schema-mismatch record has usage this build did not read. That is not zero
// usage, not unknown usage, not a provider failure and not a refusal. Nothing
// here synthesizes a token, and nothing migrates: the gate stays exact
// equality, and a record at any other version is still not read.

// smRecord is one well-formed ledger record at the given schema version.
//
// Built from the same shape ledger_test.go uses for the schema gate, so the
// only thing that differs between the fixtures below is the version number.
func smRecord(schema int, session string) string {
	return fmt.Sprintf(`{"schema":%d,"ts":"2026-09-12T00:00:00Z","session_id":%q,`+
		`"path":"/v1/messages","model":"claude-opus-5","stream":false,`+
		`"prompt":{"system_bytes":0,"tool_bytes":0,"tool_count":0,"cache_control":0,`+
		`"messages":[{"role":"user","blocks":[{"kind":"text","bytes":4}]}]},`+
		`"status":200,"latency_ms":1,`+
		`"response":{"usage":{"input_tokens":10,"cache_creation_input_tokens":0,`+
		`"cache_read_input_tokens":0,"output_tokens":1}}}`, schema, session)
}

// smLedger writes a ledger file of the given lines and returns its path.
func smLedger(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sess-1.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const smMalformed = `this line is not JSON at all`

// SM1: a readable record at another schema is not Skipped.
//
// The user-facing half of the contract, asserted through the Session because
// that is what the report reads. A count here is what makes the report say
// bytes were lost.
func TestSM1_AReadableOldSchemaRecordIsNotSkipped(t *testing.T) {
	s, err := ReadFile(smLedger(t, smRecord(SchemaVersion, "s"), smRecord(SchemaVersion-1, "s")))
	if err != nil {
		t.Fatal(err)
	}
	if s.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0: the record parsed and its bytes are intact, so "+
			"counting it as skipped tells a reader their ledger is damaged when it is "+
			"merely older than this build", s.Skipped)
	}
	if s.SchemaMismatch != 1 {
		t.Errorf("SchemaMismatch = %d, want 1: the superseded record must be counted "+
			"somewhere, or the reader has simply stopped mentioning it", s.SchemaMismatch)
	}
	if s.RequestCount() != 1 {
		t.Errorf("RequestCount = %d, want 1: the current-schema record must still be read",
			s.RequestCount())
	}
}

// SM2: a line that is not a record at all is still Skipped.
//
// The half that must not move. Splitting a population is only honest if the
// original keeps meaning what it meant.
func TestSM2_AMalformedLineIsStillSkipped(t *testing.T) {
	s, err := ReadFile(smLedger(t, smRecord(SchemaVersion, "s"), smMalformed))
	if err != nil {
		t.Fatal(err)
	}
	if s.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1: a line the parser could not read is exactly what "+
			"Skipped is for, and it must not have moved with the schema records", s.Skipped)
	}
	if s.SchemaMismatch != 0 {
		t.Errorf("SchemaMismatch = %d, want 0: nothing here carried a schema at all", s.SchemaMismatch)
	}
}

// SM3: a clean current-schema ledger reports neither population.
func TestSM3_ACurrentSchemaLedgerReportsNeither(t *testing.T) {
	s, err := ReadFile(smLedger(t, smRecord(SchemaVersion, "s"), smRecord(SchemaVersion, "s")))
	if err != nil {
		t.Fatal(err)
	}
	if s.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0 on a clean ledger", s.Skipped)
	}
	if s.SchemaMismatch != 0 {
		t.Errorf("SchemaMismatch = %d, want 0 on a clean ledger", s.SchemaMismatch)
	}
	if s.RequestCount() != 2 {
		t.Errorf("RequestCount = %d, want 2", s.RequestCount())
	}
}

// SM4: three inputs, three populations, none borrowing another's meaning.
//
// This is the case the defect was made of: one number covered two facts, so a
// reader was given one explanation for both.
func TestSM4_MixedInputReportsEachPopulationDistinctly(t *testing.T) {
	s, err := ReadFile(smLedger(t,
		smRecord(SchemaVersion, "s"),
		smMalformed,
		smRecord(SchemaVersion-1, "s"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if s.RequestCount() != 1 {
		t.Errorf("RequestCount = %d, want 1", s.RequestCount())
	}
	// One unreadable line, and only that.
	if s.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1: the malformed line alone. A 2 means the schema "+
			"record is still being counted as data loss", s.Skipped)
	}
	if s.SchemaMismatch != 1 {
		t.Errorf("SchemaMismatch = %d, want 1: the superseded record alone. A 2 means the "+
			"malformed line is being described as merely out of date", s.SchemaMismatch)
	}
}

// SM5: the separation creates nothing and moves no measurement.
//
// A schema-mismatch record contributes no request, no token and no dollar. The
// current-schema accounting beside it must be identical to what the same
// records produce on their own.
func TestSM5_AccountingIsUnchangedByTheSeparation(t *testing.T) {
	alone, err := ReadFile(smLedger(t, smRecord(SchemaVersion, "s")))
	if err != nil {
		t.Fatal(err)
	}
	beside, err := ReadFile(smLedger(t, smRecord(SchemaVersion, "s"), smRecord(SchemaVersion-1, "s")))
	if err != nil {
		t.Fatal(err)
	}
	if beside.SchemaMismatch != 1 {
		t.Fatalf("SchemaMismatch = %d, want 1: the fixture is not exercising the seam", beside.SchemaMismatch)
	}
	if alone.RequestCount() != beside.RequestCount() {
		t.Fatalf("request count moved: %d alone, %d beside a schema-mismatch record",
			alone.RequestCount(), beside.RequestCount())
	}
	for i, lane := range alone.Lanes {
		for j, req := range lane.Requests {
			other := beside.Lanes[i].Requests[j]
			if req.Usage != other.Usage {
				t.Errorf("usage changed when a schema-mismatch record sat beside it: %+v vs %+v",
					req.Usage, other.Usage)
			}
		}
	}
}

// SM7: refusal and provider-failure populations are untouched.
//
// A schema-mismatch record must not leak into either. It names no guard, so it
// is not a refusal, and no provider was reached for it by this reader, so it is
// not a provider failure.
func TestSM7_RefusalAndProviderFailureAreUnchanged(t *testing.T) {
	s, err := ReadFile(smLedger(t, smRecord(SchemaVersion, "s"), smRecord(SchemaVersion-1, "s")))
	if err != nil {
		t.Fatal(err)
	}
	if s.Refusals != 0 {
		t.Errorf("Refusals = %d, want 0: a superseded record names no guard", s.Refusals)
	}
	if got := s.ProviderFailures.Total(); got != 0 {
		t.Errorf("ProviderFailures.Total() = %d, want 0: this reader reached no provider "+
			"for a record it declined to interpret", got)
	}
}

// SM8: a torn tail comes back out of the counter that actually holds it.
//
// ReadRecords treats a rejected final line in a file with no terminating
// newline as a record still being written rather than a broken one, and takes
// it back out of the count. That correction used to have one counter to choose
// from. It now has two, and decrementing the wrong one would invent a negative
// count of lost lines while leaving a superseded record on the books.
//
// The mutation that motivated this test is the one-line form of exactly that:
// replacing the choice with an unconditional `skipped--`. It survived the rest
// of the file, because every other case here ends on a newline.
func TestSM8_ATornTailIsUncountedFromItsOwnPopulation(t *testing.T) {
	// No trailing newline: the last line is the one still in flight.
	p := filepath.Join(t.TempDir(), "sess-1.jsonl")
	body := smRecord(SchemaVersion, "s") + "\n" + smRecord(SchemaVersion-1, "s")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	recs, skipped, schemaMismatch, incomplete, err := ReadRecords(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("read %d records, want 1", len(recs))
	}
	if !incomplete {
		t.Error("the file does not end on a newline, so its last record was still being written")
	}
	if schemaMismatch != 0 {
		t.Errorf("schemaMismatch = %d, want 0: the unterminated line was taken back out of "+
			"the count, and it was counted there", schemaMismatch)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0: nothing here was unreadable, and a negative or "+
			"positive count means the correction was applied to the wrong population", skipped)
	}
}
