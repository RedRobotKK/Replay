package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failingWriter refuses every write, as a closed terminal or a full pipe does.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errInjected }

// fakeExecute replaces the measurement with an instant one and records the
// configuration the command chose.
func fakeExecute(t *testing.T, err error) *Config {
	t.Helper()
	var got Config
	swap(t, &execute, func(cfg Config, _ string, _ []string) (Result, error) {
		got = cfg
		return Result{Schema: ResultSchema, Config: cfg}, err
	})
	return &got
}

func TestRepeatsFlagSetsTheRepeatCountUnlessQuick(t *testing.T) {
	cfg := fakeExecute(t, nil)
	var out, errb strings.Builder
	if code := run([]string{"-repeats", "3", "-out", t.TempDir()}, &out, &errb); code != exitOK {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if cfg.Repeats != 3 {
		t.Errorf("-repeats 3 ran %d repeats", cfg.Repeats)
	}
	// -quick is the smoke configuration; its counts are not overridable.
	if code := run([]string{"-quick", "-repeats", "3", "-out", t.TempDir()}, &out, &errb); code != exitOK {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if cfg.Repeats != SmokeConfig().Repeats {
		t.Errorf("-quick -repeats 3 ran %d repeats, want the smoke config's %d", cfg.Repeats, SmokeConfig().Repeats)
	}
}

func TestCommandExitsWithAnErrorWhenTheRunFails(t *testing.T) {
	fakeExecute(t, errors.New("upstream vanished"))
	var out, errb strings.Builder
	if code := run([]string{"-out", t.TempDir()}, &out, &errb); code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(errb.String(), "upstream vanished") {
		t.Errorf("stderr does not say why: %q", errb.String())
	}
	if strings.Contains(out.String(), "result:") {
		t.Errorf("a failed run still reported a result: %q", out.String())
	}
}

func TestCommandExitsWithAnErrorWhenTheSidecarIsMissing(t *testing.T) {
	fakeExecute(t, nil)
	dir := t.TempDir()
	var out, errb strings.Builder
	if code := run([]string{"-out", dir, "-sidecar", filepath.Join(dir, "missing")}, &out, &errb); code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(errb.String(), "sidecar") {
		t.Errorf("stderr does not name the sidecar: %q", errb.String())
	}
}

func TestCommandExitsWithAnErrorWhenTheResultCannotBeWritten(t *testing.T) {
	fakeExecute(t, nil)
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb strings.Builder
	if code := run([]string{"-out", notADir}, &out, &errb); code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if strings.Contains(out.String(), "result:") {
		t.Errorf("an unwritten result was reported: %q", out.String())
	}
}

func TestCommandExitsWithAnErrorWhenTheSummaryCannotBeWritten(t *testing.T) {
	fakeExecute(t, nil)
	var errb strings.Builder
	if code := run([]string{"-out", t.TempDir()}, failingWriter{}, &errb); code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
}

func TestRenderRefusesAMissingOrMalformedFileAndAFailedWrite(t *testing.T) {
	dir := t.TempDir()
	var out, errb strings.Builder
	if code := run([]string{"-render", filepath.Join(dir, "missing.json")}, &out, &errb); code != exitError {
		t.Errorf("missing file: exit = %d, want %d", code, exitError)
	}
	// It says the file could not be opened, not that empty input is bad JSON.
	if !strings.Contains(errb.String(), "open ") || strings.Contains(errb.String(), "JSON") {
		t.Errorf("missing file reported as %q, want the open failure", errb.String())
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	if code := run([]string{"-render", bad}, &out, &errb); code != exitError {
		t.Errorf("malformed file: exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(errb.String(), bad) {
		t.Errorf("stderr does not name the malformed file: %q", errb.String())
	}
	good := filepath.Join(dir, "good.json")
	b, err := json.Marshal(Result{Schema: ResultSchema})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-render", good}, failingWriter{}, &errb); code != exitError {
		t.Errorf("failed write: exit = %d, want %d", code, exitError)
	}
}

func TestRecordedArgsReplaceTheSidecarPathInBothFlagForms(t *testing.T) {
	got := recordedArgs([]string{"-sidecar=/tmp/x/side", "-sidecar", "/tmp/x/side", "-quick"}, "/tmp/x/side")
	want := []string{"-sidecar=" + sidecarPlaceholder, "-sidecar", sidecarPlaceholder, "-quick"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("recordedArgs = %v, want %v", got, want)
	}
	// Without a sidecar nothing is rewritten, even an argument that looks
	// like the empty flag form.
	if got := recordedArgs([]string{"-sidecar="}, ""); got[0] != "-sidecar=" {
		t.Errorf("recordedArgs rewrote %q with no sidecar", got[0])
	}
}
