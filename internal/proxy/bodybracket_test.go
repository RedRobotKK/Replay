package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The body-hash bracket, which record.go promised and nothing wrote.
//
// BodyHashBefore and BodyHashAfter are documented as "what makes 'the proxy
// forwards bytes unchanged' checkable rather than promised: equal hashes are
// the proof". Nothing set them, so the claim stayed a promise while the number
// of body rewrites in handle grew to four — and a field whose whole purpose is
// to make a claim checkable, left unwritten, is the same defect as the package
// comment that denied those rewrites. The artefact a reader would check said
// nothing.
//
// Both cases matter and the equal one matters more. A record whose hashes
// differ names a request the proxy changed; a record whose hashes MATCH is the
// evidence for the sentence this project is built on.

func bracketRecord(t *testing.T, cfg Config, path, body string) ledger.Record {
	t.Helper()
	up := &upstream{t: t}
	upSrv := httptest.NewServer(up)
	t.Cleanup(upSrv.Close)
	target, err := url.Parse(upSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Listen, cfg.Upstream, cfg.Store = "127.0.0.1:0", target, store
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.ListenAndServe(ctx) }()

	req, err := http.NewRequest(http.MethodPost, "http://"+srv.Addr()+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-test")
	req.Header.Set(HeaderSessionID, "bracket")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("no ledger record; this test is asserting over nothing")
	}
	return recs[0]
}

const bracketBody = `{"model":"claude-opus-5","system":[{"type":"text","text":"env cc_version=abcdefgh here"}],"messages":[{"role":"user","content":"hi"}]}`

// An untouched request records two equal hashes. This is the proof.
func TestBodyBracket_AnUntouchedRequestRecordsEqualHashes(t *testing.T) {
	rec := bracketRecord(t, Config{}, "/v1/messages", bracketBody)
	if rec.BodyHashBefore == "" || rec.BodyHashAfter == "" {
		t.Fatalf("the bracket was not written at all: before=%q after=%q. The fields exist to "+
			"make 'forwards bytes unchanged' checkable, and an absent bracket checks nothing",
			rec.BodyHashBefore, rec.BodyHashAfter)
	}
	if rec.BodyHashBefore != rec.BodyHashAfter {
		t.Fatalf("no policy ran and the body still changed: %s -> %s",
			rec.BodyHashBefore, rec.BodyHashAfter)
	}
}

// A rewritten request records two different hashes, so the ledger says a
// change happened rather than leaving a reader to infer it from Policy.
func TestBodyBracket_ARewrittenRequestRecordsDifferentHashes(t *testing.T) {
	rec := bracketRecord(t, Config{FreezePrefix: true}, "/v1/messages", bracketBody)
	if rec.BodyHashBefore == "" || rec.BodyHashAfter == "" {
		t.Fatalf("the bracket was not written: before=%q after=%q", rec.BodyHashBefore, rec.BodyHashAfter)
	}
	if rec.BodyHashBefore == rec.BodyHashAfter {
		t.Fatalf("freeze-prefix rewrote the body and the bracket reports it unchanged (%s). "+
			"Equal hashes are this project's proof that bytes went through untouched; a rewrite "+
			"that produces them is the proof asserting something false", rec.BodyHashBefore)
	}
	if !rec.Frozen {
		t.Error("the record does not say which rewrite changed it")
	}
}

// A path this build cannot read writes no record, so its bracket cannot be
// mistaken for "unchanged".
//
// The first version of this test asked whether such a path carried an empty
// bracket. It does, and trivially: handle writes no ledger record for a path
// it cannot parse, so there is nothing to carry one. That is the honest
// behaviour — noteUnparsed says out loud that everything Replay offers is
// inert for that traffic — and asserting it here keeps the bracket's absence
// from being read as a claim about bytes.
func TestBodyBracket_AnUnreadablePathWritesNoRecordToBracket(t *testing.T) {
	up := &upstream{t: t}
	upSrv := httptest.NewServer(up)
	defer upSrv.Close()
	target, _ := url.Parse(upSrv.URL)
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store, FreezePrefix: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.ListenAndServe(ctx) }()

	req, _ := http.NewRequest(http.MethodPost, "http://"+srv.Addr()+"/v1/embeddings",
		strings.NewReader(`{"input":"env cc_version=abcdefgh here"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	// The body still went through untouched — that is the freeze gate, covered
	// by TestFreezeWiring_AnUnreadablePathIsNeverRewritten. What this adds is
	// that no record claims anything about it either way.
	if !strings.Contains(string(up.gotBody), "cc_version=abcdefgh") {
		t.Fatalf("an unreadable path was rewritten:\n%s", up.gotBody)
	}
	if recs := readLedger(t, dir); len(recs) != 0 {
		t.Fatalf("a path handle cannot parse produced %d ledger record(s); its bracket would be "+
			"empty and an empty bracket next to a real record reads as 'not measured', which is "+
			"a different claim from 'not recorded'", len(recs))
	}
}
