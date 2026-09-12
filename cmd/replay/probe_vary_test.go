package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProbeVary_PlansWithoutExecute(t *testing.T) {
	var hits atomic.Int64
	up := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	defer up.Close()
	t.Setenv("ANTHROPIC_BASE_URL", up.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")

	var stdout, stderr bytes.Buffer
	if err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "tools"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Nothing has been sent yet") {
		t.Fatalf("plan must say nothing was sent:\n%s", stdout.String())
	}
	if hits.Load() != 0 {
		t.Fatalf("plan sent %d request(s)", hits.Load())
	}
}

func TestProbeVary_EmptyBaseURLDefaults(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")
	var stdout, stderr bytes.Buffer
	err := runProbe(strings.NewReader("no\n"), []string{"--model", "claude-opus-5", "--vary", "tools", "--execute"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("declining must refuse: %v", err)
	}
	if !strings.Contains(stdout.String(), "api.anthropic.com") {
		t.Fatalf("empty ANTHROPIC_BASE_URL must name the default host:\n%s", stdout.String())
	}
}

func TestProbeVary_MissingKeyRefuses(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("ANTHROPIC_API_KEY", "")
	var stdout, stderr bytes.Buffer
	err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "tools", "--execute", "--yes"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("missing key must name the env var: %v", err)
	}
}

func TestProbeVary_UnknownTerm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "temperature"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown vary term") {
		t.Fatalf("unknown term: %v", err)
	}
}

func TestProbeVary_UnconfirmedExecuteSendsNothing(t *testing.T) {
	var hits atomic.Int64
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1}}`))
	}))
	defer up.Close()
	t.Setenv("ANTHROPIC_BASE_URL", up.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")
	var stdout, stderr bytes.Buffer
	err := runProbe(strings.NewReader("no\n"), []string{"--model", "claude-opus-5", "--vary", "tools", "--execute"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "nothing was sent") {
		t.Fatalf("declining must refuse: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("provider received %d request(s)", hits.Load())
	}
}

func TestProbeVary_ExecuteReportsMoved(t *testing.T) {
	up := newCLIFakeCache(t)
	defer up.Close()
	t.Setenv("ANTHROPIC_BASE_URL", up.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")
	var stdout, stderr bytes.Buffer
	if err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "tools", "--execute", "--yes"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "This term is in the provider cache key") {
		t.Fatalf("tools move this fake key:\n%s", out)
	}
}

func TestProbeVary_ExecuteReportsInconclusive(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 8+len(raw))
			return
		}
		_, _ = w.Write([]byte(`{"id":"m","usage":{"input_tokens":2000,"cache_creation_input_tokens":2000,"cache_read_input_tokens":0,"output_tokens":1}}`))
	}))
	defer up.Close()
	t.Setenv("ANTHROPIC_BASE_URL", up.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")
	var stdout, stderr bytes.Buffer
	if err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "tools", "--execute", "--yes"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	// The RESULT line, not the word. The plan printed above it already says
	// "or the run is inconclusive", so matching "inconclusive" against the
	// whole of stdout passes whichever branch the switch actually took —
	// which is exactly what guard-reachability found when it neutralised the
	// Inconclusive case and the suite stayed green.
	out := stdout.String()
	if !strings.Contains(out, "caching is not working in this window") {
		t.Fatalf("a provider that never reads must be inconclusive:\n%s", out)
	}
	if strings.Contains(out, "was not observed in the provider cache key") {
		t.Fatalf("a dead cache is not a term that held:\n%s", out)
	}
}

func TestProbeVary_ExecuteReportsHeld(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 8+len(raw))
			return
		}
		_, _ = w.Write([]byte(`{"id":"m","usage":{"input_tokens":2000,"cache_creation_input_tokens":0,"cache_read_input_tokens":2000,"output_tokens":1}}`))
	}))
	defer up.Close()
	t.Setenv("ANTHROPIC_BASE_URL", up.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")
	var stdout, stderr bytes.Buffer
	if err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "effort", "--execute", "--yes"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "was not observed") {
		t.Fatalf("a provider that always reads must not claim a move:\n%s", stdout.String())
	}
}

func TestProbeVary_ExecutePropagatesProviderError(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			// Body-derived, not a constant: sizedFiller subtracts the
			// envelope it measured, and a fixed count makes the prefix
			// zero tokens and stops the run before it reaches the
			// provider error this test is about.
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 8+len(raw))
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer up.Close()
	t.Setenv("ANTHROPIC_BASE_URL", up.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")
	var stdout, stderr bytes.Buffer
	err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "tools", "--execute", "--yes"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("provider error must surface: %v", err)
	}
}

func newCLIFakeCache(t *testing.T) *httptest.Server {
	t.Helper()
	store := map[string]int{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read", 500)
			return
		}
		if strings.Contains(r.URL.Path, "count_tokens") {
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 7+len(raw)/2)
			return
		}
		var body struct {
			System []struct {
				Text string `json:"text"`
			} `json:"system"`
			Tools json.RawMessage `json:"tools"`
		}
		_ = json.Unmarshal(raw, &body)
		sys := ""
		if len(body.System) > 0 {
			sys = body.System[0].Text
		}
		key := sys + string(body.Tools)
		size := 2000
		read, write := 0, size
		if n, ok := store[key]; ok {
			read, write = n, 0
		} else {
			store[key] = size
		}
		_, _ = fmt.Fprintf(w, `{"id":"m","usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":1}}`,
			size, write, read)
	}))
}
