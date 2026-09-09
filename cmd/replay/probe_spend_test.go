package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Declining the spend must send nothing, and the evidence has to be traffic.
//
// `replay probe --execute` is the only command in this binary that spends the
// reader's money. Its single gate is probe.go's
//
//	if !confirmSpend(stdin, stdout, ...) {
//	    return fmt.Errorf("not confirmed; nothing was sent")
//	}
//
// confirmSpend was already tested thoroughly as a pure function — C1 to C5 in
// probe_confirm_test.go cover yes, no, EOF, --yes and a closed reader. Nothing
// tested that runProbe CALLS it. Verified by mutation: replacing the guard with
//
//	if _ = confirmSpend(...); false {
//
// left `go test ./...` green across all 24 packages while two billable requests
// left the machine. That is ADR-0014's "paywall no test imported", on real
// spend.
//
// The assertion is the request counter rather than the error text. "Nothing was
// sent" is a claim about the wire, and a test that only reads the message would
// pass against a guard that returns the right words after sending the requests.

// C8: a declined confirmation reaches the provider zero times.
func TestC8_AnUnconfirmedExecuteSendsNothing(t *testing.T) {
	var hits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	t.Setenv("ANTHROPIC_BASE_URL", upstream.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")

	var stdout, stderr bytes.Buffer
	// "no" at the prompt. Not empty, not EOF: a reader who was asked and said
	// no is the case the guard exists for.
	err := runProbe(strings.NewReader("no\n"), []string{"--model", "claude-opus-5", "--execute", "--record", "-"}, &stdout, &stderr)

	if err == nil {
		t.Error("declining the spend returned no error")
	} else if !strings.Contains(err.Error(), "nothing was sent") {
		t.Errorf("declining must refuse by name, so an operator can tell a refusal from a "+
			"failure; got: %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the provider received %d request(s) after the spend was declined. "+
			"\"nothing was sent\" is false, and this is billable.", n)
	}
}

// C9: the prompt is actually shown before anything is sent.
//
// A guard that refuses without asking would satisfy C8 and be a different bug —
// `--execute` would simply never work. The plan is printed first so the answer
// is informed rather than reflexive, which is probe.go's own reasoning.
func TestC9_TheReaderIsAskedBeforeTheSpend(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer upstream.Close()
	t.Setenv("ANTHROPIC_BASE_URL", upstream.URL)
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-a-real-one")

	var stdout, stderr bytes.Buffer
	_ = runProbe(strings.NewReader("no\n"), []string{"--model", "claude-opus-5", "--execute", "--record", "-"}, &stdout, &stderr)

	out := stdout.String()
	if !strings.Contains(out, "billable") {
		t.Errorf("the reader is not told the requests are billable before being asked:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "yes to continue") {
		t.Errorf("no confirmation prompt was shown:\n%s", out)
	}
}
