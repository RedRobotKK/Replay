package selfupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// SIG9: a tag is a release tag before it becomes part of a URL.
//
// Fetch built the download base as <releases>/download/<tag> from whatever it
// was handed, and `replay upgrade --version` hands it the user's argument
// unchanged. A tag of "../../x/y/releases/download/v1" moved the download to
// another repository; with cosign present the signer identity still refused
// it, and without cosign the checksums came from the same place as the
// archive (2026-10-15). Every tag this project has published is
// vMAJOR.MINOR.PATCH with an optional -prerelease, the rule
// scripts/reproduce-release.sh already applies.
func TestSIG9_FetchRefusesATagThatIsNotAReleaseTagBeforeAnyRequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.NotFound(w, nil)
	}))
	defer srv.Close()
	c := &Client{ReleasesBase: srv.URL}

	for _, tag := range []string{
		"", "v1.0", "1.0.0", "v1.0.0/../x", "../../x/y/releases/download/v1",
		"v1.0.0\n", "v1.0.0 x", "v1.0.0?x=1", "v1.0.0#x", "--version", "v01.0.0-", "v1.0.0-",
	} {
		hits.Store(0)
		bin, sig, err := c.Fetch(context.Background(), tag, "linux", "amd64")
		if err == nil || bin != nil {
			t.Errorf("Fetch(%q) did not refuse", tag)
		}
		if sig == SignatureVerified {
			t.Errorf("Fetch(%q) reported a verified signature", tag)
		}
		if n := hits.Load(); n != 0 {
			t.Errorf("Fetch(%q) made %d request(s) before refusing the tag", tag, n)
		}
	}
}

// The control: every tag the project has published, and a prerelease, still
// reaches the server. Without it the test above passes on a Fetch that
// refuses everything.
func TestSIG9_EveryPublishedTagIsStillFetched(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.NotFound(w, nil)
	}))
	defer srv.Close()
	c := &Client{ReleasesBase: srv.URL}
	for _, tag := range []string{
		"v0.1.0", "v0.1.1", "v0.1.2", "v0.2.0", "v0.3.0", "v0.4.0", "v0.5.0", "v0.5.1",
		"v0.5.2", "v0.5.3", "v0.5.4", "v0.6.0", "v0.6.1", "v0.6.2", "v0.7.0", "v0.8.0", "v0.9.0",
		"v1.0.0-rc.1", "v10.20.30",
	} {
		hits.Store(0)
		_, _, _ = c.Fetch(context.Background(), tag, "linux", "amd64")
		if hits.Load() == 0 {
			t.Errorf("Fetch(%q) refused a release tag without asking for it", tag)
		}
	}
}
