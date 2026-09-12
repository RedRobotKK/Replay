package probe

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlanVary_SendsNothing(t *testing.T) {
	var out bytes.Buffer
	r := &Runner{Out: &out, BaseURL: "http://127.0.0.1:1"}
	if err := r.PlanVary("claude-opus-5", VaryTools); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Nothing has been sent yet") {
		t.Fatalf("plan must say nothing was sent:\n%s", got)
	}
	if !strings.Contains(got, VaryTools) {
		t.Fatalf("plan must name the term:\n%s", got)
	}
	if !strings.Contains(got, "budget       3 billable") {
		t.Fatalf("plan must budget the control arm:\n%s", got)
	}
}

func TestPlanVary_UnknownTerm(t *testing.T) {
	r := &Runner{Out: io.Discard}
	if err := r.PlanVary("claude-opus-5", "temperature"); err == nil {
		t.Fatal("unknown term must fail before any request")
	}
}

// The fake provider keys the cache on system text + tools JSON. Effort is
// ignored. Varying tools must drop cache_read; varying effort must not.
func TestVary_ToolsMoveTheKeyAndEffortDoesNot(t *testing.T) {
	up := newFakeCache(t)
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}

	tools, err := r.Vary("claude-opus-5", VaryTools)
	if err != nil {
		t.Fatal(err)
	}
	if !tools.Moved {
		t.Fatalf("tools are in this fake key; cache_read should drop: %+v", tools)
	}
	effort, err := r.Vary("claude-opus-5", VaryEffort)
	if err != nil {
		t.Fatal(err)
	}
	if effort.Moved {
		t.Fatalf("effort is not in this fake key; cache_read should hold: %+v", effort)
	}
}

func TestVary_ControlArmMakesADeadCacheInconclusive(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 8+len(raw))
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m","usage":{"input_tokens":2000,"cache_creation_input_tokens":2000,"cache_read_input_tokens":0,"output_tokens":1}}`))
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	got, err := r.Vary("claude-opus-5", VaryTools)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Inconclusive || got.Moved {
		t.Fatalf("a provider that never reads must be inconclusive, not Moved: %+v", got)
	}
}

func TestVary_MissingUsageIsNotAZeroRead(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			n := 8 + len(raw)
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, n)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil || !strings.Contains(err.Error(), "no usage") {
		t.Fatalf("missing usage must not look like cache_read=0: %v", err)
	}
}

// fakeCountTokens is what every fake in this file answers a count_tokens call
// with, and it has to VARY WITH THE BODY.
//
// sizedFiller measures the envelope once and subtracts it from every later
// count, so a fake answering a constant reports a prefix of exactly zero
// tokens and Vary stops with "the provider counted the probe prefix as no
// tokens" before it sends a single messages request. A test written that way
// passes for the wrong reason when it expects an error and hangs the search
// when it does not — neither failure names the fake.
func fakeCountTokens(raw []byte) int { return 7 + len(raw)/2 }

type fakeCache struct {
	store map[string]int
}

func newFakeCache(t *testing.T) *httptest.Server {
	t.Helper()
	f := &fakeCache{store: map[string]int{}}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, "read", 500)
			return
		}
		if strings.Contains(r.URL.Path, "count_tokens") {
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, fakeCountTokens(raw))
			return
		}
		var body struct {
			System []struct {
				Text string `json:"text"`
			} `json:"system"`
			Tools  json.RawMessage `json:"tools"`
			Effort string          `json:"effort"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			http.Error(w, "json", 400)
			return
		}
		sys := ""
		if len(body.System) > 0 {
			sys = body.System[0].Text
		}
		sum := sha256.Sum256(append([]byte(sys), body.Tools...))
		key := fmt.Sprintf("%x", sum[:8])
		size := 2000
		read, write := 0, size
		if n, ok := f.store[key]; ok {
			read, write = n, 0
		} else {
			f.store[key] = size
		}
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"m","usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":1}}`,
			size, write, read)
	}))
}

func TestPlanVary_BillingHeaderIsUnknown(t *testing.T) {
	r := &Runner{Out: io.Discard}
	if err := r.PlanVary("claude-opus-5", "billing-header"); err == nil {
		t.Fatal("billing-header varies system text, not a header; it is not a term")
	}
}

func TestVary_UnknownTerm(t *testing.T) {
	r := &Runner{Out: io.Discard, BaseURL: "http://127.0.0.1:1"}
	_, err := r.Vary("claude-opus-5", "temperature")
	if err == nil {
		t.Fatal("unknown term must fail before any request")
	}
	// The MESSAGE, not just the failure. The base URL here refuses every
	// connection, so a Vary that skipped the term check would still come back
	// with an error and this test would pass on the wrong one — which is what
	// guard-reachability reported when it neutralised the check and nothing
	// went red. Naming the term is what says the run stopped at the argument
	// rather than at the wire.
	if !strings.Contains(err.Error(), `unknown vary term "temperature"`) {
		t.Fatalf("the term check must name the term, not the network: %v", err)
	}
	if !strings.Contains(err.Error(), "tools, system, effort") {
		t.Fatalf("an unknown term must list the ones that exist: %v", err)
	}
}

func TestVary_SystemMovesTheKey(t *testing.T) {
	up := newFakeCache(t)
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	got, err := r.Vary("claude-opus-5", VarySystem)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Moved {
		t.Fatalf("system text is in this fake key; cache_read should miss: %+v", got)
	}
}

func TestVary_EffortVariantSendsLow(t *testing.T) {
	var efforts []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, "read", 500)
			return
		}
		if strings.Contains(r.URL.Path, "count_tokens") {
			n := 7 + len(raw)/2
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, n)
			return
		}
		var body struct {
			Effort string `json:"effort"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			http.Error(w, "json", 400)
			return
		}
		efforts = append(efforts, body.Effort)
		w.Header().Set("content-type", "application/json")
		read := 0
		write := 2000
		if len(efforts) > 1 {
			read, write = 2000, 0
		}
		_, _ = fmt.Fprintf(w, `{"id":"m","usage":{"input_tokens":2000,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":1}}`, write, read)
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	if _, err := r.Vary("claude-opus-5", VaryEffort); err != nil {
		t.Fatal(err)
	}
	if len(efforts) != 3 {
		t.Fatalf("want 3 messages requests, got %d %v", len(efforts), efforts)
	}
	if efforts[0] != "high" || efforts[1] != "low" || efforts[2] != "high" {
		t.Fatalf("effort variant must send low on request 2: %v", efforts)
	}
}

func TestVary_NilClientStillSends(t *testing.T) {
	up := newFakeCache(t)
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: nil, Out: io.Discard}
	if _, err := r.Vary("claude-opus-5", VaryTools); err != nil {
		t.Fatal(err)
	}
}

func TestSendVary_BadBaseURL(t *testing.T) {
	r := &Runner{BaseURL: ":", APIKey: "k", Out: io.Discard}
	_, err := r.sendVary("claude-opus-5", "filler", VaryTools, false)
	if err == nil {
		t.Fatal("an unusable base URL must fail before a request is sent")
	}
}

func TestVary_ProviderDownOnMessages(t *testing.T) {
	r := &Runner{BaseURL: "http://127.0.0.1:1", APIKey: "k", Out: io.Discard,
		Client: &http.Client{Transport: failMessages{}}}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil || !strings.Contains(err.Error(), "probe request failed") {
		t.Fatalf("messages transport error: %v", err)
	}
}

func TestVary_ReadBodyFails(t *testing.T) {
	r := &Runner{BaseURL: "http://vary.test", APIKey: "k", Out: io.Discard,
		Client: &http.Client{Transport: failRead{}}}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil {
		t.Fatal("a body that cannot be read must fail")
	}
	// A read that fails halfway is not an answer that carried no usage.
	// Without the read check the empty bytes reach parseVaryUsage and come
	// back as "carried no usage", which reads as a provider that answered
	// without billing rather than as a transport that broke.
	if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("a broken read must surface as one, not as absent usage: %v", err)
	}
}

func TestVary_ProviderStatus(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, fakeCountTokens(raw))
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("non-200 must name the status: %v", err)
	}
}

func TestVary_FirstRequestFails(t *testing.T) {
	n := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, fakeCountTokens(raw))
			return
		}
		n++
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"id":"m","usage":{"input_tokens":2000,"cache_creation_input_tokens":2000,"cache_read_input_tokens":0,"output_tokens":1}}`))
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("first messages failure must stop the run: %v", err)
	}
}

func TestVary_SecondRequestFails(t *testing.T) {
	n := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, fakeCountTokens(raw))
			return
		}
		n++
		if n == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"id":"m","usage":{"input_tokens":2000,"cache_creation_input_tokens":2000,"cache_read_input_tokens":0,"output_tokens":1}}`))
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("variant failure must stop the run: %v", err)
	}
}

func TestVary_ThirdRequestFails(t *testing.T) {
	n := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, fakeCountTokens(raw))
			return
		}
		n++
		if n == 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"id":"m","usage":{"input_tokens":2000,"cache_creation_input_tokens":2000,"cache_read_input_tokens":0,"output_tokens":1}}`))
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("control failure must stop the run: %v", err)
	}
}

func TestVary_BelowFloor(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "count_tokens") {
			n := 10
			if len(raw) > 200 {
				n = 100
			}
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, n)
			return
		}
		http.Error(w, "should not send", 500)
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil || !strings.Contains(err.Error(), "minimum cacheable prefix") {
		t.Fatalf("a prefix under the floor must refuse: %v", err)
	}
}

func TestVary_SizedFillerFails(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		http.Error(w, "should not send", 500)
	}))
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	_, err := r.Vary("claude-opus-5", VaryTools)
	if err == nil {
		t.Fatal("a counting endpoint that fails must fail the run")
	}
	// Which failure. Dropping the sizedFiller error check leaves a zero-token
	// filler, and the floor check below it then refuses the run for a
	// different reason — an error either way, so a bare err == nil assertion
	// cannot tell the two apart and passes with the check deleted.
	if !strings.Contains(err.Error(), "asked to count tokens") {
		t.Fatalf("a counting failure must surface as one, not as a floor refusal: %v", err)
	}
}

func TestVary_LegacyFloorRaisesTarget(t *testing.T) {
	up := newFakeCache(t)
	defer up.Close()
	r := &Runner{BaseURL: up.URL, APIKey: "k", Client: up.Client(), Out: io.Discard}
	if _, err := r.Vary("claude-opus-4-6", VaryTools); err != nil {
		t.Fatalf("opus-4-6 floors at 4096; the filler must be raised above it: %v", err)
	}
}

func TestParseVaryUsage_BrokenJSON(t *testing.T) {
	_, err := parseVaryUsage([]byte(`{`))
	if err == nil {
		t.Fatal("broken JSON must not look like usage")
	}
	// Unreadable and empty are different findings. With the unmarshal error
	// dropped, parsed.Usage is nil and the run reports "carried no usage" —
	// a claim about what the provider said, made about bytes nobody could
	// read. The two messages are the whole point of keeping the check.
	if !strings.Contains(err.Error(), "could not be read as usage") {
		t.Fatalf("unreadable bytes must say so, not report absent usage: %v", err)
	}
	if strings.Contains(err.Error(), "carried no usage") {
		t.Fatalf("unreadable is not the same finding as absent: %v", err)
	}
}

func TestParseVaryUsage_ZeroInput(t *testing.T) {
	_, err := parseVaryUsage([]byte(`{"id":"x","usage":{"input_tokens":0,"cache_read_input_tokens":0}}`))
	if err == nil || !strings.Contains(err.Error(), "no usage") {
		t.Fatalf("zero input is absence: %v", err)
	}
}

func TestMustJSON_PanicsOnUnmarshalable(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("mustJSON must panic on a value json cannot encode")
		}
	}()
	mustJSON(make(chan int))
}

// countingRoundTrip answers count_tokens the way the httptest fakes above do,
// so the sized-filler search converges, and hands every other path to next.
func countingRoundTrip(req *http.Request, next func() (*http.Response, error)) (*http.Response, error) {
	if !strings.Contains(req.URL.Path, "count_tokens") {
		return next()
	}
	raw := []byte{}
	if req.Body != nil {
		raw, _ = io.ReadAll(req.Body)
	}
	body := fmt.Sprintf(`{"input_tokens":%d}`, fakeCountTokens(raw))
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req}, nil
}

type failMessages struct{}

func (failMessages) RoundTrip(req *http.Request) (*http.Response, error) {
	return countingRoundTrip(req, func() (*http.Response, error) {
		return nil, fmt.Errorf("down")
	})
}

type failRead struct{}

func (failRead) RoundTrip(req *http.Request) (*http.Response, error) {
	return countingRoundTrip(req, func() (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: errReader{}, Header: make(http.Header), Request: req}, nil
	})
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (errReader) Close() error             { return nil }
