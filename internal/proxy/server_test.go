package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/learn"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
	"github.com/RedRobotKK/Replay/internal/policy"
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
	return startProxyIn(t, up, extra, t.TempDir())
}

// startProxyIn starts a proxy over an existing ledger directory, so a test
// can restart a proxy and check what it remembers.
func startProxyIn(t *testing.T, up http.Handler, extra Config, dir string) (string, string, *syncBuffer) {
	t.Helper()
	upSrv := httptest.NewServer(up)
	t.Cleanup(upSrv.Close)
	target, err := url.Parse(upSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
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
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "replay_spend_cap") {
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
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "replay_loop") {
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
	//
	// The log line is written by a different path from the ledger record, so
	// waiting for the record says nothing about whether the line has been
	// emitted yet. Asserting on it immediately after waitLedger was
	// synchronising on the wrong event: it passed whenever the log happened to
	// win the race and failed on CI with a present record and an empty buffer.
	waitFor(t, "the aborted request to be logged", func() bool {
		return strings.Contains(logs.String(), "session=session-abc")
	})
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
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" || !strings.Contains(string(body), "replay_circuit_open") {
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
	// The log line follows the ledger write, so it can land after the
	// record the test waited for.
	waitFor(t, "cache break log line", func() bool {
		return strings.Contains(logs.String(), "cache break") && strings.Contains(logs.String(), "6000 tokens re-billed")
	})
	resp, err := http.Get(base + "/replay/status")
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
	resp, err = http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	for _, want := range []string{"replay_cache_break_total 1", `replay_requests_total{class="2xx"} 2`, "replay_cached_share", "replay_request_latency_seconds_count 2"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metrics missing %q:\n%s", want, body)
		}
	}
}

func TestStatusEndpointsHonorTokenAndOrigin(t *testing.T) {
	base, _, _ := startProxy(t, &upstream{t: t}, "tok")
	resp, err := http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("metrics without token: %d", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/replay/status", nil)
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
	seen       int
	// edits makes every response report one applied context edit.
	edits  bool
	bodies [][]byte
}

func (u *invariantUpstream) requests() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.seen
}

const invariantTail = 20

func (u *invariantUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		System json.RawMessage `json:"system"`
	}
	_ = json.Unmarshal(body, &req)
	u.mu.Lock()
	u.seen++
	u.bodies = append(u.bodies, body)
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
	extra := ""
	if u.edits {
		extra = `,"context_management":{"applied_edits":[{"type":"clear_tool_uses_20250919","cleared_input_tokens":100}]}`
	}
	_, _ = fmt.Fprintf(w, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":5}%s}`, invariantTail, creation, read, extra)
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
	resp, err := http.Get(base + "/replay/status")
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

// The live what-if figures must be exactly what replay replay prints for
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
	// The system prompt changed and the tools did not, so the cause names the
	// system prompt specifically. It used to be the combined "system prompt or
	// tool definitions changed", which was true of this case and of every
	// other one, and so told an operator nothing about which to go and look at.
	if recs[1].Cache == nil || recs[1].Cache.Cause != cachemodel.CauseSystemChanged {
		t.Fatalf("a changed system prompt must be named as the cause, and named precisely: %+v",
			recs[1].Cache)
	}
	// And the detail says how far it moved, end to end through a real proxy
	// rather than only in the unit test's fixture.
	if !strings.Contains(recs[1].Cache.CauseDetail, "system prompt") {
		t.Fatalf("the break detail must say what changed: %q", recs[1].Cache.CauseDetail)
	}
	if recs[0].PrefixHash == "" || recs[0].PrefixHash == recs[1].PrefixHash {
		t.Fatalf("prefix hashes must be recorded and differ: %q %q", recs[0].PrefixHash, recs[1].PrefixHash)
	}
	st := getStatus(t, base)
	if len(st.Sessions) != 1 || st.Sessions[0].PrefixChanges != 1 || st.Sessions[0].Breaks != 1 {
		t.Fatalf("status must count the prefix change: %+v", st.Sessions)
	}
}

// bodyEcho is a fake provider that keeps every request body it saw and
// answers with a message that reports one applied edit.
type bodyEcho struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (b *bodyEcho) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	b.mu.Lock()
	b.bodies = append(b.bodies, body)
	b.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":4,"cache_creation_input_tokens":30,"cache_read_input_tokens":300,"output_tokens":2},"context_management":{"applied_edits":[{"type":"clear_tool_uses_20250919","cleared_input_tokens":1234}]}}`)
}

func (b *bodyEcho) seen() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([][]byte(nil), b.bodies...)
}

func postWith(t *testing.T, base, body string, headers map[string]string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// The live policy adds exactly one member to the client's body, is
// recorded on the ledger with the provider's applied edits, is logged with
// hashes only, and is pinned per session from the first request.
func TestContextEditPolicyIsAppliedRecordedAndPinned(t *testing.T) {
	up := &bodyEcho{}
	base, dir, logs := startProxyWith(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}})
	withBeta := map[string]string{HeaderSessionID: "sess-on", "anthropic-beta": "fast-mode-2026-02-01," + policy.BetaFeature}
	noBeta := map[string]string{HeaderSessionID: "sess-off"}

	postWith(t, base, requestBody, withBeta)
	postWith(t, base, requestBody, noBeta)
	// The session pinned on stays on; a request in it without the beta
	// header goes through unchanged and the skip is logged.
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-on"})
	// The session pinned off stays off even when a later request admits it.
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-off", "anthropic-beta": policy.BetaFeature})
	// A client that set the parameter itself is never overridden.
	clientSet := strings.TrimSuffix(requestBody, "}") + `,"context_management":{"edits":[]}}`
	postWith(t, base, clientSet, map[string]string{HeaderSessionID: "sess-client", "anthropic-beta": policy.BetaFeature})

	bodies := up.seen()
	if len(bodies) != 5 {
		t.Fatalf("upstream saw %d requests", len(bodies))
	}
	if !bytes.HasPrefix(bodies[0], []byte(strings.TrimSuffix(requestBody, "}"))) || !bytes.Contains(bodies[0], []byte(`"context_management":{"edits":[{"type":"clear_tool_uses_20250919","trigger":{"type":"input_tokens","value":150000}`)) {
		t.Fatalf("first request must carry the parameter after the client's bytes: %s", bodies[0])
	}
	for i, want := range []string{requestBody, requestBody, requestBody, clientSet} {
		if string(bodies[i+1]) != want {
			t.Fatalf("request %d must be byte-identical to the client's: %s", i+1, bodies[i+1])
		}
	}

	recs := waitLedger(t, dir, 5)
	byID := map[string][]ledger.Record{}
	for _, r := range recs {
		byID[r.SessionID] = append(byID[r.SessionID], r)
	}
	on := byID["sess-on"]
	if len(on) != 2 || on[0].Policy != policy.Name || on[1].Policy != "" || on[0].Response.AppliedEdits != 1 || on[0].Response.ClearedInputTokens != 1234 {
		t.Fatalf("ledger for the pinned-on session wrong: %+v", on)
	}
	for _, id := range []string{"sess-off", "sess-client"} {
		for _, r := range byID[id] {
			if r.Policy != "" {
				t.Fatalf("session %s must never carry the policy: %+v", id, r)
			}
		}
	}

	log := logs.String()
	if !strings.Contains(log, "policy context-edit(keep=6,trigger=150000) session=sess-on applied body sha256 before=") || strings.Contains(log, "be brief") {
		t.Fatalf("applied transformation must be logged with hashes and never content:\n%s", log)
	}
	if !strings.Contains(log, string(policy.SkipNoBeta)) {
		t.Fatalf("skip in a pinned-on session must be logged:\n%s", log)
	}

	st := getStatus(t, base)
	for _, sess := range st.Sessions {
		switch sess.Session {
		case "sess-on":
			if sess.Policy != string(policy.Applied) || sess.PolicyApplied != 1 || sess.ClearedInputTokens != 1234*2 {
				t.Fatalf("status for sess-on: %+v", sess)
			}
		case "sess-off":
			if sess.Policy != string(policy.SkipNoBeta) || sess.PolicyApplied != 0 {
				t.Fatalf("status for sess-off: %+v", sess)
			}
		case "sess-client":
			if sess.Policy != string(policy.SkipClientSet) {
				t.Fatalf("status for sess-client: %+v", sess)
			}
		}
	}
	resp, err := http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(metrics), `replay_policy_applied_total{policy="context-edit"} 1`) {
		t.Fatalf("metrics missing policy counter:\n%s", metrics)
	}
}

// flakyUpstream answers from a script of statuses, one per request, then
// a normal message. "drop" closes the connection without answering.
type flakyUpstream struct {
	mu     sync.Mutex
	script []string
	seen   int
	afters []string
}

func (f *flakyUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.ReadAll(r.Body)
	f.mu.Lock()
	i := f.seen
	f.seen++
	f.mu.Unlock()
	if i >= len(f.script) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
		return
	}
	step := f.script[i]
	if step == "drop" {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	}
	code, _ := strconv.Atoi(step)
	if i < len(f.afters) && f.afters[i] != "" {
		w.Header().Set("Retry-After", f.afters[i])
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, `{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`)
}

func (f *flakyUpstream) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seen
}

func fastRetries(attempts int) RetrySettings {
	return RetrySettings{Attempts: attempts, BaseDelay: time.Millisecond, MaxDelay: 50 * time.Millisecond}
}

func TestRetriesResendUntilSuccessAndAreRecorded(t *testing.T) {
	up := &flakyUpstream{script: []string{"529", "429", "503"}, afters: []string{"0", "", ""}}
	base, dir, logs := startProxyWith(t, up, Config{Retries: fastRetries(3)})
	resp := post(t, base, "/v1/messages", nil)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != messageResponse {
		t.Fatalf("client must see the eventual success: %d %s", resp.StatusCode, body)
	}
	if up.requests() != 4 {
		t.Fatalf("upstream requests = %d, want 4", up.requests())
	}
	recs := waitLedger(t, dir, 1)
	if recs[0].Retries != 3 || recs[0].Status != http.StatusOK {
		t.Fatalf("ledger must carry the retry count: %+v", recs[0])
	}
	// The request line is logged after the ledger write.
	waitFor(t, "the request log line", func() bool { return strings.Contains(logs.String(), "retries=3") })
	log := logs.String()
	for _, want := range []string{"retry 1/3", "after status 529", "retry 2/3", "after status 429", "retry 3/3", "after status 503", "retries=3"} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %q:\n%s", want, log)
		}
	}
	mresp, err := http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := io.ReadAll(mresp.Body)
	_ = mresp.Body.Close()
	if !strings.Contains(string(metrics), "replay_retries_total 3") {
		t.Fatalf("metrics missing retries:\n%s", metrics)
	}
}

func TestRetriesStopOnClientErrorsExhaustionAndLongRetryAfter(t *testing.T) {
	cases := []struct {
		name     string
		script   []string
		afters   []string
		attempts int
		wantReqs int
		wantCode int
	}{
		{"client error", []string{"400", "400"}, nil, 3, 1, 400},
		// A connection that drops after the request was sent may already
		// have been billed; it is never resent.
		{"dropped after sending", []string{"drop"}, nil, 3, 1, 502},
		{"exhausted", []string{"529", "529", "529"}, nil, 1, 2, 529},
		{"retry-after beyond cap", []string{"429"}, []string{"3600"}, 3, 1, 429},
		{"off", []string{"503"}, nil, 0, 1, 503},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			up := &flakyUpstream{script: c.script, afters: c.afters}
			base, _, _ := startProxyWith(t, up, Config{Retries: fastRetries(c.attempts)})
			resp := post(t, base, "/v1/messages", nil)
			_ = resp.Body.Close()
			if resp.StatusCode != c.wantCode || up.requests() != c.wantReqs {
				t.Fatalf("status %d after %d upstream requests, want %d after %d", resp.StatusCode, up.requests(), c.wantCode, c.wantReqs)
			}
		})
	}
}

func TestRetryAfterParsesSecondsAndDates(t *testing.T) {
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	if d, ok := retryAfter("7", now); !ok || d != 7*time.Second {
		t.Fatalf("seconds: %v %v", d, ok)
	}
	if d, ok := retryAfter(now.Add(90*time.Second).Format(http.TimeFormat), now); !ok || d != 90*time.Second {
		t.Fatalf("date: %v %v", d, ok)
	}
	if d, ok := retryAfter(now.Add(-time.Minute).Format(http.TimeFormat), now); !ok || d != 0 {
		t.Fatalf("past date must be zero: %v %v", d, ok)
	}
	if _, ok := retryAfter("soon", now); ok {
		t.Fatal("garbage must not parse")
	}
	if _, err := New(Config{Listen: "127.0.0.1:0", Upstream: &url.URL{Scheme: "http", Host: "x"}, Store: mustStore(t), Retries: RetrySettings{Attempts: 2}}); err == nil {
		t.Fatal("retries without delays must be rejected")
	}
}

func mustStore(t *testing.T) *ledger.Store {
	t.Helper()
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// failingBody renders turn n of a session whose every tool result is an
// error, so error content dominates the prompt.
func failingBody(turn int) string {
	filler := strings.Repeat("e", 700)
	msgs := []string{`{"role":"user","content":"` + strings.Repeat("x", 700) + `"}`}
	for i := 1; i < turn; i++ {
		id := fmt.Sprintf("t%d", i)
		msgs = append(msgs,
			`{"role":"assistant","content":[{"type":"tool_use","id":"`+id+`","name":"Bash","input":{"command":"make `+id+`"}}]}`,
			`{"role":"user","content":[{"type":"tool_result","tool_use_id":"`+id+`","is_error":true,"content":"`+filler+`"}]}`)
	}
	return `{"model":"claude-opus-5","max_tokens":50,"system":"be brief","messages":[` + strings.Join(msgs, ",") + `]}`
}

func TestErrorBudgetRefusesBeforeSpendCapAndHonorsOverride(t *testing.T) {
	up := &invariantUpstream{perTurn: 3000}
	base, dir, logs := startProxyWith(t, up, Config{ErrorBudget: ErrorBudget{Share: 0.5}})
	refusedAt := 0
	for i := 1; i <= 12 && refusedAt == 0; i++ {
		req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(failingBody(i)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(HeaderSessionID, "session-fail")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			waitLedger(t, dir, i)
			// The budget is judged from the analysis that runs after the
			// response, so let it land before the next request.
			waitFor(t, "error tokens to be scored", func() bool {
				e, p := getSessionErrorTokens(t, base, "session-fail")
				return p >= errorBudgetMinPromptTokens && e > 0 || p < errorBudgetMinPromptTokens
			})
		case http.StatusBadRequest:
			if !strings.Contains(string(body), "replay_error_budget") {
				t.Fatalf("unexpected 400: %s", body)
			}
			refusedAt = i
		default:
			t.Fatalf("request %d: status %d %s", i, resp.StatusCode, body)
		}
	}
	if refusedAt == 0 {
		t.Fatalf("error budget never tripped:\n%s", logs.String())
	}
	if up.requests() != refusedAt-1 {
		t.Fatalf("refused request must not reach the provider: %d upstream requests, refused at %d", up.requests(), refusedAt)
	}
	st := getStatus(t, base)
	if len(st.Sessions) != 1 || st.Sessions[0].ErrorShare < 0.5 {
		t.Fatalf("status must show the error share: %+v", st.Sessions)
	}
	// An override proceeds once and is logged.
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(failingBody(refusedAt)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderSessionID, "session-fail")
	req.Header.Set(HeaderOverride, "I know, keep going")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(logs.String(), "error budget overridden") {
		t.Fatalf("override must proceed once and be logged: %d\n%s", resp.StatusCode, logs.String())
	}
}

func getSessionErrorTokens(t *testing.T, base, id string) (int, int) {
	t.Helper()
	for _, s := range getStatus(t, base).Sessions {
		if s.Session == id {
			return int(s.ErrorShare * float64(s.PromptTokens)), s.PromptTokens
		}
	}
	return 0, 0
}

// writePolicyFile writes a learn result selecting a context-edit trigger,
// or selecting nothing when trigger is zero.
func writePolicyFile(t *testing.T, path string, trigger int) {
	t.Helper()
	res := learn.Result{Schema: learn.PolicyFileSchema, Rules: cachemodel.RulesVersion, Reason: "test"}
	if trigger > 0 {
		p := analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: trigger}
		res.Selected = &learn.Candidate{Name: fmt.Sprintf("context-edit(keep=6,trigger=%d)", trigger), Family: learn.FamilyContextEdit, ContextEdit: &p}
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func triggerSeen(body []byte) string {
	i := bytes.Index(body, []byte(`"trigger":{"type":"input_tokens","value":`))
	if i < 0 {
		return "none"
	}
	rest := body[i+len(`"trigger":{"type":"input_tokens","value":`):]
	j := bytes.IndexByte(rest, '}')
	return string(rest[:j])
}

// PX-8: the policy is chosen at a session's first request from the policy
// file and pinned for the session's life, on disk, so neither a rewritten
// file nor a restarted proxy changes a running session.
func TestPolicyFileIsReadAtSessionStartAndPinsSurviveRewritesAndRestarts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writePolicyFile(t, file, 200000)
	up := &bodyEcho{}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file}, dir)
	beta := func(id string) map[string]string {
		return map[string]string{HeaderSessionID: id, "anthropic-beta": policy.BetaFeature}
	}
	postWith(t, base, requestBody, beta("sess-a"))
	writePolicyFile(t, file, 400000)
	postWith(t, base, requestBody, beta("sess-a"))
	postWith(t, base, requestBody, beta("sess-b"))
	writePolicyFile(t, file, 0)
	postWith(t, base, requestBody, beta("sess-c"))
	got := up.seen()
	if len(got) != 4 {
		t.Fatalf("upstream saw %d requests", len(got))
	}
	for i, want := range []string{"200000", "200000", "400000", "none"} {
		if triggerSeen(got[i]) != want {
			t.Fatalf("request %d carried trigger %s, want %s", i, triggerSeen(got[i]), want)
		}
	}
	if !strings.Contains(logs.String(), "policy file selects nothing") {
		t.Fatalf("an empty selection must be logged:\n%s", logs.String())
	}
	st := getStatus(t, base)
	for _, sess := range st.Sessions {
		switch sess.Session {
		case "sess-a":
			if sess.PinnedPolicy != "context-edit(keep=6,trigger=200000)" || sess.Policy != string(policy.Applied) {
				t.Fatalf("sess-a status: %+v", sess)
			}
		case "sess-c":
			if sess.PinnedPolicy != "" || sess.Policy != string(policy.NotConfigured) {
				t.Fatalf("sess-c status: %+v", sess)
			}
		}
	}

	// A new proxy over the same ledger directory, with the file now
	// selecting 400k: sess-a keeps 200k from its persisted pin, sess-c
	// keeps none, and a fresh session gets 400k.
	writePolicyFile(t, file, 400000)
	up2 := &bodyEcho{}
	base2, _, logs2 := startProxyIn(t, up2, Config{PolicyFile: file}, dir)
	postWith(t, base2, requestBody, beta("sess-a"))
	postWith(t, base2, requestBody, beta("sess-c"))
	postWith(t, base2, requestBody, beta("sess-d"))
	got = up2.seen()
	for i, want := range []string{"200000", "none", "400000"} {
		if triggerSeen(got[i]) != want {
			t.Fatalf("after restart request %d carried trigger %s, want %s", i, triggerSeen(got[i]), want)
		}
	}
	if !strings.Contains(logs2.String(), "pinned earlier") {
		t.Fatalf("a restored pin must be logged:\n%s", logs2.String())
	}
}

// The explicit flag wins over the file, and a stale file applies nothing.
func TestPolicyFlagWinsOverFileAndStaleFileAppliesNothing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writePolicyFile(t, file, 400000)
	up := &bodyEcho{}
	base, _, _ := startProxyIn(t, up, Config{PolicyFile: file, ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}}, dir)
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-flag", "anthropic-beta": policy.BetaFeature})
	if got := triggerSeen(up.seen()[0]); got != "150000" {
		t.Fatalf("flag must win over the file: %s", got)
	}

	stale := filepath.Join(t.TempDir(), "stale.json")
	if err := os.WriteFile(stale, []byte(`{"schema":99,"rules":"x","selected":{"name":"context-edit","family":"context-edit","context_edit":{"KeepLast":6,"TriggerTokens":100}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	up2 := &bodyEcho{}
	base2, _, logs := startProxyWith(t, up2, Config{PolicyFile: stale})
	postWith(t, base2, requestBody, map[string]string{HeaderSessionID: "sess-stale", "anthropic-beta": policy.BetaFeature})
	if got := triggerSeen(up2.seen()[0]); got != "none" || !strings.Contains(logs.String(), "schema 99") {
		t.Fatalf("stale file must apply nothing and say why: %s\n%s", got, logs.String())
	}
}

// A failure to connect cannot have billed anything and is resent; the
// count of attempts shows in the log.
func TestRetriesResendWhenTheProviderCannotBeReached(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	target, err := url.Parse(closed.URL)
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	logs := &syncBuffer{}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: mustStore(t), Logger: log.New(logs, "", 0), Retries: fastRetries(2)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	resp := post(t, "http://"+srv.Addr(), "/v1/messages", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("unreachable provider must end in 502, got %d", resp.StatusCode)
	}
	for _, want := range []string{"retry 1/2 in", "retry 2/2 in", "after connection failed"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("log missing %q:\n%s", want, logs.String())
		}
	}
}

// PX-6: turning policies off must stop a session an earlier process
// pinned on, and a restart with no policy source must do the same.
func TestPoliciesOffOverrideAPersistedPin(t *testing.T) {
	dir := t.TempDir()
	up := &bodyEcho{}
	base, _, _ := startProxyIn(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}}, dir)
	beta := map[string]string{HeaderSessionID: "sess-pinned", "anthropic-beta": policy.BetaFeature}
	postWith(t, base, requestBody, beta)
	if triggerSeen(up.seen()[0]) != "150000" {
		t.Fatal("first process must pin the session on")
	}
	for name, cfg := range map[string]Config{
		"no policy env":    {ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}, NoPolicy: true},
		"no policy source": {},
	} {
		t.Run(name, func(t *testing.T) {
			up2 := &bodyEcho{}
			base2, _, _ := startProxyIn(t, up2, cfg, dir)
			postWith(t, base2, requestBody, beta)
			got := up2.seen()
			if len(got) != 1 || strings.Contains(string(got[0]), "context_management") {
				t.Fatalf("pinned session must run untouched: %s", got)
			}
		})
	}
}

// A body the summarizer cannot read never gets the parameter, since it
// may already carry one the summarizer failed to see.
func TestUnparsedRequestsNeverCarryThePolicy(t *testing.T) {
	up := &bodyEcho{}
	base, _, logs := startProxyWith(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}})
	odd := `{"model":"claude-opus-5","max_tokens":50,"messages":[{"role":"user","content":{"unexpected":"shape"}}]}`
	postWith(t, base, odd, map[string]string{HeaderSessionID: "sess-odd", "anthropic-beta": policy.BetaFeature})
	if got := up.seen(); len(got) != 1 || string(got[0]) != odd {
		t.Fatalf("unparsed body must pass through byte for byte: %s", got)
	}
	if !strings.Contains(logs.String(), string(policy.SkipUnparsed)) {
		t.Fatalf("skip must be logged:\n%s", logs.String())
	}
}

// Client forwarding headers are the client's bytes and reach the provider.
func TestClientForwardingHeadersAreKept(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", map[string]string{"X-Forwarded-For": "10.0.0.7", "Forwarded": "for=10.0.0.7"})
	_ = resp.Body.Close()
	up.mu.Lock()
	defer up.mu.Unlock()
	if up.gotXFF != "10.0.0.7" {
		t.Fatalf("X-Forwarded-For must pass through unchanged, got %q", up.gotXFF)
	}
}

func TestSessionStateIsBounded(t *testing.T) {
	s := newStats()
	for i := 0; i < maxSessions*2; i++ {
		s.pin(fmt.Sprintf("s-%d", i), nil, policy.NotConfigured, time.Time{})
	}
	if len(s.sessions) != maxSessions {
		t.Fatalf("sessions = %d, want %d", len(s.sessions), maxSessions)
	}
	if _, _, ok := s.pinned("s-0"); ok {
		t.Fatal("the oldest session must have been evicted")
	}
	if _, _, ok := s.pinned(fmt.Sprintf("s-%d", maxSessions*2-1)); !ok {
		t.Fatal("the newest session must remain")
	}
}

// A session gets the selection learned for its type, judged at its first
// request from the model and the prompt's size, and the type is pinned.
func TestPolicyIsChosenBySessionType(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	large := analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: 300000}
	res := learn.Result{Schema: learn.PolicyFileSchema, Rules: cachemodel.RulesVersion, Generated: time.Now(), Reason: "types disagree",
		Types: []learn.TypeResult{{Type: "opus/large-prefix", Sessions: 9, Selected: &learn.Candidate{Name: "context-edit(keep=6,trigger=300000)", Family: learn.FamilyContextEdit, ContextEdit: &large}}, {Type: "opus/small-prefix", Sessions: 9, Reason: "hurts"}}}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	up := &bodyEcho{}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file}, dir)
	big := `{"model":"claude-opus-5","max_tokens":50,"system":"` + strings.Repeat("s", 90_000) + `","messages":[{"role":"user","content":"hi"}]}`
	postSession(t, base, "sess-large", big)
	postSession(t, base, "sess-small", requestBody)
	got := up.seen()
	if triggerSeen(got[0]) != "300000" || triggerSeen(got[1]) != "none" {
		t.Fatalf("large-prefix session must get its type's policy and small none: %s / %s", triggerSeen(got[0]), triggerSeen(got[1]))
	}
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := store.Pin("sess-large"); !ok || p.Type != "opus/large-prefix" || p.Trigger != 300000 {
		t.Fatalf("large pin: %+v %v", p, ok)
	}
	if p, ok := store.Pin("sess-small"); !ok || p.Type != "opus/small-prefix" || p.Policy != "" {
		t.Fatalf("small pin: %+v %v", p, ok)
	}
	if !strings.Contains(logs.String(), "type=opus/small-prefix runs without a policy") {
		t.Fatalf("the typed no-selection must be logged:\n%s", logs.String())
	}
}

// writePolicyFileAt is writePolicyFile with a generation time, which a
// revert is tied to.
func writePolicyFileAt(t *testing.T, path string, trigger int, generated time.Time) {
	t.Helper()
	p := analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: trigger}
	res := learn.Result{Schema: learn.PolicyFileSchema, Rules: cachemodel.RulesVersion, Generated: generated, Selected: &learn.Candidate{Name: fmt.Sprintf("context-edit(keep=6,trigger=%d)", trigger), Family: learn.FamilyContextEdit, ContextEdit: &p}}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// sessionInArm finds a session id the trial assigns to the wanted arm.
func sessionInArm(t *testing.T, settings TrialSettings, treated bool) string {
	t.Helper()
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("trial-%d", i)
		if settings.treated(id) == treated {
			return id
		}
	}
	t.Fatal("no session id lands in the wanted arm")
	return ""
}

func postSession(t *testing.T, base, id, body string) {
	t.Helper()
	postWith(t, base, body, map[string]string{HeaderSessionID: id, "anthropic-beta": policy.BetaFeature})
}

// readingBody renders turn n of a session that reads the same file on
// every turn, which is what a context edit makes expensive.
func readingBody(turn int) string {
	filler := strings.Repeat("y", 700)
	msgs := []string{`{"role":"user","content":"` + strings.Repeat("x", 700) + `"}`}
	for i := 1; i < turn; i++ {
		id := fmt.Sprintf("r%d", i)
		msgs = append(msgs,
			`{"role":"assistant","content":[{"type":"tool_use","id":"`+id+`","name":"Read","input":{"file_path":"/src/a.go"}}]}`,
			`{"role":"user","content":[{"type":"tool_result","tool_use_id":"`+id+`","content":"`+filler+`"}]}`)
	}
	return `{"model":"claude-opus-5","max_tokens":50,"system":"be brief","messages":[` + strings.Join(msgs, ",") + `]}`
}

// lastBody returns the body of the request the upstream saw after the
// first n.
func lastBody(t *testing.T, up *invariantUpstream, n int) []byte {
	t.Helper()
	up.mu.Lock()
	defer up.mu.Unlock()
	if len(up.bodies) <= n {
		t.Fatalf("upstream saw %d requests, want more than %d", len(up.bodies), n)
	}
	return up.bodies[n]
}

// LN-5: a learned policy is tried on a stable share of new sessions with
// the rest held out as controls, and the arms are pinned.
func TestTrialShareSplitsSessionsIntoArms(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writePolicyFileAt(t, file, 200000, time.Now())
	settings := TrialSettings{Share: 0.5, RevertAfter: DefaultRevertAfter}
	up := &bodyEcho{}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file, Trial: settings}, dir)
	treated, control := sessionInArm(t, settings, true), sessionInArm(t, settings, false)
	postSession(t, base, treated, requestBody)
	postSession(t, base, control, requestBody)
	got := up.seen()
	if triggerSeen(got[0]) != "200000" || triggerSeen(got[1]) != "none" {
		t.Fatalf("treated must carry the parameter and control must not: %s / %s", triggerSeen(got[0]), triggerSeen(got[1]))
	}
	st := getStatus(t, base)
	if st.Trial.Treated != 1 || st.Trial.Control != 1 || st.Trial.Reverted != "" {
		t.Fatalf("trial status: %+v", st.Trial)
	}
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := store.Pin(control); !ok || p.Trial != trialControl || p.Decision != string(policy.Control) {
		t.Fatalf("control pin: %+v %v", p, ok)
	}
	if p, ok := store.Pin(treated); !ok || p.Trial != trialTreated || p.Policy != policy.Name {
		t.Fatalf("treated pin: %+v %v", p, ok)
	}
	if !strings.Contains(logs.String(), "is a control") {
		t.Fatalf("control assignment must be logged:\n%s", logs.String())
	}
}

// LN-5: treated sessions whose re-read rate after the provider's clears
// reaches the guardrail breach it; enough breaches revert the policy for
// new sessions, persistently, until a newer learning result lifts it.
func TestGuardrailBreachesRevertThePolicyUntilANewerFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	learned := time.Now().Add(-time.Hour)
	writePolicyFileAt(t, file, 200000, learned)
	settings := TrialSettings{Share: 1, ReReadRate: 0.5, RevertAfter: 2}
	up := &invariantUpstream{perTurn: 800, edits: true}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file, Trial: settings}, dir)
	const turns = 7
	// Session ids are logged truncated to twelve characters.
	for _, id := range []string{"breach-1", "breach-2"} {
		for i := 1; i <= turns; i++ {
			postSession(t, base, id, readingBody(i))
		}
		waitFor(t, "guardrail to be judged for "+id, func() bool {
			return strings.Contains(logs.String(), "guardrail session="+id+":")
		})
	}
	waitFor(t, "revert to be recorded", func() bool { return getStatus(t, base).Trial.Reverted != "" })
	st := getStatus(t, base)
	if st.Trial.Breached != 2 || !strings.Contains(st.Trial.Reverted, "re-read rate after clears") {
		t.Fatalf("trial status after breaches: %+v", st.Trial)
	}
	if !strings.Contains(logs.String(), "reverted for new sessions") {
		t.Fatalf("revert must be logged:\n%s", logs.String())
	}
	before := up.requests()
	postSession(t, base, "sess-after-revert", requestBody)
	if got := triggerSeen(lastBody(t, up, before)); got != "none" {
		t.Fatalf("a session after the revert must run without the policy, got trigger %s", got)
	}
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := store.Revert(); !ok || r.Breached != 2 || !r.PolicyGenerated.Equal(learned) {
		t.Fatalf("revert not persisted: %+v %v", r, ok)
	}
	if p, ok := store.Pin("sess-after-revert"); !ok || p.Decision != string(policy.Reverted) {
		t.Fatalf("post-revert pin: %+v %v", p, ok)
	}

	// A restarted proxy honors the revert; a newer learning result lifts it.
	up2 := &invariantUpstream{perTurn: 800}
	base2, _, _ := startProxyIn(t, up2, Config{PolicyFile: file, Trial: settings}, dir)
	postSession(t, base2, "sess-restart", requestBody)
	if got := triggerSeen(lastBody(t, up2, 0)); got != "none" {
		t.Fatalf("revert must survive a restart, got trigger %s", got)
	}
	writePolicyFileAt(t, file, 300000, time.Now())
	postSession(t, base2, "sess-relearned", requestBody)
	if got := triggerSeen(lastBody(t, up2, 1)); got != "300000" {
		t.Fatalf("a newer policy file must lift the revert, got trigger %s", got)
	}
}

// Masking replaces a secret with its placeholder before the request
// leaves, records the count on the ledger, and the secret never reaches
// the log, the ledger, or the status endpoint; a body without a secret is
// forwarded byte for byte.
func TestMaskingReplacesSecretsBeforeEgressAndKeepsThemLocal(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := &upstream{t: t}
	base, dir, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})
	const canary = "sk-ant-api03-CanaryCanaryCanaryCanaryCanary0123456789"
	body := `{"model":"claude-opus-5","max_tokens":50,"system":"be brief","messages":[{"role":"user","content":"my key is ` + canary + `"}]}`
	postWith(t, base, body, map[string]string{HeaderSessionID: "sess-mask"})
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-mask"})
	recs := waitLedger(t, dir, 2)
	up.mu.Lock()
	last := string(up.gotBody)
	up.mu.Unlock()
	if last != requestBody {
		t.Fatalf("a body without a secret must pass byte for byte: %s", last)
	}
	ph, _ := vault.Placeholder(canary, "")
	// The two records land in completion order, which the bookkeeping
	// after each response does not fix; exactly one carries the count.
	maskedRecords := 0
	for _, rec := range recs {
		if rec.Masked["anthropic-api-key"] == 1 {
			maskedRecords++
		}
	}
	if maskedRecords != 1 {
		t.Fatalf("ledger must count the masked secret once: %+v %+v", recs[0].Masked, recs[1].Masked)
	}
	for name, text := range map[string]string{"log": logs.String()} {
		if strings.Contains(text, canary) || strings.Contains(text, ph) {
			t.Fatalf("%s must hold neither the secret nor the placeholder:\n%s", name, text)
		}
	}
	if !strings.Contains(logs.String(), "masked 1 secret(s) session=sess-mask: anthropic-api-key:1") {
		t.Fatalf("masking must be logged by pattern:\n%s", logs.String())
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, f := range files {
		data, _ := os.ReadFile(f)
		if bytes.Contains(data, []byte(canary)) {
			t.Fatalf("ledger holds the secret: %s", f)
		}
	}
	st := getStatus(t, base)
	if len(st.Sessions) != 1 || st.Sessions[0].Masked != 1 {
		t.Fatalf("status must count masked secrets: %+v", st.Sessions)
	}
	resp, err := http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(metrics), `replay_masked_total{pattern="anthropic-api-key"} 1`) {
		t.Fatalf("metrics missing masked counter:\n%s", metrics)
	}
}

// The first request's upstream body carried the placeholder, not the
// secret, and the same secret masks to the same placeholder next time.
func TestMaskingIsDeterministicOnTheWire(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := &bodyEcho{}
	base, _, _ := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})
	const canary = "ghp_CanaryCanaryCanaryCanaryCanary0123456789"
	body := `{"model":"claude-opus-5","max_tokens":50,"messages":[{"role":"user","content":"token ` + canary + `"}]}`
	postWith(t, base, body, map[string]string{HeaderSessionID: "sess-a"})
	postWith(t, base, body, map[string]string{HeaderSessionID: "sess-b"})
	got := up.seen()
	ph, _ := vault.Placeholder(canary, "")
	for i, b := range got {
		if bytes.Contains(b, []byte(canary)) || !bytes.Contains(b, []byte(ph)) {
			t.Fatalf("request %d must carry the placeholder, not the secret: %s", i, b)
		}
	}
	if !bytes.Equal(got[0], got[1]) {
		t.Fatal("the same body must mask to the same bytes")
	}
}

// rehydrationUpstream echoes the placeholder it finds in the request into
// a text block, a shell command, and an in-project file edit, streamed or
// not as the request's X-Test-Mode header says, and records the request's
// Accept-Encoding header.
type rehydrationUpstream struct {
	mu       sync.Mutex
	project  string
	encoding string
}

var placeholderPattern = regexp.MustCompile(`REPLAY_SECRET_[0-9a-f]{16}`)

func (u *rehydrationUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	u.mu.Lock()
	u.encoding = r.Header.Get("Accept-Encoding")
	u.mu.Unlock()
	ph := string(placeholderPattern.Find(body))
	path, _ := json.Marshal(filepath.Join(u.project, "cfg.env"))
	cut := max(len(ph)-5, 0)
	switch mode := r.Header.Get("X-Test-Mode"); mode {
	case "stream", "stream-length":
		w.Header().Set("Content-Type", "text/event-stream")
		var events []string
		total := 0
		for _, ev := range streamFixture(ph, cut, path) {
			events = append(events, "event: x\ndata: "+ev+"\n\n")
			total += len(events[len(events)-1])
		}
		if mode == "stream-length" {
			// An intermediary that buffers the stream and declares its
			// length; the rewritten stream is longer.
			w.Header().Set("Content-Length", strconv.Itoa(total))
		}
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		for _, ev := range events {
			_, _ = io.WriteString(w, ev)
			fl.Flush()
		}
	case "gzip":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		_, _ = io.WriteString(zw, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"key is `+ph+`"}],"usage":{"input_tokens":4,"output_tokens":2}}`)
		_ = zw.Close()
	default:
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"key is `+ph+`"},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"echo `+ph+`"}},{"type":"tool_use","id":"t2","name":"Edit","input":{"file_path":`+string(path)+`,"new_string":"K=`+ph+`"}}],"usage":{"input_tokens":4,"output_tokens":2}}`)
	}
}

func streamFixture(ph string, cut int, path []byte) []string {
	return []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":4,"output_tokens":1}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"key is ` + ph[:cut] + `"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"` + ph[cut:] + ` ok"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t1","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\": \"echo ` + ph + `\"}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"t2","name":"Edit","input":{}}}`,
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"new_string\": \"K=` + ph + `\", "}}`,
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"file_path\": ` + strings.ReplaceAll(strings.ReplaceAll(string(path), `\`, `\\`), `"`, `\"`) + `}"}}`,
		`{"type":"content_block_stop","index":2}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`,
		`{"type":"message_stop"}`,
	}
}

// Rehydration end to end: the secret leaves as a placeholder, the
// provider echoes the placeholder into text, a shell command, and a file
// edit, and the client gets the secret back in the text and the edit
// only. The ledger, status, metrics, and log count it by destination and
// never hold the secret; a compressed response is forwarded untouched
// and the skip is logged.
func TestRehydrationRestoresPlaceholdersWithinScope(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	scopes, err := masking.ParseScopes(project, nil, masking.Patterns)
	if err != nil {
		t.Fatal(err)
	}
	up := &rehydrationUpstream{project: project}
	base, dir, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil), Rehydrator: masking.NewRehydrator(vault, scopes)})
	const canary = "sk-ant-api03-CanaryCanaryCanaryCanaryCanary0123456789"
	body := `{"model":"claude-opus-5","max_tokens":50,"messages":[{"role":"user","content":"my key is ` + canary + `"}]}`
	fetch := func(mode string) string {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("X-Test-Mode", mode)
		req.Header.Set(HeaderSessionID, "sess-rh")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		got, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d err %v", mode, resp.StatusCode, err)
		}
		return string(got)
	}
	ph, _ := vault.Placeholder(canary, "")

	stream := fetch("stream")
	up.mu.Lock()
	encoding := up.encoding
	up.mu.Unlock()
	if encoding != "" {
		t.Fatalf("Accept-Encoding must not reach the provider when rehydrating, got %q", encoding)
	}
	if !strings.Contains(stream, `"text":"`+canary[:5]) || !strings.Contains(stream, `"text":"key is `) || strings.Count(stream, canary) != 2 {
		t.Fatalf("stream must restore the secret in the text and the edit:\n%s", stream)
	}
	if !strings.Contains(stream, `echo `+ph+`\"}`) {
		t.Fatalf("the shell command must keep the placeholder:\n%s", stream)
	}
	if !strings.Contains(stream, `K=`+canary) {
		t.Fatalf("the in-project edit must be restored:\n%s", stream)
	}

	if withLength := fetch("stream-length"); withLength != stream {
		t.Fatalf("a declared length must not cut the rewritten stream:\n%s", withLength)
	}

	plain := fetch("json")
	if strings.Count(plain, canary) != 2 || !strings.Contains(plain, `"command":"echo `+ph+`"`) || !json.Valid([]byte(plain)) {
		t.Fatalf("json response: %s", plain)
	}

	zipped := fetch("gzip")
	zr, err := gzip.NewReader(strings.NewReader(zipped))
	if err != nil {
		t.Fatalf("a compressed response must be forwarded compressed: %v", err)
	}
	unzipped, _ := io.ReadAll(zr)
	if !strings.Contains(string(unzipped), ph) || strings.Contains(string(unzipped), canary) {
		t.Fatalf("a compressed response must pass untouched: %s", unzipped)
	}

	recs := waitLedger(t, dir, 4)
	for i := range 3 {
		if recs[i].Rehydrated["text"] != 1 || recs[i].Rehydrated["edit:Edit"] != 1 || recs[i].RehydrationDenied["tool:Bash/scope"] != 1 {
			t.Fatalf("record %d: %+v %+v", i, recs[i].Rehydrated, recs[i].RehydrationDenied)
		}
	}
	if len(recs[3].Rehydrated) != 0 || len(recs[3].RehydrationDenied) != 0 {
		t.Fatalf("compressed response must count nothing: %+v", recs[3])
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, f := range files {
		data, _ := os.ReadFile(f)
		if bytes.Contains(data, []byte(canary)) || bytes.Contains(data, []byte(ph)) || bytes.Contains(data, []byte(project)) {
			t.Fatalf("ledger holds the secret, the placeholder, or the path: %s", f)
		}
	}
	for _, want := range []string{
		"rehydrated 2 placeholder(s) session=sess-rh: edit:Edit:1, text:1",
		"rehydration denied session=sess-rh: tool:Bash/scope:1",
		"rehydration skipped session=sess-rh: compressed response",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("log missing %q:\n%s", want, logs.String())
		}
	}
	if strings.Contains(logs.String(), canary) || strings.Contains(logs.String(), ph) {
		t.Fatalf("log must hold neither the secret nor the placeholder:\n%s", logs.String())
	}
	st := getStatus(t, base)
	if len(st.Sessions) != 1 || st.Sessions[0].Rehydrated != 6 || st.Sessions[0].RehydrationDenied != 3 {
		t.Fatalf("status must count rehydration: %+v", st.Sessions)
	}
	resp, err := http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	for _, want := range []string{`replay_rehydrated_total{destination="text"} 3`, `replay_rehydrated_total{destination="edit:Edit"} 3`, `replay_rehydration_denied_total{destination="tool:Bash/scope"} 3`} {
		if !strings.Contains(string(metrics), want) {
			t.Fatalf("metrics missing %q:\n%s", want, metrics)
		}
	}
}

// With the kill switch, or without a rehydrator, responses pass through
// with their placeholders and their encoding.
func TestRehydrationOffLeavesResponsesAlone(t *testing.T) {
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scopes, _ := masking.ParseScopes(t.TempDir(), nil, masking.Patterns)
	up := &rehydrationUpstream{project: t.TempDir()}
	base, _, _ := startProxyWith(t, up, Config{Masker: masking.New(vault, nil), Rehydrator: masking.NewRehydrator(vault, scopes), NoPolicy: true})
	const canary = "sk-ant-api03-CanaryCanaryCanaryCanaryCanary0123456789"
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(`{"model":"m","max_tokens":1,"messages":[{"role":"user","content":"`+canary+`"}]}`))
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Test-Mode", "json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	up.mu.Lock()
	encoding := up.encoding
	up.mu.Unlock()
	if encoding != "gzip" || strings.Contains(string(got), "REPLAY_SECRET_") {
		t.Fatalf("kill switch: encoding %q body %s", encoding, got)
	}
}

// A client that opened a connection and sent nothing must not hold the
// proxy open: agents keep pooled connections exactly like that, and
// waiting for them turned Ctrl-C into a five-second hang and a non-zero
// exit. In-flight work still gets the grace period.
func TestShutdownClosesConnectionsThatNeverSentARequest(t *testing.T) {
	upSrv := httptest.NewServer(&upstream{t: t})
	t.Cleanup(upSrv.Close)
	target, err := url.Parse(upSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store, Logger: log.New(logs, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	const grace = 100 * time.Millisecond
	srv.shutdownGrace = grace
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	// An accepted connection that never sends a request never becomes
	// idle, so a graceful shutdown alone would wait for ReadHeaderTimeout.
	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	waitFor(t, "the server to accept the connection", func() bool {
		srv.idleMu.Lock()
		defer srv.idleMu.Unlock()
		return len(srv.idleConns) == 1
	})

	cancel()
	start := time.Now()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown must not fail because a client held an unused connection: %v", err)
		}
	case <-time.After(ReadHeaderTimeout):
		t.Fatal("shutdown hung on a connection that never sent a request")
	}
	// The connection carries no turn, so it is closed at once rather than
	// waited on for the grace period.
	if took := time.Since(start); took > grace {
		t.Fatalf("shutdown waited %v on a connection with no request in flight", took)
	}
	if !strings.Contains(logs.String(), "closed 1 connection(s) with no request in flight") {
		t.Fatalf("the close must be logged:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), "turns still running") {
		t.Fatalf("nothing was in flight, so nothing should be forced:\n%s", logs.String())
	}
}

// A shutdown with nothing open returns cleanly and logs nothing about
// forcing connections closed.
func TestShutdownIsCleanWhenNothingIsOpen(t *testing.T) {
	up := &upstream{t: t}
	base, _, logs := startProxyWith(t, up, Config{})
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-shutdown"})
	http.DefaultClient.CloseIdleConnections()
	// startProxyWith's cleanup asserts ListenAndServe returned nil.
	t.Cleanup(func() {
		if strings.Contains(logs.String(), "turns still running") {
			t.Errorf("a clean shutdown must not force a turn closed:\n%s", logs.String())
		}
	})
}

// A turn still running at shutdown gets the grace period and is then
// closed, so a wedged provider cannot hold the proxy open forever.
func TestShutdownForcesATurnThatOutlastsTheGrace(t *testing.T) {
	release := make(chan struct{})
	var inFlight atomic.Bool
	upSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		inFlight.Store(true)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	// Cleanups run last registered first: the handler must be released
	// before the upstream server's Close waits for it.
	t.Cleanup(upSrv.Close)
	t.Cleanup(func() { close(release) })
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
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store, Logger: log.New(logs, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	const grace = 200 * time.Millisecond
	srv.shutdownGrace = grace
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	base := "http://" + srv.Addr()
	started := make(chan struct{})
	go func() {
		close(started)
		req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(requestBody))
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	// Let the request reach the upstream handler, which then blocks.
	waitFor(t, "the turn to reach the provider", inFlight.Load)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a forced close is not a failure: %v", err)
		}
	case <-time.After(ReadHeaderTimeout):
		t.Fatal("shutdown hung on a running turn")
	}
	if !strings.Contains(logs.String(), "turns still running after") {
		t.Fatalf("the forced close must be logged:\n%s", logs.String())
	}
	// The provider may already have billed the turn Replay cut short, so it
	// is still recorded. Waiting for the record also lets the handler
	// finish before the test's temporary directory goes away.
	if recs := waitLedger(t, dir, 1); recs[0].Path != "/v1/messages" {
		t.Fatalf("a turn cut short by shutdown must still be recorded: %+v", recs[0])
	}
}
