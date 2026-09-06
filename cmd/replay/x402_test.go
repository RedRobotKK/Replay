package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
var allowedImports = map[string]bool{
	// Standard library, as actually used. Kept explicit: the point is that
	// adding to this list is a decision someone makes and a reviewer sees.
	"bufio": true, "bytes": true, "compress/gzip": true, "context": true,
	"encoding/hex": true, "encoding/json": true, "errors": true, "flag": true,
	"fmt": true, "hash/fnv": true, "io": true, "io/fs": true, "log": true,
	"math": true, "math/rand": true, "math/rand/v2": true, "net": true,
	"net/http": true, "net/http/httptest": true, "net/http/httputil": true,
	"net/url": true, "os": true, "os/signal": true, "path": true,
	"path/filepath": true, "reflect": true, "regexp": true, "runtime": true,
	"slices": true, "sort": true, "strconv": true, "strings": true,
	"sync": true, "sync/atomic": true, "syscall": true, "testing": true,
	"time": true, "unicode": true, "unicode/utf8": true,

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
			case ".git", "node_modules", "dist", "bin":
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
	fset := token.NewFileSet()
	werr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", "dist", "bin":
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
}

func TestX402_AllowlistIsMeaningful(t *testing.T) {
	for _, banned := range []string{
		"crypto/ecdsa", "crypto/ed25519", "crypto/elliptic", "crypto/ecdh", "math/big",
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
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if !strings.Contains(err.Error(), "http") {
		t.Errorf("the error should name the cleartext hop, got: %v", err)
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
