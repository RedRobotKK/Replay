package ledger

import (
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A refusal the proxy wrote on purpose is not a record the reader lost.
//
// SessionBuilder.Add counts any record with no usage and no messages as
// Skipped. That is exactly the shape recordRefusal writes: the proxy answered
// a request locally — a spend cap, a guard — so there is no provider usage and
// no prompt to carry. The record is complete, deliberate, and names the guard
// that produced it in Record.Refusal.
//
// So a session that hit its spend cap three times reported three skipped
// records, and Session.Skipped is what `replay cost` and the TUI show a reader
// as "records that could not be read". Working protection looked like data
// loss, which is the same collapse the torn-tail change fixed one level up and
// is why fixing only that one left the instrument still lying.
//
// Three states, not two: read, refused, unreadable.
func TestRF1_ARefusalIsCountedAsARefusalNotAsSkipped(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// One real request, one refusal, one genuinely unusable record.
	usable := Record{SessionID: "s1"}
	usable.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	usable.Response = Response{Usage: &transcript.Usage{Input: 10, Output: 2}}
	refused := Record{
		SessionID:     "s1",
		Refusal:       "spend_cap",
		RefusalReason: "daily cap reached",
	}
	// No usage, no messages, and no refusal naming a guard: nothing explains
	// this one, so it stays Skipped.
	broken := Record{SessionID: "s1"}

	for _, r := range []Record{usable, refused, broken} {
		if err := s.Append(r); err != nil {
			t.Fatal(err)
		}
	}

	sess, err := ReadFile(filepath.Join(dir, sessionFileName("s1")+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	if sess.Refusals != 1 {
		t.Errorf("Refusals = %d, want 1. The proxy answered a request locally and said "+
			"which guard did it; that is a recorded decision, not a lost record", sess.Refusals)
	}
	if sess.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 — only the record that explains nothing.\n"+
			"      A refusal counted here tells a reader their spend cap working is "+
			"their ledger breaking", sess.Skipped)
	}
	if got := len(sess.Lanes); got != 1 {
		t.Fatalf("lanes = %d, want 1", got)
	}
	if got := len(sess.Lanes[0].Requests); got != 1 {
		t.Errorf("requests = %d, want 1: only the usable record becomes a request", got)
	}
}
