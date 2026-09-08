// Package feed fetches dated vendor facts and refuses to believe them.
//
// The problem it exists for is measured rather than assumed. The published
// v0.5.0 binary carried a price table 75 days old, and this tool already
// detects a provider changing behaviour underneath it: `replay corpus` flags
// claude-opus-4-8 as stale from its own calibration. Model prices, caching
// rules and wire-format behaviour move faster than releases do, and a
// correctness instrument reasoning from stale facts is worse than none,
// because it is confidently wrong.
//
// The split this package encodes is the whole design: fetching is automatic,
// believing is not.
//
// A table arriving over the network is a supply-chain input, and this binary
// publishes figures people act on. So a bundle is verified against a key
// compiled in here, cached, and reported as available. It does not change a
// number until someone adopts it with a typed command, the same path
// `replay rules --update` already takes. That keeps the promise in rules.go
// intact for results while still letting a reader learn their facts are old,
// which is the part they cannot work out alone.
//
// Adding crypto/ed25519 reopened an import allowlist that named it as
// deliberately absent. The reasoning is recorded beside the entry in
// cmd/replay/x402_test.go rather than here, because that is where a reviewer
// meets it.
package feed

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// vendorHosts is the set of hosts a feed may be fetched from.
//
// Separate from, and additional to, the import allowlist. That list governs
// what the binary is capable of; this governs where it will talk. A signature
// makes the host untrusted-but-verifiable rather than trusted, so this is
// defence in depth and not the primary control: a compromised host serving
// content it cannot sign gets refused by Verify, and this stops the fetch
// happening at all.
var vendorHosts = map[string]bool{
	"redrobot.jp": true,
}

// publicKey is the vendor key a bundle must be signed with.
//
// Deliberately empty in this tree. No key has been provisioned, so every
// verification fails closed and says why, and the feature cannot be switched
// on by accident before somebody decides where the private half lives. A
// verifier with no key that returns success is the defect this project keeps
// finding, so the zero value is the refusing one.
var publicKey ed25519.PublicKey

// VerifyFromVendor checks a bundle against the key compiled into this build.
//
// The entry point callers should use. Verify takes a key so it can be tested
// against a generated one, and a caller choosing its own key is a caller that
// can be handed the wrong one: which key to trust is a property of the build,
// not of the call site.
//
// With no key provisioned this refuses everything and says so, which is the
// current state of this tree.
func VerifyFromVendor(raw, sig []byte) (Bundle, error) { return Verify(raw, sig, publicKey) }

// HasVendorKey reports whether this build can verify a feed at all.
//
// For `replay doctor`, so the answer to "why is nothing being fetched" is on
// the screen rather than in a comment.
func HasVendorKey() bool { return len(publicKey) == ed25519.PublicKeySize }

// MaxBundle caps what will be read from the network.
//
// A signature is checked after the bytes are in memory, so the size limit is
// the only thing standing between a hostile response and this process's
// memory. It is enforced before verification, not after.
const MaxBundle = 1 << 20

// Bundle is a dated set of vendor facts.
//
// It carries versions rather than the tables themselves at this stage: what a
// reader needs first is whether theirs is behind, and answering that needs no
// payload. Adding the tables is a later change, and a larger one, because it
// is the change that lets network content reach a figure.
type Bundle struct {
	// Version is monotonic and exists for rollback refusal. A date alone is
	// not enough: dates can repeat and can be wound back.
	Version int `json:"version"`
	// Published is the human-facing date, shown beside any staleness note.
	Published string `json:"published"`
	// RulesVersion and PriceTableVersion are what the vendor currently
	// publishes, to compare against what this binary compiled in.
	RulesVersion      string `json:"rulesVersion"`
	PriceTableVersion string `json:"priceTableVersion"`
	// FactsVersion covers the wire-format behaviour table.
	FactsVersion string `json:"factsVersion,omitempty"`

	// Applied is false for everything this package produces, and is here so a
	// caller cannot price a report from network content by forgetting to ask.
	// Only an explicit adoption path may set it, and none exists yet.
	Applied bool `json:"-"`
}

// Verify checks a detached signature over exactly these bytes and parses them.
//
// The order matters and is the reason this is one function rather than two: a
// caller that parses first and verifies later has a window in which unverified
// structure is available to be acted on, and callers take that window.
func Verify(raw, sig []byte, pub ed25519.PublicKey) (Bundle, error) {
	if len(pub) != ed25519.PublicKeySize {
		return Bundle{}, errors.New("no vendor key is configured in this build, so no feed can be trusted; " +
			"figures come from the compiled tables")
	}
	if len(sig) != ed25519.SignatureSize {
		return Bundle{}, fmt.Errorf("signature is %d bytes, want %d", len(sig), ed25519.SignatureSize)
	}
	if !ed25519.Verify(pub, raw, sig) {
		return Bundle{}, errors.New("the feed's signature does not match its contents or this build's vendor key; nothing was read from it")
	}
	var b Bundle
	if err := json.Unmarshal(raw, &b); err != nil {
		return Bundle{}, fmt.Errorf("the feed verified but is not readable: %w", err)
	}
	if b.Version <= 0 {
		return Bundle{}, errors.New("the feed carries no version, so a rollback could not be detected")
	}
	return b, nil
}

// NewerThan reports whether b may replace held, or why it may not.
//
// A signature proves origin, not freshness. Without this an attacker able to
// serve responses replays an old but validly signed bundle and pins a reader
// to prices that were true once.
func (b Bundle) NewerThan(held Bundle) error {
	if b.Version <= held.Version {
		return fmt.Errorf("the feed offers version %d and this machine already holds %d, so it was not taken: "+
			"a validly signed but older bundle is how a rollback looks", b.Version, held.Version)
	}
	return nil
}

// CheckSource reports whether a URL may be fetched at all.
func CheckSource(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a URL: %s", raw)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("refusing to fetch a feed over %q: only https", u.Scheme)
	}
	// Hostname(), not Host, so a port cannot smuggle a different name past
	// the comparison, and an exact match so redrobot.jp.evil.test does not
	// pass a suffix test.
	if !vendorHosts[strings.ToLower(u.Hostname())] {
		return fmt.Errorf("%s is not a vendor this build fetches from", u.Hostname())
	}
	return nil
}

// Fetch retrieves a bundle and its detached signature.
//
// It returns bytes and does not verify them: verification needs the key and
// belongs to the caller that holds it, and a fetch that verified would tempt
// somebody to use its return value when it errored.
func Fetch(c *http.Client, base string) (raw, sig []byte, err error) {
	if raw, err = get(c, base); err != nil {
		return nil, nil, err
	}
	if sig, err = get(c, base+".sig"); err != nil {
		return nil, nil, err
	}
	return raw, sig, nil
}

func get(c *http.Client, u string) ([]byte, error) {
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Get(u)
	if err != nil {
		// Said plainly, because this is the ordinary case on a laptop that is
		// not online and it must not read as a fault in the tool.
		return nil, fmt.Errorf("could not reach the feed at %s: %w", u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not reach the feed at %s: HTTP %d", u, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxBundle+1))
	if err != nil {
		return nil, fmt.Errorf("could not reach the feed at %s: %w", u, err)
	}
	if len(b) > MaxBundle {
		return nil, fmt.Errorf("the feed at %s is larger than %d bytes and was not read", u, MaxBundle)
	}
	return b, nil
}
