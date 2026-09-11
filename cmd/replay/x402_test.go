package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// Conformance for the x402 surface on `replay rules --update`.
//
// The claim under test is narrow and worth stating plainly: when a rules feed
// asks to be paid, Replay reports the demand exactly and installs nothing, and
// there is no path through this program that pays. Each case names its pass and
// fail condition, because a payment surface that quietly does nothing looks the
// same as one that quietly does something.
//
//   X1  a well-formed 402 is parsed, and the terms survive intact
//   X2  a 402 that is not x402 is an error, never a silent success
//   X3  --x402-json emits valid JSON that states paid:false
//   X4  the prose names amount, network and payee, and refuses in words
//   X5  nothing is installed and no existing rules file is touched
//   X6  no code in this module can hold a key or sign a transaction
//   X7  a 200 still installs — the 402 branch did not break the happy path

const validDemand = `{
  "x402Version": 1,
  "error": "payment required",
  "accepts": [{
    "scheme": "exact",
    "network": "base",
    "maxAmountRequired": "2.50",
    "asset": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
    "payTo": "0x585ef883e750694E4ba1463bc20820e9C4fBF369",
    "resource": "https://example.test/rules.json",
    "mimeType": "application/json",
    "maxTimeoutSeconds": 60,
    "description": "Replay rules feed, monthly"
  }]
}`

// demandServer answers every request with the given status and body.
func demandServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

// updateFrom runs the update path against a server, with the client's TLS
// trust widened to the test server's own certificate only.
func updateFrom(t *testing.T, srv *httptest.Server, rulesFile string, asJSON bool) (string, error) {
	t.Helper()
	restore := swapTransport(srv.Client().Transport.(*http.Transport))
	t.Cleanup(restore)
	var out bytes.Buffer
	err := updateRules(srv.URL, rulesFile, &out, false, asJSON)
	return out.String(), err
}

// X1: a well-formed 402 parses, and every field arrives as the server sent it.
// PASS: the error is a *paymentRequiredError and each field round-trips.
// FAIL: any other error type, a nil error, or a mangled field.
func TestX402_DemandIsParsed(t *testing.T) {
	srv := demandServer(t, http.StatusPaymentRequired, validDemand)
	_, err := updateFrom(t, srv, filepath.Join(t.TempDir(), rulesFileName), false)

	var pay *paymentRequiredError
	if !errors.As(err, &pay) {
		t.Fatalf("a 402 must surface as *paymentRequiredError, got %T: %v", err, err)
	}
	if pay.Terms.X402Version != 1 {
		t.Errorf("x402Version = %d, want 1", pay.Terms.X402Version)
	}
	if len(pay.Terms.Accepts) != 1 {
		t.Fatalf("accepts = %d options, want 1", len(pay.Terms.Accepts))
	}
	got := pay.Terms.Accepts[0]
	for _, c := range []struct{ field, got, want string }{
		{"maxAmountRequired", got.Amount, "2.50"},
		{"network", got.Network, "base"},
		{"payTo", got.PayTo, "0x585ef883e750694E4ba1463bc20820e9C4fBF369"},
		{"scheme", got.Scheme, "exact"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.field, c.got, c.want)
		}
	}
}

// X2: a 402 carrying something other than x402 terms is an error.
// PASS: an error mentioning 402, and not a *paymentRequiredError.
// FAIL: a nil error, or terms invented from a body that carried none.
func TestX402_NonX402BodyIsAnError(t *testing.T) {
	for _, body := range []string{
		`{"message":"pay up"}`,
		`<html>402</html>`,
		``,
		`{"x402Version":1,"accepts":[]}`,
	} {
		srv := demandServer(t, http.StatusPaymentRequired, body)
		_, err := updateFrom(t, srv, filepath.Join(t.TempDir(), rulesFileName), false)
		if err == nil {
			t.Fatalf("body %q: a 402 without usable terms must be an error", body)
		}
		var pay *paymentRequiredError
		if errors.As(err, &pay) {
			t.Errorf("body %q: reported payment terms that the body did not contain", body)
		}
	}
}

// X3: --x402-json emits JSON a spending policy can read.
// PASS: stdout parses as JSON and states paid:false with the terms intact.
// FAIL: prose on stdout, invalid JSON, or a missing/true paid field.
func TestX402_JSONOutput(t *testing.T) {
	srv := demandServer(t, http.StatusPaymentRequired, validDemand)
	out, _ := updateFrom(t, srv, filepath.Join(t.TempDir(), rulesFileName), true)

	var got struct {
		Resource string `json:"resource"`
		Paid     *bool  `json:"paid"`
		Terms    struct {
			Accepts []struct {
				Amount string `json:"maxAmountRequired"`
				PayTo  string `json:"payTo"`
			} `json:"accepts"`
		} `json:"payment_required"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("--x402-json must emit JSON only; got %q: %v", out, err)
	}
	if got.Paid == nil {
		t.Fatal("the JSON must state paid explicitly; a reader cannot assume it")
	}
	if *got.Paid {
		t.Fatal("paid:true — replay must never report having paid, because it cannot pay")
	}
	if len(got.Terms.Accepts) != 1 || got.Terms.Accepts[0].Amount != "2.50" {
		t.Errorf("terms did not survive into the JSON: %q", out)
	}
	if got.Terms.Accepts[0].PayTo != "0x585ef883e750694E4ba1463bc20820e9C4fBF369" {
		t.Errorf("payee did not survive into the JSON: %q", out)
	}
}

// X4: the prose is usable by a person: it names the cost and refuses in words.
// PASS: amount, network and payee appear, and the refusal is stated.
// FAIL: any of them missing, so a reader cannot tell what was asked or why
// nothing happened.
func TestX402_ProseNamesTermsAndRefuses(t *testing.T) {
	srv := demandServer(t, http.StatusPaymentRequired, validDemand)
	out, _ := updateFrom(t, srv, filepath.Join(t.TempDir(), rulesFileName), false)

	for _, want := range []string{
		"2.50",
		"base",
		"0x585ef883e750694E4ba1463bc20820e9C4fBF369",
		"will not pay",
		"holds no wallet",
		"--update ./rules.json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the explanation must contain %q; got:\n%s", want, out)
		}
	}
	// The seller's own words are shown, but quoted, so they read as the
	// seller's claim rather than as Replay's.
	if !strings.Contains(out, `"Replay rules feed, monthly"`) {
		t.Errorf("the seller's description must be shown quoted; got:\n%s", out)
	}
}

// X5: a 402 installs nothing and leaves an existing document alone.
// PASS: the rules file is byte-identical afterwards.
// FAIL: any write, truncation or deletion.
func TestX402_InstallsNothing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, rulesFileName)
	before := []byte(`{"version":"pinned-by-this-test"}`)
	if err := os.WriteFile(file, before, 0o600); err != nil {
		t.Fatal(err)
	}
	srv := demandServer(t, http.StatusPaymentRequired, validDemand)
	if _, err := updateFrom(t, srv, file, false); err == nil {
		t.Fatal("a 402 must not report success")
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("the existing rules document was removed: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the rules document changed on a 402:\nbefore %s\nafter  %s", before, after)
	}
}

// X6: this module cannot sign a transaction, enforced by an import allowlist.
//
// The load-bearing promise of ADR-0013, and the reason the binary is safe to
// pipe from curl onto a machine holding provider credentials.
//
// This replaced a grep for banned strings on 2026-09-05, after two independent
// reviewers each wrote a working ECDSA signer that passed it. The grep
// constrained spelling, not capability: `crypto/ecdsa` is not needed to sign —
// `crypto/elliptic` with `math/big` is enough, and secp256k1, which is what
// Ethereum actually uses, is not in `crypto/ecdsa` anyway. `X-PAYMENT` was
// case-sensitive while Go canonicalises header names, so `x-payment` produced
// a byte-identical wire header and passed. `PrivateKey` and `Sign(` are
// identifier names: rename to `secret` and `authorize` and they vanish. It also
// false-positived on `big.Int.Sign()`, which creates pressure to loosen it.
//
// An allowlist inverts that. Signing needs arbitrary-precision arithmetic or a
// curve implementation, and both arrive as imports. A new import is a
// deliberate line in this list, reviewed next to the reason it exists, rather
// than a spelling nobody thought to ban.
//
// PASS: every import in every .go file is listed, go.mod declares no
// dependency, and there is no cgo or assembly to hide an implementation in.
// FAIL: anything else, which is a prompt to think rather than to widen.
// execExempt names the files allowed to import os/exec outside the `mutation`
// build tag, one path at a time.
//
// The confinement exists so this binary cannot be made to run arbitrary things.
// internal/selfupdate/fetch.go breaks it for one call, and the exemption is
// written as a path rather than a package so widening it is an edit somebody
// reviews rather than a side effect of adding a file.
//
// What the call is, at fetch.go:215:
//
//	exec.CommandContext(ctx, staged, "version")
//
// `staged` is a path this process built itself from os.Executable's directory
// and a PID suffix — no part of it comes from the archive, the network or the
// user. The bytes at that path were checksum-verified against checksums.txt
// before it was written. The argument is the constant "version". There is no
// shell, no PATH lookup, and a 20-second timeout.
//
// Why it earns the exemption: everything before it verifies the bytes that
// arrived, and none of it proves the result runs HERE. A binary for the wrong
// architecture, a chmod that did not take, a libc mismatch — each installs
// cleanly, fails on first use, and would be reported as a successful upgrade.
// Running the staged copy once is what turns that into a refusal with the old
// binary still in place.
//
// The narrower reading is the one to hold onto: this does not permit
// internal/selfupdate to run things. It permits it to run the file it just
// verified, by absolute path, with a fixed argument. TestX402_SelfUpdateExecIsNotArbitrary
// asserts that shape, so the exemption cannot quietly grow into a general one.
var execExempt = map[string]bool{
	"internal/selfupdate/fetch.go": true,
	// scripts/guard-reachability is a developer tool carrying //go:build
	// ignore, so it is excluded from every build of this module and cannot
	// reach the shipped binary. It runs `git diff` and `go test`, which is the
	// job: it neutralises each conditional a change touches and reports the
	// ones no test observes.
	//
	// Exempted by path rather than by the build tag, on purpose. A tag is one
	// line anybody can add, and "it says ignore" is a weaker claim than a
	// reviewer having agreed this specific file may exec. The stale-exemption
	// check below still applies: if it stops importing os/exec the entry has
	// to come out.
	"scripts/guard-reachability/main.go": true,
	// scripts/refusal-reachability is the same kind of tool under the same
	// //go:build ignore, and earns its exemption on the same terms: it is
	// excluded from every build of this module, it runs only `go test` on
	// packages of this repository, and the exec is the job — it neutralises the
	// guard in front of each refusal and reports the refusals no test can tell
	// from any other outcome.
	//
	// It runs no `git` and takes no path from a user: the packages it tests come
	// from walking cmd/ and internal/ for .go files. Adding it here is a second
	// entry a reviewer reads, which is the intended cost.
	"scripts/refusal-reachability/main.go": true,
}

var allowedImports = map[string]bool{
	// Standard library, as actually used. Kept explicit: the point is that
	// adding to this list is a decision someone makes and a reviewer sees.
	"bufio": true, "bytes": true, "compress/gzip": true, "context": true,
	"encoding/hex": true, "encoding/json": true, "errors": true, "flag": true,
	"fmt": true, "hash/fnv": true, "io": true, "io/fs": true, "log": true,
	// html, for escaping text into the generated screen SVGs in
	// screens_svg_test.go. Escaping only: html.EscapeString takes a string and
	// returns one. It opens nothing, runs nothing and signs nothing, and the
	// alternative is hand-rolled replacement of five characters, which is how
	// an image of a screen ends up carrying a broken tspan the day somebody
	// puts an ampersand in a model name.
	"html": true,
	// archive/tar, for the release tarball in internal/selfupdate. It reads
	// and never writes, which is the whole reason it is admissible here: the
	// hazard with tar is an entry whose name escapes the destination
	// directory, and unpack() has no destination directory. It matches on
	// filepath.Base(hdr.Name), reads the one matching regular file into
	// memory through an io.LimitReader, and refuses a symlink, a hard link or
	// anything that is not a regular file rather than following it. Nothing in
	// the archive ever names a path this code writes to.
	//
	// It also cannot sign or pay, which is what this test is actually about.
	"archive/tar": true,
	"math":        true, "math/rand": true, "math/rand/v2": true, "net": true,
	"net/http": true, "net/http/httptest": true, "net/http/httputil": true,
	"net/url": true, "os": true, "os/signal": true, "path": true,
	"path/filepath": true, "reflect": true, "regexp": true, "runtime": true,
	"slices": true, "sort": true, "strconv": true, "strings": true,
	"sync": true, "sync/atomic": true, "syscall": true, "testing": true,
	"time": true, "unicode": true, "unicode/utf8": true,
	// unicode/utf16, for one thing: pairing surrogate halves when measuring
	// a JSON string escape. internal/transcript/wire.go calls IsSurrogate and
	// DecodeRune, both pure functions over runes. ContentBytes has to agree
	// with encoding/json byte for byte on what "\ud83d\ude00" weighs, and
	// encoding/json pairs surrogates through this same package.
	//
	// It opens nothing, runs nothing and signs nothing, which is what this
	// test is about.
	"unicode/utf16": true,

	// unsafe, for exactly one thing: handing a termios struct to
	// syscall.Syscall so `replay tui` can read one keypress at a time.
	// internal/tui/term_unix.go is its only use, and the ioctl numbers are
	// per-platform constants beside it.
	//
	// Weighed against the two alternatives, which each spend something real. A
	// dependency on golang.org/x/term ends a go.mod with no requires at all,
	// and for a binary that sits in front of your traffic holding a token
	// that is a claim worth more than thirty lines. Asking for Enter after
	// every keystroke is a worse surface for the people this is built for.
	//
	// What this does NOT enable is the point of the list. unsafe grants no
	// cryptography: crypto/ecdsa, crypto/ed25519, crypto/elliptic, crypto/ecdh
	// and math/big remain absent and still fail this test. Pointer arithmetic
	// cannot produce a signature over a transaction, and the guard against
	// that is unchanged.
	"unsafe": true,

	// crypto/ed25519, for verifying a signed vendor feed and nothing else.
	//
	// This list named ed25519 as deliberately absent, so adding it is the
	// conversation the list exists to force, and this is that conversation
	// written down rather than a quiet edit.
	//
	// What forced it. Model prices, caching rules and wire-format behaviour
	// move faster than releases: the published v0.5.0 binary carried a price
	// table 75 days old, and `replay corpus` already detects a provider
	// changing behaviour underneath this build. A correctness instrument
	// reasoning from stale facts is worse than none, because it is
	// confidently wrong. Learning that your facts are old needs a fetch, and
	// a fetch that drives figures needs its contents authenticated rather
	// than its transport.
	//
	// Why not the alternatives, each of which was preferred and does not
	// work. cosign is the project's existing signing story and needs os/exec,
	// which this binary keeps out on purpose. crypto/hmac is already here but
	// needs a shared secret, and a secret compiled into a public binary is
	// not one. TLS alone authenticates the host, not the bytes, so a
	// compromise of redrobot.jp would silently change every reader's figures,
	// which is a worse provenance than "compiled in, 75 days old, and it says
	// so".
	//
	// What this costs, stated plainly rather than argued away. The guard was
	// capability-based on purpose: importing ed25519 gives this binary the
	// ability to sign, not only to verify, and the reason the guard existed is
	// that this binary sits in front of your traffic holding a token. That
	// capability now exists and the compensating controls are structural
	// rather than a promise. internal/feed verifies and never signs; it has
	// no private key and no code path that could produce one, since
	// ed25519.GenerateKey and ed25519.Sign appear in no non-test file.
	// TestX402_NoSigningCapability still fails on crypto/ecdsa,
	// crypto/elliptic, crypto/ecdh and math/big, so the x402 payment path is
	// as closed as it was: an ed25519 signature is not an Ethereum one.
	//
	// The narrower guard that replaces the blanket one is
	// TestFeedVerifiesAndNeverSigns, which reads the tree rather than trusting
	// this paragraph.
	"crypto/ed25519": true,

	// Cryptography, narrowly. These are for the secret vault in `serve
	// --mask`: symmetric encryption of masked values at rest, and hashing for
	// identity. None of them can produce a signature over a transaction.
	//
	// What is deliberately absent is the whole point: crypto/ecdsa,
	// crypto/ed25519, crypto/elliptic, crypto/ecdh and math/big. Adding any of
	// them fails this test, and that is the conversation this list exists to
	// force.
	"crypto/aes": true, "crypto/cipher": true, "crypto/hmac": true,
	"crypto/rand": true, "crypto/sha256": true,

	// Used by this test to read the imports of every other file. It caught
	// itself on the first run, which is the cheapest possible demonstration
	// that the walk reaches real files and the list is enforced.
	"go/parser": true, "go/token": true,

	// go/ast: internal/observation's own import allowlist walks its syntax
	// tree. Reading code, not emitting it.
	"go/ast": true,

	// os/exec: internal/mutation invokes `go build` and `go test` to apply a
	// mutant and ask whether a test notices. It cannot be avoided — the
	// harness has to run the compiler. It is the strongest capability on this
	// list, so it is bounded structurally rather than by promise: the test
	// below requires every file importing it to sit behind the `mutation`
	// build tag, which excludes it from every ordinary build and from
	// `go test -c` without that tag.
	"os/exec": true,

	// go/build/constraint: parses build tags so the os/exec confinement above
	// is a real constraint check and not a substring match.
	"go/build/constraint": true,

	// Raster images, for the social card `replay cost --share --png` writes.
	// internal/card decodes embedded glyph atlases and composes pixels into a
	// PNG. embed reads files compiled into the binary at build time; the other
	// three decode, allocate and encode raster images.
	//
	// None of them opens a socket, runs a process, reads a key or touches the
	// filesystem at runtime. They are also what keeps this list short: the
	// alternative to drawing text this way is a font library, which would be
	// the first require line in go.mod and would fail the check above.
	"embed": true, "image": true, "image/color": true, "image/png": true,
}

func TestX402_NoSigningCapability(t *testing.T) {
	root := filepath.Join("..", "..")

	// A dependency could carry a signer regardless of what our own files
	// import, so the zero-dependency claim is part of the guarantee.
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mod), "require") {
		t.Errorf("go.mod declares a dependency; a wallet library must never be one:\n%s", mod)
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
		t.Error("go.sum exists, so something is depended on; this module must stay dependency-free")
	}

	var offenders []string
	fset := token.NewFileSet()
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			// .claude holds agent worktrees: full copies of this repo, which
			// would be walked as if they were source.
			case ".git", ".claude", "node_modules", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		// Assembly and C are compiled into the binary and contain no Go
		// imports, so an allowlist over imports would never see them. There
		// are none today; this keeps it that way.
		switch filepath.Ext(path) {
		case ".s", ".c", ".h", ".cc", ".cpp":
			offenders = append(offenders, path+": non-Go source can hold an implementation this test cannot read")
			return nil
		case ".go":
		default:
			return nil
		}

		// _test.go files are included. `go test -c` produces a binary, so a
		// signer in a test file is still a signer that ships if anyone builds
		// one.
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", path, perr)
		}
		rel, _ := filepath.Rel(root, path)
		for _, im := range f.Imports {
			p, uerr := strconv.Unquote(im.Path.Value)
			if uerr != nil {
				continue
			}
			if p == "C" {
				offenders = append(offenders, rel+": cgo, which can call anything")
				continue
			}
			if strings.HasPrefix(p, "github.com/RedRobotKK/Replay/") {
				continue
			}
			if !allowedImports[p] {
				offenders = append(offenders, rel+": "+p)
			}
		}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.Contains(c.Text, "go:generate") {
					offenders = append(offenders, rel+": go:generate can produce code this test never sees")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("imports outside the allowlist — this binary must not be able to sign or pay.\n"+
			"If one of these is genuinely needed, add it to allowedImports with a comment saying why:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// X6b: the allowlist is not vacuous.
//
// A list that happened to contain everything, or a walk that visited nothing,
// would pass X6 while enforcing nothing at all.
// PASS: the walk sees real files, and the curve and bignum packages a signer
// needs are absent from the list.
// FAIL: either, which means X6 is decoration.

// requiresTag reports whether a file is compiled ONLY when tag is set.
//
// Deliberately not a substring match. The first version of this check tested
// strings.Contains(body, "//go:build mutation"), which `//go:build mutationX`
// satisfies — so removing the real tag left the check green. It could not
// fail, which is the defect class this whole file exists to prevent, reached
// by writing the guard carelessly rather than by anyone weakening it.
//
// The constraint is parsed and evaluated twice: once with only tag true, once
// with nothing true. A file that builds in the second case does not require
// the tag at all.
func requiresTag(body, tag string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//go:build") {
			if line != "" && !strings.HasPrefix(line, "//") {
				return false // past the header
			}
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			return false
		}
		withTag := expr.Eval(func(t string) bool { return t == tag })
		without := expr.Eval(func(string) bool { return false })
		return withTag && !without
	}
	return false
}

// X6c: the strongest import on the allowlist is confined to a build tag.
//
// os/exec can call anything, including a signer this test cannot read. It is
// on the list because the mutation harness must invoke the compiler, and that
// is a real need — but "only the mutation harness uses it" is a promise unless
// something checks. This checks: a file importing os/exec must carry the
// `mutation` build tag, so it is absent from every ordinary build and from a
// `go test -c` that does not ask for it.
//
// PASS: every os/exec importer is build-tagged.
// FAIL: one that is not, which would put an arbitrary-execution capability
// into the shipped binary through the back door this allowlist exists to shut.
func TestX402_ExecIsConfinedToTheMutationHarness(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	var seen int
	exempted := map[string]bool{}
	fset := token.NewFileSet()
	werr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			// .claude holds agent worktrees: full copies of this repo, which
			// would be walked as if they were source.
			case ".git", ".claude", "node_modules", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
		if perr != nil {
			return nil
		}
		uses := false
		for _, im := range f.Imports {
			if p, uerr := strconv.Unquote(im.Path.Value); uerr == nil && p == "os/exec" {
				uses = true
			}
		}
		if !uses {
			return nil
		}
		seen++
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if execExempt[rel] {
			exempted[rel] = true
			return nil
		}
		if !requiresTag(string(body), "mutation") {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if werr != nil {
		t.Fatal(werr)
	}
	if seen == 0 {
		t.Fatal("no file imports os/exec, so this check asserts nothing. Remove os/exec from " +
			"allowedImports rather than keeping a permission nothing uses.")
	}
	if len(offenders) > 0 {
		t.Errorf("os/exec imported outside the `mutation` build tag:\n  %s",
			strings.Join(offenders, "\n  "))
	}
	// An exemption that no longer applies is a permission nobody is using and
	// everybody is trusting. If the file stopped importing os/exec, the entry
	// comes out — the same rule this test already applies to the allowlist as
	// a whole a few lines up.
	for name := range execExempt {
		if !exempted[name] {
			t.Errorf("%s is exempted from the os/exec confinement and does not import "+
				"os/exec. Remove the exemption rather than leaving a permission open.", name)
		}
	}
}

func TestX402_AllowlistIsMeaningful(t *testing.T) {
	// crypto/ed25519 left this list on 2026-09-08, and the reasoning is at its
	// entry in allowedImports above.
	//
	// What this test protects is that the binary cannot sign an x402 payment.
	// That needs a secp256k1 ECDSA signature, which needs crypto/ecdsa with
	// crypto/elliptic or crypto/ecdh, and math/big. All four are still banned
	// here and an ed25519 signature is not an Ethereum one, so the payment
	// path is exactly as closed as it was.
	//
	// The broader property, that the binary cannot sign ANYTHING, is genuinely
	// weaker than it was, and pretending otherwise would be the failure this
	// file is built to prevent. It is replaced by a narrower check that reads
	// the tree rather than the list: TestFeedVerifiesAndNeverSigns.
	for _, banned := range []string{
		"crypto/ecdsa", "crypto/elliptic", "crypto/ecdh", "math/big",
	} {
		if allowedImports[banned] {
			t.Errorf("%s is allowlisted; a signer can be written with it", banned)
		}
	}
	var seen int
	_ = filepath.Walk(filepath.Join("..", ".."), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".go") {
			seen++
		}
		return nil
	})
	if seen < 30 {
		t.Errorf("the walk found only %d Go files; it is not covering the module", seen)
	}
}

// X7: a 200 still installs. The 402 branch sits in the same function as the
// success path, so this guards against having broken it.
// PASS: the document is written and the returned error is nil.
// FAIL: an error, or nothing on disk.
func TestX402_SuccessPathStillWorks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, rulesFileName)
	doc, err := os.ReadFile("testdata/rules-valid.json")
	if err != nil {
		// Not a skip. Go reports a skipped test as a passing package, so a
		// fixture missing from a checkout — which it would be if testdata were
		// ever left untracked — would silently delete the only guard that the
		// 402 branch did not break the 200 path.
		t.Fatalf("the rules fixture is required for this test to mean anything: %v", err)
	}
	srv := demandServer(t, http.StatusOK, string(doc))
	if _, err := updateFrom(t, srv, file, false); err != nil {
		t.Fatalf("a 200 must still install: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("nothing was installed: %v", err)
	}
}

// X8: the exit code distinguishes "pay for this" from "this is broken".
// PASS: a payment demand exits 2, anything else exits 1.
// FAIL: both exiting 1, which forces an agent to parse prose to tell the two
// apart, or an ordinary failure exiting 2, which invites it to pay for a bug.
func TestX402_ExitCode(t *testing.T) {
	pay := &paymentRequiredError{Resource: "https://example.test/r"}
	if got := exitCode(pay); got != 2 {
		t.Errorf("a payment demand must exit 2, got %d", got)
	}
	if got := exitCode(fmt.Errorf("wrapped: %w", pay)); got != 2 {
		t.Errorf("a wrapped payment demand must still exit 2, got %d", got)
	}
	if got := exitCode(errors.New("the file is missing")); got != 1 {
		t.Errorf("an ordinary failure must exit 1, got %d", got)
	}
	if got := exitCode(errUsage); got != 1 {
		t.Errorf("a usage error must exit 1, got %d", got)
	}
}

// X9: a hostile 402 body cannot write control characters to the terminal.
//
// The 402 body comes from whatever URL a user typed, so every string in it is
// attacker-controlled. An earlier version printed amount, asset, network and
// payTo with %s, and a reviewer demonstrated clearing the screen, painting a
// fake "RULES INSTALLED OK", and forging lines in Replay's own voice by
// injecting newlines into the amount.
//
// The escapes are written as JSON \u sequences so this file stays plain ASCII;
// they decode to real control bytes before Explain ever sees them.
//
// PASS: no C0 or C1 control byte reaches the output, and no injected sentence
// survives as a line of its own.
// FAIL: any escape or newline passes through, which lets a hostile feed tell
// the user something Replay did not say.
func TestX402_HostileTermsCannotPaintTheTerminal(t *testing.T) {
	body := `{"x402Version":1,"accepts":[{"scheme":"exact","network":"base\u001b[2J\u001b[H\u001b[32mRULES INSTALLED OK\u001b[0m","maxAmountRequired":"0.00\n\nReplay has already paid this invoice.\n","asset":"0x\u001b]0;pwned\u0007","payTo":"0xdead\r\nAuthorized by Replay.","description":"\u001b[31mred\u001b[0m and \u202eoverridden","resource":"https://attacker.test/rules.json"}]}`

	terms, err := cachemodel.ParsePaymentRequired([]byte(body))
	if err != nil {
		t.Fatalf("the body is valid x402 and must parse: %v", err)
	}
	out := terms.Explain("https://attacker.test/rules.json")

	for i, r := range out {
		// Newline is the only control character this output legitimately uses.
		if r == '\n' {
			continue
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			t.Errorf("control character %#U reached the terminal at byte %d; a hostile feed can repaint the screen", r, i)
			break
		}
	}
	for _, forged := range []string{
		"Replay has already paid this invoice.",
		"Authorized by Replay.",
	} {
		for _, line := range strings.Split(out, "\n") {
			if strings.TrimSpace(line) == forged {
				t.Errorf("a hostile field became its own line: %q", forged)
			}
		}
	}
	if !strings.Contains(out, "will not pay") {
		t.Error("the refusal was lost")
	}
}

// X10: rendering is bounded even when the body is not.
// PASS: a body with thousands of options renders a capped, short explanation
// that says options were omitted.
// FAIL: unbounded output, which pushes the refusal out of scrollback and is a
// spoofing amplifier alongside anything that survives quoting.
func TestX402_ManyOptionsAreCapped(t *testing.T) {
	one := `{"scheme":"exact","network":"base","maxAmountRequired":"1","payTo":"0xabc","asset":"0xdef"}`
	opts := make([]string, 5000)
	for i := range opts {
		opts[i] = one
	}
	body := `{"x402Version":1,"accepts":[` + strings.Join(opts, ",") + `]}`
	terms, err := cachemodel.ParsePaymentRequired([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	out := terms.Explain("https://attacker.test/r")
	if lines := strings.Count(out, "\n"); lines > 120 {
		t.Errorf("rendered %d lines for 5000 options; output must stay bounded", lines)
	}
	if !strings.Contains(out, "more payment options, not shown") {
		t.Error("the cap must say options were omitted, or the reader is misled about the terms")
	}
}

// X11: the cleartext refusal survives a redirect.
//
// The scheme check runs once, on the URL the user typed, and Go follows
// redirects across schemes by default. A reviewer stood up an https server
// that 302s to plain http and watched the rules install, with the stored
// document recording the original https address — so every later report
// asserted TLS that never happened.
//
// PASS: the fetch fails, nothing is installed, and the error names cleartext.
// FAIL: an install, or a success recording an origin the bytes did not come
// from.
func TestX402_RedirectToCleartextIsRefused(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schema":"replay.rules.v1","version":"served-over-cleartext","models":[{"match":"x","minPrefix":1}]}`))
	}))
	defer plain.Close()

	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, http.StatusFound)
	}))
	defer tls.Close()

	restore := swapTransport(tls.Client().Transport.(*http.Transport))
	defer restore()

	dir := t.TempDir()
	file := filepath.Join(dir, rulesFileName)
	var out bytes.Buffer
	err := updateRules(tls.URL, file, &out, false, false)
	if err == nil {
		t.Fatal("a redirect to plain http must not install rules")
	}
	// "cleartext", not "http". Go wraps a failed fetch in *url.Error, whose
	// message always begins `Get "http://...`, so a check for "http" was
	// satisfied by the wrapper no matter what the refusal said. Verified:
	// replacing the refusal with errors.New("nope") left this test green.
	//
	// The behavioural assertions either side of this one are not vacuous —
	// deleting CheckRedirect entirely makes the fetch succeed and both fire.
	// This is about the operator who has to read the error and understand why
	// their update refused.
	if !strings.Contains(err.Error(), "cleartext") && !strings.Contains(err.Error(), "plain http") {
		t.Errorf("the error does not name the cleartext hop, so an operator sees only a "+
			"failed fetch and retries it: %v", err)
	}
	if _, statErr := os.Stat(file); statErr == nil {
		t.Error("rules were installed over a cleartext hop")
	}
}

// X12: the flags are wired to the behaviour they name.
//
// Every earlier test called updateRules directly, so flag parsing was never
// exercised — which is how `--update <url> --export` came to exit 0 with a
// price table on stdout, having fetched and installed nothing. A script
// reading that as a successful update would be wrong and never know.
//
// PASS: conflicting flags refuse; --export alone emits an installable
// document.
// FAIL: any combination that silently does something other than what was
// asked.
func TestX402_FlagWiring(t *testing.T) {
	conflicts := [][]string{
		{"rules", "--update", "https://example.invalid/r.json", "--export"},
		{"rules", "--export", "--check-prices"},
		{"rules", "--update", "https://example.invalid/r.json", "--check-prices"},
		{"rules", "--export", "--dry-run"},
	}
	for _, args := range conflicts {
		var out, errb bytes.Buffer
		err := run(args, &out, &errb)
		if err == nil {
			t.Errorf("%v: must refuse rather than silently pick one", args)
		}
		if out.Len() > 0 {
			t.Errorf("%v: wrote %d bytes to stdout while refusing; a caller could read that as output", args, out.Len())
		}
	}

	var out, errb bytes.Buffer
	if err := run([]string{"rules", "--export"}, &out, &errb); err != nil {
		t.Fatalf("--export alone must work: %v", err)
	}
	var doc cachemodel.Rules
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("--export must emit JSON only: %v", err)
	}
	if doc.Schema != cachemodel.RulesSchema || len(doc.Models) == 0 {
		t.Errorf("--export emitted a document this build would not install: schema=%q models=%d", doc.Schema, len(doc.Models))
	}
}

// TestFeedVerifiesAndNeverSigns replaces the blanket ban on crypto/ed25519.
//
// The old guard was a property of a list. This one is a property of the tree,
// which is the stronger form: it fails when somebody writes a signer, not when
// somebody edits a permission.
//
// Three things are checked, and the third is the one that would actually
// catch a mistake. Only internal/feed may import ed25519, so the capability
// stays in one reviewable place. No non-test file may call Sign or
// GenerateKey, so the package is used for verification and nothing else. And
// the check refuses to pass when it has found no ed25519 at all, because a
// guard that asserts nothing about an absent feature reads exactly like one
// that is working.
func TestFeedVerifiesAndNeverSigns(t *testing.T) {
	root := filepath.Join("..", "..")
	var importers, production, signers []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// The same exclusions its two siblings use, and for the same reason.
		// .claude holds agent worktrees: whole copies of this repository, which
		// this walk otherwise reads as if they were source. It reported
		// internal/feed twice per worktree and failed a check about where the
		// signing capability lives — a true statement about a copy, and a
		// false one about the tree under test.
		if err == nil && info.IsDir() {
			switch info.Name() {
			case ".git", ".claude", "node_modules", "dist", "bin":
				return filepath.SkipDir
			}
		}
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		src := string(body)
		// Imports are parsed, not grepped.
		//
		// The first version matched the quoted path in the source and so
		// flagged this file, whose allowlist entry and banned-list comment
		// both contain the string. A guard that reports its own documentation
		// as a violation is a guard people switch off.
		f, perr := parser.ParseFile(token.NewFileSet(), path, body, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		imported := false
		for _, im := range f.Imports {
			if im.Path != nil && im.Path.Value == `"crypto/ed25519"` {
				imported = true
			}
		}
		if !imported {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		importers = append(importers, rel)
		if strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		production = append(production, rel)
		for _, call := range []string{"ed25519.Sign(", "ed25519.GenerateKey(", "ed25519.NewKeyFromSeed("} {
			if strings.Contains(src, call) {
				signers = append(signers, rel+": "+call)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Counted over NON-TEST files, and that distinction was a real hole.
	//
	// Counting every importer, the guard stayed silent when ed25519 was
	// removed from internal/feed and only its test still used it: the binary
	// would have carried no ed25519 at all while a permission for it sat in
	// the allowlist, and this check would have reported that as fine. A guard
	// whose evidence is its own test fixture is asserting nothing about the
	// thing it guards.
	if len(production) == 0 {
		t.Fatal("no non-test file imports crypto/ed25519, so the binary does not use it and " +
			"this guard asserts nothing. Put it back on the banned list in " +
			"TestX402_AllowlistIsMeaningful rather than keeping a permission nothing uses.")
	}
	for _, f := range importers {
		if !strings.HasPrefix(f, "internal/feed/") {
			t.Errorf("%s imports crypto/ed25519; only internal/feed may, so the signing "+
				"capability stays in one reviewable place", f)
		}
	}
	if len(signers) > 0 {
		t.Errorf("crypto/ed25519 is allowlisted for VERIFICATION only, and these produce "+
			"signatures or keys outside a test:\n  %s", strings.Join(signers, "\n  "))
	}
}

// TestX402_SelfUpdateExecIsNotArbitrary pins the shape of the one exempted
// exec call.
//
// execExempt lets internal/selfupdate/fetch.go import os/exec outside the
// mutation build tag. That permission is only defensible because of what the
// call looks like: a path this process constructed, bytes it checksum-verified,
// a constant argument, a context timeout, no shell and no PATH lookup.
//
// None of that is enforced by the exemption itself. An exemption granted for a
// narrow call is exactly the thing that widens later, because the next person
// sees a package that is already allowed to exec and adds a second call to it.
// So the shape is asserted rather than described.
func TestX402_SelfUpdateExecIsNotArbitrary(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "selfupdate", "fetch.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing the exempted file: %v", err)
	}

	var calls []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "exec" {
			return true
		}
		calls = append(calls, sel.Sel.Name)

		// CommandContext, so the child cannot outlive a timeout. Command
		// without one is how a hung upgrade becomes a hung terminal.
		if sel.Sel.Name != "CommandContext" {
			t.Errorf("exec.%s is used; only CommandContext is exempted, because a child "+
				"with no deadline outlives the upgrade that started it", sel.Sel.Name)
			return true
		}
		// Every argument after ctx and the binary path must be a literal. A
		// variable here would mean something outside this function chooses
		// what the freshly downloaded binary is asked to do.
		if len(call.Args) < 2 {
			t.Errorf("exec.CommandContext called with %d arguments", len(call.Args))
			return true
		}
		for i, a := range call.Args[2:] {
			lit, ok := a.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Errorf("argument %d to the staged binary is not a string literal, so "+
					"what it is asked to run is decided somewhere else", i+1)
				continue
			}
			if lit.Value != `"version"` {
				t.Errorf("the staged binary is run with %s; the exemption covers the "+
					"smoke test only, which is `version`", lit.Value)
			}
		}
		return true
	})

	// The exemption must cover something. If the call is gone, the entry in
	// execExempt should go with it.
	if len(calls) == 0 {
		t.Fatal("internal/selfupdate/fetch.go makes no exec call, so its entry in " +
			"execExempt is a permission nothing uses")
	}
	if len(calls) > 1 {
		t.Errorf("internal/selfupdate/fetch.go now makes %d exec calls (%v). The exemption "+
			"was granted for one smoke test; a second call is a new decision.",
			len(calls), calls)
	}
}
