package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// Consent is three states and each refusal is its own guard.
//
// readCorpusConsent returns Granted for exactly one thing. Unset and Declined
// are different refusals with different remedies — one tells you how to turn it
// on, the other tells you it is already off on purpose — and a test that only
// checks "it refused" cannot tell which fired.
func TestCalibrationContributionConsentStates(t *testing.T) {
	cals := []analysis.ModelCalibration{{Model: "m", Sessions: 1, Compared: 10, Matched: 10}}
	now := time.Date(2026, 9, 10, 5, 0, 0, 0, time.UTC)

	t.Run("unset asks", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		_, err := contributeCalibration("c", t.TempDir(), nil, cals, now)
		if err == nil {
			t.Fatal("a report was built with no consent file")
		}
		if !strings.Contains(err.Error(), "off until you turn it on") {
			t.Errorf("an absent decision must say how to make one: %v", err)
		}
	})

	t.Run("declined refuses differently", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		cfg := filepath.Join(home, ".config", "replay")
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		if err := os.MkdirAll(cfg, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfg, "corpus-consent.toml"),
			[]byte("corpus_opt_in = false\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := contributeCalibration("c", t.TempDir(), nil, cals, now)
		if err == nil {
			t.Fatal("a report was built against a declined decision")
		}
		if !strings.Contains(err.Error(), "declined") {
			t.Errorf("a declined decision must not be reported as an absent one: %v", err)
		}
	})

	t.Run("granted writes, and dedupes client versions", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		cfg := filepath.Join(home, ".config", "replay")
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		if err := os.MkdirAll(cfg, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfg, "corpus-consent.toml"),
			[]byte("corpus_opt_in = true\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out := t.TempDir()
		// Two rows share a client version and one carries none. The set must
		// hold each version once and must not carry an empty string, which
		// would publish "some session reported no version" as a version.
		rows := []corpusRow{{client: "2.1.257"}, {client: "2.1.257"}, {client: "2.1.260"}, {client: ""}}
		path, err := contributeCalibration("c", out, rows, cals, now)
		if err != nil {
			t.Fatalf("a granted decision did not write: %v", err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		if strings.Count(s, "2.1.257") != 1 {
			t.Errorf("a repeated client version was carried more than once:\n%s", s)
		}
		if strings.Contains(s, `""`) {
			t.Errorf("an empty client version was published as a version:\n%s", s)
		}
	})
}

// A campaign is required, and the refusal is its own guard.
//
// LocalTag refuses an empty campaign because the campaign is the salt: without
// one, every campaign's tags for a machine would be the same value and linkable
// across them. Nothing exercised that path from here.
func TestCalibrationContributionRequiresACampaign(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := filepath.Join(home, ".config", "replay")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "corpus-consent.toml"),
		[]byte("corpus_opt_in = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cals := []analysis.ModelCalibration{{Model: "m", Sessions: 1, Compared: 10, Matched: 10}}
	_, err := contributeCalibration("", t.TempDir(), nil, cals, time.Now())
	if err == nil {
		t.Fatal("a report was built with no campaign; the campaign is the salt")
	}
	if !strings.Contains(err.Error(), "campaign is required") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
}

// A consent file this build cannot parse is an error, never a yes.
//
// The read-failure branch was unobserved: every test supplied a file that
// parsed. A file that does not is a third state, and it must not fall through
// to either of the other two.
func TestCalibrationContributionRefusesAnUnreadableDecision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := filepath.Join(home, ".config", "replay")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	// Two decisions that contradict each other: parseable lines, unresolvable
	// intent. consent refuses rather than picking one.
	if err := os.WriteFile(filepath.Join(cfg, "corpus-consent.toml"),
		[]byte("corpus_opt_in = true\ncorpus_opt_in = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cals := []analysis.ModelCalibration{{Model: "m", Sessions: 1, Compared: 10, Matched: 10}}
	_, err := contributeCalibration("c", t.TempDir(), nil, cals, time.Now())
	if err == nil {
		t.Fatal("a self-contradictory consent file was treated as a decision")
	}
	if !strings.Contains(err.Error(), "cannot be read, so it is not a decision") {
		t.Errorf("an unreadable decision must not be reported as absent or declined: %v", err)
	}
}

// runCorpus must actually reach the contribution path, and must surface its
// refusal rather than reporting a corpus and swallowing it.
//
// Both guards in runCorpus were unobserved: no test invoked the command with
// --contribute, so neither "was the flag given" nor "did it fail" had ever
// fired. A contribution that silently did not happen is the absence nobody
// notices until the pool is short.
func TestRunCorpusContributeFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	// A transcript root with one readable session.
	root := filepath.Join(home, "projects", "p")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl"))
	if err != nil {
		t.Skipf("no transcript fixture available: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "s.jsonl"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	// Without consent the command must fail rather than print a corpus and
	// quietly write nothing.
	var out, errOut strings.Builder
	err = runCorpus([]string{filepath.Join(home, "projects"), "--contribute", "c",
		"--contribute-dir", t.TempDir()}, &out, &errOut)
	if err == nil {
		t.Fatal("--contribute with no consent returned no error")
	}
	if !strings.Contains(err.Error(), "off until you turn it on") {
		t.Errorf("the refusal must be the consent one: %v", err)
	}

	// And with no flag at all, the command reports as it always did.
	var out2, err2 strings.Builder
	if err := runCorpus([]string{filepath.Join(home, "projects")}, &out2, &err2); err != nil {
		t.Fatalf("corpus without --contribute failed: %v", err)
	}
	if !strings.Contains(out2.String(), "Calibration Corpus") {
		t.Errorf("the corpus report was not printed:\n%s", out2.String())
	}
}

// The machine identity must not be taken from a redirected path.
//
// contributorSecret refuses a symlinked secret, and that refusal reaches the
// caller through a guard nothing exercised. A tag derived through a symlink is
// a tag somebody else can point at whatever they like — the identity would no
// longer be this machine's.
func TestCalibrationContributionRefusesASymlinkedSecret(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := filepath.Join(home, ".config", "replay")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "corpus-consent.toml"),
		[]byte("corpus_opt_in = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replayDir := filepath.Join(home, ".replay")
	if err := os.MkdirAll(replayDir, 0o700); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(home, "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("not this machine's secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(replayDir, "contributor-secret")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	cals := []analysis.ModelCalibration{{Model: "m", Sessions: 1, Compared: 10, Matched: 10}}
	_, err := contributeCalibration("c", t.TempDir(), nil, cals, time.Now())
	if err == nil {
		t.Fatal("a report was built from a secret read through a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
}
