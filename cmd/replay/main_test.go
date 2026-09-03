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
		{name: "no args prints usage", args: nil, wantOut: "Usage:"},
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
