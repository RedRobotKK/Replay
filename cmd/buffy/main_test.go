package main

import (
	"bytes"
	"errors"
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
		{name: "version", args: []string{"version"}, wantOut: "buffy "},
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

func TestCorpusOnFixture(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"corpus", "../../internal/transcript/testdata"}, &out, &errOut)
	if err != nil {
		t.Fatalf("corpus: %v (stderr: %s)", err, errOut.String())
	}
	got := out.String()
	for _, want := range []string{"# Calibration Corpus", "| Session |", "Overall match rate:", "## Break causes", "client re-rendered history"} {
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
