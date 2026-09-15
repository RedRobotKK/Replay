package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/RedRobotKK/Replay/internal/masking"
)

// /v1/responses is the path GPT-6 Astra speaks, and the path this proxy
// forwarded with the masker switched off.
//
// The masking gate in passthrough.go reads `if messages && ...`, so a
// credential pasted into a prompt on any other path reached the provider as
// typed. That gate was right when it was written: rewriting a body this build
// cannot read is the F2 defect the family gate exists to stop, and the comment
// above it records a POST to /v1/embeddings coming out the other side pinned.
//
// But masking is not like the other rewrites in that handler. freezeBillingHeader
// and applyPolicy need to understand the body to change the right field.
// Masker.Mask does not: it walks every JSON string value in the body and
// replaces secrets inside the literals, leaving every byte outside a match
// exactly as it was. It is shape-agnostic by construction, so the reason to
// withhold it from an unreadable path never applied to it.
//
// What does NOT change here: this build still cannot read a Responses body for
// usage, so the path stays unparsed and says so. Masked and read are different
// claims and only one of them is now true.

// respUpstream captures what actually left the proxy.
type respUpstream struct {
	mu   sync.Mutex
	body []byte
	path string
}

func (u *respUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	u.mu.Lock()
	u.body, u.path = b, r.URL.Path
	u.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"resp_1","model":"gpt-6-astra","usage":{"input_tokens":12,"input_tokens_details":{"cached_tokens":0},"output_tokens":3}}`))
}

func (u *respUpstream) seen() (string, string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return string(u.body), u.path
}

func postResponses(t *testing.T, base, body string) {
	t.Helper()
	resp, err := http.Post(base+responsesPath, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	_, _ = io.Copy(io.Discard, resp.Body)
}

// The leak, closed. A key in a Responses prompt must not reach the provider.
func TestRES1_ACredentialOnTheResponsesPathIsMaskedBeforeEgress(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := &respUpstream{}
	base, _, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})

	const canary = "sk-ant-api03-CanaryCanaryCanaryCanaryCanary0123456789"
	postResponses(t, base, `{"model":"gpt-6-astra","input":[{"role":"user","content":[{"type":"input_text","text":"my key is `+canary+`"}]}]}`)

	got, path := up.seen()
	if path != responsesPath {
		t.Fatalf("upstream saw path %q, want %q", path, responsesPath)
	}
	if strings.Contains(got, canary) {
		t.Fatalf("the credential reached the provider in clear on %s.\nbody: %s", responsesPath, got)
	}
	// The placeholder must not leak into the operator's log either.
	ph, _ := vault.Placeholder(canary, "")
	if s := logs.String(); strings.Contains(s, canary) || strings.Contains(s, ph) {
		t.Fatalf("log holds the secret or its placeholder:\n%s", s)
	}
}

// A body with nothing to mask must pass byte for byte. Masking is the only
// rewrite being extended here; if anything else starts touching this path the
// forwarded bytes stop matching what the client sent.
func TestRES2_AResponsesBodyWithNoSecretIsForwardedByteForByte(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := &respUpstream{}
	base, _, _ := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})

	const body = `{"model":"gpt-6-astra","input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]}`
	postResponses(t, base, body)

	got, _ := up.seen()
	if got != body {
		t.Fatalf("a clean Responses body was rewritten.\n sent: %s\n  got: %s", body, got)
	}
}

// The chat-completions family gets withUsageReporting, which re-encodes the
// body with json.Marshal. On a Responses body that would be a rewrite of a
// shape it does not understand, and stream_options is not even a field there.
// Responses must not be folded into that family to get masking.
func TestRES3_TheResponsesPathIsNotTreatedAsChatCompletions(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := &respUpstream{}
	base, _, _ := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})

	postResponses(t, base, `{"model":"gpt-6-astra","stream":true,"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	got, _ := up.seen()
	if strings.Contains(got, "stream_options") {
		t.Errorf("stream_options was injected into a Responses body; that is the chat-completions "+
			"rewrite applied to a shape that has no such field:\n%s", got)
	}
}

// Masked is not read. This build still cannot take usage off a Responses
// reply, so the path must keep saying it is unparsed. Silence here would be
// the mistake noteExperimentalUnmasked was written about: a gap that used to
// be announced and quietly stops being announced.
func TestRES4_TheResponsesPathStillDeclaresItselfUnparsed(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := &respUpstream{}
	base, _, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})

	postResponses(t, base, `{"model":"gpt-6-astra","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	s := logs.String()
	if !strings.Contains(s, responsesPath) {
		t.Fatalf("nothing in the log names %s, so an operator is told nothing about this traffic:\n%s", responsesPath, s)
	}
	if !bytes.Contains([]byte(s), []byte("NOT PARSED")) {
		t.Errorf("the unparsed disclosure stopped firing for %s. Usage is still not read on this "+
			"path, and a masked request is not a read one:\n%s", responsesPath, s)
	}
}

// The unparsed line said "no secret masking apply to it". Once masking runs
// on this path that clause is false, and a disclosure that overstates what is
// NOT happening misleads about credentials just as surely as one that
// overstates what is.
//
// This asserts the LOG TEXT. The first version of this test asserted the
// absence of "everything Replay offers is inert", a sentence that only ever
// existed in a source comment and never reached the log, so it passed against
// every implementation including the broken one. A mutation sweep reddened
// nothing when the old claim was restored, which is how it was found.
func TestRES5_TheUnparsedLineDoesNotDenyMaskingThatIsRunning(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := &respUpstream{}
	base, _, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})

	postResponses(t, base, `{"model":"gpt-6-astra","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	s := logs.String()
	if !strings.Contains(s, "NOT PARSED "+responsesPath) {
		t.Fatalf("no unparsed line for %s:\n%s", responsesPath, s)
	}
	if strings.Contains(s, "no secret masking apply to it") {
		t.Errorf("the disclosure denies masking on a path where the masker is running, so an "+
			"operator is told their credentials are unprotected when they are not:\n%s", s)
	}
	if !strings.Contains(s, "secrets ARE masked on this path") {
		t.Errorf("the disclosure never says masking DOES run here, so the operator cannot tell "+
			"this path from one where it does not:\n%s", s)
	}
}

// The guard against the smaller diff someone will eventually try: adding
// responsesPath to isChatCompletions to get it routed. That family is
// re-encoded by withUsageReporting, so it would rewrite a shape nothing here
// parses and inject a field the Responses API does not have.
func TestRES6_ResponsesIsNotAMemberOfTheChatCompletionsFamily(t *testing.T) {
	if isChatCompletions(responsesPath) {
		t.Fatal("isChatCompletions matches " + responsesPath + "; that family is rewritten by " +
			"withUsageReporting, which would re-encode a Responses body and add stream_options to a " +
			"shape that has no such field")
	}
	if !isResponses(responsesPath) {
		t.Error("isResponses does not match its own path")
	}
	if isResponses(chatCompletionsPath) {
		t.Error("isResponses matches the chat-completions path; the two families must stay distinct")
	}
}
