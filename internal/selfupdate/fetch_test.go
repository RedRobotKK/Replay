package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// tarGz builds a one-entry archive holding body as the named regular file.
func tarGz(t *testing.T, name string, body []byte, typeflag byte, link string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: typeflag, Linkname: link}
	if typeflag == tar.TypeSymlink {
		hdr.Size = 0
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if typeflag == tar.TypeReg {
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// release serves a fake GitHub release: the latest redirect, the archive and
// checksums.txt. checksumsFor lets a test lie about the digest.
func releaseServer(t *testing.T, tag string, archive []byte, checksums string) *httptest.Server {
	t.Helper()
	name := ArchiveName(tag, runtime.GOOS, runtime.GOARCH)
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/"+tag+"/"+name, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	return httptest.NewServer(mux)
}

// Resolving the latest tag must not touch api.github.com. install.sh documents
// that endpoint's 60/hr unauthenticated limit as routinely hit on CI and behind
// shared NAT, and a self-updater that inherits it fails exactly when a fleet
// upgrades at once.
//
// PASS: the tag comes from the releases/latest redirect.
// FAIL: any request to an api. host, or the redirect being followed blindly.
func TestLatestUsesTheRedirectNotTheAPI(t *testing.T) {
	srv := releaseServer(t, "v0.5.4", nil, "")
	defer srv.Close()

	c := &Client{ReleasesBase: srv.URL}
	got, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "v0.5.4" {
		t.Errorf("Latest = %q, want v0.5.4", got)
	}
}

// A digest that does not match must stop the upgrade with nothing written.
// This is the case the whole package exists to get right.
//
// PASS: a corrupted archive is refused.
// FAIL: bytes that do not hash to the recorded digest are unpacked anyway.
func TestFetchRefusesAChecksumMismatch(t *testing.T) {
	tag := "v0.5.4"
	archive := tarGz(t, Binary, []byte("#!/bin/sh\necho hi\n"), tar.TypeReg, "")
	// checksums.txt records a digest for the right filename, wrong content.
	name := ArchiveName(tag, runtime.GOOS, runtime.GOARCH)
	bad := fmt.Sprintf("%s  %s\n", strings.Repeat("a", 64), name)

	srv := releaseServer(t, tag, archive, bad)
	defer srv.Close()

	c := &Client{ReleasesBase: srv.URL}
	_, err := c.Fetch(context.Background(), tag, runtime.GOOS, runtime.GOARCH)
	if err == nil {
		t.Fatal("a checksum mismatch must be an error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("the error must name the checksum, got %v", err)
	}
}

// A release whose checksums.txt cannot be fetched is not installable. install.sh
// dies here rather than falling back, and so must this.
//
// PASS: a missing checksums.txt aborts.
// FAIL: the archive is accepted unverified.
func TestFetchRefusesWhenChecksumsAreMissing(t *testing.T) {
	tag := "v0.5.4"
	archive := tarGz(t, Binary, []byte("binary"), tar.TypeReg, "")
	name := ArchiveName(tag, runtime.GOOS, runtime.GOARCH)

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/download/"+tag+"/"+name, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	// checksums.txt deliberately absent -> 404
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Client{ReleasesBase: srv.URL}
	if _, err := c.Fetch(context.Background(), tag, runtime.GOOS, runtime.GOARCH); err == nil {
		t.Fatal("a missing checksums.txt must abort the upgrade")
	}
}

// A tar member that is a symlink named `replay` would, if followed, install a
// 0755 executable holding whatever it pointed at on this machine. install.sh
// rejects this explicitly; the same archive must be rejected here.
//
// PASS: a symlink member is refused.
// FAIL: the link is materialised.
func TestFetchRefusesASymlinkMember(t *testing.T) {
	tag := "v0.5.4"
	archive := tarGz(t, Binary, nil, tar.TypeSymlink, "/etc/passwd")
	name := ArchiveName(tag, runtime.GOOS, runtime.GOARCH)
	sums := fmt.Sprintf("%s  %s\n", sha256hex(archive), name)

	srv := releaseServer(t, tag, archive, sums)
	defer srv.Close()

	c := &Client{ReleasesBase: srv.URL}
	_, err := c.Fetch(context.Background(), tag, runtime.GOOS, runtime.GOARCH)
	if err == nil {
		t.Fatal("a symlink where the binary should be must be refused")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("the error must say what was wrong, got %v", err)
	}
}

// The happy path: matching digest, regular file, bytes returned intact.
//
// PASS: the unpacked binary is returned byte-for-byte.
// FAIL: any truncation or transformation of the payload.
func TestFetchReturnsTheVerifiedBinary(t *testing.T) {
	tag := "v0.5.4"
	payload := []byte("#!/bin/sh\necho replay 0.5.4\n")
	archive := tarGz(t, Binary, payload, tar.TypeReg, "")
	name := ArchiveName(tag, runtime.GOOS, runtime.GOARCH)
	sums := fmt.Sprintf("%s  %s\n", sha256hex(archive), name)

	srv := releaseServer(t, tag, archive, sums)
	defer srv.Close()

	c := &Client{ReleasesBase: srv.URL}
	got, err := c.Fetch(context.Background(), tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("Fetch returned %q, want %q", got, payload)
	}
}

// A binary that does not run on this machine must not reach PATH, and the
// working copy already installed must survive. install.sh stages a sibling,
// runs it, and only then moves it into place, because a failed upgrade that
// overwrote first leaves the user with nothing where they started with
// something.
//
// PASS: the existing binary is byte-for-byte unchanged after a failed upgrade.
// FAIL: the destination is touched before the new binary is proved to run.
func TestApplyKeepsTheWorkingBinaryWhenTheNewOneCannotRun(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, Binary)
	existing := []byte("#!/bin/sh\necho replay 0.4.0\n")
	if err := os.WriteFile(dest, existing, 0o755); err != nil {
		t.Fatal(err)
	}

	// Not an executable this machine can run: an ELF header on darwin, and
	// nonsense everywhere. The exec check is what must catch it.
	broken := []byte("\x7fELF\x02\x01\x01\x00 not a real binary")
	err := Apply(broken, dest)
	if err == nil {
		t.Fatal("Apply must fail when the staged binary does not run")
	}

	after, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("the existing binary was destroyed: %v", readErr)
	}
	if !bytes.Equal(after, existing) {
		t.Errorf("the existing binary was modified: got %q", after)
	}

	// And no staged leftovers.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "."+Binary+".new") {
			t.Errorf("a staged file was left behind: %s", e.Name())
		}
	}
}

// The successful path replaces the destination and leaves it executable.
//
// PASS: the new bytes land at dest with mode 0755 and no leftovers.
// FAIL: a partial write, a wrong mode, or a stray staged file.
func TestApplyReplacesAtomicallyAndLeavesItExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no shell binaries to exec on windows")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, Binary)
	if err := os.WriteFile(dest, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A shell script is a runnable "binary" for the purposes of the exec check.
	fresh := []byte("#!/bin/sh\necho replay 0.5.4\n")
	if err := Apply(fresh, dest); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fresh) {
		t.Errorf("dest holds %q, want %q", got, fresh)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("dest is not executable: mode %v", info.Mode())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected only the binary in %s, got %d entries", dir, len(entries))
	}
}
