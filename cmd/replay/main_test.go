package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBasicCommands(t *testing.T) {
	t.Setenv(envDisabled, "1")
	cases := []struct {
		name    string
		args    []string
		wantErr error
		wantOut string
	}{
		// No case for a bare `replay` here. It now reports on whatever
		// transcripts the machine has, so run with nothing to isolate it this
		// case read the developer's own ~/.claude/projects: 1,491 real
		// transcripts, several seconds, and an assertion whose answer depended
		// on whose laptop it was. Both branches are covered in bare_test.go,
		// each under a HOME the test owns.
		{name: "version", args: []string{"version"}, wantOut: "replay "},
		{name: "help", args: []string{"help"}, wantOut: "replay"},
		{name: "serve honors the kill switch", args: []string{"serve"}, wantErr: errDisabled},
		{name: "replay needs a path", args: []string{"replay"}, wantErr: errUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			err := run(tc.args, &out, &errOut)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("run(%v) error = %v, want %v", tc.args, err, tc.wantErr)
			}
			if tc.wantOut != "" && !strings.Contains(out.String(), tc.wantOut) {
				t.Fatalf("run(%v) output %q does not contain %q", tc.args, out.String(), tc.wantOut)
			}
		})
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"bogus"}, &out, &errOut); err == nil {
		t.Fatal("expected an error for an unknown command")
	}
}

func TestServeUsageNeverShowsTheToken(t *testing.T) {
	t.Setenv(envToken, "s3cret-token-value")
	var out, errOut bytes.Buffer
	_ = run([]string{"serve", "-h"}, &out, &errOut)
	if strings.Contains(errOut.String(), "s3cret") || strings.Contains(out.String(), "s3cret") {
		t.Fatalf("usage text leaks the token:\n%s", errOut.String())
	}
}

func TestDoctorReportsWithoutFailing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone
	// leaves the command pointed at the real home.
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv(envBaseURL, "http://127.0.0.1:1") // nothing listens there
	var out, errOut bytes.Buffer
	if err := run([]string{"doctor"}, &out, &errOut); err != nil {
		t.Fatalf("doctor: %v", err)
	}
	for _, want := range []string{"transcripts   none found", "nothing answered", "ledger        empty"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("doctor output missing %q:\n%s", want, out.String())
		}
	}
}

func TestCorpusOnFixture(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"corpus", "../../internal/transcript/testdata"}, &out, &errOut)
	if err != nil {
		t.Fatalf("corpus: %v (stderr: %s)", err, errOut.String())
	}
	got := out.String()
	for _, want := range []string{"# Calibration Corpus", "| Session |", "Overall match rate:", "## Per model", "minimum cacheable prefix", "## Break causes", "client re-rendered history"} {
		if !strings.Contains(got, want) {
			t.Errorf("corpus output missing %q", want)
		}
	}
	if strings.Contains(got, "testdata") || strings.Contains(got, "/") && strings.Contains(got, ".jsonl") {
		t.Errorf("corpus output leaks a path:\n%s", got)
	}
}

func TestReplayOnFixture(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"replay", "../../internal/transcript/testdata/session-redacted.jsonl"}, &out, &errOut)
	if err != nil {
		t.Fatalf("replay: %v (stderr: %s)", err, errOut.String())
	}
	for _, want := range []string{"Tier:", "Calibration:", "Assumption:", "Rules:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("replay output missing mandatory line %q", want)
		}
	}
}

func TestContextEditFromFlags(t *testing.T) {
	if p, err := contextEditFromFlags(0, 6, false); p != nil || err != nil {
		t.Fatalf("off by default: %v %v", p, err)
	}
	if p, err := contextEditFromFlags(200000, 6, true); p != nil || err != nil {
		t.Fatalf("%s must win over the flag: %v %v", envNoPolicy, p, err)
	}
	if _, err := contextEditFromFlags(-5, 6, false); err == nil {
		t.Fatal("negative trigger must be rejected")
	}
	p, err := contextEditFromFlags(200000, 6, false)
	if err != nil || p == nil || p.TriggerTokens != 200000 || p.KeepLast != 6 {
		t.Fatalf("flags not applied: %+v %v", p, err)
	}
}

func TestLearnOnTheFixtureSelectsNothingAndWritesTheFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "policy.json")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"learn", "-out", out, "../../internal/transcript/testdata"}, &stdout, &stderr); err != nil {
		t.Fatalf("learn: %v\n%s", err, stderr.String())
	}
	for _, want := range []string{"1 found, 1 calibrated", "Selected: none", "context-edit(keep=6,trigger=200000) *", "rejected: fewer than"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output missing %q:\n%s", want, stdout.String())
		}
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Schema   int `json:"schema"`
		Selected any `json:"selected"`
	}
	if err := json.Unmarshal(data, &file); err != nil || file.Schema != 1 || file.Selected != nil {
		t.Fatalf("policy file wrong: %v %s", err, data)
	}
	if err := run([]string{"learn", "-out", "-"}, &stdout, &stderr); err == nil {
		t.Fatal("learn without a directory must fail")
	}
}

// The tool is named for its own first command, so a path given without a
// subcommand runs the replay analysis. A mistyped command is still an
// error, and a subcommand name always wins over a path of the same name.
func TestPathWithoutASubcommandRunsReplay(t *testing.T) {
	var out, errOut bytes.Buffer
	fixture := "../../internal/transcript/testdata"
	if err := run([]string{fixture}, &out, &errOut); err != nil {
		t.Fatalf("a path must run the replay analysis: %v (stderr: %s)", err, errOut.String())
	}
	direct := out.String()
	if !strings.Contains(direct, "Calibration:") {
		t.Fatalf("expected a replay report:\n%s", direct)
	}
	// It is the same report the explicit form prints.
	var explicit bytes.Buffer
	if err := run([]string{"replay", fixture}, &explicit, &errOut); err != nil {
		t.Fatal(err)
	}
	if explicit.String() != direct {
		t.Fatal("the implicit and explicit forms must print the same report")
	}
	// A leading flag is not a path and must not be taken as one.
	var leading bytes.Buffer
	if err := run([]string{"-dollars", fixture}, &leading, &errOut); err == nil {
		t.Fatal("a leading flag is not a path and must not be taken as one")
	}

	// ...but the documented form puts the flag after the path, and README,
	// `replay --help` and the install-with-AI prompt all tell people to run it
	// that way. Go's flag package stops parsing at the first non-flag argument,
	// so without hoisting, --dollars was passed to os.Stat and every documented
	// invocation of it failed with "stat --dollars: no such file or directory".
	for _, args := range [][]string{
		{fixture, "--dollars"},
		{fixture, "-dollars"},
		{"replay", fixture, "--dollars"},
		{"blame", fixture, "--dollars"},
	} {
		var trailing bytes.Buffer
		if err := run(args, &trailing, &errOut); err != nil {
			t.Fatalf("replay %v: %v", args, err)
		}
		if trailing.Len() == 0 {
			t.Fatalf("replay %v printed nothing", args)
		}
	}

	// The flag has to actually change the report, not merely be tolerated.
	var plain, priced bytes.Buffer
	if err := run([]string{fixture}, &plain, &errOut); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{fixture, "--dollars"}, &priced, &errOut); err != nil {
		t.Fatal(err)
	}
	if plain.String() == priced.String() {
		t.Fatal("--dollars was accepted but changed nothing in the report")
	}
	// A name that is not a command and not a path stays an error.
	if err := run([]string{"relpay", fixture}, &out, &errOut); err == nil {
		t.Fatal("a mistyped command must not be treated as a path")
	}
	// A subcommand name wins even when a file of that name exists.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doctor"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	var doctorOut bytes.Buffer
	if err := run([]string{"doctor"}, &doctorOut, &errOut); err != nil {
		t.Fatalf("the subcommand must win over a file of the same name: %v", err)
	}
	if !strings.Contains(doctorOut.String(), "transcripts") {
		t.Fatalf("expected doctor output:\n%s", doctorOut.String())
	}
}

// Claude Code stores transcripts one level down, as
// ~/.claude/projects/<project>/*.jsonl, and `replay doctor` reports on exactly
// that layout. Directory walking used filepath.Glob, which does not recurse, so
// the obvious first command a new user types, `replay ~/.claude/projects/`,
// answered "no .jsonl transcripts found" while doctor was reporting sessions in
// the same place.
func TestTranscriptFilesFindsNestedProjectDirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "-Users-someone-project")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(nested, "session.jsonl")
	if err := os.WriteFile(want, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A hidden directory is skipped: caches and VCS metadata are not sessions.
	hidden := filepath.Join(root, ".cache")
	if err := os.MkdirAll(hidden, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "ignore.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := transcriptFiles([]string{root})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if len(files) != 1 || files[0] != want {
		t.Fatalf("want exactly [%s], got %v", want, files)
	}
}
