//go:build mutation

package main

// The production path: the command built and run as a child process, which is
// how the count will actually be made. Behind the mutation tag because os/exec
// is confined to tagged files in this repository.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildSimexp(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "simexp")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runBin(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var o, e bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exit):
		code = exit.ExitCode()
	default:
		t.Fatalf("run %v: %v", args, err)
	}
	return o.String(), e.String(), code
}

func TestBinary_CountsAFrozenDatasetDeterministically(t *testing.T) {
	bin := buildSimexp(t)
	d := baseDataset()
	qualifying(d, 0)
	qualifying(d, 1)
	qualifying(d, 2)
	setRow(d, 3, "exposedOn", "UNKNOWN")
	raw := encode(t, d)
	path := filepath.Join(t.TempDir(), "dataset.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	first, stderr, code := runBin(t, bin, "count", path)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	second, _, _ := runBin(t, bin, "count", path)
	if first != second {
		t.Error("two runs over the same frozen file differ")
	}
	var r result
	if err := json.Unmarshal([]byte(first), &r); err != nil {
		t.Fatalf("stdout is not the result: %v\n%s", err, first)
	}
	sum := sha256.Sum256(raw)
	if r.DatasetSHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("digest %s is not the file's", r.DatasetSHA256)
	}
	if r.Primary.Qualifying != 3 || r.Denominator != 10 || r.Secondary.NotExposed != 1 || r.Primary.Decision != decisionLive {
		t.Errorf("qualifying=%d denominator=%d notExposed=%d decision=%q", r.Primary.Qualifying, r.Denominator, r.Secondary.NotExposed, r.Primary.Decision)
	}
	// The file is read, never written.
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, raw) {
		t.Error("the dataset file changed under a count")
	}
}

func TestBinary_RefusesBadInputOnTheRightStream(t *testing.T) {
	bin := buildSimexp(t)
	if out, errs, code := runBin(t, bin); code != 1 || !strings.Contains(errs, "usage") || out != "" {
		t.Errorf("no args: code=%d stdout=%q stderr=%q", code, out, errs)
	}
	d := baseDataset()
	d["participants"] = rowsOf(d)[:9]
	path := filepath.Join(t.TempDir(), "nine.json")
	if err := os.WriteFile(path, encode(t, d), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, errs, code := runBin(t, bin, "count", path); code != 2 || !strings.Contains(errs, "dataset rejected") || !strings.Contains(errs, "10") || out != "" {
		t.Errorf("nine rows: code=%d stdout=%q stderr=%q; want 2, nothing, a rejection naming ten", code, out, errs)
	}
	if out, errs, code := runBin(t, bin, "count", filepath.Join(t.TempDir(), "absent.json")); code != 2 || !strings.Contains(errs, "dataset rejected") || out != "" {
		t.Errorf("missing file: code=%d stdout=%q stderr=%q", code, out, errs)
	}
}
