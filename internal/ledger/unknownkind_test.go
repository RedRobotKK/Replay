package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P3. An event kind this reader does not handle must be PRESERVED, not reported
// as lost conversation data.
//
// store.go:374 is an unconditional `b.session.Skipped++` fallthrough, and
// `Record` has no kind discriminator. transcript.Session documents Skipped as
// "lines the parser could not interpret ... reported so a format change does
// not pass silently", and analysis/report.go:186 surfaces it.
//
// So a record that parses perfectly and merely carries a kind this binary does
// not implement is reported to the user as a PARSE FAILURE. That conflates two
// different facts, which is the distinction this repository exists to keep:
//
//	unparseable  the bytes are gone, something is wrong
//	unknown kind the bytes are fine, this reader is older than the writer
//
// The second is the normal, expected consequence of the additive-optional
// schema rule at record.go:36-39. It must not read as data loss.

// writeLines puts raw JSONL into a ledger file and reads it back the way
// production does.
func ukWrite(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sess.jsonl")
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A well-formed schema-2 record carrying a kind this binary does not implement.
// Everything else about it is valid: session id, timestamp, status.
const unknownKindLine = `{"schema":2,"ts":"2026-10-02T09:00:00Z","session_id":"s1",` +
	`"kind":"state_change","path":"/v1/messages","status":200}`

// A genuinely broken line: the positive control for Skipped.
const unparseableLine = `{"schema":2,"ts":"2026-10-02T09:00:01Z","sess`

// A normal request record: the negative control, which must still parse.
const requestLine = `{"schema":2,"ts":"2026-10-02T09:00:02Z","session_id":"s1",` +
	`"path":"/v1/messages","status":200,"model":"claude-opus-5",` +
	`"prompt":{"system_bytes":10,"messages":[{"role":"user","blocks":` +
	`[{"kind":"text","label":"u","bytes":40}]}]},` +
	`"response":{"usage":{"input_tokens":5,"output_tokens":3},"blocks":` +
	`[{"kind":"text","label":"a","bytes":20}]}}`

func TestUK1_AnUnknownEventKindIsNotReportedAsLostConversation(t *testing.T) {
	// POSITIVE CONTROL first: a truly unparseable line must raise Skipped, or
	// this test cannot tell preservation from a broken counter.
	// Paired with a valid record: a file of only a bad line has no records at
	// all and ReadFile refuses it before any counter moves, which would make
	// this control prove nothing.
	broken := ukWrite(t, requestLine, unparseableLine)
	bs, err := ReadFile(broken)
	if err != nil {
		t.Fatalf("a file with one bad line must still read: %v", err)
	}
	if bs.Skipped == 0 {
		t.Fatalf("POSITIVE CONTROL BROKEN: an unparseable line did not raise Skipped, "+
			"so this test cannot distinguish preservation from a dead counter. got %+v", bs)
	}

	// NEGATIVE CONTROL: a normal request still parses into a lane.
	good := ukWrite(t, requestLine)
	gs, err := ReadFile(good)
	if err != nil {
		t.Fatalf("a normal record must read: %v", err)
	}
	if len(gs.Lanes) == 0 {
		t.Fatalf("NEGATIVE CONTROL BROKEN: a normal request produced no lane; the "+
			"fixture does not exercise the parse path. got %+v", gs)
	}

	// THE CLAIM.
	// Same pairing, for the same reason.
	unk := ukWrite(t, requestLine, unknownKindLine)
	us, err := ReadFile(unk)
	if err != nil {
		t.Fatalf("a record with an unknown kind must still read: %v", err)
	}
	if us.Skipped > 0 {
		t.Errorf("a record carrying an unknown event kind was counted in Skipped "+
			"(%d). Skipped is documented as \"lines the parser could not interpret\" "+
			"and is surfaced to the user as a format change. This record parsed "+
			"perfectly; this binary simply does not implement its kind, which is the "+
			"expected consequence of the additive-optional schema rule. Reporting it "+
			"as lost data is the unknown-versus-unparseable conflation.", us.Skipped)
	}
}

// UK2. Preservation is not enough on its own: an unknown kind must be COUNTED,
// or the reader has silently dropped a record and said nothing. Absence of a
// complaint is not the same as absence of the record.
func TestUK2_AnUnknownEventKindIsCountedSoItIsNotSilentlyDropped(t *testing.T) {
	// Same pairing, for the same reason.
	unk := ukWrite(t, requestLine, unknownKindLine)
	s, err := ReadFile(unk)
	if err != nil {
		t.Fatal(err)
	}
	if s.UnknownKinds == 0 {
		t.Errorf("an unknown event kind was neither counted as Skipped nor counted as "+
			"unknown, so it vanished without a word. A reader older than its writer "+
			"must be able to say how much it could not interpret. got %+v", s)
	}
}

// UK3. HISTORICAL COMPATIBILITY. Every record written before `Kind` existed has
// no kind at all, and must follow exactly the path it followed before.
//
// This is the whole basis of the migration strategy: SchemaVersion stays at 2
// and an absent optional field keeps its original meaning. If the switch's
// empty case diverted anything, months of existing ledgers would be
// reinterpreted.
func TestUK3_RecordsWithoutAKindAreUnchanged(t *testing.T) {
	const refusalLine = `{"schema":2,"ts":"2026-10-02T09:00:03Z","session_id":"s1",` +
		`"path":"/v1/messages","status":400,"refusal":"spend-cap"}`

	s, err := ReadFile(ukWrite(t, requestLine, refusalLine, unparseableLine))
	if err != nil {
		t.Fatal(err)
	}
	// Each pre-existing path still reached, and none of them diverted into the
	// new counter.
	if len(s.Lanes) == 0 {
		t.Errorf("a kindless request no longer builds a lane: %+v", s)
	}
	if s.Refusals != 1 {
		t.Errorf("a kindless refusal no longer counts as a refusal: Refusals=%d", s.Refusals)
	}
	if s.Skipped != 1 {
		t.Errorf("a genuinely unparseable line no longer counts as Skipped: Skipped=%d", s.Skipped)
	}
	if s.UnknownKinds != 0 {
		t.Errorf("a record with no kind was diverted into UnknownKinds (%d). Every "+
			"record written before this field existed has no kind, so this would "+
			"reinterpret every historical ledger.", s.UnknownKinds)
	}
}

// UK4. The two counters must not both fire for one record. Double counting
// would make a version gap look like a version gap AND a parse failure.
func TestUK4_AnUnknownKindIsCountedOnce(t *testing.T) {
	s, err := ReadFile(ukWrite(t, requestLine, unknownKindLine))
	if err != nil {
		t.Fatal(err)
	}
	if s.UnknownKinds+s.Skipped != 1 {
		t.Errorf("one unknown-kind record produced UnknownKinds=%d and Skipped=%d; "+
			"exactly one of them must fire", s.UnknownKinds, s.Skipped)
	}
}
