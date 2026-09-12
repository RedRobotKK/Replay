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

func TestVary_MissingUsageIsNotAZeroRead(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			n := 8 + len(raw)
			w.Header().Set("content-type", "application/json")
			fmt.Fprintf(w, `{"input_tokens":%d}`, n)
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

type fakeCache struct {
	lastKey  string
	lastSize int
}

func newFakeCache(t *testing.T) *httptest.Server {
	t.Helper()
	f := &fakeCache{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, "read", 500)
			return
		}
		if strings.Contains(r.URL.Path, "count_tokens") {
			n := 7 + len(raw)/2
			w.Header().Set("content-type", "application/json")
			fmt.Fprintf(w, `{"input_tokens":%d}`, n)
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
		if f.lastKey == key {
			read, write = f.lastSize, 0
		} else {
			f.lastKey, f.lastSize = key, size
		}
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w, `{"id":"m","usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":1}}`,
			size, write, read)
	}))
}
