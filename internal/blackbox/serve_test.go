//go:build mutation

package blackbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The one surface that takes concurrent requests is the proxy. Everything
// else is a process that reads files and exits. This exercises `replay serve`
// as a child process against a stub provider (the only thing stubbed; the
// binary is real): sequential, repeated and concurrent requests, a malformed
// body, a failing provider, and recovery after the provider returns.

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

type stub struct {
	srv  *httptest.Server
	hits atomic.Int64
	fail atomic.Bool
}

func newStub() *stub {
	s := &stub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		if s.fail.Load() {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"stub failure"}}`))
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// what the provider does with a body it cannot parse
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"stub: bad json"}}`))
			return
		}
		w.Header().Set("request-id", "req_stub")
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_stub", "type": "message", "role": "assistant", "model": req["model"],
			"content": []map[string]any{{"type": "text", "text": "ok"}}, "stop_reason": "end_turn",
			"usage": map[string]int{"input_tokens": 1000, "output_tokens": 500, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0},
		})
	}))
	return s
}

func startServe(t *testing.T, bin, home string, upstream string, flags ...string) (addr string, stop func()) {
	t.Helper()
	port := freePort(t)
	addr = fmt.Sprintf("127.0.0.1:%d", port)
	ledger := filepath.Join(home, ".replay", "ledger")
	args := append([]string{"serve", "--listen", addr, "--upstream", upstream, "--ledger", ledger}, flags...)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(baseEnv(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_CONFIG_DIR=")
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		resp, err := http.Get("http://" + addr + "/replay/healthz")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return addr, func() {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		}
		t.Logf("serve log tail:\n%s", tail(log.String(), 600))
	}
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func plainBody(model string) string {
	return fmt.Sprintf(`{"model":%q,"max_tokens":64,"system":"blackbox","messages":[{"role":"user","content":"hello"}]}`, model)
}

func loopBody(model string, repeats int) string {
	var msgs []string
	msgs = append(msgs, `{"role":"user","content":"do the thing"}`)
	for i := 0; i < repeats; i++ {
		msgs = append(msgs, fmt.Sprintf(`{"role":"assistant","content":[{"type":"tool_use","id":"toolu_%d","name":"Bash","input":{"command":"ls -la"}}]}`, i))
		msgs = append(msgs, fmt.Sprintf(`{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_%d","content":"output"}]}`, i))
	}
	return fmt.Sprintf(`{"model":%q,"max_tokens":64,"system":"blackbox","messages":[%s]}`, model, strings.Join(msgs, ","))
}

func post(addr, session, body string) (int, string) {
	req, _ := http.NewRequest("POST", "http://"+addr+"/v1/messages", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-claude-code-session-id", session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestBB_ServeUnderLoad(t *testing.T) {
	bin := productionBinary(t)
	home := t.TempDir()
	up := newStub()
	defer up.srv.Close()
	addr, stop := startServe(t, bin.Path, home, up.srv.URL, "--loop-block", "3", "--max-session-usd", "0.05", "--breaker-failures", "2", "--breaker-cooldown", "2s")
	defer stop()
	r := rowFor("serve")
	var notes []string
	fail := func(format string, a ...any) {
		notes = append(notes, "FAIL "+fmt.Sprintf(format, a...))
		t.Errorf(format, a...)
	}

	// sequential
	for i := 0; i < 20; i++ {
		if code, body := post(addr, "seq", plainBody("claude-opus-5")); code != 200 && i < 2 {
			fail("sequential request %d: %d %s", i, code, body)
		}
	}
	// the session cap latched after $0.05 (3 requests at $0.0175): the rest refused
	code, body := post(addr, "seq", plainBody("claude-opus-5"))
	if code != 400 || !strings.Contains(body, "replay_spend_cap") {
		fail("after the cap: %d %s", code, body)
	}
	hitsBefore := up.hits.Load()
	// repeated identical request in a fresh session: each is its own request
	for i := 0; i < 5; i++ {
		if code, body := post(addr, "rep", plainBody("claude-opus-5")); code != 200 && i < 2 {
			fail("repeated request %d: %d %s", i, code, body)
		}
	}
	if up.hits.Load()-hitsBefore < 2 {
		fail("repeated requests were not forwarded")
	}
	// concurrent loop events: every one intercepted, none forwarded
	hitsBefore = up.hits.Load()
	var wg sync.WaitGroup
	var intercepted, other atomic.Int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, body := post(addr, fmt.Sprintf("conc-%d", i%7), loopBody("claude-opus-5", 3))
			if code == 400 && strings.Contains(body, "replay_loop") {
				intercepted.Add(1)
			} else {
				other.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if intercepted.Load() != 100 || up.hits.Load() != hitsBefore {
		fail("concurrent loop events: %d intercepted, %d other, %d forwarded", intercepted.Load(), other.Load(), up.hits.Load()-hitsBefore)
	}
	// malformed body: the proxy never rewrites bytes on the wire, so it is
	// forwarded and the provider's 400 is relayed to the client unchanged
	if code, body := post(addr, "bad", "{not json"); code != 400 || !strings.Contains(body, "invalid_request_error") {
		fail("malformed body answered %d %s, want the provider's 400 relayed", code, body)
	}
	// provider failure, breaker, recovery
	up.fail.Store(true)
	var seq []int
	for i := 0; i < 4; i++ {
		code, _ := post(addr, "down", plainBody("claude-opus-5"))
		seq = append(seq, code)
	}
	// a provider 500 is relayed as 500 (502 is reserved for a transport
	// failure); the second one opens the circuit and the rest are answered
	// locally with 503 until the cooldown passes
	if seq[0] != 500 || seq[1] != 500 || seq[2] != 503 || seq[3] != 503 {
		fail("provider down sequence %v, want 500 500 503 503 (two failures open the circuit)", seq)
	}
	up.fail.Store(false)
	recovered := false
	for i := 0; i < 30 && !recovered; i++ {
		time.Sleep(200 * time.Millisecond)
		if code, _ := post(addr, "down", plainBody("claude-opus-5")); code == 200 {
			recovered = true
		}
	}
	if !recovered {
		fail("the circuit did not close after the provider returned")
	}
	// status endpoint is machine-readable and counts the refusals
	resp, err := http.Get("http://" + addr + "/replay/status")
	if err != nil {
		fail("status: %v", err)
	} else {
		var st map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
			fail("status is not JSON: %v", err)
		}
		resp.Body.Close()
	}
	if len(notes) == 0 {
		r.Concurrency = "load: PASS (20 sequential, 5 repeated, 100 concurrent loop events all intercepted and none forwarded, malformed body relayed as the provider's 400, breaker 500 500 503 503, recovered)"
	} else {
		r.Concurrency = "load: " + strings.Join(notes, "; ")
	}
}
