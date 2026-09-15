package transcript

import (
	"strings"
	"testing"
)

// 610,551,532 Codex tokens priced as nothing, and the reason was never the
// price table.
//
// `replay burn` reported the Codex surface with "no price" and advised
// installing a rules document. A dated OpenAI document WAS installed on
// 2026-09-15 carrying gpt-5.4, gpt-5.4-mini, gpt-5.6-terra and gpt-6-astra,
// and the column did not move, because no rules document could have moved it:
// CodexSession carried no model, so burnCodex had nothing to price with and
// never called PriceForAt at all.
//
// The rollout has carried the model the whole time. It sits on turn_context,
// which is a line type this reader's switch does not handle.
//
// The advice is the part that stings. A tool telling an operator to install a
// document that cannot possibly help is worse than one that says nothing:
// they do the work, see no change, and conclude the pricing is broken rather
// than the reading.
func TestCodexSessionCapturesTheModelItRan(t *testing.T) {
	const rollout = `{"type":"session_meta","payload":{"id":"s1","cli_version":"0.154.0"}}
{"type":"turn_context","payload":{"model":"gpt-5.6-terra","effort":"medium"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1200,"cached_input_tokens":800},"total_token_usage":{"input_tokens":1200,"cached_input_tokens":800}}}}
`
	s, err := ParseCodex(strings.NewReader(rollout))
	if err != nil {
		t.Fatal(err)
	}
	if s.Model == "" {
		t.Fatal("the session recorded no model, so nothing downstream can price it; " +
			"turn_context carried gpt-5.6-terra and the reader dropped it")
	}
	if s.Model != "gpt-5.6-terra" {
		t.Errorf("model = %q, want gpt-5.6-terra", s.Model)
	}
}

// A rollout with no turn_context must leave the model empty rather than
// inventing one. An unknown model priced against a guess is the failure that
// makes every figure downstream wrong in a direction nobody can see.
func TestCodexSessionWithNoTurnContextHasNoModel(t *testing.T) {
	const rollout = `{"type":"session_meta","payload":{"id":"s2","cli_version":"0.154.0"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0}}}}
`
	s, err := ParseCodex(strings.NewReader(rollout))
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != "" {
		t.Errorf("model = %q on a rollout that never named one; absence must stay absent", s.Model)
	}
}

// The last turn_context wins. A session that switches model mid-run is priced
// against something, and the most recent statement is the only defensible
// choice from a single field. That this is lossy is the point of the comment
// on Model: a per-session model cannot describe a session that used two.
func TestCodexSessionTakesTheLatestModel(t *testing.T) {
	const rollout = `{"type":"session_meta","payload":{"id":"s3"}}
{"type":"turn_context","payload":{"model":"gpt-5.4"}}
{"type":"turn_context","payload":{"model":"gpt-5.6-terra"}}
`
	s, err := ParseCodex(strings.NewReader(rollout))
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != "gpt-5.6-terra" {
		t.Errorf("model = %q, want the latest turn_context's gpt-5.6-terra", s.Model)
	}
}
