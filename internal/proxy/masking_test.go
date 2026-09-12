package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/RedRobotKK/Replay/internal/masking"
)

// Tests for masking.go: the request half (secrets leave as placeholders,
// deterministically, and a vault failure blind-scrubs rather than falling
// back to the original body) and the response half (placeholders come back
// only within scope, a compressed response is forwarded untouched and the
// skip is logged).
//
// They sit beside the code they drive because masking is the one thing in
// this program that fails secure, and a test that has to be found in a
// two-thousand-line file is a test the next reader does not check before
// widening the guard. Nothing in them changed in the move; the fail-closed
// case they do NOT cover from the outside still lives in
// maskfailclosed_test.go.

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
