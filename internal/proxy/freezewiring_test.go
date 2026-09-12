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

// The freeze-prefix WIRING, as opposed to the two pure functions under it.
//
// A review mutated five things in the wiring and the whole suite stayed green:
// forcing the flag on for every request, deleting the REPLAY_NO_POLICY
// override, making the rewrite never happen, blanking the epoch, and dropping
// the epoch in the ledger reader. Both halves of the safety argument — off by
// default, and the environment switch forces it off — could be deleted without
// one test going red, because nothing anywhere set Config.FreezePrefix.
//
// freeze.go's own tests are good and they are not the point. They prove the
// functions; these prove that the proxy calls them when it should and does not
// when it should not. The bug the function tests would catch is a bug in
// freeze.go. The bug nobody would catch is the proxy rewriting every body it
// sees.

// sendFreeze drives one request through a real server and returns what the
// upstream actually received.
func sendFreeze(t *testing.T, path, body string, cfg Config) *upstream {
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
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	// Settle before the temp dir is removed. The proxy writes its record
	// after answering the client, so returning on the response alone races
	// t.TempDir's cleanup — which fails the test for a reason that has
	// nothing to do with what it asserts.
	waitLedger(t, dir, 1)
	return up
}

const freezeBody = `{"model":"claude-opus-5","system":[{"type":"text","text":"env cc_version=abcdefgh here"}],"messages":[{"role":"user","content":"hi"}]}`

// Off by default means the bytes go through untouched.
func TestFreezeWiring_OffByDefaultForwardsUnchanged(t *testing.T) {
	up := sendFreeze(t, "/v1/messages", freezeBody, Config{})
	if !strings.Contains(string(up.gotBody), "cc_version=abcdefgh") {
		t.Fatalf("the body was rewritten with the flag off:\n%s", up.gotBody)
	}
}

// On, the version is pinned.
func TestFreezeWiring_OnPinsTheVersion(t *testing.T) {
	up := sendFreeze(t, "/v1/messages", freezeBody, Config{FreezePrefix: true})
	if strings.Contains(string(up.gotBody), "cc_version=abcdefgh") {
		t.Fatalf("the flag was on and the version was not pinned:\n%s", up.gotBody)
	}
	if !strings.Contains(string(up.gotBody), frozenVersion) {
		t.Fatalf("the constant is not on the wire:\n%s", up.gotBody)
	}
}

// REPLAY_NO_POLICY forces it off at the proxy, not only at the flag.
//
// serve.go also clears the flag under that variable, and that belt is worth
// having, but a test that only covered the flag would leave the braces
// untested: Config is public and a caller constructing one directly gets
// whatever the proxy enforces, not whatever the CLI enforces.
func TestFreezeWiring_NoPolicyForcesItOff(t *testing.T) {
	up := sendFreeze(t, "/v1/messages", freezeBody, Config{FreezePrefix: true, NoPolicy: true})
	if !strings.Contains(string(up.gotBody), "cc_version=abcdefgh") {
		t.Fatalf("NoPolicy did not stop the rewrite:\n%s", up.gotBody)
	}
}

// A path this build cannot read is forwarded unchanged, flag or no flag.
//
// This is the one the review actually ran. POST /v1/embeddings carrying a
// cc_version came out the other side pinned, on a path where noteUnparsed has
// just told the operator that everything Replay offers is inert for it.
// Protection that quietly is not there is one defect; mutation that quietly IS
// there is the worse one.
func TestFreezeWiring_AnUnreadablePathIsNeverRewritten(t *testing.T) {
	const embeds = `{"input":"env cc_version=abcdefgh here"}`
	up := sendFreeze(t, "/v1/embeddings", embeds, Config{FreezePrefix: true})
	if !strings.Contains(string(up.gotBody), "cc_version=abcdefgh") {
		t.Fatalf("a path this build cannot read had its body rewritten:\n%s\n\n"+
			"handle() calls noteUnparsed for this path and says everything Replay offers "+
			"is inert for it. A rewrite here contradicts that in the one handler whose "+
			"header says the order of operations is the invariant.", up.gotBody)
	}
	if strings.Contains(string(up.gotBody), frozenVersion) {
		t.Fatalf("the pin reached an unreadable path:\n%s", up.gotBody)
	}
}

// The epoch is recorded on the ledger record when the flag is on, and is
// absent when it is off.
func TestFreezeWiring_TheEpochIsRecordedOnlyWhenLabelling(t *testing.T) {
	const withTools = `{"model":"claude-opus-5","tools":[{"name":"read"}],"system":[{"type":"text","text":"x"}],"messages":[{"role":"user","content":"hi"}]}`
	for _, c := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"flag on", Config{FreezePrefix: true}, true},
		{"flag off", Config{}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			store, err := ledger.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			up := &upstream{t: t}
			upSrv := httptest.NewServer(up)
			t.Cleanup(upSrv.Close)
			target, _ := url.Parse(upSrv.URL)
			cfg := c.cfg
			cfg.Listen, cfg.Upstream, cfg.Store = "127.0.0.1:0", target, store
			srv, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			go func() { _ = srv.ListenAndServe(ctx) }()
			req, _ := http.NewRequest(http.MethodPost, "http://"+srv.Addr()+"/v1/messages", strings.NewReader(withTools))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", "sk-test")
			req.Header.Set(HeaderSessionID, "epoch-session")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			recs := waitLedger(t, dir, 1)
			if len(recs) == 0 {
				t.Fatal("no ledger record was written; this test is asserting over nothing")
			}
			has := recs[0].Epoch != ""
			if has != c.want {
				t.Fatalf("epoch labelled = %v (%q), want %v. A label written when nobody asked "+
					"for one, or missing when they did, is a ledger that does not say what happened",
					has, recs[0].Epoch, c.want)
			}
		})
	}
}
