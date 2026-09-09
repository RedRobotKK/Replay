package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// The front door, which nothing was watching.
//
// fetchRules refuses a plain-http source at rules.go:214, one scope above the
// redirect refusal at :233. X11 covers the redirect: an https server that 302s
// to http, watched not to install. Nothing covered someone typing an http URL
// in the first place.
//
// Verified by mutation before writing this: `if false && u.Scheme != "https"`
// left the whole suite green.
//
// It matters because of what rules ARE. They drive the cost gates, and the
// installed document records the source URL as typed — rules.go's own comment
// about the redirect case says a cleartext hop means "every later report
// asserted TLS that never happened". That reasoning applies at least as much to
// the address the reader supplied.
//
// The assertion is the request counter, not the error string. "Refusing to
// fetch" is a claim about the wire, and a test that reads only the message
// would pass against code that fetched first and complained afterwards.

// RC1: an http source is refused, nothing is fetched, nothing is installed.
func TestRC1_APlainHTTPSourceIsNeverFetched(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"schema":"replay.rules.v1","version":"served-over-cleartext","models":[{"match":"x","minPrefix":1}]}`))
	}))
	defer srv.Close()
	// httptest.NewServer is http://, which is exactly the case under test.
	if !strings.HasPrefix(srv.URL, "http://") {
		t.Fatalf("the fixture is not cleartext: %s", srv.URL)
	}

	dir := t.TempDir()
	file := filepath.Join(dir, rulesFileName)
	var out bytes.Buffer
	err := updateRules(srv.URL, file, &out, false, false)

	if err == nil {
		t.Fatal("a plain-http rules source was accepted")
	}
	if !strings.Contains(err.Error(), "plain http") && !strings.Contains(err.Error(), "cleartext") {
		t.Errorf("the refusal does not name the scheme, so an operator sees a failed fetch "+
			"and retries it: %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the server was contacted %d time(s). \"Refusing to fetch\" is a claim "+
			"about the wire, and the request was made before the refusal.", n)
	}
	if _, statErr := os.Stat(file); statErr == nil {
		t.Error("rules were installed from a cleartext source")
	}
}

// RC2: https is still accepted.
//
// The guard against closing RC1 by refusing every source. A scheme check that
// rejects both is indistinguishable from a broken updater, and passes RC1.
func TestRC2_AnHTTPSSourceStillWorks(t *testing.T) {
	body := `{"schema":"replay.rules.v1","version":"2026-09-09","models":[{"match":"x","minPrefix":1}]}`
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer tls.Close()
	restore := swapTransport(tls.Client().Transport.(*http.Transport))
	defer restore()

	dir := t.TempDir()
	file := filepath.Join(dir, rulesFileName)
	var out bytes.Buffer
	if err := updateRules(tls.URL, file, &out, false, false); err != nil {
		t.Fatalf("an https source was refused: %v", err)
	}
	if _, statErr := os.Stat(file); statErr != nil {
		t.Errorf("an https source installed nothing: %v", statErr)
	}
}
