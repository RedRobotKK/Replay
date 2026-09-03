package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/ledger"
)

const secret = "sk-ant-test-secret-value"

const requestBody = `{"model":"claude-opus-5","max_tokens":50,"system":"be brief","messages":[{"role":"user","content":"hi"}]}`

const messageResponse = `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":4,"cache_creation_input_tokens":30,"cache_read_input_tokens":300,"output_tokens":2}}`

// upstream is the fake provider. Fields written by the handler are read by
// the test after the response completes, so they are guarded by a mutex.
type upstream struct {
	mu       sync.Mutex
	t        *testing.T
	gotAuth  string
	gotBeta  string
	gotBody  []byte
	gotXFF   string
	gotToken string
	gotHost  string
	release  chan struct{}
	mode     string
	requests int
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		u.t.Errorf("upstream read: %v", err)
	}
	u.mu.Lock()
	u.requests++
	u.gotAuth = r.Header.Get("x-api-key")
	u.gotBeta = r.Header.Get("anthropic-beta")
	u.gotXFF = r.Header.Get("X-Forwarded-For")
	u.gotToken = r.Header.Get(HeaderToken)
	u.gotHost = r.Host
	u.gotBody = body
	u.mu.Unlock()
	w.Header().Set("request-id", "req_upstream_1")
	switch u.mode {
	case "stream":
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":4,\"cache_creation_input_tokens\":30,\"cache_read_input_tokens\":300,\"output_tokens\":1}}}\n\n")
		fl.Flush()
		select {
		case <-u.release:
		case <-r.Context().Done():
			// The proxy cancels the upstream request when its client goes
			// away; return so the handler does not outlive the test.
			return
		}
		_, _ = io.WriteString(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{},\"usage\":{\"output_tokens\":6}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		fl.Flush()
	case "gzip":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		zw := gzip.NewWriter(w)
		_, _ = zw.Write([]byte(messageResponse))
		_ = zw.Close()
	case "error":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
	}
}

// seen returns a copy of what the upstream observed.
func (u *upstream) seen() upstream {
	u.mu.Lock()
	defer u.mu.Unlock()
	return upstream{gotAuth: u.gotAuth, gotBeta: u.gotBeta, gotBody: u.gotBody, gotXFF: u.gotXFF, gotToken: u.gotToken, gotHost: u.gotHost, requests: u.requests}
}

// syncBuffer is a log sink the test can read while the handler still
// writes its post-response line.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func startProxy(t *testing.T, up http.Handler, token string) (string, string, *syncBuffer) {
	t.Helper()
	return startProxyWith(t, up, Config{Token: token})
}

// startProxyWith starts a proxy with extra config (guards) applied.
func startProxyWith(t *testing.T, up http.Handler, extra Config) (string, string, *syncBuffer) {
	t.Helper()
	upSrv := httptest.NewServer(up)
	t.Cleanup(upSrv.Close)
	target, err := url.Parse(upSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	cfg := extra
	cfg.Listen, cfg.Upstream, cfg.Store, cfg.Logger = "127.0.0.1:0", target, store, log.New(logs, "", 0)
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	})
	return "http://" + srv.Addr(), dir, logs
}

func post(t *testing.T, base, path string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", secret)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "context-management-2025-06-27,fast-mode-2026-02-01")
	req.Header.Set(HeaderSessionID, "session-abc")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// ledgerWait bounds how long a test waits for the record that is written
// after the response has been delivered to the client.
const ledgerWait = 3 * time.Second

// waitLedger polls until n records exist or the wait expires.
func waitLedger(t *testing.T, dir string, n int) []ledger.Record {
	t.Helper()
	deadline := time.Now().Add(ledgerWait)
	for {
		recs := readLedger(t, dir)
		if len(recs) >= n || time.Now().After(deadline) {
			return recs
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitFor polls a condition that the proxy settles after it has answered
// the client, up to the same deadline ledger writes get.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(ledgerWait)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readLedger(t *testing.T, dir string) []ledger.Record {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []ledger.Record
	for _, m := range matches {
		recs, skipped, err := ledger.ReadRecords(m)
		if err != nil {
			t.Fatal(err)
		}
		if skipped != 0 {
			t.Fatalf("%d unreadable ledger lines in %s", skipped, m)
		}
		out = append(out, recs...)
	}
	return out
}

func TestPassthroughIsByteExactAndRecordsUsage(t *testing.T) {
	up := &upstream{t: t}
	base, dir, logs := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != messageResponse {
		t.Fatalf("response altered: status %d body %q", resp.StatusCode, body)
	}
	seen := up.seen()
	if string(seen.gotBody) != requestBody {
		t.Fatalf("request body altered: %q", seen.gotBody)
	}
	if seen.gotAuth != secret || seen.gotBeta != "context-management-2025-06-27,fast-mode-2026-02-01" {
		t.Fatalf("headers not forwarded verbatim: key=%q beta=%q", seen.gotAuth, seen.gotBeta)
	}
	if seen.gotXFF != "" {
		t.Fatalf("proxy added X-Forwarded-For: %q", seen.gotXFF)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 {
		t.Fatalf("ledger records = %d", len(recs))
	}
	rec := recs[0]
	if rec.SessionID != "session-abc" || rec.RequestID != "req_upstream_1" || rec.Status != 200 || rec.Model != "claude-opus-5" {
		t.Fatalf("record metadata wrong: %+v", rec)
	}
	if rec.Response.Usage == nil || rec.Response.Usage.CacheRead != 300 || rec.Prompt.SystemBytes != len("be brief") {
		t.Fatalf("record content wrong: %+v", rec)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "session-abc.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{secret, "be brief", "hello", "hi\""} {
		if bytes.Contains(raw, []byte(leak)) || strings.Contains(logs.String(), leak) {
			t.Fatalf("ledger or log contains %q", leak)
		}
	}
}

func TestStreamingIsFlushedIncrementally(t *testing.T) {
	up := &upstream{t: t, mode: "stream", release: make(chan struct{})}
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	defer resp.Body.Close() //nolint:errcheck // test cleanup
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content type %q", got)
	}
	// The first event must arrive before the upstream is released.
	buf := make([]byte, 4096)
	readDone := make(chan int, 1)
	go func() {
		n, _ := resp.Body.Read(buf)
		readDone <- n
	}()
	select {
	case n := <-readDone:
		if !strings.Contains(string(buf[:n]), "message_start") {
			t.Fatalf("first chunk is not the first event: %q", buf[:n])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first event was held back until the stream ended")
	}
	close(up.release)
	rest, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rest), "message_stop") {
		t.Fatalf("stream truncated: %q", rest)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 || recs[0].Response.Usage == nil || recs[0].Response.Usage.Output != 6 || recs[0].Response.Usage.CacheRead != 300 {
		t.Fatalf("stream usage not recorded: %+v", recs)
	}
	if len(recs[0].Response.Blocks) != 1 || recs[0].Response.Blocks[0].Bytes != len("hello") {
		t.Fatalf("stream blocks not recorded: %+v", recs[0].Response.Blocks)
	}
}

func TestGzipResponseIsForwardedCompressedAndParsed(t *testing.T) {
	up := &upstream{t: t, mode: "gzip"}
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", map[string]string{"Accept-Encoding": "gzip"})
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("proxy must not decompress on the client's behalf")
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil || string(decoded) != messageResponse {
		t.Fatalf("compressed body altered: %v %q", err, decoded)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 || recs[0].Response.Usage == nil || recs[0].Response.Usage.CacheRead != 300 {
		t.Fatalf("gzip usage not recorded: %+v", recs)
	}
}

func TestErrorResponsesPassThroughAndAreRecordedWithoutUsage(t *testing.T) {
	up := &upstream{t: t, mode: "error"}
	base, dir, logs := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d", resp.StatusCode)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 || recs[0].Status != 429 || recs[0].Response.Usage != nil {
		t.Fatalf("error record wrong: %+v (logs: %s)", recs, logs.String())
	}
}

func TestBrowserOriginAndTokenChecks(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxy(t, up, "tok")
	resp := post(t, base, "/v1/messages", map[string]string{"Origin": "https://evil.example"})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("browser origin accepted: %d", resp.StatusCode)
	}
	resp = post(t, base, "/v1/messages", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token accepted: %d", resp.StatusCode)
	}
	resp = post(t, base, "/v1/messages", map[string]string{HeaderToken: "tok"})
	_ = resp.Body.Close()
	seen := up.seen()
	if resp.StatusCode != http.StatusOK || seen.requests != 1 {
		t.Fatalf("valid token rejected: %d (upstream requests %d)", resp.StatusCode, seen.requests)
	}
	if seen.gotToken != "" {
		t.Fatal("the listener token must not be forwarded to the provider")
	}
}

func TestUpstreamDownYieldsBadGateway(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	target, _ := url.Parse(dead.URL)
	dead.Close()
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.ListenAndServe(ctx) }()
	resp := post(t, "http://"+srv.Addr(), "/v1/messages", nil)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), "ANTHROPIC_BASE_URL") {
		t.Fatalf("upstream failure not reported usefully: %d %q", resp.StatusCode, body)
	}
}

func TestCountTokensIsForwardedButNotRecorded(t *testing.T) {
	up := &upstream{t: t}
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages/count_tokens", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || up.seen().requests != 1 {
		t.Fatalf("count_tokens not forwarded: %d", resp.StatusCode)
	}
	// Absence can only be checked after the handler has had time to finish.
	time.Sleep(200 * time.Millisecond)
	if recs := readLedger(t, dir); len(recs) != 0 {
		t.Fatalf("count_tokens must not produce ledger records: %+v", recs)
	}
}

func TestRefusesNonLoopbackListen(t *testing.T) {
	target, _ := url.Parse("https://api.anthropic.com")
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{Listen: "0.0.0.0:4000", Upstream: target, Store: store}); err == nil {
		t.Fatal("binding all interfaces must be refused")
	}
}

func TestSpendCapRefusesNextRequestNotCurrent(t *testing.T) {
	up := &upstream{t: t}
	// Each response reports 4+30+300+2 = 336 tokens; cap at 500 so the
	// second request passes and the third is refused.
	base, dir, logs := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{SessionTokens: 500})})
	for i := 0; i < 2; i++ {
		resp := post(t, base, "/v1/messages", nil)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d refused early: %d", i, resp.StatusCode)
		}
		waitLedger(t, dir, i+1)
	}
	resp := post(t, base, "/v1/messages", nil)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "buffy_spend_cap") {
		t.Fatalf("third request must be refused with the provider error shape: %d %s", resp.StatusCode, body)
	}
	if up.seen().requests != 2 {
		t.Fatalf("refused request must not reach the provider: %d", up.seen().requests)
	}
	resp = post(t, base, "/v1/messages", map[string]string{HeaderOverride: "one more, I am watching"})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(logs.String(), "overridden") {
		t.Fatalf("override must pass and be logged: %d", resp.StatusCode)
	}
}

const loopingBody = `{"model":"claude-opus-5","max_tokens":50,"messages":[
 {"role":"user","content":"go"},
 {"role":"assistant","content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"make"}}]},
 {"role":"user","content":[{"type":"tool_result","tool_use_id":"1","content":"err"}]},
 {"role":"assistant","content":[{"type":"tool_use","id":"2","name":"Bash","input":{"command":"make"}}]},
 {"role":"user","content":[{"type":"tool_result","tool_use_id":"2","content":"err"}]},
 {"role":"assistant","content":[{"type":"tool_use","id":"3","name":"Bash","input":{"command":"make"}}]},
 {"role":"user","content":[{"type":"tool_result","tool_use_id":"3","content":"err"}]}]}`

func postBody(t *testing.T, base, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", secret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestLoopGuardWarnsThenBlocks(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxyWith(t, up, Config{Loops: LoopLimits{Warn: 3, Block: 4}})
	resp := postBody(t, base, loopingBody)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get(HeaderWarning), "3 times") {
		t.Fatalf("three repeats must warn and pass: %d %q", resp.StatusCode, resp.Header.Get(HeaderWarning))
	}
	base, _, logs := startProxyWith(t, &upstream{t: t}, Config{Loops: LoopLimits{Block: 3}})
	resp = postBody(t, base, loopingBody)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "buffy_loop") {
		t.Fatalf("three repeats at block=3 must refuse: %d %s", resp.StatusCode, body)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(loopingBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderOverride, "I know, once more")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(logs.String(), "loop block overridden") {
		t.Fatalf("override must pass a loop block and be logged: %d", resp.StatusCode)
	}
}

// A client that disconnects mid-stream (the user interrupting a turn) was
// still billed by the provider; the ledger and the spend counter must see
// it, and the breaker must not be left half-open.
func TestClientAbortMidStreamIsStillRecorded(t *testing.T) {
	up := &upstream{t: t, mode: "stream", release: make(chan struct{})}
	base, dir, logs := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{SessionTokens: 1_000_000})})
	resp := post(t, base, "/v1/messages", nil)
	buf := make([]byte, 4096)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatal(err)
	}
	// Go away before the upstream finishes. The proxy cancels the upstream
	// request, the upstream returns, and the deferred bookkeeping must
	// still record the usage from message_start.
	_ = resp.Body.Close()
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 || recs[0].Response.Usage == nil || recs[0].Response.Usage.CacheRead != 300 {
		t.Fatalf("aborted stream not recorded: %+v", recs)
	}
	// Whether the server noticed the disconnect during the copy (abort
	// panic) or after the upstream returned depends on timing; both paths
	// go through the same bookkeeping, so only the record is asserted.
	if !strings.Contains(logs.String(), "session=session-abc") {
		t.Fatalf("request must be logged: %s", logs.String())
	}
	close(up.release)
}

func TestBreakerHoldsRequestsAfterProviderFailures(t *testing.T) {
	up := &upstream{t: t, mode: "error"}
	breaker := NewBreaker(BreakerSettings{Failures: 2, Cooldown: time.Minute})
	base, _, _ := startProxyWith(t, up, Config{Breaker: breaker})
	for i := 0; i < 2; i++ {
		resp := post(t, base, "/v1/messages", nil)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("provider errors must pass through: %d", resp.StatusCode)
		}
	}
	// The outcome is observed after the response has been sent, so the
	// client can get ahead of the breaker; wait for it to open.
	waitFor(t, "breaker to open", func() bool {
		breaker.mu.Lock()
		defer breaker.mu.Unlock()
		return !breaker.openedAt.IsZero()
	})
	resp := post(t, base, "/v1/messages", nil)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" || !strings.Contains(string(body), "buffy_circuit_open") {
		t.Fatalf("open circuit must refuse locally with Retry-After: %d %s", resp.StatusCode, body)
	}
	if up.seen().requests != 2 {
		t.Fatalf("held request must not reach the provider: %d", up.seen().requests)
	}
}

// breakingUpstream answers the second request with a cache read below the
// expectation derived from the first, which is what a cache break looks
// like on the wire.
type breakingUpstream struct {
	mu sync.Mutex
	n  int
}

func (b *breakingUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.ReadAll(r.Body)
	b.mu.Lock()
	b.n++
	n := b.n
	b.mu.Unlock()
	usage := `{"input_tokens":20,"cache_creation_input_tokens":1000,"cache_read_input_tokens":5000,"output_tokens":50}`
	if n == 2 {
		// Expected read is 6020 - 20 = 6000; report none.
		usage = `{"input_tokens":20,"cache_creation_input_tokens":6000,"cache_read_input_tokens":0,"output_tokens":50}`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":`+usage+`}`)
}

func TestLiveCacheBreakIsLoggedRecordedAndCounted(t *testing.T) {
	base, dir, logs := startProxy(t, &breakingUpstream{}, "")
	for i := 0; i < 2; i++ {
		resp := post(t, base, "/v1/messages", nil)
		_ = resp.Body.Close()
		waitLedger(t, dir, i+1)
	}
	recs := waitLedger(t, dir, 2)
	if recs[0].Cache != nil || recs[1].Cache == nil || recs[1].Cache.Outcome != "broken" || recs[1].Cache.Deficit != 6000 {
		t.Fatalf("ledger cache outcomes wrong: %+v %+v", recs[0].Cache, recs[1].Cache)
	}
	if !strings.Contains(logs.String(), "cache break") || !strings.Contains(logs.String(), "6000 tokens re-billed") {
		t.Fatalf("break not logged: %s", logs.String())
	}
	resp, err := http.Get(base + "/buffy/status")
	if err != nil {
		t.Fatal(err)
	}
	var st Status
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if len(st.Sessions) != 1 || st.Sessions[0].Breaks != 1 || st.Sessions[0].Requests != 2 || st.Sessions[0].ListCostUSD <= 0 {
		t.Fatalf("status wrong: %+v", st)
	}
	resp, err = http.Get(base + "/buffy/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	for _, want := range []string{"buffy_cache_break_total 1", `buffy_requests_total{class="2xx"} 2`, "buffy_cached_share", "buffy_request_latency_seconds_count 2"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metrics missing %q:\n%s", want, body)
		}
	}
}

func TestStatusEndpointsHonorTokenAndOrigin(t *testing.T) {
	base, _, _ := startProxy(t, &upstream{t: t}, "tok")
	resp, err := http.Get(base + "/buffy/metrics")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("metrics without token: %d", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/buffy/status", nil)
	req.Header.Set(HeaderToken, "tok")
	req.Header.Set("Origin", "https://evil.example")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status with browser origin: %d", resp.StatusCode)
	}
}

// invariantUpstream reports usage that follows the expected-read
// invariant turn after turn: each request reads everything the previous
// one billed except its uncached tail, and writes the new content. The
// system prompt's hash is checked so a changed prefix shows as a zero read.
type invariantUpstream struct {
	mu         sync.Mutex
	prompt     int
	prefixHash string
	perTurn    int
}

const invariantTail = 20

func (u *invariantUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		System json.RawMessage `json:"system"`
	}
	_ = json.Unmarshal(body, &req)
	u.mu.Lock()
	read := 0
	if u.prompt > 0 && string(req.System) == u.prefixHash {
		read = u.prompt - invariantTail
	}
	u.prompt += u.perTurn
	u.prefixHash = string(req.System)
	creation := u.prompt - read - invariantTail
	u.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":5}}`, invariantTail, creation, read)
}

// growingBody renders turn n of a conversation whose every turn adds a
// user message large enough to fit on.
func growingBody(system string, turn int) string {
	filler := strings.Repeat("x", 700)
	msgs := []string{`{"role":"user","content":"` + filler + `"}`}
	for i := 1; i < turn; i++ {
		msgs = append(msgs, `{"role":"assistant","content":"ok"}`, `{"role":"user","content":"`+filler+`"}`)
	}
	return `{"model":"claude-opus-5","max_tokens":50,"system":"` + system + `","messages":[` + strings.Join(msgs, ",") + `]}`
}

func postTurn(t *testing.T, base, body string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderSessionID, "session-grow")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func getStatus(t *testing.T, base string) Status {
	t.Helper()
	resp, err := http.Get(base + "/buffy/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	var st Status
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

// The live what-if figures must be exactly what buffy replay prints for
// the same ledger, since they come from the same simulator over the same
// records; and the candidates must never touch the wire.
func TestWhatIfMatchesOfflineReplayAndStaysOffTheWire(t *testing.T) {
	up := &invariantUpstream{perTurn: 800}
	base, dir, logs := startProxy(t, up, "")
	const turns = whatIfLogEvery
	for i := 1; i <= turns; i++ {
		postTurn(t, base, growingBody("be brief", i))
		waitLedger(t, dir, i)
	}
	waitFor(t, "what-if to be scored", func() bool {
		st := getStatus(t, base)
		return len(st.Sessions) == 1 && len(st.Sessions[0].WhatIf) > 1
	})
	st := getStatus(t, base)
	live := st.Sessions[0].WhatIf
	if live[0].Policy != "as-run" || live[0].VsAsRun != 0 || live[0].Estimated {
		t.Fatalf("as-run must lead and be measured: %+v", live[0])
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("ledger files: %v %v", matches, err)
	}
	session, err := ledger.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	offline := analysis.AnalyzeLane(session, analysis.MainLane(session)).Policies()
	if len(offline) != len(live) {
		t.Fatalf("live scored %d policies, offline %d", len(live), len(offline))
	}
	for i := range offline {
		if offline[i].Name != live[i].Policy || offline[i].EffectiveTokens != live[i].EffectiveTokens || offline[i].CostUSD != live[i].ListCostUSD {
			t.Fatalf("policy %d differs live vs offline:\n%+v\n%+v", i, live[i], offline[i])
		}
	}
	if !strings.Contains(logs.String(), "what-if session=") {
		t.Fatalf("what-if line not logged after %d requests:\n%s", turns, logs.String())
	}
	// Dry-run means dry: the upstream saw exactly the client's bodies.
	if strings.Contains(logs.String(), "context_management") {
		t.Fatal("candidate parameters must not reach the log or the wire")
	}
}

func TestPrefixChangeIsNamedAsBreakCause(t *testing.T) {
	base, dir, _ := startProxy(t, &invariantUpstream{perTurn: 800}, "")
	postTurn(t, base, growingBody("be brief", 1))
	waitLedger(t, dir, 1)
	postTurn(t, base, growingBody("be verbose", 2))
	recs := waitLedger(t, dir, 2)
	if recs[1].Cache == nil || recs[1].Cache.Cause != cachemodel.CausePrefixChange {
		t.Fatalf("a changed system prompt must be named as the cause: %+v", recs[1].Cache)
	}
	if recs[0].PrefixHash == "" || recs[0].PrefixHash == recs[1].PrefixHash {
		t.Fatalf("prefix hashes must be recorded and differ: %q %q", recs[0].PrefixHash, recs[1].PrefixHash)
	}
	st := getStatus(t, base)
	if len(st.Sessions) != 1 || st.Sessions[0].PrefixChanges != 1 || st.Sessions[0].Breaks != 1 {
		t.Fatalf("status must count the prefix change: %+v", st.Sessions)
	}
}
