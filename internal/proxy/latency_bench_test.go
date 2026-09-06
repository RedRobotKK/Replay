package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The README publishes a figure for the latency the proxy adds, and
// docs/evidence/proxy-latency-2026-09-03.md records how it was taken. That was a
// one-off on a machine nobody else has, which makes it the kind of claim that is
// true on the day it is written and silently false a hundred commits later.
//
// This reproduces the shape of that measurement so the number can be regenerated
// rather than remembered:
//
//	go test ./internal/proxy -run '^$' -bench BenchmarkAddedLatency -benchtime 300x
//
// It deliberately does not assert a threshold. Shared CI runners are noisy enough
// that a tight bound would fail for reasons that have nothing to do with this
// code, and a benchmark that cries wolf gets muted, which is worse than no
// benchmark at all. It reports; a human compares it against the evidence file.

// benchResponse is the fixed ~2KB Messages reply the evidence file describes.
func benchResponse() []byte {
	return []byte(`{"id":"msg_bench","type":"message","role":"assistant",` +
		`"model":"claude-fable-5-1","content":[{"type":"text","text":"` +
		strings.Repeat("x", 1800) + `"}],"stop_reason":"end_turn",` +
		`"usage":{"input_tokens":412,"output_tokens":98,` +
		`"cache_creation_input_tokens":0,"cache_read_input_tokens":40960}}`)
}

// benchRequest builds a body in the same range as the evidence file's 46KB:
// a system prompt, tool definitions, and a run of messages with long results.
func benchRequest() []byte {
	var b bytes.Buffer
	b.WriteString(`{"model":"claude-fable-5-1","max_tokens":1024,"system":"`)
	b.WriteString(strings.Repeat("s", 4096))
	b.WriteString(`","tools":[`)
	for i := range 20 {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"name":"tool_%d","description":"%s",`+
			`"input_schema":{"type":"object","properties":{}}}`, i, strings.Repeat("d", 200))
	}
	b.WriteString(`],"messages":[`)
	for i := range 41 {
		if i > 0 {
			b.WriteString(",")
		}
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		fmt.Fprintf(&b, `{"role":"%s","content":[{"type":"text","text":"%s"}]}`,
			role, strings.Repeat("m", 800))
	}
	b.WriteString(`]}`)
	return b.Bytes()
}

// percentile returns the p-th percentile of an already-sorted slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(p * float64(len(sorted)-1))
	return sorted[i]
}

// sample sends n requests to base and returns the sorted per-request durations,
// discarding the first warm requests.
func sample(b *testing.B, client *http.Client, base string, body []byte, n, warm int) []time.Duration {
	b.Helper()
	out := make([]time.Duration, 0, n)
	for i := range n + warm {
		req, err := http.NewRequest(http.MethodPost, base+messagesPath, bytes.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("content-type", "application/json")
		req.Header.Set("x-api-key", "sk-bench")
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		elapsed := time.Since(start)
		if i >= warm {
			out = append(out, elapsed)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// BenchmarkAddedLatency reports the difference between going straight to a local
// fake provider and going through the proxy, so the figure is the proxy's own
// overhead and not the network's. b.N is the sample count per path.
//
// It runs at two upstream delays, and the gap between them is the point:
//
//	instant    the proxy's total work, with nothing to overlap it against. A
//	           worst case no real provider produces, and the number to watch for
//	           regressions because it hides nothing.
//	realistic  a provider that takes 44ms, as the evidence file's fake did. The
//	           proxy's work overlaps the round trip, so most of it never reaches
//	           the caller's wall clock. This is the figure the README publishes
//	           and the one a user actually experiences.
//
// Reading only the first number and calling it a regression against the README
// would be comparing two different quantities.
func BenchmarkAddedLatency(b *testing.B) {
	for _, up := range []struct {
		name  string
		delay time.Duration
	}{
		{"instant", 0},
		{"realistic-44ms", 44 * time.Millisecond},
	} {
		b.Run(up.name, func(b *testing.B) { addedLatency(b, up.delay) })
	}
}

func addedLatency(b *testing.B, upstreamDelay time.Duration) {
	resp := benchResponse()
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if upstreamDelay > 0 {
			time.Sleep(upstreamDelay)
		}
		w.Header().Set("content-type", "application/json")
		w.Header().Set("request-id", "req_bench")
		_, _ = w.Write(resp)
	})
	upSrv := httptest.NewServer(up)
	defer upSrv.Close()

	target, err := url.Parse(upSrv.URL)
	if err != nil {
		b.Fatal(err)
	}
	store, err := ledger.Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	srv, err := New(Config{
		Listen:   "127.0.0.1:0",
		Upstream: target,
		Store:    store,
		Logger:   log.New(io.Discard, "", 0),
	})
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			b.Errorf("server exit: %v", err)
		}
	}()

	body := benchRequest()
	client := &http.Client{Timeout: 30 * time.Second}
	const warm = 20

	b.ResetTimer()
	direct := sample(b, client, upSrv.URL, body, b.N, warm)
	proxied := sample(b, client, "http://"+srv.Addr(), body, b.N, warm)
	b.StopTimer()

	for _, p := range []struct {
		name string
		q    float64
	}{{"p50", 0.50}, {"p99", 0.99}} {
		added := percentile(proxied, p.q) - percentile(direct, p.q)
		b.ReportMetric(float64(added.Nanoseconds())/1000, "added_"+p.name+"_µs")
	}
	b.ReportMetric(float64(len(body)), "request_bytes")
}
