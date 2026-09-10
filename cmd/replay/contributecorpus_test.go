package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `replay cost --contribute` builds a corpus submission and sends nothing.
//
// It hangs off cost rather than probe because that is where the five figures
// are computed. A contribution path that re-derived them would be a second
// implementation of the money argument, free to disagree with the one the
// reader ran, and the disagreement would surface as a pooled figure nobody
// could reproduce from their own terminal.
//
// The probe path's printed message ends "no prompts, no paths, no spend". That
// sentence is the bargain a contributor accepted, and this payload breaks it on
// purpose: aggregate money is the entire point. So this is a different flag, a
// different writer, a different file name, and a different statement — the
// contributor is told, in the same breath they are told nothing was sent, that
// spend IS in the file.

// withCorpusConsent puts a consenting HOME under a corpus of transcripts.
func withCorpusConsent(t *testing.T) string {
	t.Helper()
	corpus(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	writeConsent(t, home, "corpus_opt_in = true\n")
	return home
}

func readOneCorpus(t *testing.T, dir string) map[string]any {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly one submission in %s, found %d", dir, len(entries))
	}
	b, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("the submission is not JSON: %v", err)
	}
	return m
}

// CN1: the payload carries the same five figures the reader just saw.
//
// PASS: the file's totalUsd equals the total the same run printed.
// FAIL: a contribution that pools a number the contributor's own terminal
// never showed them.
func TestCN1_TheSubmissionMatchesTheReportedFigures(t *testing.T) {
	withCorpusConsent(t)
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost", "--json", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr); err != nil {
		t.Fatalf("cost --contribute: %v\n%s", err, stderr.String())
	}
	var report struct {
		Summary struct {
			Tasks                                             int
			TotalUSD, AvoidableUSD, AvoidableShare, MedianUSD float64
		} `json:"summary"`
		Unpriced int `json:"unpriced"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("the cost report is not JSON: %v", err)
	}

	got := readOneCorpus(t, out)
	for _, c := range []struct {
		key  string
		want float64
	}{
		{"tasks", float64(report.Summary.Tasks)},
		{"totalUsd", report.Summary.TotalUSD},
		{"avoidableUsd", report.Summary.AvoidableUSD},
		{"avoidableShare", report.Summary.AvoidableShare},
		{"medianTaskUsd", report.Summary.MedianUSD},
		{"unpriced", float64(report.Unpriced)},
	} {
		if got[c.key] != c.want {
			t.Errorf("%s in the submission is %v, the report printed %v", c.key, got[c.key], c.want)
		}
	}
	if got["pricedAt"] == "" || got["rulesVersion"] == "" {
		t.Error("the submission does not say what priced it, so a pool cannot tell " +
			"whether it may be added to another")
	}
}

// CN2: the contributor is told that spend is in the file.
//
// The probe path says "no spend". Reusing that sentence here would be a false
// statement about a payload whose whole content is money.
func TestCN2_ThePrintedStatementSaysSpendIsIncluded(t *testing.T) {
	withCorpusConsent(t)
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr); err != nil {
		t.Fatalf("cost --contribute: %v\n%s", err, stderr.String())
	}
	said := stdout.String()
	if strings.Contains(said, "no spend") {
		t.Error("the corpus path reuses the probe path's promise, which is false of this payload")
	}
	low := strings.ToLower(said)
	if !strings.Contains(low, "spend") {
		t.Errorf("the contributor is not told that spend is in the file:\n%s", said)
	}
	if !strings.Contains(low, "nothing was sent") {
		t.Errorf("the contributor is not told that nothing left the machine:\n%s", said)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 1 {
		t.Fatalf("want one submission, found %d", len(entries))
	}
	if !strings.Contains(said, entries[0].Name()) {
		t.Errorf("the path of the file to read before sending is not printed:\n%s", said)
	}
}

// CN3: nothing identifying travels.
//
// The corpus report already refuses to print a path or a project name. A
// contribution built from it must not reintroduce one, and the machine's HOME
// is the string most likely to arrive by accident.
func TestCN3_NoPathsOrProjectNamesTravel(t *testing.T) {
	home := withCorpusConsent(t)
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr); err != nil {
		t.Fatalf("cost --contribute: %v\n%s", err, stderr.String())
	}
	entries, _ := os.ReadDir(out)
	b, err := os.ReadFile(filepath.Join(out, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, needle := range []string{home, os.Getenv("REPLAY_TRANSCRIPTS"), "proj", "/Users/", "session-redacted"} {
		if needle == "" {
			continue
		}
		if strings.Contains(body, needle) {
			t.Errorf("the submission carries %q", needle)
		}
	}
}

// CN4: contributing is refused without consent, and writes nothing.
//
// Same gate as the probe path, checked separately because it is a separate
// call site and a gate that is only enforced on one of two paths is not a gate.
func TestCN4_NoConsentNoSubmission(t *testing.T) {
	corpus(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := run([]string{"cost", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a submission was built with no consent on file")
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("the refusal left %d file(s) behind", len(entries))
	}
}

// CN5: a corpus that priced nothing contributes nothing.
//
// The same refusal as the cost gate, for the same reason: an empty submission
// pooled into a population total moves the denominator without moving the
// numerator, which biases everyone else's share downward.
func TestCN5_NothingMeasuredIsNotAContribution(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("REPLAY_TRANSCRIPTS", empty)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	writeConsent(t, home, "corpus_opt_in = true\n")
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := run([]string{"cost", "--contribute", testCampaign, "--contribute-dir", out, empty}, &stdout, &stderr)
	if err == nil {
		t.Fatal("an empty corpus produced a contribution")
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("the refusal left %d file(s) behind", len(entries))
	}
}

// CN6: without the flag, cost is unchanged.
func TestCN6_AbsentFlagWritesNothing(t *testing.T) {
	withCorpusConsent(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost"}, &stdout, &stderr); err != nil {
		t.Fatalf("plain cost failed: %v", err)
	}
	if strings.Contains(stdout.String(), "wrote ") {
		t.Error("cost wrote a submission nobody asked for")
	}
}
