package proxy

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// handle's refusals, and what the operator is told about them.
//
// #233 lists twelve conditionals; measured by neutralising each and running
// the package, seven survive. They are not one subject, but they answer one
// question between them: when the proxy cannot do its job for a request, does
// anyone find out?
//
// A proxy that silently does nothing is worse than no proxy, because the
// operator has already decided to trust it. That is this file's own stated
// reason for noteUnparsed, and it applies to every guard below.

func okJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"m","type":"message","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`))
}

// A POST to a path this build cannot read is announced, once.
//
// The operator installed a proxy that masks secrets, caps spend and counts
// tokens. On a path it cannot parse, none of that runs. Saying so is the
// difference between a gap and a gap nobody knows about.
func TestPassthrough_AnUnreadablePostIsAnnounced(t *testing.T) {
	base, _, logs := startProxyWith(t, http.HandlerFunc(okJSON), Config{})

	resp := post(t, base, "/v1/embeddings", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if !strings.Contains(logs.String(), "NOT PARSED") {
		t.Fatalf("a POST to an unreadable path was not announced:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "/v1/embeddings") {
		t.Errorf("the warning does not name the path, so nobody can act on it:\n%s", logs.String())
	}
}

// A GET to the same path is not announced.
//
// The guard is `!readable && POST` and the POST half is load-bearing: a GET
// carries no work to be inert about, and warning on every probe or health
// check would train the operator to ignore the warning that matters.
func TestPassthrough_AGetToTheSamePathIsNotAnnounced(t *testing.T) {
	base, _, logs := startProxyWith(t, http.HandlerFunc(okJSON), Config{})

	req, err := http.NewRequest(http.MethodGet, base+"/v1/embeddings", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if strings.Contains(logs.String(), "NOT PARSED") {
		t.Fatalf("a GET was announced as unparsed work. Warning on every probe teaches the "+
			"operator to ignore the one that means something:\n%s", logs.String())
	}
}

// A request body over the limit is refused with a status that says so.
//
// Not truncated and not forwarded. The proxy reads the body to summarise it,
// so an unbounded one is unbounded memory — and a silently truncated request
// would reach the provider as a different question from the one asked.
func TestPassthrough_AnOversizeRequestIsRefusedNotTruncated(t *testing.T) {
	var upstreamSaw int
	base, _, _ := startProxyWith(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamSaw++
		okJSON(w, r)
	}), Config{})

	big := append([]byte(`{"model":"claude-opus-5","pad":"`), bytes.Repeat([]byte("x"), MaxRequestBytes+1024)...)
	big = append(big, []byte(`"}`)...)

	req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", bytes.NewReader(big))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("an oversize request answered %d, want 413: %s", resp.StatusCode, body)
	}
	if upstreamSaw != 0 {
		t.Fatal("an oversize request was forwarded. Truncating it would send the provider a " +
			"different question from the one the client asked")
	}
}

// A ledger that cannot be written never fails the request.
//
// The ledger is derived data. Losing a record loses a measurement; failing the
// request loses the user's work, and they did not install a proxy to have it
// drop turns when a disk fills.
func TestPassthrough_ALedgerWriteFailureDoesNotFailTheRequest(t *testing.T) {
	base, dir, logs := startProxyWith(t, http.HandlerFunc(okJSON), Config{})

	// Make the session file un-writable by putting a directory where it goes.
	// Done after the proxy started, so Open succeeded and only the append can
	// fail — a different failure from a store that never opened.
	if err := os.Mkdir(filepath.Join(dir, "bracket-session.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}

	resp := post(t, base, "/v1/messages", map[string]string{HeaderSessionID: "bracket-session"})
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a ledger write failure failed the request: %d %s. The ledger is derived "+
			"data; losing a record loses a measurement, failing the request loses the work",
			resp.StatusCode, body)
	}
	if len(body) == 0 {
		t.Error("the client got no response body")
	}
	// The record is appended in deferred bookkeeping AFTER the client has its
	// response, so reading the log on the response alone races the write. That
	// ordering is deliberate — the tap must never delay delivery — and it is
	// why the package has waitFor.
	waitFor(t, "the ledger write failure to be reported", func() bool {
		return strings.Contains(logs.String(), "ledger write failed")
	})
}
