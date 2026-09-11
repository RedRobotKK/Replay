package feed

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The feed fetches vendor facts and does not apply them.
//
// That split is the whole design and it is worth stating before the tests.
// Model prices, caching rules and wire-format behaviour go stale: the shipped
// v0.5.0 binary carried a 75-day-old price table, and a correctness instrument
// whose facts are stale is worse than none, because it is confidently wrong.
//
// But a table arriving over the network is a supply-chain input, and this tool
// publishes figures. So the fetched bundle is verified, cached, and reported
// as available. It does not change a number until somebody adopts it with a
// typed command, which is the same path `replay rules --update` already takes.
// Fetching is automatic; believing is not.

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func bundleBytes(t *testing.T, version int) []byte {
	t.Helper()
	b, err := json.Marshal(Bundle{Version: version, Published: "2026-09-08", RulesVersion: "anthropic-2026-09-01", PriceTableVersion: "2026-09-07"})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// serve returns a test server handing out a bundle and its detached signature.
func serve(t *testing.T, raw, sig []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".sig"):
			_, _ = w.Write(sig)
		default:
			_, _ = w.Write(raw)
		}
	}))
}

// FD-1: a bundle with a valid signature verifies.
func TestFD1_AGoodSignatureVerifies(t *testing.T) {
	pub, priv := testKey(t)
	raw := bundleBytes(t, 2)
	b, err := Verify(raw, ed25519.Sign(priv, raw), pub)
	if err != nil {
		t.Fatalf("a correctly signed bundle was refused: %v", err)
	}
	if b.Version != 2 {
		t.Errorf("version = %d, want 2", b.Version)
	}
}

// FD-2: a bad signature is refused, and so is a bundle altered after signing.
//
// The second half is the one that matters. A signature check that passes on
// the signature alone, without binding it to these exact bytes, is a check
// that cannot fail in the way it is meant to.
func TestFD2_ATamperedBundleIsRefused(t *testing.T) {
	pub, priv := testKey(t)
	raw := bundleBytes(t, 2)
	sig := ed25519.Sign(priv, raw)

	if _, err := Verify(raw, []byte("not a signature"), pub); err == nil {
		t.Error("a malformed signature was accepted")
	}
	tampered := append([]byte{}, raw...)
	tampered[len(tampered)-2] ^= 0x20
	if _, err := Verify(tampered, sig, pub); err == nil {
		t.Error("a bundle altered after signing was accepted with the original signature")
	}
	_, other := testKey(t)
	if _, err := Verify(raw, ed25519.Sign(other, raw), pub); err == nil {
		t.Error("a bundle signed by a different key was accepted")
	}
}

// FD-3: an unset public key refuses everything rather than accepting anything.
//
// The key is not provisioned in this tree. Until it is, the honest behaviour
// is to fail closed and say so: a verifier with no key that returns success is
// the exact shape of defect this project has spent its life finding.
func TestFD3_NoKeyMeansNoTrust(t *testing.T) {
	raw := bundleBytes(t, 1)
	_, priv := testKey(t)
	if _, err := Verify(raw, ed25519.Sign(priv, raw), nil); err == nil {
		t.Fatal("verification succeeded with no public key configured")
	}
	if _, err := Verify(raw, nil, ed25519.PublicKey{}); err == nil {
		t.Fatal("verification succeeded with an empty public key")
	}
}

// FD-4: a bundle no newer than the one already held is refused.
//
// Without this an attacker who can serve responses replays an old, validly
// signed bundle and pins the reader to prices that were true once. A signature
// proves origin, not freshness.
func TestFD4_ARollbackIsRefused(t *testing.T) {
	pub, priv := testKey(t)
	old := bundleBytes(t, 3)
	sig := ed25519.Sign(priv, old)
	b, err := Verify(old, sig, pub)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.NewerThan(Bundle{Version: 3}); err == nil {
		t.Error("a bundle at the same version was accepted as an update")
	}
	if err := b.NewerThan(Bundle{Version: 4}); err == nil {
		t.Error("a bundle older than the cached one was accepted")
	}
	if err := b.NewerThan(Bundle{Version: 2}); err != nil {
		t.Errorf("a genuinely newer bundle was refused: %v", err)
	}
}

// FD-5: only https, and only an allowlisted host.
func TestFD5_TransportAndHostAreConstrained(t *testing.T) {
	for _, u := range []string{
		"http://redrobot.jp/feed",
		"https://example.com/feed",
		"https://redrobot.jp.evil.test/feed",
		"ftp://redrobot.jp/feed",
	} {
		if err := CheckSource(u); err == nil {
			t.Errorf("%s was accepted as a feed source", u)
		}
	}
	if err := CheckSource("https://redrobot.jp/feed/v1"); err != nil {
		t.Errorf("the vendor's own https URL was refused: %v", err)
	}
}

// FD-6: the network being down is not an error the tool dies on.
//
// A correctness instrument that stops working when someone else's host is
// unreachable has made itself the weakest part of the reader's setup. The
// fetch reports what happened and the caller keeps its compiled tables.
func TestFD6_OfflineIsNotFatal(t *testing.T) {
	srv := serve(t, nil, nil)
	srv.Close() // closed on purpose: nothing is listening
	if _, _, err := Fetch(srv.Client(), srv.URL); err == nil {
		t.Fatal("a dead host produced no error")
	} else if !strings.Contains(err.Error(), "could not reach") {
		t.Errorf("the error should say the fetch failed, not something cryptic: %v", err)
	}
}

// FD-7: a fetched bundle changes no figure by itself.
//
// The decision this package exists to encode. Applied returns false for
// anything that merely arrived, so a caller cannot accidentally price a report
// from network content by forgetting to check.
func TestFD7_FetchedIsNotApplied(t *testing.T) {
	pub, priv := testKey(t)
	raw := bundleBytes(t, 9)
	b, err := Verify(raw, ed25519.Sign(priv, raw), pub)
	if err != nil {
		t.Fatal(err)
	}
	if b.Applied {
		t.Error("a bundle was marked applied merely by being verified; " +
			"fetching is automatic and believing is not")
	}
}

// FD-8: this build cannot verify a feed, and says so rather than pretending.
//
// The key is not provisioned. Until somebody decides where the private half
// lives, the honest state is that no feed can be trusted, and the two entry
// points a caller actually uses must both reflect it. A verifier that returns
// success with no key is the defect this whole file guards.
func TestFD8_ThisBuildHasNoVendorKey(t *testing.T) {
	if HasVendorKey() {
		t.Fatal("a vendor key is compiled in; if that is deliberate, this test should assert " +
			"its fingerprint rather than its absence")
	}
	_, priv := testKey(t)
	raw := bundleBytes(t, 1)
	if _, err := VerifyFromVendor(raw, ed25519.Sign(priv, raw)); err == nil {
		t.Fatal("VerifyFromVendor accepted a bundle with no vendor key configured")
	}
}

// The six guards below were found by mutation, not by reading, and each was
// removable with this package's suite still green.
//
// They were missed by scripts/refusal-reachability because that tool selects on
// the vocabulary this project uses when it declines to answer — "NOT MEASURED",
// "refusing to" — and these say "could not reach", "was not read" and "not a
// URL". The vocabulary list IS the tool's blind spot, which is why a general
// per-file sweep of every conditional was run alongside it. internal/feed had
// the worst score in the tree: 7 of 16 conditionals survived.
//
// They are worth closing rather than noting because of what they guard. This is
// the package that reads a supply-chain input: it takes bytes from a vendor
// host and turns them into the price table a reader is invoiced against.

// FD-9: a wrong-length signature is refused by the length check, by name.
//
// ed25519.Verify also rejects a short signature, so it shadows this guard
// completely: remove the length check and Verify refuses anyway, with a
// different sentence, and any test asserting only "an error came back" stays
// green. The length check earns its place by saying which of the two things is
// wrong — a truncated download and a forged signature need different actions —
// so the assertion is on the sentence only it produces.
func TestFD9_AWrongLengthSignatureIsNamedAsSuch(t *testing.T) {
	pub, priv := testKey(t)
	raw := bundleBytes(t, 3)
	short := ed25519.Sign(priv, raw)[:ed25519.SignatureSize-1]

	_, err := Verify(raw, short, pub)
	if err == nil {
		t.Fatal("a signature one byte short verified")
	}
	if !strings.Contains(err.Error(), "signature is 63 bytes, want 64") {
		t.Errorf("refused, but not by the length check, so a truncated download reads as "+
			"a forgery: %v", err)
	}
}

// FD-10: a bundle with no version is refused even though it verifies.
//
// Version 0 is what an unversioned or zero-valued bundle deserialises to, and
// NewerThan compares versions to detect a rollback. A bundle at version 0
// signed by the real key would pass verification and then defeat the rollback
// check for every bundle after it, since nothing is <= 0 except 0 itself.
func TestFD10_AVersionlessBundleIsRefusedAfterVerifying(t *testing.T) {
	pub, priv := testKey(t)
	raw := bundleBytes(t, 0)

	_, err := Verify(raw, ed25519.Sign(priv, raw), pub)
	if err == nil {
		t.Fatal("a bundle with no version was accepted from a valid signature")
	}
	if !strings.Contains(err.Error(), "carries no version") {
		t.Errorf("refused, but not by the version guard: %v", err)
	}
	// The signature itself was fine. If this stops holding, the test above is
	// passing for the wrong reason and proves nothing about the version.
	if strings.Contains(err.Error(), "signature") {
		t.Fatalf("the signature was rejected, so this fixture never reaches the "+
			"version guard: %v", err)
	}
}

// FD-11: a string that is not a URL is refused before any scheme check.
//
// CheckSource's later guards read u.Scheme and u.Hostname() off the parse
// result. Remove this one and a malformed source is judged on the zero value
// of a failed parse, which has an empty scheme and an empty hostname — so it
// is refused, but for a reason that has nothing to do with what is wrong, and
// the person holding a typo'd URL is told https is required.
func TestFD11_AMalformedSourceIsRefusedAsMalformed(t *testing.T) {
	err := CheckSource("https://[::1")
	if err == nil {
		t.Fatal("a string that does not parse as a URL was accepted as a feed source")
	}
	if !strings.Contains(err.Error(), "not a URL") {
		t.Errorf("refused, but not as a parse failure, so a typo'd address is reported "+
			"as the wrong protocol: %v", err)
	}
}

// FD-12: the three guards inside get(), which nothing reached.
//
// FD-6 covers the connection failing. These are the three ways a host that
// ANSWERS can still not have given us a feed, and each is a way an attacker or
// a broken CDN gets bytes in front of the verifier.
func TestFD12_AHostThatAnswersCanStillFail(t *testing.T) {
	t.Run("a non-200 response is not a feed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		}))
		defer srv.Close()
		_, _, err := Fetch(srv.Client(), srv.URL)
		if err == nil {
			t.Fatal("a 404 body was returned as a feed")
		}
		// Without the status check the 404 body is read and handed to the
		// verifier as content, which then reports a signature failure — a
		// misconfigured CDN would look like an attack.
		if !strings.Contains(err.Error(), "HTTP 404") {
			t.Errorf("the status was not reported, so an error page reaches the "+
				"verifier as content: %v", err)
		}
	})

	t.Run("a body over the cap is not read", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(make([]byte, MaxBundle+1))
		}))
		defer srv.Close()
		_, _, err := Fetch(srv.Client(), srv.URL)
		if err == nil {
			t.Fatalf("a body of %d bytes was accepted against a cap of %d", MaxBundle+1, MaxBundle)
		}
		if !strings.Contains(err.Error(), "larger than") {
			t.Errorf("the cap did not refuse it, so the only limit left is the reader's "+
				"memory: %v", err)
		}
	})

	t.Run("a body that stops mid-stream is an error, not a short feed", func(t *testing.T) {
		// Content-Length promises more than the handler delivers, and aborting
		// closes the connection without the rest. io.ReadAll returns what it
		// got plus an error, and without the check that error is dropped and a
		// TRUNCATED body goes to the verifier. That is the worst of the three:
		// a signature check over a prefix of the bundle.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "4096")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(make([]byte, 16))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			panic(http.ErrAbortHandler)
		}))
		srv.Config.ErrorLog = log.New(io.Discard, "", 0) // the abort is the point, not a fault
		defer srv.Close()
		// A path, and the reason is that the first version of this subtest was
		// itself vacuous. It fetched srv.URL bare, so Fetch's second call went
		// to "http://127.0.0.1:PORT.sig" — not a URL — and errored in the
		// TRANSPORT check above. With the read check neutralised the subtest
		// still went green, off a refusal from a different guard entirely.
		// Reported by the per-file mutation sweep, on the test written to close
		// that sweep's last survivor.
		_, _, err := Fetch(srv.Client(), srv.URL+"/feed.json")
		if err == nil {
			t.Fatal("a body that stopped early was returned as a complete feed, so the " +
				"signature would be checked over a prefix of the bundle")
		}
		if !strings.Contains(err.Error(), "could not reach") {
			t.Errorf("the read failure was not reported as a fetch failure: %v", err)
		}
	})

	t.Run("a nil client is given one rather than dereferenced", func(t *testing.T) {
		pub, priv := testKey(t)
		raw := bundleBytes(t, 4)
		srv := serve(t, raw, ed25519.Sign(priv, raw))
		defer srv.Close()
		// A caller with no client of its own is the ordinary case, and every
		// call in this package passes the client straight to c.Get. Remove the
		// nil default and this is a nil-pointer panic in a fetch path, not an
		// error somebody can report.
		// A path, not a bare host: Fetch appends ".sig" to the base, and
		// "http://127.0.0.1:60072" + ".sig" is not a URL.
		got, sig, err := Fetch(nil, srv.URL+"/feed.json")
		if err != nil {
			t.Fatalf("a nil client did not get a default: %v", err)
		}
		if _, err := Verify(got, sig, pub); err != nil {
			t.Errorf("the bytes a defaulted client fetched do not verify: %v", err)
		}
	})
}
