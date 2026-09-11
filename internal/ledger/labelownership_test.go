package ledger

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// TestLO6TheLedgerOwnsItsToolNameMap guards a property the transcript
// package now depends on. DecodeBlock trusts a tool_use id that is already
// in the toolNames map and does not consult the label function for it, which
// makes the map an input as well as an output: whoever passes it is asserting
// that its entries came from the label function they are passing.
//
// The ledger's labels are keyed with the store's secret, so a plain label
// arriving in that map would be written into the ledger unhashed, and the
// file would carry the path that the key exists to hide. Nothing about the
// skip in DecodeBlock can detect that; only this can, by starting from the
// real entry point and looking for the path in the output.
//
// Mutating summarize.go to seed its map — toolNames := map[string]string{
// "t1": "Read /tmp/x.go"} — is what this is written against, and it fails.
func TestLO6TheLedgerOwnsItsToolNameMap(t *testing.T) {
	const secretPath = "/tmp/x.go"
	if !strings.Contains(sampleRequest, secretPath) {
		t.Fatalf("the fixture no longer carries %q, so this proves nothing", secretPath)
	}

	sum, err := SummarizeRequest([]byte(sampleRequest), NewLabeler([]byte("test-key")))
	if err != nil {
		t.Fatal(err)
	}

	// Whole-summary sweep: wherever the label ends up, the path must not.
	encoded, err := json.Marshal(sum)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secretPath) {
		t.Fatalf("the summary carries the raw path %q:\n%s", secretPath, encoded)
	}

	// And the label is present, not merely stripped: a summary that dropped
	// the block would pass the sweep above while losing the attribution.
	//
	// The map's value surfaces on the tool RESULT, which carries only the id
	// of the call it answers and has to look the name up. That is the path a
	// plain label would travel: "tool result: Read /tmp/x.go".
	var results, hashed int
	for _, m := range sum.Prompt.Messages {
		for _, b := range m.Blocks {
			if b.Kind != transcript.KindToolResult {
				continue
			}
			results++
			if strings.Contains(b.Label, "r/") {
				hashed++
			}
		}
	}
	if results == 0 {
		t.Fatal("no tool result in the summary: the fixture has moved and this proves nothing")
	}
	if hashed != results {
		t.Fatalf("%d of %d tool-result labels are hashed, want all of them", hashed, results)
	}
}
