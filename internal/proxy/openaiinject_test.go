package proxy

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The usage-injection policy, exercised through the proxy rather than as a
// function call.
//
// withUsageReporting has unit tests. Its WIRING had none, and the reason is
// worth recording: every existing OpenAI end-to-end test sends a body that
// already contains "stream_options":{"include_usage":true}, so the policy
// finds nothing to change and returns changed=false. The injection branch and
// the openai summarizer selection were both reported INERT by
// guard-reachability (#238) -- they ran, and no test depended on whether they
// had.
//
// What is at stake: without the injection an OpenAI stream reports no usage at
// all, so every request on this surface prices as free and every guard
// downstream sees a request that cost nothing.
func TestOpenAIStreamGetsUsageReportingInjectedAndRecorded(t *testing.T) {
	var mu sync.Mutex
	var seen string
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = string(b)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(openaiSSE(10, 0, 3)))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	base, dir, _ := startProxy(t, up, "")

	// A client that asked for a stream and did NOT ask for usage. This is the
	// case every other test in this package skips.
	body := `{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(base+chatCompletionsPath, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	forwarded := seen
	mu.Unlock()

	// 1. The rewrite reached the provider. This is the whole purpose: asking
	//    the provider for usage the client did not ask for.
	if !strings.Contains(forwarded, `"include_usage":true`) {
		t.Fatalf("the provider received a body with no usage request:\n      %s\n"+
			"      The client did not ask for stream_options, so the proxy must. Without "+
			"it the stream carries no usage frame and every request on this surface "+
			"prices as free.", forwarded)
	}

	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record for a forwarded chat completion")
	}
	rec := recs[0]

	// 2. The record names the policy that did it. An unattributed rewrite is
	//    the thing the body-hash bracket exists to make impossible.
	if rec.Policy != "openai-include-usage" {
		t.Errorf("record Policy = %q, want %q: the body was rewritten and the record "+
			"does not say what did it", rec.Policy, "openai-include-usage")
	}

	// 3. The bracket disagrees with itself, which is the proof of a rewrite.
	if rec.BodyHashBefore == "" || rec.BodyHashAfter == "" {
		t.Fatalf("the body-hash bracket is incomplete: before=%q after=%q",
			rec.BodyHashBefore, rec.BodyHashAfter)
	}
	if rec.BodyHashBefore == rec.BodyHashAfter {
		t.Errorf("BodyHashBefore == BodyHashAfter == %q for a request the proxy rewrote. "+
			"Equal hashes are the claim that the request went out as it came in, and "+
			"here that claim is false", rec.BodyHashBefore)
	}

	// 4. The OpenAI summarizer ran, not the Messages one.
	//
	//    Asserting Model == "gpt-4o" does NOT establish this and the first
	//    version of this test wrongly said it did: both body shapes carry a
	//    top-level "model", so the Anthropic summarizer reads it too and the
	//    assertion passed with the selection neutralised. The two summarizers
	//    are distinguished by what they make of the rest -- they disagree on
	//    system_bytes and therefore on PrefixHash -- so the record is compared
	//    against both, and must match the OpenAI one and not the Anthropic one.
	wantOAI, err := ledger.SummarizeOpenAIRequest([]byte(body), nil)
	if err != nil {
		t.Fatal(err)
	}
	wantAnthropic, err := ledger.SummarizeRequest([]byte(body), nil)
	if err != nil {
		t.Fatal(err)
	}
	if wantOAI.PrefixHash == wantAnthropic.PrefixHash {
		t.Fatal("both summarizers produce the same prefix hash for this body, so this " +
			"check cannot tell them apart; it needs a body on which they differ")
	}
	if rec.PrefixHash == wantAnthropic.PrefixHash {
		t.Errorf("the request was summarised by the Messages parser (prefix hash %q). "+
			"An OpenAI body read by the Anthropic summarizer produces a prefix hash "+
			"that no OpenAI request will ever match, so cache-break attribution on "+
			"this surface silently compares against the wrong baseline",
			rec.PrefixHash)
	}
	if rec.PrefixHash != wantOAI.PrefixHash {
		t.Errorf("RequestSummary.PrefixHash = %q, want %q from SummarizeOpenAIRequest",
			rec.PrefixHash, wantOAI.PrefixHash)
	}
	if !rec.Stream {
		t.Error("RequestSummary.Stream is false for a request that set stream:true")
	}
}

// A request the policy has nothing to add to is left exactly as it arrived.
//
// This is the other half of the `changed` guard. Without it a test could pass
// by rewriting every body unconditionally, and the bracket would report a
// rewrite on requests the proxy did not touch.
func TestOpenAIRequestThatAlreadyAsksForUsageIsNotRewritten(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(openaiSSE(10, 0, 3)))
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
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}

	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record")
	}
	rec := recs[0]
	if rec.Policy == "openai-include-usage" {
		t.Error("the record claims an injection policy ran on a request that already " +
			"carried stream_options; nothing needed changing")
	}
	if rec.BodyHashBefore != rec.BodyHashAfter {
		t.Errorf("BodyHashBefore %q != BodyHashAfter %q for a request the proxy had no "+
			"reason to touch", rec.BodyHashBefore, rec.BodyHashAfter)
	}
}
