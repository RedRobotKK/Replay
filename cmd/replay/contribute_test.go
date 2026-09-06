package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contribution is gated on a file the user wrote, and on nothing else.
//
// internal/consent has existed since before anything read it — the installer
// wrote corpus-consent.toml and no Go code ever opened it, so the package was
// dead state in the shipped binary and the promise it encodes was enforced by
// nobody. These are the tests that make it load-bearing.
//
// The gate has three states because two would merge "never asked" with "said
// no", and a tool that merges them either nags somebody who declined or never
// asks somebody who was never asked.

const testCampaign = "2026-09-anthropic-floor"

// withSeries lays down a HOME containing one recorded reading, and returns it.
func withSeries(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dir := filepath.Join(home, ".replay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	line := `{"takenAt":"2026-09-06T02:39:19Z","method":"2026-09-06.1","model":"claude-opus-5",` +
		`"answeredBy":"claude-opus-5","serviceTier":"standard","geo":"global",` +
		`"above":508,"atMost":512,"documented":512,"probes":12,"confirm":3}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "measurements.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func writeConsent(t *testing.T, home, body string) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "replay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "corpus-consent.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// C1: with no consent file, nothing is built and the user is told what to write.
//
// Unset is the normal state for everyone who has not opted in, which is
// everyone. The refusal has to name the file and the line, or the only way to
// contribute is to read the source.
//
// PASS: an error, no file written, and the message names the path.
// FAIL: a payload built for somebody who was never asked.
func TestC1_ContributeRefusesWithoutConsent(t *testing.T) {
	home := withSeries(t)
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := run([]string{"probe", "--model", "claude-opus-5", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a submission was built for a user who has never opted in")
	}
	if !strings.Contains(err.Error(), "corpus-consent.toml") {
		t.Errorf("the refusal does not name the file to write: %v", err)
	}
	if !strings.Contains(err.Error(), "corpus_opt_in") {
		t.Errorf("the refusal does not name the line to put in it: %v", err)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 0 {
		t.Errorf("files were written despite the refusal: %v", entries)
	}
	_ = home
}

// C2: a recorded refusal is remembered and not argued with.
//
// Someone who wrote `corpus_opt_in = false` answered the question. Asking
// again, or treating it as undecided, is how a tool teaches people that
// declining does not work.
//
// PASS: refused, and the message does not invite them to opt in again.
// FAIL: a payload, or a nag.
func TestC2_ContributeHonoursARecordedRefusal(t *testing.T) {
	home := withSeries(t)
	writeConsent(t, home, "corpus_opt_in = false\n")
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := run([]string{"probe", "--model", "claude-opus-5", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a declined user had a submission built anyway")
	}
	if !strings.Contains(err.Error(), "declined") {
		t.Errorf("the refusal does not say the decision was already made: %v", err)
	}
	// The whole reason the command checks this itself, rather than leaving it
	// to Build, is that a declined user must not be told how to opt in. They
	// answered. Repeating the invitation is how a tool teaches people that
	// declining does not take.
	if strings.Contains(err.Error(), "corpus_opt_in = true") {
		t.Errorf("a user who declined is being invited to opt in again: %v", err)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("files were written despite the refusal: %v", entries)
	}
}

// C3: a consent file anyone can write is not this user's decision.
//
// The consent package refuses a group- or world-writable file and refuses a
// symlink. That refusal is worth nothing if the command treats an error as
// "no": an unreadable file must stop the build, not quietly become a decision
// either way.
//
// PASS: refused, and the reason names the permissions.
// FAIL: the file read as consent, or the error swallowed into a default.
func TestC3_AWorldWritableConsentFileIsRefused(t *testing.T) {
	home := withSeries(t)
	path := writeConsent(t, home, "corpus_opt_in = true\n")
	if err := os.Chmod(path, 0o666); err != nil {
		t.Skipf("cannot change permissions here: %v", err)
	}
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := run([]string{"probe", "--model", "claude-opus-5", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a consent file writable by anyone on the machine was accepted as this user's decision")
	}
	// It must fail as unreadable, not as undecided. Swallowing the error and
	// falling through to "you have not opted in" tells the user to write a
	// file they have already written, and hides that their config is unsafe.
	if !strings.Contains(err.Error(), "writable by group or other") {
		t.Errorf("the refusal does not say why the file was rejected, so the user cannot fix it: %v", err)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("files were written despite the refusal: %v", entries)
	}
}

// C4: with consent, a file is written and nothing is sent.
//
// The payload has to be the recorded reading, the path has to be printed, and
// the output has to say plainly that sending is the human's job — because the
// whole design rests on somebody reading the file before it moves.
//
// PASS: a parseable payload naming the campaign, and a message that says
// nothing was sent.
// FAIL: a silent write, or output implying a submission happened.
func TestC4_ContributeWritesAPayloadAndSendsNothing(t *testing.T) {
	home := withSeries(t)
	writeConsent(t, home, "corpus_opt_in = true\n")
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := run([]string{"probe", "--model", "claude-opus-5", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr); err != nil {
		t.Fatalf("contribute with consent granted: %v (stderr %s)", err, stderr.String())
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 1 {
		t.Fatalf("want exactly one submission file, got %v", entries)
	}
	body, err := os.ReadFile(filepath.Join(out, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("the file a human is asked to read does not parse: %v", err)
	}
	if got["campaign"] != testCampaign {
		t.Errorf("campaign = %v, want %q", got["campaign"], testCampaign)
	}
	if got["model"] != "claude-opus-5" {
		t.Errorf("model = %v", got["model"])
	}
	if got["takenAt"] != "2026-09-06T02:00:00Z" {
		t.Errorf("takenAt = %v, want the hour-truncated reading time", got["takenAt"])
	}
	if got["sourceTag"] == nil || got["sourceTag"] == "" {
		t.Error("no contributor tag; submissions could not be deduplicated")
	}
	if got["tagBasis"] != "local" {
		t.Errorf("tagBasis = %v, want \"local\" for a machine-minted secret", got["tagBasis"])
	}
	printed := stdout.String()
	if !strings.Contains(printed, entries[0].Name()) {
		t.Errorf("the path was not printed, so nobody can find the file to send:\n%s", printed)
	}
	if !strings.Contains(strings.ToLower(printed), "nothing was sent") {
		t.Errorf("the output does not say that sending is still the human's job:\n%s", printed)
	}
	_ = home
}

// C5: the contributor secret is generated once, kept owner-only, and reused.
//
// Reused, because a tag that changed every run would make one contributor look
// like many and destroy the only thing the tag is for. Owner-only, because
// anyone who can read it can compute this machine's tag for any campaign.
//
// PASS: same tag across two runs, file 0600.
// FAIL: a fresh tag each run, or a secret readable by others.
func TestC5_TheContributorSecretIsStableAndOwnerOnly(t *testing.T) {
	home := withSeries(t)
	writeConsent(t, home, "corpus_opt_in = true\n")

	tagOf := func(dir string) string {
		var stdout, stderr bytes.Buffer
		if err := run([]string{"probe", "--model", "claude-opus-5", "--contribute", testCampaign, "--contribute-dir", dir}, &stdout, &stderr); err != nil {
			t.Fatalf("contribute: %v (stderr %s)", err, stderr.String())
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Fatalf("want one file, got %v", entries)
		}
		body, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		return got["sourceTag"].(string)
	}

	first := tagOf(t.TempDir())
	second := tagOf(t.TempDir())
	if first != second {
		t.Errorf("the tag changed between runs (%s then %s); one contributor would look like many", first, second)
	}

	secret := filepath.Join(home, ".replay", "contributor-secret")
	info, err := os.Stat(secret)
	if err != nil {
		t.Fatalf("the secret was not persisted: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("%s is %04o, want 0600: anyone who reads it can compute this machine's tag", secret, perm)
	}
}

// C6: with nothing measured, there is nothing to contribute.
//
// A submission built from no reading would be a row asserting a measurement
// that was never taken. The refusal must say what to run instead.
//
// PASS: refused, and the message names the probe that would produce one.
// FAIL: an empty payload written, which pollutes the pool with a null row.
func TestC6_ContributeRefusesWhenNothingWasMeasured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	writeConsent(t, home, "corpus_opt_in = true\n")
	out := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := run([]string{"probe", "--model", "claude-opus-5", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a submission was built with no reading behind it")
	}
	if !strings.Contains(err.Error(), "--execute") {
		t.Errorf("the refusal does not say how to produce a reading: %v", err)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("files were written: %v", entries)
	}
}
