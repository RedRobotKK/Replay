package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RedRobotKK/Replay/internal/proxy"
)

// `replay doctor` issues a GET to $ANTHROPIC_BASE_URL/replay/healthz to find
// out whether a proxy is already running. The host therefore comes from an
// environment variable, and doctor.go guards it:
//
//	if !isLoopbackURL(base) {
//	    return false, "not probed: only a loopback address is contacted, ..."
//	}
//
// Without that guard, ANTHROPIC_BASE_URL=http://internal.corp/admin turns the
// one command whose job is "what can Replay see here" into a request
// generator pointed at somebody's network. The guard's own comment says so.
//
// Nothing tested it. `grep -rn "isLoopbackURL\|probeProxy" *_test.go`
// returned nothing before this file: the security property was written down
// in a comment, described in docs/SURFACES.md, and never checked.
//
// docs/SURFACES.md described it backwards — it told a security-conscious
// reader that doctor WILL probe a remote gateway. That is the rarer
// direction for a defect of this kind: the document made the tool sound
// worse than it is. Both halves are fixed in this change, and the test is
// the half that cannot rot.
func TestDP1_ANonLoopbackBaseIsNeverContacted(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("replay"))
	}))
	defer srv.Close()

	// The server is real and on loopback. These hosts are not it, and not
	// loopback: each is the shape of a mistake or an attack — a corporate
	// name, a private range, the AWS metadata endpoint, a public host.
	for _, base := range []string{
		"http://internal.corp/admin",
		"http://10.0.0.1:4000",
		"http://169.254.169.254",
		"https://example.com",
		"http://192.168.1.1:4000",
	} {
		t.Run(base, func(t *testing.T) {
			ok, detail := probeProxy(base)
			if ok {
				t.Errorf("probeProxy(%q) reported a proxy. It must refuse "+
					"anything that is not loopback.", base)
			}
			if !strings.Contains(detail, "not probed") {
				t.Errorf("probeProxy(%q) = %q; want a refusal saying it was not "+
					"probed. If this now reports a connection error instead, the "+
					"guard has been removed and the GET was actually attempted.",
					base, detail)
			}
		})
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("the test server was contacted %d time(s) while probing "+
			"non-loopback hosts", n)
	}
}

// The other arm. A guard that refuses everything is not a guard, it is a
// broken feature, and this is what stops the fix for DP1 being `return
// false` — which would pass DP1 on its own.
func TestDP2_ALoopbackProxyIsStillFound(t *testing.T) {
	var hits int32
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		path = r.URL.Path
		// The contract probeProxy checks: 200 with a body of exactly "ok".
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ok, detail := probeProxy(srv.URL)
	if !ok {
		t.Fatalf("probeProxy(%q) did not find the proxy on loopback: %q\n"+
			"A check that refuses every address would satisfy DP1 and break "+
			"the feature.", srv.URL, detail)
	}
	if atomic.LoadInt32(&hits) == 0 {
		t.Error("probeProxy reported a proxy without contacting the server, so " +
			"DP1 proves nothing: it would pass against a function that never " +
			"makes a request at all")
	}
	if path != proxy.HealthPath {
		t.Errorf("probeProxy asked for %q, want %q", path, proxy.HealthPath)
	}
}

// A loopback address is necessary and not sufficient. Something else
// listening on the port must not be reported as a Replay proxy, and the
// message must say which — an operator who is told "healthy" about their own
// dev server learns nothing and trusts it.
func TestDP4_SomethingElseOnThePortIsNotAProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><title>my dev server</title>"))
	}))
	defer srv.Close()

	ok, detail := probeProxy(srv.URL)
	if ok {
		t.Error("a server answering 200 with something other than \"ok\" was " +
			"reported as a Replay proxy")
	}
	if !strings.Contains(detail, "something other than replay") {
		t.Errorf("detail = %q; it should say what was found, not just that the "+
			"probe failed", detail)
	}
}

// isLoopbackURL directly, including the cases that are easy to get wrong:
// a bare name, IPv6 loopback, the whole 127/8 range, and an address that
// merely starts with the right digits.
func TestDP3_WhatCountsAsLoopback(t *testing.T) {
	for _, c := range []struct {
		base string
		want bool
	}{
		{"http://localhost:4000", true},
		{"http://127.0.0.1:4000", true},
		{"http://[::1]:4000", true},
		{"http://127.0.0.53:4000", true}, // all of 127/8 is loopback
		{"http://10.0.0.1:4000", false},
		{"http://127.0.0.1.example.com", false}, // prefix, not an address
		{"http://example.com", false},
		{"http://169.254.169.254", false},
		{"", false},
		{"::::", false},
	} {
		if got := isLoopbackURL(c.base); got != c.want {
			t.Errorf("isLoopbackURL(%q) = %v, want %v", c.base, got, c.want)
		}
	}
}
