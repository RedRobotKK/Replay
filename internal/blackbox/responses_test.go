//go:build mutation

package blackbox

// R-1, proven on the shipped binary. The parser and proxy packages prove the
// chain in-process; this proves the PRODUCTION binary reaches that code: a
// child `replay serve` is pointed at a fake Responses upstream, a Codex-shaped
// request is posted to /v1/responses, non-streaming and streaming, and the
// result is read back from the ledger files on disk and priced by `replay
// cost` on the same files. The spend cap is proven by a refusal the binary
// itself issues. Nothing is stubbed but the provider.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func responsesFixtureBB(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "ledger", "testdata", "openai-responses", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// responsesStub answers /v1/responses with one fixture, switchable between
// the non-streaming reply and the stream.
type responsesStub struct {
	srv   *httptest.Server
	hits  atomic.Int64
	mu    sync.Mutex
	body  []byte
	ctype string
	paths []string
}

func newResponsesStub(t *testing.T) *responsesStub {
	s := &responsesStub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		s.hits.Add(1)
		s.mu.Lock()
		s.paths = append(s.paths, r.URL.Path)
		body, ctype := s.body, s.ctype
		s.mu.Unlock()
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(200)
		_, _ = w.Write(body)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *responsesStub) serve(body []byte, ctype string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body, s.ctype = body, ctype
}

func postResponsesBB(addr string, body []byte) (int, []byte) {
	resp, err := http.Post("http://"+addr+"/v1/responses", "application/json", bytes.NewReader(body))
	if err != nil {
		return -1, []byte(err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// ledgerRecords reads every record the binary wrote, as JSON, without going
// through the ledger package: the point is what is on disk.
func ledgerRecords(t *testing.T, dir string) []map[string]any {
	t.Helper()
	var out []map[string]any
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range bytes.Split(b, []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var rec map[string]any
			if json.Unmarshal(line, &rec) == nil {
				out = append(out, rec)
			}
		}
	}
	return out
}

func usageOf(rec map[string]any) map[string]any {
	resp, _ := rec["response"].(map[string]any)
	u, _ := resp["usage"].(map[string]any)
	return u
}

func TestBB_ResponsesPathReachesTheLedgerAndTheEconomics(t *testing.T) {
	bin := productionBinary(t)
	home := t.TempDir()
	// The OpenAI rules document, installed the way an operator installs it,
	// so the binary prices gpt-6-astra rather than bounding it.
	if r := run(bin.Path, home, "", "rules", "--update", filepath.Join(repoRoot(t), "docs", "rules", "openai-2026-09-15.json")); r.Exit != 0 {
		t.Fatalf("rules --update: exit %d\n%s%s", r.Exit, r.Stdout, r.Stderr)
	}
	up := newResponsesStub(t)
	up.serve(responsesFixtureBB(t, "nonstream-cached-reasoning.json"), "application/json")
	addr, stop := startServe(t, bin.Path, home, up.srv.URL, "--max-session-usd", "0.05")
	defer stop()
	ledger := filepath.Join(home, ".replay", "ledger")
	req := responsesFixtureBB(t, "request-codex.json")

	// 1. non-streaming, three times under a $0.05 cap at $0.021 each: the
	// fourth is refused by the binary, so the usage reached the cap.
	for i := 1; i <= 3; i++ {
		if code, body := postResponsesBB(addr, req); code != 200 {
			t.Fatalf("request %d: %d %s", i, code, body)
		}
		waitRecords(t, ledger, i)
	}
	if code, body := postResponsesBB(addr, req); code != 400 || !strings.Contains(string(body), "replay_spend_cap") {
		t.Fatalf("the fourth request under the cap was not refused by the binary: %d %s", code, body)
	}
	if up.hits.Load() != 3 {
		t.Errorf("upstream hits = %d, want 3", up.hits.Load())
	}
	// The refusal is itself a ledger record, as on the other paths.
	waitRecords(t, ledger, 4)

	// 2. streaming, in a fresh session (another prompt_cache_key).
	up.serve(responsesFixtureBB(t, "stream-completed.sse"), "text/event-stream")
	streamReq := bytes.Replace(req, []byte(`"prompt_cache_key":"019a2c6e-7f3b-7c1d-9d2e-4b5f6a7b8c9d"`), []byte(`"prompt_cache_key":"0199ffff-0000-7000-8000-000000000000"`), 1)
	code, got := postResponsesBB(addr, streamReq)
	if code != 200 || !bytes.Equal(got, responsesFixtureBB(t, "stream-completed.sse")) {
		t.Fatalf("stream: %d, delivered %d bytes, want the fixture byte for byte", code, len(got))
	}
	recs := waitRecords(t, ledger, 5)

	// 3. what is on disk: four provider requests and one refusal.
	sessions := map[string]int{}
	refusals := 0
	for i, rec := range recs {
		if kind, ok := rec["refusal"]; ok {
			refusals++
			if kind != "spend_cap" {
				t.Errorf("record %d: refusal %v, want spend_cap", i, kind)
			}
			continue
		}
		if rec["path"] != "/v1/responses" {
			t.Errorf("record %d: path %v", i, rec["path"])
		}
		if rec["model"] != "gpt-6-astra" {
			t.Errorf("record %d: model %v", i, rec["model"])
		}
		sid, _ := rec["session_id"].(string)
		sessions[sid]++
		u := usageOf(rec)
		if u == nil {
			t.Fatalf("record %d carries no usage: %v", i, rec)
		}
		for k, want := range map[string]float64{"input_tokens": 500, "cache_read_input_tokens": 1000, "output_tokens": 300, "thinking_tokens": 200} {
			if u[k] != want {
				t.Errorf("record %d: %s = %v, want %v", i, k, u[k], want)
			}
		}
		if _, ok := u["cache_creation_input_tokens"]; ok && u["cache_creation_input_tokens"] != float64(0) {
			t.Errorf("record %d: a cache write was invented: %v", i, u["cache_creation_input_tokens"])
		}
		if _, ok := rec["cache"]; ok {
			t.Errorf("record %d carries a cache classification on the Responses path: %v", i, rec["cache"])
		}
		if raw, _ := rec["response"].(map[string]any)["raw_usage"].(map[string]any); raw == nil {
			t.Errorf("record %d: raw usage not kept", i)
		}
	}
	if len(sessions) != 2 || sessions[""] != 0 {
		t.Errorf("sessions on disk = %v, want two keyed sessions", sessions)
	}
	if refusals != 1 {
		t.Errorf("refusal records = %d, want 1", refusals)
	}
	for _, rec := range recs {
		b, _ := json.Marshal(rec)
		for _, secret := range []string{"list the files", "You are Codex", "Running the listing", "019a2c6e-7f3b"} {
			if bytes.Contains(b, []byte(secret)) {
				t.Errorf("the ledger holds request content or the raw cache key: %q", secret)
			}
		}
	}

	// 4. the existing analysis prices the same files.
	r := run(bin.Path, home, "", "cost", "--json", ledger)
	if r.Exit != 0 {
		t.Fatalf("cost --json: exit %d\n%s%s", r.Exit, r.Stdout, r.Stderr)
	}
	var cost map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &cost); err != nil {
		t.Fatalf("cost --json is not JSON: %v\n%s", err, r.Stdout)
	}
	summary, _ := cost["summary"].(map[string]any)
	total, _ := summary["totalUsd"].(float64)
	priced, _ := summary["pricedRequests"].(float64)
	if priced != 4 || total < 0.08 || total > 0.09 {
		t.Errorf("cost: pricedRequests=%v totalUsd=%v, want 4 requests at $0.021 (%s)", priced, total, r.Stdout)
	}
	fmt.Fprintf(os.Stderr, "BB responses: %d records, %d sessions, cost %.4f over %v priced requests\n", len(recs), len(sessions), total, priced)
}

func waitRecords(t *testing.T, dir string, n int) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		recs := ledgerRecords(t, dir)
		if len(recs) >= n || time.Now().After(deadline) {
			if len(recs) < n {
				t.Fatalf("ledger has %d records, want %d", len(recs), n)
			}
			return recs
		}
		time.Sleep(50 * time.Millisecond)
	}
}
