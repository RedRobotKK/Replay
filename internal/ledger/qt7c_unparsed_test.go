package ledger

import "testing"

// QT-7c at the parsers. A body that is not JSON, or on the Messages path not
// a message, is marked unparsed; a message that parsed and carried no usage
// is not, and a legal message with no content and no usage is not either.
// The mark is set only where the parser knows it could not read the body,
// never inferred from an empty result.
func TestQT7c_ParsersMarkWhatTheyCouldNotRead(t *testing.T) {
	cases := []struct {
		name     string
		got      Response
		unparsed bool
	}{
		{"messages: not JSON", ParseResponse([]byte("<html>")), true},
		{"messages: an error object", ParseResponse([]byte(`{"type":"error","error":{"type":"overloaded_error"}}`)), true},
		{"messages: cut short", ParseResponse([]byte(`{"type":"message","content":[{"type":"text","te`)), true},
		{"messages: parsed, no usage", ParseResponse([]byte(`{"type":"message","content":[{"type":"text","text":"hi"}]}`)), false},
		{"messages: parsed, no content, no usage", ParseResponse([]byte(`{"type":"message","content":[]}`)), false},
		{"messages: parsed with usage", ParseResponse([]byte(`{"type":"message","content":[],"usage":{"input_tokens":3,"output_tokens":1}}`)), false},
		{"openai: not JSON", ParseOpenAIResponse([]byte("not json")), true},
		{"openai: parsed, usage empty", ParseOpenAIResponse([]byte(`{"id":"x","choices":[],"usage":{}}`)), false},
		{"openai: parsed with usage", ParseOpenAIResponse([]byte(`{"id":"x","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":1}}`)), false},
	}
	for _, c := range cases {
		if c.got.Unparsed != c.unparsed {
			t.Errorf("%s: unparsed=%v, want %v", c.name, c.got.Unparsed, c.unparsed)
		}
		if c.unparsed && (c.got.Usage != nil || len(c.got.Blocks) != 0) {
			t.Errorf("%s: an unparsed body must carry no usage and no blocks: %+v", c.name, c.got)
		}
	}
}
