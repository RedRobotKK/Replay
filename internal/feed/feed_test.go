package feed

import (
	"crypto/ed25519"
	"encoding/json"
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
