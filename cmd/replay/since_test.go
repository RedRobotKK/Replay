package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// What happened while you were away.
//
// This is the return surface, and it is deliberately the cheapest possible
// version of the mobile tether: no server, no phone number, no account, no
// pairing. It answers the same question a push notification would — what did I
// miss — at the moment the person comes back on their own.
//
// It is also the honest way to find out whether anyone wants the notification.
// If nobody reads the digest, nobody wanted the buzz either, and that has been
// learned for the price of one screen instead of a service with a PII store.
//
// Two failure modes matter more than the feature.
//
// A first run has no previous look to measure from. Reporting "0 sessions" then
// would be an absence rendered as a finding, which is the defect this
// repository has found more often than any other. It says it has no marker
// instead.
//
// And a quiet window is not an empty one. "Nothing since you last looked" is a
// result; "$0.00" is a number that reads as a measurement of spend rather than
// of nothing having happened.

// atFixture writes a one-session transcript stamped at a chosen time, so a test
// can place work either side of a marker.
func atFixture(t *testing.T, root, name string, when time.Time) {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	proj := filepath.Join(root, name)
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(proj, "s.jsonl")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatal(err)
	}
}

// sinceEnv points the command at a private state dir and transcript root.
func sinceEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := t.TempDir()
	t.Setenv("REPLAY_TRANSCRIPTS", root)
	t.Setenv("LC_ALL", "en_US.UTF-8")
	return root
}

// SN1: a first run has no marker, and says so rather than inventing a window.
func TestSN1_FirstRunHasNoMarker(t *testing.T) {
	root := sinceEnv(t)
	atFixture(t, root, "proj", time.Now().Add(-2*time.Hour))

	var stdout, stderr bytes.Buffer
	if err := run([]string{"since"}, &stdout, &stderr); err != nil {
		t.Fatalf("since failed: %v\n%s", err, stderr.String())
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(out), "no previous look") {
		t.Errorf("a first run does not say it has no marker, so its window is unexplained:\n%s", out)
	}
}

// SN2: the marker advances, so the second run reports a quiet window.
//
// The behaviour that makes this a return surface rather than a report: looking
// consumes what was there.
func TestSN2_LookingAdvancesTheMarker(t *testing.T) {
	root := sinceEnv(t)
	atFixture(t, root, "proj", time.Now().Add(-2*time.Hour))

	var a, ae bytes.Buffer
	if err := run([]string{"since"}, &a, &ae); err != nil {
		t.Fatalf("first run: %v", err)
	}
	var b, be bytes.Buffer
	if err := run([]string{"since"}, &b, &be); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second := b.String() + be.String()
	if strings.Contains(second, "$") && !strings.Contains(strings.ToLower(second), "nothing") {
		t.Errorf("the second run still reports spend, so the marker did not advance:\n%s", second)
	}
}

// SN3: --peek reports without consuming.
//
// Reading the digest twice should be possible. A surface that empties itself on
// a glance punishes the glance.
func TestSN3_PeekDoesNotAdvanceTheMarker(t *testing.T) {
	root := sinceEnv(t)
	atFixture(t, root, "proj", time.Now().Add(-2*time.Hour))

	var a, ae bytes.Buffer
	if err := run([]string{"since"}, &a, &ae); err != nil {
		t.Fatal(err)
	}
	// A second session lands after the first look.
	atFixture(t, root, "proj2", time.Now())

	var p1, p1e bytes.Buffer
	if err := run([]string{"since", "--peek"}, &p1, &p1e); err != nil {
		t.Fatal(err)
	}
	var p2, p2e bytes.Buffer
	if err := run([]string{"since", "--peek"}, &p2, &p2e); err != nil {
		t.Fatal(err)
	}
	if p1.String() != p2.String() {
		t.Errorf("two --peek runs differ, so peeking consumed the window:\n--- first\n%s\n--- second\n%s",
			p1.String(), p2.String())
	}
}

// SN4: a quiet window is reported as quiet, never as a zero.
//
// "$0.00 since you last looked" reads as a measurement of spend. It is a
// measurement of nothing having happened, and the two are different claims.
func TestSN4_AQuietWindowIsNotAZero(t *testing.T) {
	sinceEnv(t) // no transcripts at all
	// A quiet window needs a marker to be quiet *since*. Without this the run
	// below is a first run, and "no previous look recorded" is the right answer
	// to a different question — which is what the first version of this test
	// asserted against, wrongly.
	var m, me bytes.Buffer
	if err := run([]string{"since"}, &m, &me); err != nil {
		t.Fatalf("setting the marker: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := run([]string{"since"}, &stdout, &stderr); err != nil {
		t.Fatalf("since failed on an empty corpus: %v", err)
	}
	out := stdout.String() + stderr.String()
	if strings.Contains(out, "$0.00") {
		t.Errorf("a quiet window printed $0.00, which reads as a spend measurement:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "nothing") {
		t.Errorf("a quiet window does not say plainly that nothing happened:\n%s", out)
	}
}

// SN5: work older than the marker is excluded.
func TestSN5_OlderWorkIsExcluded(t *testing.T) {
	root := sinceEnv(t)
	atFixture(t, root, "old", time.Now().Add(-72*time.Hour))

	// Consume it, then confirm the same session does not reappear.
	var a, ae bytes.Buffer
	if err := run([]string{"since"}, &a, &ae); err != nil {
		t.Fatal(err)
	}
	var b, be bytes.Buffer
	if err := run([]string{"since"}, &b, &be); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(b.String()+be.String()), "nothing") {
		t.Errorf("a session already reported appeared again:\n%s", b.String()+be.String())
	}
}
