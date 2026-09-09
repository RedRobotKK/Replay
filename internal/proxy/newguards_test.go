package proxy

import (
	"net/url"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// New's two required fields, which nothing checked.
//
// `New` refuses a nil Upstream and a nil Store, and until now no test in this
// package asserted either. Verified by mutation: replacing the Store guard with
// `_ = cfg.Store` left `go test ./internal/proxy/` green, so the guard could
// have been deleted in any refactor and the suite would have agreed.
//
// The consequence is not a tidy error. Every request path writes a ledger
// record, so a Server built with no Store hands back a proxy that accepts a
// connection, forwards a turn — spending real money — and then panics on the
// write. The refusal is what turns that into a startup failure the operator
// sees before anything is billed.
//
// The loopback guard beside these two is already covered by server_test.go:458.
// These are the ones that were not.

// NG1: a nil upstream is refused, and the refusal names the field.
func TestNG1_ANilUpstreamIsRefused(t *testing.T) {
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: "127.0.0.1:0", Store: store})
	if err == nil {
		t.Fatalf("New accepted a nil Upstream and returned %v; the server would forward "+
			"every turn to nowhere", srv)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "upstream") {
		t.Errorf("the refusal does not name the missing field, so an operator cannot act "+
			"on it: %v", err)
	}
}

// NG2: a nil store is refused, and the refusal names the field.
//
// This is the one the mutation showed escaping.
func TestNG2_ANilStoreIsRefused(t *testing.T) {
	target, err := url.Parse("https://api.anthropic.com")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target})
	if err == nil {
		t.Fatalf("New accepted a nil Store and returned %v; the proxy would forward a "+
			"turn, spend on it, and panic writing the record", srv)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "store") &&
		!strings.Contains(strings.ToLower(err.Error()), "ledger") {
		t.Errorf("the refusal does not name the missing field: %v", err)
	}
}

// NG3: a complete config is accepted.
//
// Without this, NG1 and NG2 are satisfied by a New that refuses everything —
// the failure mode a pair of refusal tests invites, and the reason they are
// never written alone.
func TestNG3_ACompleteConfigIsAccepted(t *testing.T) {
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse("https://api.anthropic.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store}); err != nil {
		t.Fatalf("a config with everything New requires was refused: %v", err)
	}
}
