package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
	"github.com/RedRobotKK/Replay/internal/proxy"
)

// Workload shape. The 45KB body is internal/proxy's benchRequest exactly
// (4KB system prompt, 20 tools, 41 messages of 800 characters), so the
// harness and BenchmarkAddedLatency measure the same request; the larger
// bodies add messages of the same shape until they reach their size.
const (
	systemPromptBytes = 4096
	toolCount         = 20
	toolDescBytes     = 200
	baseMessages      = 41
	messageTextBytes  = 800
	responseTextBytes = 1800

	messagesPath = "/v1/messages"

	// headerSSEEvents asks the mock upstream for a stream of this many
	// content deltas. The proxy forwards client headers, so it reaches the
	// upstream on both arms.
	headerSSEEvents = "x-bench-sse-events"

	// sseFinalMarker is the last event of a Messages stream. The time from
	// the read that completes it to EOF is the tail the proxy adds after the
	// provider has finished: its end-of-stream bookkeeping.
	sseFinalMarker = `{"type":"message_stop"}`

	clientTimeout       = 120 * time.Second
	clientIdleConns     = 512
	clientIdleTimeout   = 90 * time.Second
	readBufferBytes     = 32 << 10
	proxyStartupTimeout = 10 * time.Second
	proxyStopTimeout    = 30 * time.Second
)

// requestBody builds a Messages request of at least minBytes, never fewer
// than the base 41 messages. stream adds "stream":true.
func requestBody(minBytes int, stream bool) []byte {
	var b bytes.Buffer
	b.WriteString(`{"model":"claude-fable-5-1","max_tokens":1024,`)
	if stream {
		b.WriteString(`"stream":true,`)
	}
	b.WriteString(`"system":"`)
	b.WriteString(strings.Repeat("s", systemPromptBytes))
	b.WriteString(`","tools":[`)
	for i := range toolCount {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"name":"tool_%d","description":"%s",`+
			`"input_schema":{"type":"object","properties":{}}}`, i, strings.Repeat("d", toolDescBytes))
	}
	b.WriteString(`],"messages":[`)
	text := strings.Repeat("m", messageTextBytes)
	for i := 0; i < baseMessages || b.Len()+len(`]}`) < minBytes; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		fmt.Fprintf(&b, `{"role":"%s","content":[{"type":"text","text":"%s"}]}`, role, text)
	}
	b.WriteString(`]}`)
	return b.Bytes()
}

func jsonResponse() []byte {
	return []byte(`{"id":"msg_bench","type":"message","role":"assistant",` +
		`"model":"claude-fable-5-1","content":[{"type":"text","text":"` +
		strings.Repeat("x", responseTextBytes) + `"}],"stop_reason":"end_turn",` +
		`"usage":{"input_tokens":412,"output_tokens":98,` +
		`"cache_creation_input_tokens":0,"cache_read_input_tokens":40960}}`)
}

// upstreamHandler is the mock provider: it reads the whole request, then
// answers with a fixed JSON message (with Content-Length) or, when asked, an
// SSE stream flushed event by event. It never sleeps: any delay here would
// overlap the proxy's work and hide it.
func upstreamHandler() http.Handler {
	resp := jsonResponse()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("request-id", "req_bench")
		n, _ := strconv.Atoi(r.Header.Get(headerSSEEvents)) // absent or malformed means JSON
		if n <= 0 {
			w.Header().Set("content-type", "application/json")
			w.Header().Set("content-length", strconv.Itoa(len(resp)))
			_, _ = w.Write(resp) // a failed write is the client's disconnect; nothing to do
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		fl, _ := w.(http.Flusher) // httptest's writer always flushes
		write := func(s string) {
			_, _ = io.WriteString(w, s) // see above
			if fl != nil {
				fl.Flush()
			}
		}
		write("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_bench\",\"model\":\"claude-fable-5-1\",\"usage\":{\"input_tokens\":412,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":40960,\"output_tokens\":1}}}\n\n")
		write("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		for i := range n {
			write(fmt.Sprintf("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"token %d \"}}\n\n", i))
		}
		write("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		write(fmt.Sprintf("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":%d}}\n\n", n))
		write("event: message_stop\ndata: " + sseFinalMarker + "\n\n")
	})
}

func newUpstream() *httptest.Server { return httptest.NewServer(upstreamHandler()) }

func newClient() *http.Client {
	return &http.Client{
		Timeout: clientTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        clientIdleConns,
			MaxIdleConnsPerHost: clientIdleConns,
			IdleConnTimeout:     clientIdleTimeout,
			DisableCompression:  true,
		},
	}
}

// proxyInstance is one real proxy.Server on a loopback port with its own
// ledger directory, configured as `replay serve` would be.
type proxyInstance struct {
	srv     *proxy.Server
	base    string
	cancel  context.CancelFunc
	done    chan error
	dir     string
	startup time.Duration

	closeOnce sync.Once
	closeErr  error
}

func startProxy(upstream string, mask bool) (*proxyInstance, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "replay-bench-")
	if err != nil {
		return nil, err
	}
	store, err := ledger.Open(filepath.Join(dir, "ledger"))
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	cfg := proxy.Config{
		Listen:   "127.0.0.1:0",
		Upstream: target,
		Store:    store,
		Logger:   log.New(io.Discard, "", 0),
	}
	if mask {
		vault, verr := masking.OpenVault(filepath.Join(dir, "vault"))
		if verr != nil {
			return nil, errors.Join(verr, os.RemoveAll(dir))
		}
		cfg.Masker = masking.New(vault, nil)
	}
	start := time.Now()
	srv, err := proxy.New(cfg)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &proxyInstance{srv: srv, cancel: cancel, done: make(chan error, 1), dir: dir}
	go func() { p.done <- srv.ListenAndServe(ctx) }()
	ready := make(chan string, 1)
	go func() { ready <- srv.Addr() }()
	select {
	case addr := <-ready:
		p.base = "http://" + addr
		p.startup = time.Since(start)
	case err := <-p.done:
		cancel()
		return nil, errors.Join(fmt.Errorf("proxy exited before listening: %w", err), os.RemoveAll(dir))
	case <-time.After(proxyStartupTimeout):
		cancel()
		return nil, errors.Join(errors.New("proxy did not start listening"), os.RemoveAll(dir))
	}
	return p, nil
}

// Close stops the proxy and removes its directory. It is idempotent: a
// second call returns the first call's error instead of waiting for an exit
// that has already been received.
func (p *proxyInstance) Close() error {
	p.closeOnce.Do(func() {
		p.cancel()
		var err error
		select {
		case err = <-p.done:
		case <-time.After(proxyStopTimeout):
			err = errors.New("proxy did not stop")
		}
		p.closeErr = errors.Join(err, os.RemoveAll(p.dir))
	})
	return p.closeErr
}

// timing is one request seen from the client.
type timing struct {
	total time.Duration // request start to EOF
	ttfb  time.Duration // request start to first body byte
	tail  time.Duration // SSE only: final event read to EOF
}

// do sends one request and reads the response to EOF, timing it from the
// client. Durations come from time.Since, which uses the monotonic clock.
func do(client *http.Client, base string, body []byte, sseEvents int, session string) (timing, error) {
	req, err := http.NewRequest(http.MethodPost, base+messagesPath, bytes.NewReader(body))
	if err != nil {
		return timing{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", "sk-bench-not-a-key")
	req.Header.Set("anthropic-version", "2023-06-01")
	if sseEvents > 0 {
		req.Header.Set(headerSSEEvents, strconv.Itoa(sseEvents))
	}
	if session != "" {
		req.Header.Set(proxy.HeaderSessionID, session)
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return timing{}, err
	}
	defer func() { _ = resp.Body.Close() }() // read to EOF below; close cannot lose data
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, readBufferBytes)) // best effort, for the error text
		return timing{}, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var t timing
	var lastEvent time.Time
	buf := make([]byte, readBufferBytes)
	marker := []byte(sseFinalMarker)
	var carry []byte
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			now := time.Now()
			if t.ttfb == 0 {
				t.ttfb = now.Sub(start)
			}
			if sseEvents > 0 && lastEvent.IsZero() {
				window := make([]byte, 0, len(carry)+n)
				window = append(window, carry...)
				window = append(window, buf[:n]...)
				if bytes.Contains(window, marker) {
					lastEvent = now
				}
				keep := min(len(window), len(marker)-1)
				carry = append(carry[:0], window[len(window)-keep:]...)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return timing{}, rerr
		}
	}
	end := time.Now()
	t.total = end.Sub(start)
	if sseEvents > 0 {
		if lastEvent.IsZero() {
			return timing{}, errors.New("stream ended without message_stop")
		}
		t.tail = end.Sub(lastEvent)
	}
	return t, nil
}
