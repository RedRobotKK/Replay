package proxy

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// The cache-break line never prints an empty pair of parentheses.
//
// passthrough.go builds the detail suffix behind a guard, with a comment that
// states the rule: "It is omitted rather than printed empty: a bare '()' reads
// as a measurement that came back nothing, which is a different claim from one
// that was not taken." guard-reachability reported that guard INERT (#238) --
// it ran, and nothing depended on whether it did, so the rule its own comment
// insists on was unchecked.
//
// The empty case is ordinary, not exotic. breakCause returns an empty detail
// from two of its paths, and the plainest cache break in this package -- the
// one TestLiveCacheBreakIsLoggedRecordedAndCounted already produces -- lands on
// one of them: Cause="system prompt or tool definitions changed", CauseDetail="".
func TestABreakWithNoDetailPrintsNoEmptyParentheses(t *testing.T) {
	base, dir, logs := startProxy(t, &breakingUpstream{}, "")
	for i := 0; i < 2; i++ {
		resp := post(t, base, "/v1/messages", nil)
		_ = resp.Body.Close()
		waitLedger(t, dir, i+1)
	}
	recs := waitLedger(t, dir, 2)

	// The premise. If a detail ever starts arriving here, this test stops
	// exercising the empty branch and must say so rather than pass quietly.
	if recs[1].Cache == nil {
		t.Fatal("the second request recorded no cache outcome")
	}
	if recs[1].Cache.CauseDetail != "" {
		t.Skipf("this break now carries a detail (%q); the empty-detail branch needs "+
			"another way in", recs[1].Cache.CauseDetail)
	}

	waitFor(t, "the cache break line", func() bool {
		return strings.Contains(logs.String(), "cache break")
	})
	line := breakLine(t, logs.String())
	if strings.Contains(line, "()") {
		t.Fatalf("the break line printed an empty detail:\n      %s\n"+
			"      A bare () reads as a measurement that came back nothing, which is a "+
			"different claim from one that was never taken.", line)
	}
	// And it still ends by naming a cause, so the absent detail did not take
	// the cause with it.
	if !strings.Contains(line, "likely cause: ") {
		t.Errorf("the break line names no cause at all:\n      %s", line)
	}
}

// A break that HAS a detail prints it, in parentheses.
//
// The other half of the guard. Without this, deleting the suffix entirely
// would pass the test above, and the detail -- the part that names the
// thirty-four tools that arrived -- would silently stop reaching the operator
// who never opens the ledger.
func TestABreakWithADetailPrintsItInParentheses(t *testing.T) {
	up := &prefixChangingUpstream{}
	base, dir, logs := startProxy(t, up, "")

	// Two requests whose tool definitions differ, so the cause carries a
	// detail naming what changed.
	first := `{"model":"claude-sonnet-5","system":"you are a helpful assistant",` +
		`"tools":[{"name":"read","description":"read a file","input_schema":{"type":"object"}}],` +
		`"messages":[{"role":"user","content":"hi"}]}`
	second := `{"model":"claude-sonnet-5","system":"you are a helpful assistant",` +
		`"tools":[{"name":"read","description":"read a file","input_schema":{"type":"object"}},` +
		`{"name":"write","description":"write a file","input_schema":{"type":"object"}},` +
		`{"name":"list","description":"list files","input_schema":{"type":"object"}}],` +
		`"messages":[{"role":"user","content":"hi"}]}`
	for i, body := range []string{first, second} {
		postInSession(t, base, body, "session-detail")
		waitLedger(t, dir, i+1)
	}
	recs := waitLedger(t, dir, 2)
	if recs[1].Cache == nil || recs[1].Cache.Deficit == 0 {
		t.Skipf("no cache break produced: %+v", recs[1].Cache)
	}
	detail := recs[1].Cache.CauseDetail
	if detail == "" {
		t.Skip("this pair produced no detail; the with-detail branch needs another way in")
	}

	waitFor(t, "the cache break line", func() bool {
		return strings.Contains(logs.String(), "cache break")
	})
	line := breakLine(t, logs.String())
	if !strings.Contains(line, " ("+detail+")") {
		t.Fatalf("the record carries the detail %q and the log line does not:\n      %s\n"+
			"      The detail is what names the change to an operator who never opens "+
			"the ledger.", detail, line)
	}
}

// prefixChangingUpstream breaks the cache on the second request, as
// breakingUpstream does, but reads the body so the proxy sees two different
// prefixes.
type prefixChangingUpstream struct {
	mu sync.Mutex
	n  int
}

func (p *prefixChangingUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.ReadAll(r.Body)
	p.mu.Lock()
	p.n++
	n := p.n
	p.mu.Unlock()
	usage := `{"input_tokens":20,"cache_creation_input_tokens":1000,"cache_read_input_tokens":5000,"output_tokens":50}`
	if n == 2 {
		usage = `{"input_tokens":20,"cache_creation_input_tokens":6000,"cache_read_input_tokens":0,"output_tokens":50}`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":`+usage+`}`)
}

// postInSession sends a body under an explicit session id. The package's
// postBody helper sends none, and without one the proxy derives the session
// from the prefix -- so two requests with DIFFERENT tool sets would land in
// different sessions and never correlate into a cache break at all.
func postInSession(t *testing.T, base, body, session string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", secret)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set(HeaderSessionID, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// breakLine returns the cache break line from the log.
func breakLine(t *testing.T, log string) string {
	t.Helper()
	for _, l := range strings.Split(log, "\n") {
		if strings.Contains(l, "cache break") {
			return l
		}
	}
	t.Fatalf("no cache break line in the log:\n%s", log)
	return ""
}
