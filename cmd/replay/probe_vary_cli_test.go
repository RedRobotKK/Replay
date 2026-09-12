package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The `--vary` path through runProbe.
//
// guard-reachability reported every branch of it unobserved: whether the flag
// was given at all, whether the spend was confirmed, what happens when the
// experiment errors, and each of the two verdicts the command can print. The
// verdict branches matter most — they are the only output of a command that
// spends three billable requests, and a test that never reads them lets the
// command report the wrong one forever.

// varyProvider answers token counts and messages. reads controls what the
// second and third messages report, which is what decides the verdict.
func varyProvider(t *testing.T, secondRead, thirdRead int) (*httptest.Server, *int) {
	t.Helper()
	msgs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "count_tokens") {
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 7+len(raw)/2)
			return
		}
		msgs++
		create, read := 0, 0
		switch msgs {
		case 1:
			create = 4096
		case 2:
			read = secondRead
		default:
			read = thirdRead
		}
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprintf(w,
			`{"id":"m","usage":{"input_tokens":5,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":1}}`,
			create, read)
	}))
	t.Cleanup(srv.Close)
	return srv, &msgs
}

// Without --execute the command plans and sends nothing.
func TestProbeVaryCLI_PlansWithoutExecuteAndSendsNothing(t *testing.T) {
	srv, msgs := varyProvider(t, 4096, 4096)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
	t.Setenv("ANTHROPIC_API_KEY", "k")
	var out, errb bytes.Buffer
	if err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "tools"}, &out, &errb); err != nil {
		t.Fatalf("planning must not fail: %v", err)
	}
	if *msgs != 0 {
		t.Fatalf("a plan sent %d billable requests; it must send none", *msgs)
	}
	if !strings.Contains(out.String(), "Nothing has been sent") {
		t.Fatalf("the plan does not say nothing was sent:\n%s", out.String())
	}
}

// An unknown term is refused, and refused while planning — before the key
// check, because a typo should not require a credential to report.
func TestProbeVaryCLI_AnUnknownTermIsRefused(t *testing.T) {
	srv, msgs := varyProvider(t, 4096, 4096)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
	t.Setenv("ANTHROPIC_API_KEY", "k")
	var out, errb bytes.Buffer
	err := runProbe(strings.NewReader(""), []string{"--model", "claude-opus-5", "--vary", "nonsense"}, &out, &errb)
	if err == nil || !strings.Contains(err.Error(), "unknown vary term") {
		t.Fatalf("an unknown term must be refused by name: %v", err)
	}
	if *msgs != 0 {
		t.Fatalf("refusing a term sent %d requests", *msgs)
	}
}

// Declining the spend sends nothing.
func TestProbeVaryCLI_DecliningTheSpendSendsNothing(t *testing.T) {
	srv, msgs := varyProvider(t, 4096, 4096)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
	t.Setenv("ANTHROPIC_API_KEY", "k")
	var out, errb bytes.Buffer
	err := runProbe(strings.NewReader("no\n"),
		[]string{"--model", "claude-opus-5", "--vary", "tools", "--execute"}, &out, &errb)
	if err == nil || !strings.Contains(err.Error(), "not confirmed") {
		t.Fatalf("declining must refuse and say so: %v", err)
	}
	if *msgs != 0 {
		t.Fatalf("declining still sent %d billable requests", *msgs)
	}
}

// A missing key is refused before anything is sent, and the refusal says why
// the key is not a flag.
func TestProbeVaryCLI_AMissingKeyIsRefusedBeforeSending(t *testing.T) {
	srv, msgs := varyProvider(t, 4096, 4096)
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
	t.Setenv("ANTHROPIC_API_KEY", "")
	var out, errb bytes.Buffer
	err := runProbe(strings.NewReader("yes\n"),
		[]string{"--model", "claude-opus-5", "--vary", "tools", "--execute"}, &out, &errb)
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("a missing key must be refused by name: %v", err)
	}
	if !strings.Contains(err.Error(), "shell history") {
		t.Fatalf("the refusal does not say why the key is not a flag: %v", err)
	}
	if *msgs != 0 {
		t.Fatalf("a run with no key sent %d requests", *msgs)
	}
}

// The three verdicts, each read from the output rather than from the struct.
//
// These are the branches that decide what an operator is told after three
// billable requests, and they were all unobserved. The fixtures differ only in
// what the provider reports on requests 2 and 3, which is exactly what the
// verdict is supposed to depend on.
func TestProbeVaryCLI_EachVerdictIsPrinted(t *testing.T) {
	for _, c := range []struct {
		name           string
		second, third  int
		want           string
		mustNotContain string
	}{
		// The phrases have to DISTINGUISH, not merely appear. Both verdicts end
		// in "in the provider cache key on this run" — one says the term IS in
		// it, the other that it was NOT OBSERVED in it — so asserting that
		// substring passes whichever branch ran, and deleting `case res.Moved`
		// falls through to the default with the test still green. That is what
		// guard-reachability called INERT, and it was right.
		//
		// Control read on request 3, variant did not: the term moved the key.
		{"moved", 0, 4096, "request 2's cache_read is 0", "was not observed"},
		// Control did NOT read: caching is not working in this window, so the
		// run says nothing about the term either way.
		{"inconclusive", 0, 0, "inconclusive:", "request 2's cache_read is 0"},
		// Variant still read: the term was not observed in the key.
		{"held", 4096, 4096, "was not observed", "request 2's cache_read is 0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv, msgs := varyProvider(t, c.second, c.third)
			t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
			t.Setenv("ANTHROPIC_API_KEY", "k")
			var out, errb bytes.Buffer
			if err := runProbe(strings.NewReader("yes\n"),
				[]string{"--model", "claude-opus-5", "--vary", "tools", "--execute"}, &out, &errb); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if *msgs != 3 {
				t.Fatalf("%s: sent %d messages, want 3", c.name, *msgs)
			}
			// After the result line only. The PLAN printed above it explains
			// what the run will look for, in the same words the verdict uses —
			// so scanning the whole buffer matches the explanation and says
			// nothing about which verdict was chosen.
			verdict := verdictSection(t, out.String())
			if !strings.Contains(verdict, c.want) {
				t.Fatalf("%s verdict missing %q:\n%s", c.name, c.want, verdict)
			}
			if strings.Contains(verdict, c.mustNotContain) {
				t.Fatalf("%s also printed %q, so the verdicts are not exclusive:\n%s",
					c.name, c.mustNotContain, verdict)
			}
		})
	}
}

// A provider failure during the experiment surfaces rather than printing a
// verdict about a run that did not happen.
func TestProbeVaryCLI_AProviderFailureSurfaces(t *testing.T) {
	msgs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "count_tokens") {
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 7+len(raw)/2)
			return
		}
		msgs++
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer srv.Close()
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
	t.Setenv("ANTHROPIC_API_KEY", "k")
	var out, errb bytes.Buffer
	err := runProbe(strings.NewReader("yes\n"),
		[]string{"--model", "claude-opus-5", "--vary", "tools", "--execute"}, &out, &errb)
	if err == nil {
		t.Fatal("a 502 during the experiment must surface, not be summarised as a verdict")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("the error does not name what the provider answered: %v", err)
	}
	// A failed run must not reach the result line at all, so there is no
	// verdict section to scan — its absence is the assertion.
	if strings.Contains(out.String(), "\nvary ") {
		t.Fatalf("a failed run printed a result line:\n%s", out.String())
	}
}

// verdictSection is everything the command printed after the result line.
func verdictSection(t *testing.T, out string) string {
	t.Helper()
	i := strings.Index(out, "\nvary ")
	if i < 0 {
		t.Fatalf("no result line in the output; the run did not reach a verdict:\n%s", out)
	}
	return out[i:]
}
