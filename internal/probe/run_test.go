package probe

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Running probes against a real provider.
//
// This is the first thing in Replay that originates a billable request. The
// proxy forwards what an agent already sent; this creates traffic on purpose,
// with the operator's credential, and spends their money to learn something.
// Every constraint below follows from that.
//
//	R1  a plan is shown and nothing is sent until execution is asked for
//	R2  the credential is never written to output, however the run ends
//	R3  the credential comes from the environment, never from a flag
//	R4  each probe is unique, so it can never read an existing cache entry
//	R5  the run stops at the authorised number of requests
//	R6  a provider error ends the run rather than being counted as evidence

func newRunner(t *testing.T, h http.HandlerFunc) (*Runner, *httptest.Server, *bytes.Buffer) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	var out bytes.Buffer
	return &Runner{
		BaseURL: srv.URL,
		APIKey:  "sk-ant-SECRETVALUE-do-not-print",
		Client:  srv.Client(),
		Out:     &out,
	}, srv, &out
}

// R1: PASS: Plan writes the sizes and the cost, and sends nothing.
// FAIL: a request during planning. Money must not move because someone ran a
// command to find out what it would cost.
func TestR1_PlanSendsNothing(t *testing.T) {
	var requests int
	r, _, out := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(200)
	})
	r.Plan(Config{Min: 0, Max: 65536, Resolution: 512, MaxProbes: 12, Confirm: 1}, "claude-opus-5")

	if requests != 0 {
		t.Errorf("planning sent %d request(s); it must send none", requests)
	}
	s := out.String()
	for _, want := range []string{"claude-opus-5", "12", "probe"} {
		if !strings.Contains(s, want) {
			t.Errorf("the plan must mention %q; got:\n%s", want, s)
		}
	}
}

// R2: PASS: the key appears nowhere in output, on success or failure.
// FAIL: a leak. This output is pasted into issues and terminals, and a
// provider key in it is a compromise rather than a bug.
func TestR2_TheCredentialIsNeverPrinted(t *testing.T) {
	// Success path.
	r, _, out := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":2048,"cache_read_input_tokens":0}}`))
	})
	_, _ = r.Run(Config{Min: 0, Max: 4096, Resolution: 512, MaxProbes: 6, Confirm: 1}, "claude-opus-5")
	if strings.Contains(out.String(), "SECRETVALUE") {
		t.Error("the credential appeared in successful output")
	}

	// Failure path: an error message is where a key most often leaks, because
	// the request gets dumped.
	r2, _, out2 := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
	_, err := r2.Run(Config{Min: 0, Max: 4096, Resolution: 512, MaxProbes: 6, Confirm: 1}, "claude-opus-5")
	if err == nil {
		t.Fatal("a 401 must end the run")
	}
	if strings.Contains(out2.String()+err.Error(), "SECRETVALUE") {
		t.Error("the credential appeared in an error")
	}
}

// R3: PASS: the key is taken from the environment only.
// FAIL: a flag. A credential on a command line lands in shell history and in
// the process table, where every other user on the box can read it.
func TestR3_TheCredentialComesFromTheEnvironment(t *testing.T) {
	src, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"flag.String(\"api-key", "flag.String(\"key", "--api-key"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("a credential flag (%s) puts the key in shell history and the process table", banned)
		}
	}
}

// R4: PASS: no two probes send the same prefix content.
// FAIL: repeated content, which caches on the first probe and is then READ by
// every later one — and a read tests nothing, so the whole run learns nothing
// while costing full price.
func TestR4_EveryProbeIsUnique(t *testing.T) {
	seen := map[string]int{}
	r, _, _ := newRunner(t, func(w http.ResponseWriter, req *http.Request) {
		var body bytes.Buffer
		_, _ = body.ReadFrom(req.Body)
		seen[body.String()]++
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":1024,"cache_read_input_tokens":0}}`))
	})
	_, _ = r.Run(Config{Min: 0, Max: 65536, Resolution: 256, MaxProbes: 12, Confirm: 1}, "claude-opus-5")

	if len(seen) < 2 {
		t.Fatalf("only %d distinct bodies sent; the run did not probe", len(seen))
	}
	for body, n := range seen {
		if n > 1 {
			t.Errorf("a probe body was sent %d times; the second would read the first's cache entry and test nothing (%.40s)", n, body)
		}
	}
}

// R5: PASS: never more requests than authorised.
// FAIL: one over. This spends the operator's money, and a cap that is
// approximately respected is not a cap.
func TestR5_StopsAtTheAuthorisedRequestCount(t *testing.T) {
	var requests int
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":1024,"cache_read_input_tokens":0}}`))
	})
	_, _ = r.Run(Config{Min: 0, Max: 1 << 20, Resolution: 1, MaxProbes: 5, Confirm: 1}, "claude-opus-5")
	if requests > 5 {
		t.Errorf("sent %d requests against an authorised 5", requests)
	}
}

// R6: PASS: a non-200 ends the run and is not recorded as evidence.
// FAIL: treating an error body's absent usage as "cached nothing", which would
// push the reported floor above a size that was never actually tested.
func TestR6_AProviderErrorIsNotEvidence(t *testing.T) {
	calls := 0
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(529)
			_, _ = w.Write([]byte(`{"error":"overloaded"}`))
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":1024,"cache_read_input_tokens":0}}`))
	})
	s, err := r.Run(Config{Min: 0, Max: 65536, Resolution: 64, MaxProbes: 20, Confirm: 1}, "claude-opus-5")
	if err == nil {
		t.Fatal("a provider error must end the run")
	}
	if s != nil && s.Probes() > 1 {
		t.Errorf("recorded %d decisions; the errored request must not be one", s.Probes())
	}
}

// R7: inconclusive probes consume budget, and only the runner's cap stops them.
//
// A read does not advance the search, so Search's own MaxProbes never trips: it
// counts decisions, and an inconclusive probe decides nothing. A provider that
// keeps answering with reads would loop forever, paying full price each time.
// This is the case where the runner's request cap is the only thing standing
// between an operator and unbounded spend — a mutation removing it survived
// until this test existed, because every other test reached a decision.
//
// PASS: the run stops at the authorised request count.
// FAIL: any number above it, and in the real world an unbounded bill.
func TestR7_InconclusiveProbesStillConsumeBudget(t *testing.T) {
	var requests int
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests > 50 {
			// Fail fast rather than hang. Without the runner's cap this loop
			// is infinite — the search never advances on a read — and a
			// mutation removing the cap would otherwise run until the test
			// timeout, which is a worse failure than a red test.
			w.WriteHeader(500)
			return
		}
		w.Header().Set("content-type", "application/json")
		// Always a read: found an existing entry, decided nothing.
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":512,"cache_read_input_tokens":4096}}`))
	})
	s, _ := r.Run(Config{Min: 0, Max: 65536, Resolution: 1, MaxProbes: 4, Confirm: 1}, "claude-opus-5")
	if requests > 4 {
		t.Errorf("sent %d requests against an authorised 4; inconclusive probes must still spend budget", requests)
	}
	if s.Probes() != 0 {
		t.Errorf("recorded %d decisions from probes that all read; none of them decided anything", s.Probes())
	}
}

// R8: a transport failure cannot leak the credential either.
//
// The 401 path in R2 covers a provider that answered. This covers one that did
// not: connection refused, TLS failure, timeout. It is the path where a naive
// implementation wraps the *error*, and Go's http errors carry the request URL
// — and where a hand-written message is most likely to helpfully include the
// key that was being tried.
//
// PASS: no credential in the error or the output.
// FAIL: a leak into text that reaches terminals and issue trackers.
func TestR8_ATransportFailureDoesNotLeakTheCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // Nothing is listening now.

	var out bytes.Buffer
	r := &Runner{BaseURL: url, APIKey: "sk-ant-SECRETVALUE-do-not-print", Client: srv.Client(), Out: &out}
	_, err := r.Run(Config{Min: 0, Max: 4096, Resolution: 512, MaxProbes: 4, Confirm: 1}, "claude-opus-5")
	if err == nil {
		t.Fatal("an unreachable provider must end the run")
	}
	if strings.Contains(err.Error()+out.String(), "SECRETVALUE") {
		t.Errorf("the credential leaked into a transport error: %v", err)
	}
}
