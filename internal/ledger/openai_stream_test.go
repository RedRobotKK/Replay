package ledger

import "testing"

// Cursor streams. So does every other client of this family by default, which
// means a chat/completions path that only reads whole-body responses reads
// nothing at all in practice.
//
// OpenAI's SSE is not Anthropic's: no event: lines, no message_start, just
// data: frames carrying choice deltas, and a final frame carrying usage.
func TestOpenAIStreamParserReadsUsageFromTheFinalFrame(t *testing.T) {
	sse := "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"he\"}}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"llo\"}}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[],\"usage\":{\"prompt_tokens\":150,\"completion_tokens\":20,\"prompt_tokens_details\":{\"cached_tokens\":50}}}\n\n" +
		"data: [DONE]\n\n"

	p := &OpenAIStreamParser{}
	if _, err := p.Write([]byte(sse)); err != nil {
		t.Fatal(err)
	}
	got := p.Result()
	if got.Usage == nil {
		t.Fatal("usage in the final frame was not read, so streamed traffic is unmeasured")
	}
	if got.Usage.Input != 100 || got.Usage.CacheRead != 50 {
		t.Fatalf("inclusive counting must be converted in the stream path too: %+v", got.Usage)
	}
	if got.Usage.PromptTotal() != 150 {
		t.Fatalf("prompt total %d", got.Usage.PromptTotal())
	}
	if len(got.RawUsage) == 0 {
		t.Fatal("raw usage dropped in the stream path")
	}
}

// Split across writes, because that is how it arrives off a socket.
func TestOpenAIStreamParserSurvivesFrameSplitting(t *testing.T) {
	full := "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":30,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n"
	p := &OpenAIStreamParser{}
	for i := 0; i < len(full); i += 7 {
		end := i + 7
		if end > len(full) {
			end = len(full)
		}
		if _, err := p.Write([]byte(full[i:end])); err != nil {
			t.Fatal(err)
		}
	}
	if got := p.Result(); got.Usage == nil || got.Usage.Input != 30 {
		t.Fatalf("a usage frame split across writes was lost: %+v", got.Usage)
	}
}

// The dangerous case. This provider sends NO usage on a stream unless the
// client set stream_options.include_usage. Reporting zero would tell the spend
// cap the request was free, which is the silent-failure shape this project
// keeps finding. Absence must stay absent.
func TestAStreamWithNoUsageReportsNoneNotZero(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"
	p := &OpenAIStreamParser{}
	_, _ = p.Write([]byte(sse))
	if got := p.Result(); got.Usage != nil {
		t.Fatalf("no usage was sent, so none may be reported: %+v", got.Usage)
	}
}
