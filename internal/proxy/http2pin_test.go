package proxy

import (
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// GO-2026-6610 (response smuggling from an HTTP/2 upstream to an HTTP/1
// client through a reverse proxy) and the HTTP/2 rows of the 2026-10-08
// advisory set were dispositioned "not applicable" on one fact about this
// binary: the upstream leg is HTTP/1.1 only. That follows from net/http's
// documented rule, quoted from the Transport type: "ForceAttemptHTTP2
// controls whether HTTP/2 is enabled when a non-zero Dial, DialTLS, or
// DialContext func or TLSClientConfig is provided. By default, use of any
// those fields conservatively disables HTTP/2." The proxy sets DialContext
// and leaves ForceAttemptHTTP2 unset, and the listener carries no TLS
// configuration, so no HTTP/2 server exists either.
//
// A disposition that rests on a construction-time fact is only as good as the
// thing that keeps the fact true. This pins it. Setting ForceAttemptHTTP2 on
// the transport makes the test fail, which is the moment the advisory rows
// become applicable again and the disposition has to be redone.
//
// PASS: the upstream Transport has a custom DialContext, ForceAttemptHTTP2 is
// false and TLSClientConfig is nil; the listener has no TLSConfig.
// FAIL: any of those changed, and the HTTP/2 advisory dispositions are stale.
func TestGV6610_TheUpstreamLegIsHTTP1OnlyByConstruction(t *testing.T) {
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse("http://127.0.0.1:9")
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store, Logger: log.New(nopWriter{}, "", 0)})
	if err != nil {
		t.Fatal(err)
	}

	var tr *http.Transport
	switch rt := srv.rp.Transport.(type) {
	case *http.Transport:
		tr = rt
	case *retryTransport:
		inner, ok := rt.next.(*http.Transport)
		if !ok {
			t.Fatalf("retryTransport wraps %T, not *http.Transport", rt.next)
		}
		tr = inner
	default:
		t.Fatalf("upstream RoundTripper is %T; this test knows how to inspect *http.Transport and retryTransport", rt)
	}
	if tr.DialContext == nil {
		t.Errorf("DialContext is nil: the documented rule that disables HTTP/2 no longer applies")
	}
	if tr.ForceAttemptHTTP2 {
		t.Errorf("ForceAttemptHTTP2 is set: the upstream leg can negotiate HTTP/2, and the GO-2026-6610, 6617, 6612, 6611 and 6603 dispositions must be redone")
	}
	if tr.TLSClientConfig != nil && len(tr.TLSClientConfig.NextProtos) > 0 {
		t.Errorf("TLSClientConfig.NextProtos = %v: ALPN is offering protocols the disposition assumed absent", tr.TLSClientConfig.NextProtos)
	}
	if srv.http.TLSConfig != nil {
		t.Errorf("the listener carries a TLSConfig: an HTTP/2 or TLS server now exists in the shipped binary")
	}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
