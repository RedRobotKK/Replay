package ledger

import "testing"

// Record.SessionID is documented as "the client-supplied session header, or a
// hash of the stable prefix when the client sent none", and the proxy
// implements that fallback by copying RequestSummary.SessionHash. The Anthropic
// summariser sets it; this one did not, so every OpenAI-compatible client that
// does not send Claude Code's private x-claude-code-session-id header had its
// ledger record dropped on the floor — silently, with a correct-looking log
// line. Found on 2026-09-05 against a live Ollama endpoint.
//
// PrefixHash has the same omission and a worse failure: the sibling gate keys
// on it, so an empty value makes every unrelated OpenAI request share one gate
// key and serialise behind each other.
func TestSummarizeOpenAIRequestSetsHashes(t *testing.T) {
	body := []byte(`{"model":"qwen2.5-coder:7b","messages":[
		{"role":"system","content":"You are terse."},
		{"role":"user","content":"Reply with exactly: OK"}]}`)

	sum, err := SummarizeOpenAIRequest(body, nil)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if sum.SessionHash == "" {
		t.Error("SessionHash is empty: the proxy's fallback assigns it to SessionID, " +
			"so an empty value means the record is never written to the ledger")
	}
	if sum.PrefixHash == "" {
		t.Error("PrefixHash is empty: --hold-siblings keys on it, so an empty value " +
			"collapses every request onto one gate key")
	}
}

// Two sessions that differ only in their first user message must not be treated
// as the same session, or their turns interleave in one ledger file.
func TestSummarizeOpenAIRequestSessionHashDistinguishes(t *testing.T) {
	one := []byte(`{"model":"m","messages":[
		{"role":"system","content":"You are terse."},
		{"role":"user","content":"first question here"}]}`)
	two := []byte(`{"model":"m","messages":[
		{"role":"system","content":"You are terse."},
		{"role":"user","content":"an entirely different question"}]}`)

	a, err := SummarizeOpenAIRequest(one, nil)
	if err != nil {
		t.Fatalf("summarize one: %v", err)
	}
	b, err := SummarizeOpenAIRequest(two, nil)
	if err != nil {
		t.Fatalf("summarize two: %v", err)
	}
	if a.SessionHash == b.SessionHash {
		t.Errorf("distinct first messages share a session hash: %q", a.SessionHash)
	}
	// The system prompt is the same, so the cacheable prefix is the same and
	// the sibling gate should recognise them as siblings.
	if a.PrefixHash != b.PrefixHash {
		t.Errorf("same system prompt gave different prefix hashes: %q vs %q",
			a.PrefixHash, b.PrefixHash)
	}
}
