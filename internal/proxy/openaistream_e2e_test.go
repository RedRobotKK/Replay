package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// The OpenAI response path had no test that went through the proxy.
//
// `openai_usage_test.go` covers the parser well, but all four of its tests
// call withUsageReporting() directly. Nothing drove a streamed
// /v1/chat/completions through the server, so the line that chooses which
// parser reads the response —
//
//	if t.openai { t.ostream = &ledger.OpenAIStreamParser{} } else { ... }
//
// was entered by no test. guard-reachability reported it and its neighbours
// UNREACHED while the suite stayed green.
//
// The consequence is on the money path rather than in a log. The parser
// chosen here produces the token counts the ledger stores, and the ledger is
// what `replay cost` prices. Read an OpenAI stream with the Anthropic parser
// and the usage frame is not recognised: the request records zero tokens and
// costs nothing, and a zero is indistinguishable in the report from a cheap
// turn. Absence reported as zero, on the surface the tool exists to measure.

// openaiSSE is one streamed chat completion, ending in the usage frame that
// `stream_options: {"include_usage": true}` asks for.
func openaiSSE(promptTokens, cachedTokens, completionTokens int) string {
	return strings.Join([]string{
		`data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4o",` +
			`"choices":[{"index":0,"delta":{"role":"assistant","content":"hi"}}]}`,
		`data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4o",` +
			`"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		fmt.Sprintf(`data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-4o",`+
			`"choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,`+
			`"total_tokens":%d,"prompt_tokens_details":{"cached_tokens":%d}}}`,
			promptTokens, completionTokens, promptTokens+completionTokens, cachedTokens),
		"data: [DONE]",
		"",
	}, "\n\n")
}

// TestOAE1_AStreamedChatCompletionIsRecordedThroughTheProxy drives the whole
// path: client -> proxy -> upstream SSE -> tap -> ledger.
func TestOAE1_AStreamedChatCompletionIsRecordedThroughTheProxy(t *testing.T) {
	const (
		wantPrompt     = 4242
		wantCached     = 4000
		wantCompletion = 77
	)
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != chatCompletionsPath {
			t.Errorf("upstream saw %q, want %q", r.URL.Path, chatCompletionsPath)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(openaiSSE(wantPrompt, wantCached, wantCompletion))); err != nil {
			t.Error(err)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	base, dir, _ := startProxy(t, up, "")

	body := `{"model":"gpt-4o","stream":true,` +
		`"stream_options":{"include_usage":true},` +
		`"messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(base+chatCompletionsPath, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}

	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("the proxy forwarded a chat completion and wrote no ledger record")
	}
	got := recs[0].Response.Usage
	if got == nil {
		t.Fatal("the record carries no usage at all: the response was read by a parser " +
			"that found none, and the request will price as free")
	}

	// The whole point: the OpenAI parser ran. The Anthropic parser does not
	// recognise this frame and would leave every count at zero, which is why
	// a zero here is the failure rather than a smaller number.
	if got.PromptTotal() == 0 && got.Output == 0 {
		t.Fatalf("the response was read by a parser that found no usage in it: %+v.\n"+
			"      An OpenAI stream parsed as an Anthropic one records nothing, and a "+
			"request that recorded nothing prices as free", got)
	}
	if got.Output != wantCompletion {
		t.Errorf("completion tokens = %d, want %d", got.Output, wantCompletion)
	}
	if got.CacheRead != wantCached {
		t.Errorf("cached tokens = %d, want %d: the cached share is what this tool "+
			"exists to report, and on this surface it arrives nested in "+
			"prompt_tokens_details", got.CacheRead, wantCached)
	}
}

// TestOAE2_TheParserIsChosenByTheRouteNotTheContentType pins the branch.
//
// Both surfaces answer with text/event-stream, so the content type cannot
// choose the parser — the route does. Without this, `if t.openai` can be
// forced either way: forced true, the Anthropic path breaks; forced false,
// this test does.
func TestOAE2_TheParserIsChosenByTheRouteNotTheContentType(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(openaiSSE(1000, 900, 5)))
	})
	base, dir, _ := startProxy(t, up, "")

	body := `{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(base+chatCompletionsPath, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}

	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record")
	}
	if recs[0].Response.Usage == nil {
		t.Fatal("no usage recorded on an OpenAI route")
	}
	if recs[0].Response.Usage.CacheRead != 900 {
		t.Errorf("cached tokens = %d, want 900; the OpenAI stream parser did not run "+
			"on an OpenAI route", recs[0].Response.Usage.CacheRead)
	}
}
