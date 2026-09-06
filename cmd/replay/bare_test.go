package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A bare `replay` should answer the question, not offer a menu.
//
// The install line is `curl | sh`, and the first thing a person types after it
// is the name of the thing they just installed. What they got was a usage list
// of sixteen commands in which `cost` — the one that reports money, and the
// reason the tool exists — sits eleventh. A menu is what a tool prints when it
// does not know what you want; this one does know, because `defaultTranscriptRoots`
// already finds the transcripts unaided and `replay doctor` has been reporting
// on them from the same path all along.
//
// The fallback is the part that has to keep working. On a machine with no
// transcripts there is no finding to lead with, and inventing one — an empty
// report, a zero total — reads as "this tool is broken" rather than "there is
// nothing here yet". There the menu is the correct answer, so it stays.

// homeWithTranscript points the process at a HOME whose ~/.claude/projects
// holds one real, priceable transcript.
//
// The transcript is linked rather than copied: it is 1.4 MB and every test in
// this file wants its own HOME, and a test that copies megabytes per case is a
// test people start skipping.
func homeWithTranscript(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := filepath.Abs(filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, filepath.Join(proj, "session.jsonl")); err != nil {
		t.Fatal(err)
	}
	isolateHome(t, home)
	return home
}

// isolateHome points every path this binary derives from the environment at a
// directory the test owns.
//
// CLAUDE_CONFIG_DIR outranks HOME in claudeConfigDir, so setting HOME alone
// would leave a developer who has relocated their own transcripts running these
// assertions against their real corpus — which is both slow and a test that
// passes or fails depending on whose machine it is.
//
// USERPROFILE for the same reason on Windows. os.UserHomeDir reads HOME on unix
// and USERPROFILE on Windows, so setting only HOME left every one of these
// tests pointed at the CI runner's real home: they found no transcripts,
// failed on empty output, and read as a product bug for as long as the
// windows-latest job stayed red. Setting it on every platform is deliberate —
// a helper that isolates on the platform you happen to be on is not isolation,
// it is a coincidence, and FrozenDefectsTest FD-4 fails if this line is lost.
func isolateHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
}

// BR-1: a bare `replay` leads with the cost report.
//
// PASS: the report headline, and no usage block above it.
// FAIL: the menu, which is what the command printed before 2026-09-06.
func TestBR1_BareReplayLeadsWithTheCostReport(t *testing.T) {
	homeWithTranscript(t)
	var out, errOut bytes.Buffer
	if err := run(nil, &out, &errOut); err != nil {
		t.Fatalf("bare replay: %v (stderr: %s)", err, errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "Cost per task") {
		t.Fatalf("a bare replay must lead with the cost report:\n%s", got)
	}
	if strings.Contains(got, "Usage:") {
		t.Fatalf("a bare replay must not lead with a usage menu:\n%s", got)
	}
}

// BR-2: with nothing discoverable, the menu is still the right answer.
//
// This is the branch that must not be lost. A report about no transcripts is
// not a finding, and printing one would replace a usable menu with a confusing
// blank.
func TestBR2_BareReplayFallsBackToUsageWhenNothingIsDiscoverable(t *testing.T) {
	isolateHome(t, t.TempDir())
	var out, errOut bytes.Buffer
	if err := run(nil, &out, &errOut); err != nil {
		t.Fatalf("bare replay with no transcripts: %v (stderr: %s)", err, errOut.String())
	}
	// Assert on the list itself rather than on a heading. The heading was
	// "Usage:" and is now "Start here:", because help is ranked by value
	// instead of listed flat; a test pinned to one word would break on every
	// such change while telling you nothing about whether the list appeared.
	for _, want := range []string{"Start here:", "replay doctor", "replay serve"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("with no transcripts the command list is the correct output, and %q "+
				"is missing from it:\n%s", want, out.String())
		}
	}
}

// BR-3: the report is followed by the way out of it.
//
// Leading with one answer is only an improvement if the other commands are
// still findable. A reader who wanted `doctor` must not have to already know
// that `--help` exists.
func TestBR3_BareReplayPointsAtTheOtherCommands(t *testing.T) {
	homeWithTranscript(t)
	var out, errOut bytes.Buffer
	if err := run(nil, &out, &errOut); err != nil {
		t.Fatalf("bare replay: %v (stderr: %s)", err, errOut.String())
	}
	got := out.String()
	for _, want := range []string{"replay doctor", "replay blame", "replay --help"} {
		if !strings.Contains(got, want) {
			t.Errorf("the footer must name %q so the rest of the tool stays reachable:\n%s", want, got)
		}
	}
	// Below the report, not above it: the finding is the point.
	headline := strings.Index(got, "Cost per task")
	footer := strings.Index(got, "replay --help")
	if headline < 0 || footer < headline {
		t.Errorf("the pointer to other commands must come after the report, not before it:\n%s", got)
	}
}

// BR-4: a path argument still runs the replay engine.
//
// `replay <dir>` is the documented headline form and the tool is named for it.
// The HOME here has a discoverable transcript on purpose: without it this test
// would pass for the wrong reason, because the no-argument branch would not be
// taken either way.
func TestBR4_APathArgumentStillRunsTheReplayEngine(t *testing.T) {
	homeWithTranscript(t)
	var out, errOut bytes.Buffer
	if err := run([]string{filepath.Join("..", "..", "internal", "transcript", "testdata")}, &out, &errOut); err != nil {
		t.Fatalf("replay <dir>: %v (stderr: %s)", err, errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "Tier: estimated (transcripts only)") {
		t.Fatalf("a path argument must still produce the replay analysis:\n%s", got)
	}
	if strings.Contains(got, "Cost per task") {
		t.Fatalf("a path argument must not be rerouted into the cost report:\n%s", got)
	}
}

// BR-5: `replay --help` is a request for the menu and still gets it.
//
// The flag is checked before anything defaults, so a machine that does have
// transcripts must not have its help request answered with a cost report.
func TestBR5_HelpStillPrintsTheMenuOnAMachineWithTranscripts(t *testing.T) {
	homeWithTranscript(t)
	for _, arg := range []string{"--help", "-h", "help"} {
		var out, errOut bytes.Buffer
		if err := run([]string{arg}, &out, &errOut); err != nil {
			t.Fatalf("replay %s: %v (stderr: %s)", arg, err, errOut.String())
		}
		got := out.String()
		// "Start here:" rather than "Usage:": help is now ranked by value into
		// sections instead of listed flat. Asserting the section heading keeps
		// the test about whether the list appeared.
		if !strings.Contains(got, "Start here:") {
			t.Errorf("replay %s must print the command list:\n%s", arg, got)
		}
		if strings.Contains(got, "Cost per task") {
			t.Errorf("replay %s must not run the cost report:\n%s", arg, got)
		}
	}
}

// BR-4: with nothing found, say where you looked before showing the menu.
//
// BR-2 settled that the menu is the right answer when there are no
// transcripts, and it is. But a menu on its own does not tell a first-time
// reader WHY they got one. Pasting `replay` and receiving sixteen commands
// reads as "this needs arguments" or "this is broken", and both are wrong: the
// tool ran, looked in one specific place, and found nothing there.
//
// This matters more since 2026-09-06, when the installer began pointing every
// new user at bare `replay` as their first command. On a machine that has
// never run Claude Code, that first impression is this branch.
//
// PASS: the path it probed is named, and the reason is one line.
// FAIL: the menu appears with no explanation, or the path is not named.
func TestBR4_NothingFoundNamesWhereItLooked(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	var out, errOut bytes.Buffer
	if err := run(nil, &out, &errOut); err != nil {
		t.Fatalf("bare replay with no transcripts: %v (stderr: %s)", err, errOut.String())
	}
	got := out.String() + errOut.String()

	if !strings.Contains(got, "projects") {
		t.Errorf("the output does not name the directory it searched, so a reader whose "+
			"transcripts live elsewhere cannot tell that is the problem:\n%s", got)
	}
	if !strings.Contains(got, "No transcripts") && !strings.Contains(got, "no transcripts") {
		t.Errorf("the output does not say nothing was found, so the menu reads as a usage "+
			"error rather than an empty result:\n%s", got)
	}
	// The menu must still be there. BR-2's decision stands.
	if !strings.Contains(out.String(), "Start here:") {
		t.Errorf("the command list must still print; this adds a line, it does not "+
			"replace the menu:\n%s", out.String())
	}
}
