package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/selfupdate"
)

// UP. `replay upgrade` had no tests, and that is why it could lie.
//
// This command downloads a binary over the network and replaces the one you are
// running. It was the least covered path in the repository: no upgrade_test.go
// existed at all. The consequence surfaced on 2026-09-13 when a mutation that
// deleted the signature line from its output SURVIVED, because nothing read its
// output.
//
// What it printed until then was "✓ Checksum verified" in all three outcomes:
// signature verified, cosign absent, signature skipped. install.sh has always
// printed three distinct lines. So somebody who installed the documented way
// got signature verification, and the same person upgrading a week later did
// not, and nothing told them the guarantee had changed.

// fakeRelease serves a release whose archive matches its checksums.
func fakeRelease(t *testing.T, tag string, withSignature bool) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho hi\n")
	_ = tw.WriteHeader(&tar.Header{Name: "replay", Size: int64(len(body)), Mode: 0o755, Typeflag: tar.TypeReg})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()
	archive := buf.Bytes()

	name := selfupdate.ArchiveName(tag, runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(archive)
	sums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)

	mux := http.NewServeMux()
	base := "/releases/download/" + tag
	mux.HandleFunc(base+"/"+name, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(archive) })
	mux.HandleFunc(base+"/checksums.txt", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(sums)) })
	if withSignature {
		for _, suffix := range []string{".pem", ".sig"} {
			mux.HandleFunc(base+"/checksums.txt"+suffix, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("-----BEGIN EXAMPLE-----\n"))
			})
		}
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// UP1: the output says which of the two checks actually happened.
//
// Asserted on the printed text rather than on a returned value, because the
// printed text is the only part a user ever sees and it was the half that was
// wrong.
func TestUP1_UpgradeSaysWhetherTheSignatureWasChecked(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", true)
	restore := newUpdateClient
	t.Cleanup(func() { newUpdateClient = restore })
	newUpdateClient = func() *selfupdate.Client {
		return &selfupdate.Client{ReleasesBase: srv.URL}
	}

	var out, errOut bytes.Buffer
	if err := runUpgrade([]string{"--version", "v9.9.9", "--dry-run"}, &out, &errOut); err != nil {
		t.Fatalf("dry-run upgrade failed: %v\n%s", err, errOut.String())
	}
	got := errOut.String()

	if !strings.Contains(got, "Checksum verified") {
		t.Errorf("the checksum result is not reported:\n%s", got)
	}
	// The line this test exists for. On a machine without cosign it must say
	// the signature was NOT checked rather than leave the reader to assume.
	if !strings.Contains(got, "signature") && !strings.Contains(got, "Signature") {
		t.Errorf("the output never mentions the signature, so a user cannot tell which "+
			"promise they got:\n%s", got)
	}
}

// UP2: a machine without cosign is told so, in words, and still upgrades.
//
// THE LIMIT OF THESE TWO TESTS, stated rather than left for someone to discover.
// This package cannot reach the VERIFIED branch. lookCosign and runCosign are
// unexported seams in internal/selfupdate, so from here only the
// no-cosign-on-this-machine path executes, and a mutation replacing the branch
// condition with `false` survives because both arms then print the same thing.
// That is an equivalent mutant on this machine, not an untested branch: the
// verified path is covered at the package level by TestSIG4, which captures
// cosign's real argv through the seam, and by the SignatureStatus String test.
//
// Exporting a hook purely to close it here would widen the package's API to
// satisfy a mutation score, which is the wrong trade.
func TestUP2_NoCosignIsReportedRatherThanImplied(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", true)
	restore := newUpdateClient
	t.Cleanup(func() { newUpdateClient = restore })
	newUpdateClient = func() *selfupdate.Client {
		return &selfupdate.Client{ReleasesBase: srv.URL}
	}

	var out, errOut bytes.Buffer
	err := runUpgrade([]string{"--version", "v9.9.9", "--dry-run"}, &out, &errOut)
	if err != nil {
		t.Fatalf("upgrade refused on a machine without cosign, which would strand "+
			"everyone who installed the documented way: %v", err)
	}
	got := errOut.String()
	if strings.Contains(got, "Signature verified") && !strings.Contains(got, "not checked") {
		t.Errorf("this machine has no cosign and the output claims a verified "+
			"signature:\n%s", got)
	}
	// The exact words, so deleting the line fails rather than degrading quietly.
	if !strings.Contains(got, "not checked") {
		t.Errorf("the unchecked case must say so in words a user can act on:\n%s", got)
	}
	if !strings.Contains(out.String(), "was not touched") {
		t.Errorf("a dry run must say the binary was left alone:\n%s", out.String())
	}
}

// UP3: both signature outcomes produce a line that says which one happened.
//
// The branch this covers was UNREACHED as a branch inside runUpgrade, and I had
// written that off in a comment as an equivalent mutant because the seams that
// reach it are unexported. `guard reachability` did not accept the excuse and
// was right: an untested branch does not become tested by a paragraph
// explaining why it is hard. Extracting it made both arms reachable without
// exporting anything.
func TestUP3_BothSignatureOutcomesAreDistinguishable(t *testing.T) {
	verified := signatureLine(selfupdate.SignatureVerified)
	unchecked := signatureLine(selfupdate.SignatureUnchecked)

	if verified == unchecked {
		t.Fatalf("both outcomes print the same line, which is the defect this whole "+
			"change exists to fix: %q", verified)
	}
	if !strings.Contains(verified, "Signature verified") {
		t.Errorf("the verified line does not say so: %q", verified)
	}
	if !strings.Contains(unchecked, "not checked") {
		t.Errorf("the unchecked line does not say the signature went unchecked: %q", unchecked)
	}
	// And the unchecked line must not read as a success. A leading tick is how
	// a reader skims, and this outcome is not one.
	if strings.HasPrefix(strings.TrimSpace(unchecked), "✓") {
		t.Errorf("the unchecked line is marked with a tick and will be read as a pass: %q", unchecked)
	}
}
