package observation

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/consent"
	"github.com/RedRobotKK/Replay/internal/probe"
)

// The submission payload.
//
// This package exists to draw one line: the set of things that may leave this
// machine, and nothing else. Everything below is that line written as an
// assertion, because a privacy boundary described only in a comment is a
// boundary that moves the first time somebody adds a convenient field.
//
// The binary does not send. It builds a file and prints its path; a human
// moves it. O7 is what stops that eroding. Note the scope: it is a claim about
// THIS PACKAGE, not the binary — the proxy reaches the network by definition
// and `probe --execute` originates billable requests. The auditable property
// is that contribution has no transport to reach, not that nothing does.

// sample is a reading with every field populated, so a test that walks the
// payload cannot pass by accident on a mostly-empty struct.
func sample() probe.Reading {
	return probe.Reading{
		TakenAt:      "2026-09-06T02:39:19Z",
		Method:       probe.MethodVersion,
		Model:        "claude-opus-5",
		AnsweredBy:   "claude-opus-5",
		ServiceTier:  "standard",
		Geo:          "global",
		Above:        508,
		AtMost:       512,
		Documented:   512,
		Outcome:      "non-deterministic",
		Anomalies:    []probe.Anomaly{{Kind: "non-deterministic", Size: 510, Wrote: 1, DidNotWrite: 1}},
		Inconclusive: 2,
		Probes:       12,
		Confirm:      3,
	}
}

func granted() consent.Decision {
	return consent.Decision{State: consent.Granted, Path: "/test/corpus-consent.toml"}
}

// O1: consent is a parameter, not a convention.
//
// The gate lives in this pure function rather than only at the call site,
// because a gate that only exists in a command is a gate the next command
// forgets. Unset and Declined are both refusals here, and they must stay
// distinguishable to the caller: somebody who was never asked should be asked,
// and somebody who said no should not be.
//
// PASS: only Granted builds anything.
// FAIL: a payload returned for any other state, which would be a submission
// built for somebody who never agreed to make one.
func TestO1_BuildRequiresGrantedConsent(t *testing.T) {
	for _, state := range []consent.State{consent.Unset, consent.Declined} {
		obs, err := Build("2026-09-anthropic-floor", consent.Decision{State: state}, Tag{Value: "abc", Basis: BasisLocal}, sample())
		if err == nil {
			t.Errorf("consent %s built a payload: %+v", state, obs)
		}
		if !reflect.DeepEqual(obs, Observation{}) {
			t.Errorf("consent %s returned a non-zero payload alongside its error", state)
		}
	}
	if _, err := Build("2026-09-anthropic-floor", granted(), Tag{Value: "abc", Basis: BasisLocal}, sample()); err != nil {
		t.Fatalf("granted consent must build: %v", err)
	}
}

// O2: the payload is exactly this set of fields.
//
// Serialised from a fully-populated reading, so nothing is missing merely
// because it was empty. Adding a field to Observation without adding it here
// fails, which is the point: the reviewable artifact is this list, and it
// should take a deliberate edit to lengthen it.
//
// PASS: the marshalled keys equal the list.
// FAIL: any key not on it — the shape in which "just one more field for
// debugging" becomes a disclosure.
func TestO2_ThePayloadCarriesExactlyTheseFields(t *testing.T) {
	want := []string{
		"anomalies", "answeredBy", "atMost", "above", "campaign", "confirm",
		"documented", "geo", "inconclusive", "method", "model", "outcome",
		"probes", "schema", "serviceTier", "sourceTag", "tagBasis", "takenAt",
	}
	sort.Strings(want)

	obs, err := Build("2026-09-anthropic-floor", granted(), Tag{Value: "abc", Basis: BasisLocal}, sample())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("the payload's fields have changed.\n got: %s\nwant: %s\n\n"+
			"Every field here leaves the machine. Adding one is a disclosure decision, "+
			"not a formatting one.", strings.Join(keys, ","), strings.Join(want, ","))
	}
}

// O3: the timestamp is truncated to the hour.
//
// A second-resolution timestamp is a correlation handle against the provider's
// own request logs, and it buys nothing: a campaign window is hours wide, so
// the extra precision is pure identifiability.
//
// PASS: minutes and seconds are zero and the string still parses as UTC.
// FAIL: the reading's own timestamp passed through.
func TestO3_TheTimestampIsTruncatedToTheHour(t *testing.T) {
	obs, err := Build("c", granted(), Tag{Value: "abc", Basis: BasisLocal}, sample())
	if err != nil {
		t.Fatal(err)
	}
	if obs.TakenAt == sample().TakenAt {
		t.Fatalf("the reading's exact timestamp was passed through: %s", obs.TakenAt)
	}
	ts, err := time.Parse(time.RFC3339, obs.TakenAt)
	if err != nil {
		t.Fatalf("takenAt does not parse: %v", err)
	}
	if ts.Minute() != 0 || ts.Second() != 0 || ts.Nanosecond() != 0 {
		t.Errorf("takenAt is not hour-truncated: %s", obs.TakenAt)
	}
	if ts.Location() != time.UTC {
		t.Errorf("takenAt is not UTC: %s", obs.TakenAt)
	}
}

// O4: tags dedup within a campaign and do not link across campaigns.
//
// Deduplication is the whole reason a tag exists — "do two readings come from
// one account" is unanswerable without it. Unlinkability across campaigns is
// what stops it becoming a persistent installation id, which is the thing the
// design refuses to ship.
//
// PASS: same identity and campaign agree; a different campaign does not.
// FAIL: a stable tag across campaigns, which is cohort tracking however it is
// labelled.
func TestO4_TagsDedupWithinACampaignAndNotAcross(t *testing.T) {
	a, err := LocalTag("campaign-one", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	again, err := LocalTag("campaign-one", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if a.Value != again.Value {
		t.Error("the same identity produced two tags in one campaign; submissions cannot be deduplicated")
	}
	other, err := LocalTag("campaign-two", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if other.Value == a.Value {
		t.Error("the tag is stable across campaigns, which makes it a persistent installation id")
	}
	different, err := LocalTag("campaign-one", "another-secret")
	if err != nil {
		t.Fatal(err)
	}
	if different.Value == a.Value {
		t.Error("two identities collided in one campaign")
	}
	if a.Basis != BasisLocal {
		t.Errorf("a locally minted tag must say so: %q", a.Basis)
	}
	if _, err := LocalTag("", "secret"); err == nil {
		t.Error("an empty campaign is not a salt; it must be refused")
	}
	if _, err := LocalTag("c", ""); err == nil {
		t.Error("an empty secret must be refused rather than tagging everyone identically")
	}
}

// O5: the secret is never recoverable from the payload.
//
// A keyed digest is not content, but it is a fingerprint, so the secret itself
// must not appear and neither must anything it was derived from.
//
// PASS: no sentinel anywhere in the serialised payload.
// FAIL: the secret, or a prefix of it, present in any field.
func TestO5_TheSecretNeverAppearsInThePayload(t *testing.T) {
	const secret = "SENTINEL-SECRET-VALUE-DO-NOT-DISCLOSE"
	tag, err := LocalTag("campaign-one", secret)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := Build("campaign-one", granted(), tag, sample())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(obs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), secret) {
		t.Fatalf("the secret is in the payload: %s", body)
	}
	for n := 8; n <= len(secret); n += 8 {
		if strings.Contains(string(body), secret[:n]) {
			t.Fatalf("a %d-character prefix of the secret is in the payload", n)
		}
	}
}

// O6: an account-derived tag and a locally minted one are never confused.
//
// A local secret is dedup only: anyone can mint unlimited ones, so it is not
// Sybil resistance and must not be counted as if it were. The distinction is
// carried in the payload rather than in documentation, because the person
// aggregating these has to weight them differently.
//
// PASS: the basis names which it is, and only the two known values exist.
// FAIL: one basis standing in for the other, or an unlabelled tag.
func TestO6_TheTagSaysWhereItCameFrom(t *testing.T) {
	local, err := LocalTag("c", "secret")
	if err != nil {
		t.Fatal(err)
	}
	account, err := AccountTag("c", "org-12345")
	if err != nil {
		t.Fatal(err)
	}
	if local.Basis != BasisLocal || account.Basis != BasisAccount {
		t.Fatalf("bases are wrong: local=%q account=%q", local.Basis, account.Basis)
	}
	if local.Value == account.Value {
		t.Error("a local secret and an account id produced the same tag")
	}
	if _, err := Build("c", granted(), Tag{Value: "abc", Basis: "invented"}, sample()); err == nil {
		t.Error("an unrecognised basis must be refused; the aggregator weights the two differently")
	}
	if _, err := Build("c", granted(), Tag{Value: "", Basis: BasisLocal}, sample()); err == nil {
		t.Error("an empty tag must be refused; it would silently merge every contributor")
	}
}

// O7: this package cannot make a network call.
//
// The claim being protected is that CONTRIBUTION has no transport: this
// package cannot send what it builds. A grep would not survive a reviewer who
// wanted to get around it, so this walks the imports and refuses anything not
// on the list.
// The list is short on purpose: lengthening it should require an argument.
//
// PASS: every import is on the allowlist.
// FAIL: net, net/http, os/exec, or anything else that could reach outward.
func TestO7_ThisPackageCannotSend(t *testing.T) {
	allowed := map[string]bool{
		"crypto/hmac": true, "crypto/sha256": true, "encoding/hex": true,
		"encoding/json": true, "errors": true, "fmt": true, "os": true,
		"path/filepath": true, "sort": true, "strings": true, "time": true,
		"github.com/RedRobotKK/Replay/internal/consent": true,
		"github.com/RedRobotKK/Replay/internal/probe":   true,
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if !allowed[path] {
					t.Errorf("%s imports %q, which is not on the allowlist. This package builds a "+
						"file and prints its path; it must not be able to send one.", name, path)
				}
			}
			ast.Inspect(file, func(_ ast.Node) bool { return true })
		}
	}
	// The allowlist must be able to fail: if it admitted a transport, it would
	// be decoration.
	for _, banned := range []string{"net", "net/http", "os/exec", "net/url"} {
		if allowed[banned] {
			t.Errorf("%q is on the allowlist; the allowlist is not a guarantee", banned)
		}
	}
}

// O8: the file is written where the user can see it, and never over something.
//
// The file is the whole submission mechanism: a human reads it and decides
// whether to move it. So it must be owner-only, must not silently replace a
// previous submission, and must not follow a symlink somebody planted at the
// path it is about to write.
//
// PASS: created 0600, refuses a second write, refuses a symlink.
// FAIL: any of those, each of which turns "look at it before you send it" into
// a formality.
func TestO8_TheFileIsOwnerOnlyAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	obs, err := Build("campaign-one", granted(), Tag{Value: "abcd1234", Basis: BasisLocal}, sample())
	if err != nil {
		t.Fatal(err)
	}

	path, err := WriteObservation(dir, obs)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("written outside the directory asked for: %s", path)
	}
	if base := filepath.Base(path); !strings.Contains(base, "campaign-one") || !strings.Contains(base, "abcd1234") {
		t.Errorf("the filename does not identify the campaign and contributor: %s", base)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s is %04o, want 0600", path, perm)
		}
	}
	var round Observation
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &round); err != nil {
		t.Fatalf("the file this asks a human to read does not parse: %v", err)
	}
	if !reflect.DeepEqual(round, obs) {
		t.Errorf("the file does not round-trip:\n got %+v\nwant %+v", round, obs)
	}

	// O_EXCL alone would refuse the second write, but only with a bare
	// "file exists". The explicit check exists to say what to do about it,
	// because this file is the one thing a human is asked to act on.
	_, err = WriteObservation(dir, obs)
	if err == nil {
		t.Fatal("a second write replaced the first submission without saying so")
	}
	if !strings.Contains(err.Error(), "move or delete") {
		t.Errorf("the refusal does not say what to do about it: %v", err)
	}

	link := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(link, observationFileName(obs))); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	if _, err := WriteObservation(link, obs); err == nil {
		t.Error("a symlink at the target path was followed")
	}
}
