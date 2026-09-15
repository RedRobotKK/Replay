package main

import (
	"os"
	"path/filepath"
	"testing"
)

// burn reported 610,551,532 Codex tokens as "no price" and told the operator
// to install a rules document. One was installed on 2026-09-15 carrying
// gpt-5.4, gpt-5.4-mini, gpt-5.6-terra and gpt-6-astra, and the column did not
// move: burnCodex never called the price table at all, because CodexSession
// carried no model to look one up with.
//
// So the advice was unfollowable. That is worse than silence, because the
// operator does the work, sees nothing change, and concludes the prices are
// broken rather than the reading.
func TestBurnPricesCodexOnceTheSessionNamesItsModel(t *testing.T) {
	dir := t.TempDir()
	cx := filepath.Join(dir, "codex")
	if err := os.MkdirAll(cx, 0o755); err != nil {
		t.Fatal(err)
	}
	// A rollout that names its model on turn_context, as a real one does.
	rollout := `{"type":"session_meta","payload":{"id":"s1","cli_version":"0.154.0"}}
{"type":"turn_context","payload":{"model":"gpt-5.4"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":200000,"cached_input_tokens":0},"total_token_usage":{"input_tokens":200000,"cached_input_tokens":0}}}}
`
	if err := os.WriteFile(filepath.Join(cx, "rollout-x.jsonl"), []byte(rollout), 0o600); err != nil {
		t.Fatal(err)
	}

	s := burnCodex("", dir)

	// What this can assert in an isolated HOME: the request reached the price
	// table and was ACCOUNTED FOR, priced or explicitly not. Before this
	// change it was neither, and the surface reported "no price" with nothing
	// saying how many requests that covered.
	//
	// It cannot assert a dollar figure here. The compiled fallback carries no
	// OpenAI rows, so pricing needs the dated document installed in a real
	// HOME, and the rules loader reads that once per process. Verified by hand
	// on 2026-09-15 instead: with docs/rules/openai-2026-09-15.json installed,
	// `replay burn` moved this surface from "no price" on 610,566,576 tokens
	// to "$14.29 (21%)". The 21% is honest, not a bug: the other sessions
	// never named a model and are counted below rather than guessed at.
	if s.pricedReqs+s.unpricedReqs == 0 {
		t.Fatalf("a Codex session naming gpt-5.4 was neither priced nor counted unpriced: "+
			"priced=%d unpriced=%d. The price table was never consulted, which is why "+
			"installing a rules document changed nothing", s.pricedReqs, s.unpricedReqs)
	}
}

// The priced arm, actually taken.
//
// guard-reachability reported the pricing condition UNREACHED even with the
// test above passing, and it was right: in an isolated HOME the compiled
// fallback carries no OpenAI rows, so PriceFor always misses and only the
// unpriced arm ever ran. Both outcomes looked identical from the assertion.
//
// The model here is a stand-in, and deliberately one the COMPILED table knows,
// because what is under test is the wiring rather than OpenAI's prices: does
// burnCodex consult the price table at all and accumulate a cost from a
// session's own usage. A Codex rollout would not really name this model. The
// alternative was asserting nothing about the arm that spends money.
func TestBurnAccumulatesCostWhenThePriceTableKnowsTheModel(t *testing.T) {
	dir := t.TempDir()
	cx := filepath.Join(dir, "codex")
	if err := os.MkdirAll(cx, 0o755); err != nil {
		t.Fatal(err)
	}
	rollout := `{"type":"session_meta","payload":{"id":"s3"}}
{"type":"turn_context","payload":{"model":"claude-opus-5"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000000,"cached_input_tokens":0},"total_token_usage":{"input_tokens":1000000,"cached_input_tokens":0}}}}
`
	if err := os.WriteFile(filepath.Join(cx, "rollout-z.jsonl"), []byte(rollout), 0o600); err != nil {
		t.Fatal(err)
	}

	s := burnCodex("", dir)
	if s.pricedReqs == 0 {
		t.Fatalf("the price table knew this model and nothing was priced: priced=%d unpriced=%d",
			s.pricedReqs, s.unpricedReqs)
	}
	if s.costUSD <= 0 {
		t.Errorf("priced %d request(s) at $%v; a priced request that costs nothing is a free "+
			"session, which is the figure this tool exists to stop printing", s.pricedReqs, s.costUSD)
	}
}

// A rollout that never names a model must stay UNPRICED rather than be priced
// against a default. A guessed model produces a wrong figure with nothing on
// the reader's screen to show it was guessed, which is the failure this whole
// tool exists to refuse.
func TestBurnLeavesAModellessCodexSessionUnpriced(t *testing.T) {
	dir := t.TempDir()
	cx := filepath.Join(dir, "codex")
	if err := os.MkdirAll(cx, 0o755); err != nil {
		t.Fatal(err)
	}
	rollout := `{"type":"session_meta","payload":{"id":"s2"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":5000,"cached_input_tokens":0},"total_token_usage":{"input_tokens":5000,"cached_input_tokens":0}}}}
`
	if err := os.WriteFile(filepath.Join(cx, "rollout-y.jsonl"), []byte(rollout), 0o600); err != nil {
		t.Fatal(err)
	}

	s := burnCodex("", dir)
	if s.costUSD != 0 {
		t.Errorf("a session that named no model was priced at $%v; the model was guessed", s.costUSD)
	}
	if s.pricedReqs != 0 {
		t.Errorf("pricedReqs = %d on a session with no model", s.pricedReqs)
	}
}
