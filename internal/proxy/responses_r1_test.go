package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/masking"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// withOpenAIRules installs the dated OpenAI rules document for one test, as
// `replay rules --update docs/rules/openai-2026-09-15.json` does for an
// operator. The compiled table carries Anthropic rows only, so without it an
// OpenAI model is charged at the dearest known row as an upper bound and the
// cap's CapNotEnforced flag arms: existing behaviour, tested below as well.
func withOpenAIRules(t *testing.T) {
	t.Helper()
	r, err := cachemodel.LoadRules(filepath.Join("..", "..", "docs", "rules", "openai-2026-09-15.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cachemodel.Override(r))
}

// R-1: the Responses API read through the proxy.
//
// Before this change /v1/responses was forwarded unread: a Codex CLI user
// pointed at the proxy saw traffic flow and got no ledger record, no cap and
// no figure. The transport probe of 2026-10-04 showed Codex reaches the proxy
// over ordinary HTTP with SSE, so the path can be read the way the other two
// are. These tests drive the whole production chain, client to proxy to a
// fake upstream to tap to ledger to guards, on the wire fixtures in
// internal/ledger/testdata/openai-responses. Written RED against the build
// that forwarded the path unread.

// responsesFixture reads one wire fixture from the ledger package's testdata,
// so the proxy tests and the parser tests read the same bytes.
func responsesFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "ledger", "testdata", "openai-responses", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fixedUpstream answers every request with one body, counting hits.
type fixedUpstream struct {
	contentType string
	body        []byte
	gzip        bool
	hits        atomic.Int64
	mu          sync.Mutex
	paths       []string
}

func (u *fixedUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	u.hits.Add(1)
	u.mu.Lock()
	u.paths = append(u.paths, r.URL.Path)
	u.mu.Unlock()
	w.Header().Set("Content-Type", u.contentType)
	if u.gzip {
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(u.body)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func jsonResponsesUpstream(t *testing.T, fixture string) *fixedUpstream {
	t.Helper()
	return &fixedUpstream{contentType: "application/json", body: responsesFixture(t, fixture)}
}

func sseResponsesUpstream(t *testing.T, fixture string) *fixedUpstream {
	t.Helper()
	return &fixedUpstream{contentType: "text/event-stream", body: responsesFixture(t, fixture)}
}

// postResponsesBody posts one Responses request and returns the status and
// the bytes the client received.
func postResponsesBody(t *testing.T, base string, body []byte) (int, []byte) {
	t.Helper()
	resp, err := http.Post(base+responsesPath, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, got
}

// withCacheKey returns the Codex-shaped request with its prompt_cache_key
// replaced, or removed when key is empty.
func withCacheKey(t *testing.T, key string) []byte {
	t.Helper()
	var req map[string]json.RawMessage
	if err := json.Unmarshal(responsesFixture(t, "request-codex.json"), &req); err != nil {
		t.Fatal(err)
	}
	if key == "" {
		delete(req, "prompt_cache_key")
	} else {
		req["prompt_cache_key"] = json.RawMessage(`"` + key + `"`)
	}
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// withUserText returns the Codex-shaped request with a different first
// message, so two requests can share a cache key and nothing else.
func withUserText(t *testing.T, body []byte, text string) []byte {
	t.Helper()
	return bytes.Replace(body, []byte(`"text":"list the files"`), []byte(`"text":"`+text+`"`), 1)
}

func wantUsage(t *testing.T, got *transcript.Usage, want transcript.Usage) {
	t.Helper()
	if got == nil {
		t.Fatal("the record carries no usage at all, so the request prices as free")
	}
	if got.Input != want.Input || got.CacheRead != want.CacheRead || got.CacheCreation != want.CacheCreation ||
		got.Output != want.Output || got.ThinkingTokens != want.ThinkingTokens {
		t.Errorf("usage = %+v, want %+v", *got, want)
	}
}

// The non-streaming reply, through the proxy to the ledger, with the
// provider's inclusive input split into uncached and cached.
func TestR1_ANonStreamingResponsesReplyIsLedgeredWithNormalisedUsage(t *testing.T) {
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, dir, _ := startProxy(t, up, "")

	status, _ := postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("the proxy forwarded a Responses request and wrote no ledger record")
	}
	rec := recs[0]
	if rec.Path != responsesPath {
		t.Errorf("path = %q", rec.Path)
	}
	if rec.Model != "gpt-6-astra" {
		t.Errorf("model = %q, want the request's model", rec.Model)
	}
	if rec.SessionID == "" {
		t.Error("no session identity; the ledger drops records without one")
	}
	wantUsage(t, rec.Response.Usage, transcript.Usage{Input: 500, CacheRead: 1000, Output: 300, ThinkingTokens: 200})
	if !strings.Contains(string(rec.Response.RawUsage), `"cached_tokens":1000`) {
		t.Errorf("raw usage not kept verbatim: %s", rec.Response.RawUsage)
	}
	if rec.Prompt.SystemBytes != len("You are Codex, running in a sandbox.") {
		t.Errorf("instructions not counted as the system prefix: %d", rec.Prompt.SystemBytes)
	}
	var tool, text, thinking bool
	for _, b := range rec.Response.Blocks {
		switch b.Kind {
		case transcript.KindToolUse:
			tool = b.ToolName == "shell"
		case transcript.KindText:
			text = b.Bytes == len("Running the listing now.")
		case transcript.KindThinking:
			thinking = b.Bytes > 0
		}
	}
	if !tool || !text || !thinking {
		t.Errorf("response structure not reduced (tool=%v text=%v thinking=%v): %+v", tool, text, thinking, rec.Response.Blocks)
	}
}

// The streamed reply. Usage sits on the terminal event, text and reasoning
// arrive as deltas, the tool call as a finished item. The bytes reach the
// client unchanged.
func TestR1_AStreamedResponsesReplyIsLedgeredFromTheCompletedEvent(t *testing.T) {
	up := sseResponsesUpstream(t, "stream-completed.sse")
	base, dir, _ := startProxy(t, up, "")

	status, got := postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !bytes.Equal(got, up.body) {
		t.Errorf("the stream was not delivered byte for byte:\n%s", got)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record for a streamed Responses reply")
	}
	rec := recs[0]
	wantUsage(t, rec.Response.Usage, transcript.Usage{Input: 500, CacheRead: 1000, Output: 300, ThinkingTokens: 200})
	var textBytes, thinkingBytes int
	var tool string
	for _, b := range rec.Response.Blocks {
		switch b.Kind {
		case transcript.KindText:
			textBytes += b.Bytes
		case transcript.KindThinking:
			thinkingBytes += b.Bytes
		case transcript.KindToolUse:
			tool = b.ToolName
		}
	}
	if textBytes != len("Running the listing now.") || thinkingBytes != len("Weighing the two options.") || tool != "shell" {
		t.Errorf("stream structure: text=%d thinking=%d tool=%q", textBytes, thinkingBytes, tool)
	}
}

// response.incomplete carries usage too, and Codex's own parser reads it
// there; a turn cut at max_output_tokens was still billed.
func TestR1_AnIncompleteStreamStillCarriesItsUsage(t *testing.T) {
	up := sseResponsesUpstream(t, "stream-incomplete.sse")
	base, dir, _ := startProxy(t, up, "")
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record")
	}
	wantUsage(t, recs[0].Response.Usage, transcript.Usage{Input: 1500, Output: 64})
}

// A stream that never reached its terminal event has no usage. That is the
// QT-7a state, counted under the existing key, and not the QT-7c one.
func TestR1_AStreamCutBeforeItsTerminalEventRecordsNoUsage(t *testing.T) {
	up := sseResponsesUpstream(t, "stream-cut.sse")
	base, dir, _ := startProxy(t, up, "")
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record")
	}
	if recs[0].Response.Usage != nil {
		t.Errorf("a cut stream recorded usage %+v; nothing was reported", *recs[0].Response.Usage)
	}
	raw := statusRaw(t, base)
	if got := raw["responses_without_usage"]; got != float64(1) {
		t.Errorf("responses_without_usage = %v, want 1", got)
	}
	if got := raw["responses_unparsed"]; got != float64(0) {
		t.Errorf("responses_unparsed = %v, want 0: a readable stream with no usage is not an unreadable body", got)
	}
}

// "usage": {} is the absence of a measurement, as it is on chat completions.
func TestR1_AnEmptyUsageObjectIsAbsenceNotZero(t *testing.T) {
	up := jsonResponsesUpstream(t, "nonstream-empty-usage.json")
	base, dir, _ := startProxy(t, up, "")
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record")
	}
	if recs[0].Response.Usage != nil {
		t.Errorf("an empty usage object was recorded as a measured %+v", *recs[0].Response.Usage)
	}
	if recs[0].Response.Unparsed {
		t.Error("a reply that parsed and carried an empty usage was marked unparsed")
	}
	raw := statusRaw(t, base)
	if got := raw["responses_without_usage"]; got != float64(1) {
		t.Errorf("responses_without_usage = %v, want 1", got)
	}
}

// QT-7c holds on this path: a body nobody could read is told from a message
// that carried no usage.
func TestR1_AnUnreadableResponsesBodyIsUnparsedNotWithoutUsage(t *testing.T) {
	up := &fixedUpstream{contentType: "application/json", body: []byte("<html>upstream proxy error</html>")}
	base, dir, _ := startProxy(t, up, "")
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	up.mu.Lock()
	up.body = responsesFixture(t, "nonstream-error-200.json")
	up.mu.Unlock()
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	recs := waitLedger(t, dir, 2)
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	for i, rec := range recs {
		if !rec.Response.Unparsed {
			t.Errorf("record %d: an unreadable body is not marked unparsed: %+v", i, rec.Response)
		}
	}
	raw := statusRaw(t, base)
	if got := raw["responses_unparsed"]; got != float64(2) {
		t.Errorf("responses_unparsed = %v, want 2", got)
	}
	if got := raw["responses_without_usage"]; got != float64(0) {
		t.Errorf("responses_without_usage = %v, want 0", got)
	}
}

// The spend cap. gpt-6-astra is priced in the rules document at $10 per
// million input and $50 per million output, with reads at a tenth: the
// fixture's 500 uncached, 1,000 cached and 300 output tokens cost $0.021.
// Under a $0.05 session cap the fourth request in the session is refused,
// so the usage read off this path reached the guard that spends it.
func TestR1_TheSpendCapSeesResponsesUsage(t *testing.T) {
	withOpenAIRules(t)
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, dir, _ := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{SessionUSD: 0.05})})
	body := responsesFixture(t, "request-codex.json")
	for i := 1; i <= 3; i++ {
		if status, _ := postResponsesBody(t, base, body); status != http.StatusOK {
			t.Fatalf("request %d: status %d before the cap", i, status)
		}
		waitLedger(t, dir, i)
	}
	status, got := postResponsesBody(t, base, body)
	if status != http.StatusBadRequest || !strings.Contains(string(got), "replay_spend_cap") {
		t.Fatalf("the fourth request under a $0.05 cap was not refused: %d %s", status, got)
	}
	if up.hits.Load() != 3 {
		t.Errorf("upstream hits = %d, want 3: the refused request must not be forwarded", up.hits.Load())
	}
	raw := statusRaw(t, base)
	if cost, _ := raw["cost_usd"].(float64); cost < 0.06 || cost > 0.07 {
		t.Errorf("cost_usd = %v, want three requests at $0.021", raw["cost_usd"])
	}
	if raw["spend_cap_not_enforced"] == true {
		t.Error("the cap is flagged as an over-estimate while every request was priced")
	}
}

// Without the OpenAI rules document installed, the compiled table cannot
// price gpt-6-astra. The existing rule (QT-7b) charges it at the dearest
// known row as an UPPER BOUND, so the cap still fires, early rather than
// never, and the status says the total is a bound. R-1 changes none of
// that; this pins that the Responses path inherits it rather than falling
// back to free.
func TestR1_AnUnpricedResponsesModelIsBoundedNotFree(t *testing.T) {
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, dir, _ := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{SessionUSD: 0.05})})
	body := responsesFixture(t, "request-codex.json")
	refused := 0
	for i := 1; i <= 6; i++ {
		status, got := postResponsesBody(t, base, body)
		if status == http.StatusBadRequest && strings.Contains(string(got), "replay_spend_cap") {
			refused++
			break
		}
		waitLedger(t, dir, i)
	}
	if refused == 0 {
		t.Fatal("six requests on an unpriced model never reached a $0.05 cap: the path fell back to free")
	}
	raw := statusRaw(t, base)
	if raw["spend_cap_not_enforced"] != true {
		t.Errorf("the status does not say the cap total is an upper bound: %v", raw["spend_cap_not_enforced"])
	}
}

// Session identity. The Responses protocol carries the client's own
// prompt_cache_key, which Codex sets to its conversation id; the proxy keys
// the session on it when present and falls back to the structural hash the
// chat-completions path uses when absent. Never the raw key: a hash of it.
func TestR1_SessionIdentityIsThePromptCacheKey(t *testing.T) {
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, dir, _ := startProxy(t, up, "")
	const key = "019a2c6e-7f3b-7c1d-9d2e-4b5f6a7b8c9d"
	postResponsesBody(t, base, withCacheKey(t, key))
	postResponsesBody(t, base, withUserText(t, withCacheKey(t, key), "now delete them"))
	postResponsesBody(t, base, withCacheKey(t, "0199ffff-0000-7000-8000-000000000000"))
	postResponsesBody(t, base, withCacheKey(t, ""))
	recs := waitLedger(t, dir, 4)
	if len(recs) != 4 {
		t.Fatalf("records = %d, want 4", len(recs))
	}
	byTS := map[string]int{}
	for _, r := range recs {
		byTS[r.SessionID]++
		if r.SessionID == "" {
			t.Error("a record with no session identity")
		}
		if strings.Contains(r.SessionID, key) {
			t.Errorf("the raw prompt_cache_key is written to the ledger as the session id: %q", r.SessionID)
		}
	}
	if len(byTS) != 3 {
		t.Errorf("sessions = %d, want 3 (two requests sharing a key, one with another key, one with none): %v", len(byTS), byTS)
	}
}

// The loop guard reads the summarised prompt. Three identical function_call
// items in a row on the Responses shape must trip a block of 3.
func TestR1_TheLoopGuardSeesRepeatedResponsesToolCalls(t *testing.T) {
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, _, _ := startProxyWith(t, up, Config{Loops: LoopLimits{Block: 3}})
	call := `{"type":"function_call","call_id":"call_%d","name":"shell","arguments":"{\"command\":[\"ls\",\"-la\"]}"},{"type":"function_call_output","call_id":"call_%d","output":"total 0"}`
	var items []string
	items = append(items, `{"type":"message","role":"user","content":[{"type":"input_text","text":"list"}]}`)
	for i := 0; i < 3; i++ {
		items = append(items, strings.ReplaceAll(call, "%d", string(rune('1'+i))))
	}
	body := `{"model":"gpt-6-astra","instructions":"x","input":[` + strings.Join(items, ",") + `],"prompt_cache_key":"k1"}`
	status, got := postResponsesBody(t, base, []byte(body))
	if status != http.StatusBadRequest || !strings.Contains(string(got), "replay_loop") {
		t.Fatalf("three identical tool calls were not stopped: %d %s", status, got)
	}
	if up.hits.Load() != 0 {
		t.Errorf("the looping request reached the provider")
	}
}

// Cache-break classification is NOT MEASURED on this path. OpenAI's cache is
// addressed by the client's prompt_cache_key and reported as cached_tokens;
// nothing on the wire establishes that the previous prompt total is the
// expected read, which is the Messages rule. So no outcome is written, no
// break is counted and no cause is named, while the cost is still tallied.
func TestR1_NoCacheBreakIsClassifiedOnTheResponsesPath(t *testing.T) {
	withOpenAIRules(t)
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, dir, logs := startProxy(t, up, "")
	body := responsesFixture(t, "request-codex.json")
	postResponsesBody(t, base, body)
	waitLedger(t, dir, 1)
	up.mu.Lock()
	up.body = responsesFixture(t, "nonstream-cache-write.json") // reads nothing
	up.mu.Unlock()
	postResponsesBody(t, base, body)
	recs := waitLedger(t, dir, 2)
	if len(recs) != 2 {
		t.Fatalf("records = %d", len(recs))
	}
	for i, r := range recs {
		if r.Cache != nil {
			t.Errorf("record %d carries a cache classification %+v on a path where none is measured", i, *r.Cache)
		}
	}
	if strings.Contains(logs.String(), "cache break") {
		t.Errorf("a cache break was named on the Responses path:\n%s", logs.String())
	}
	raw := statusRaw(t, base)
	if cost, _ := raw["cost_usd"].(float64); cost <= 0 {
		t.Errorf("cost_usd = %v: the two requests were not priced", raw["cost_usd"])
	}
}

// Masking, after the path is read. RES1 proves the credential does not
// reach the provider; this proves it reaches neither the ledger file nor the
// log once the request is summarised and recorded.
func TestR1_ASecretOnTheResponsesPathReachesNeitherLedgerNorLog(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, dir, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})
	const canary = "sk-ant-api03-CanaryCanaryCanaryCanaryCanary0123456789"
	body := withUserText(t, responsesFixture(t, "request-codex.json"), "my key is "+canary)
	postResponsesBody(t, base, body)
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record")
	}
	masked := 0
	for _, n := range recs[0].Masked {
		masked += n
	}
	if masked == 0 {
		t.Error("the record does not say a secret was masked")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if bytes.Contains(b, []byte(canary)) {
			t.Fatalf("the credential is in the ledger file %s", f)
		}
	}
	if strings.Contains(logs.String(), canary) {
		t.Fatal("the credential is in the log")
	}
}

// Ordinary Go and Rust clients accept gzip. The tap skips the incremental
// parsers on a gzip stream and reparses the decoded body by route, and the
// route must choose the Responses parser there too.
func TestR1_AGzipResponsesStreamIsStillRead(t *testing.T) {
	up := &fixedUpstream{contentType: "text/event-stream", gzip: true, body: gzipBytes(t, responsesFixture(t, "stream-completed.sse"))}
	base, dir, _ := startProxy(t, up, "")
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record")
	}
	wantUsage(t, recs[0].Response.Usage, transcript.Usage{Input: 500, CacheRead: 1000, Output: 300, ThinkingTokens: 200})
}

// The disclosure. The path is read now, so the NOT PARSED line must stop,
// and a line must say what IS true of it: read, guarded, masked, and
// cache-break classification not measured. Silence on a path whose status
// just changed is the gap noteExperimentalUnmasked was written about.
func TestR1_TheResponsesPathDisclosesWhatIsAndIsNotMeasured(t *testing.T) {
	up := jsonResponsesUpstream(t, "nonstream-cached-reasoning.json")
	base, dir, logs := startProxy(t, up, "")
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	postResponsesBody(t, base, responsesFixture(t, "request-codex.json"))
	waitLedger(t, dir, 2)
	s := logs.String()
	if strings.Contains(s, "NOT PARSED "+responsesPath) {
		t.Errorf("the path is read and still announced as NOT PARSED:\n%s", s)
	}
	if n := strings.Count(s, "READ "+responsesPath); n != 1 {
		t.Errorf("the READ disclosure for %s fired %d times, want once:\n%s", responsesPath, n, s)
	}
	for _, want := range []string{"NOT MEASURED", "masked"} {
		if !strings.Contains(s, want) {
			t.Errorf("the disclosure does not say %q:\n%s", want, s)
		}
	}
	if m := metricsText(t, base); !strings.Contains(m, "replay_unparsed_requests_total 0") {
		t.Errorf("read traffic is still counted as unparsed:\n%s", m)
	}
}
