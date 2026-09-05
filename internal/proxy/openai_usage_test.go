package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

// This provider sends no usage on a stream unless the client asked for it with
// stream_options.include_usage. Cursor does not ask. So without this the spend
// cap, the error budget and every cost figure see nothing on the only traffic
// shape real clients actually send.
//
// Setting it is admissible under ADR-0003's first kind: a request parameter the
// client left unset. It is not a policy choice and it changes no output, it
// only asks the provider to report what it already billed.
func TestIncludeUsageIsAddedWhenTheClientLeftItUnset(t *testing.T) {
	body := []byte(`{"model":"gpt-x","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	out, changed := withUsageReporting(body)
	if !changed {
		t.Fatal("a streaming request with no stream_options must be asked to report usage")
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	so, ok := got["stream_options"].(map[string]any)
	if !ok || so["include_usage"] != true {
		t.Fatalf("stream_options.include_usage not set: %s", out)
	}
	if got["model"] != "gpt-x" || got["stream"] != true {
		t.Fatalf("the rest of the request was disturbed: %s", out)
	}
}

// A client that set it keeps its own value. ADR-0003 never overrides a
// parameter the client set, whatever it set it to.
func TestAClientsOwnStreamOptionsAreNeverOverridden(t *testing.T) {
	body := []byte(`{"model":"gpt-x","stream":true,"stream_options":{"include_usage":false},"messages":[]}`)
	out, changed := withUsageReporting(body)
	if changed {
		t.Fatal("the client set stream_options and it must be left alone")
	}
	if !strings.Contains(string(out), `"include_usage":false`) {
		t.Fatalf("the client's value was altered: %s", out)
	}
}

// A non-streaming request already reports usage and must not be touched.
func TestANonStreamingRequestIsLeftAlone(t *testing.T) {
	body := []byte(`{"model":"gpt-x","messages":[]}`)
	if _, changed := withUsageReporting(body); changed {
		t.Fatal("a non-streaming request needs no stream_options")
	}
}

// Anything unparseable is forwarded exactly as it arrived. G5: fail open.
func TestGarbageIsForwardedUnchanged(t *testing.T) {
	body := []byte(`not json at all`)
	out, changed := withUsageReporting(body)
	if changed || string(out) != string(body) {
		t.Fatalf("a body this build cannot read must pass through byte for byte: %s", out)
	}
}
