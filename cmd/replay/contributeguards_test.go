package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Guards on the contribution path that no test observed.
//
// Reported by the guard-reachability reviewer. Most were reached by an existing
// test that could not tell them apart: contributeCorpus has three consecutive
// refusals that all produce an error, so a test asserting "it failed" holds
// whichever one fired. Each test here pins a SPECIFIC refusal to the branch
// that owes it.

func figures() corpusFigures {
	return corpusFigures{Tasks: 115, Unpriced: 6, TotalUSD: 3382.13,
		RebilledUSD: 161.66, RebilledShare: 0.0478, MedianTaskUSD: 0.77}
}

// consentHome lays down a HOME with the given consent file body, or none.
func consentHome(t *testing.T, body string, write bool) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if write {
		writeConsent(t, home, body)
	}
	return home
}

// CG-A: the three refusals say three different things.
//
// OI1 proved none of them contributes. It could not prove the right one fired,
// because an unreadable file, an unanswered question and a declined answer all
// returned an error and the assertion was only that an error came back. The
// distinction is the whole reason consent has three states rather than two: a
// tool that merges them either nags someone who declined or never asks someone
// who was never asked.
func TestCGA_EachConsentStateGivesItsOwnRefusal(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		write      bool
		want       string
	}{
		{name: "never asked", write: false, want: "contributing is off"},
		{name: "declined", body: "corpus_opt_in = false\n", write: true, want: "declined in"},
		{name: "unreadable", body: "corpus_opt_in = maybe\n", write: true, want: "not a decision"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			consentHome(t, tc.body, tc.write)
			_, _, err := contributeCorpus(testCampaign, t.TempDir(), figures(), time.Now())
			if err == nil {
				t.Fatal("a submission was built")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("state %q was refused by the wrong branch.\n  got:  %v\n  want it to mention: %q",
					tc.name, err, tc.want)
			}
		})
	}
}

// CG-B: a machine identity that cannot be trusted stops the build.
//
// contributorSecret refuses a symlinked secret, because anyone who can redirect
// it chooses this machine's tag, and it refuses a truncated one, because a tag
// derived from a short secret is not the tag it claims to be. Neither refusal
// had ever been reached from the corpus path.
func TestCGB_AnUntrustworthySecretStopsTheBuild(t *testing.T) {
	t.Run("symlinked", func(t *testing.T) {
		home := consentHome(t, "corpus_opt_in = true\n", true)
		dir := filepath.Join(home, ".replay")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		elsewhere := filepath.Join(t.TempDir(), "secret")
		if err := os.WriteFile(elsewhere, []byte(strings.Repeat("a", 64)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, filepath.Join(dir, contributorSecretName)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		out := t.TempDir()
		_, _, err := contributeCorpus(testCampaign, out, figures(), time.Now())
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Errorf("a redirected identity was accepted: %v", err)
		}
		if e, _ := os.ReadDir(out); len(e) != 0 {
			t.Error("the refusal still wrote a submission")
		}
	})

	t.Run("truncated", func(t *testing.T) {
		home := consentHome(t, "corpus_opt_in = true\n", true)
		dir := filepath.Join(home, ".replay")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, contributorSecretName), []byte("short"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := contributeCorpus(testCampaign, t.TempDir(), figures(), time.Now())
		if err == nil || !strings.Contains(err.Error(), "too short") {
			t.Errorf("a truncated secret was accepted: %v", err)
		}
	})
}

// CG-C: an empty directory means the working directory, and a failed write
// propagates.
//
// The `dir == ""` default had no caller that passed one, so nothing established
// where a submission lands when the flag is absent — and the answer matters,
// because it is the difference between a file the contributor finds and one
// they do not.
func TestCGC_TheDefaultDirectoryIsHereAndAFailedWriteIsReported(t *testing.T) {
	consentHome(t, "corpus_opt_in = true\n", true)
	work := t.TempDir()
	t.Chdir(work)

	path, _, err := contributeCorpus(testCampaign, "", figures(), time.Now())
	if err != nil {
		t.Fatalf("contributeCorpus: %v", err)
	}
	if filepath.Dir(path) != "." {
		t.Errorf("an empty --contribute-dir wrote to %q, not the working directory", path)
	}
	if _, err := os.Stat(filepath.Join(work, filepath.Base(path))); err != nil {
		t.Errorf("the submission is not in the working directory: %v", err)
	}

	// The same call again collides with the file just written, and the writer's
	// refusal has to reach the caller rather than being swallowed.
	if _, _, err := contributeCorpus(testCampaign, "", figures(), time.Now()); err == nil {
		t.Error("a second identical submission overwrote the first")
	}
}

// CG-D: an earlier submission from this machine is reported and left alone.
//
// The supersedes branch is the contributor-facing half of the double-count
// defect: a corpus is cumulative, so attaching both files has this machine's
// spend pooled twice. Nothing had ever exercised the path that says so.
func TestCGD_AnEarlierSubmissionIsNamedAndKept(t *testing.T) {
	consentHome(t, "corpus_opt_in = true\n", true)
	out := t.TempDir()

	first, older, err := contributeCorpus(testCampaign, out, figures(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(older) != 0 {
		t.Fatalf("the first submission superseded %v", older)
	}

	moved := figures()
	moved.TotalUSD += 100
	moved.Tasks += 5
	second, supersedes, err := contributeCorpus(testCampaign, out, moved, time.Now().Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(supersedes) != 1 || supersedes[0] != filepath.Base(first) {
		t.Fatalf("the later submission reports superseding %v, want [%s]", supersedes, filepath.Base(first))
	}
	if _, err := os.Stat(first); err != nil {
		t.Errorf("the earlier submission was deleted; this path never removes a "+
			"contributor's files: %v", err)
	}

	note := corpusContributionNote(second, supersedes)
	if !strings.Contains(note, "SUPERSEDES 1 earlier") || !strings.Contains(note, filepath.Base(first)) {
		t.Errorf("the note does not name what it supersedes:\n%s", note)
	}
	if !strings.Contains(note, "counted twice") {
		t.Errorf("the note does not say why sending both is wrong:\n%s", note)
	}
	// And with nothing superseded the section is absent, not empty.
	if plain := corpusContributionNote(first, nil); strings.Contains(plain, "SUPERSEDES") {
		t.Errorf("a first submission claims to supersede something:\n%s", plain)
	}
}

// CG-E: --share keeps the path off stdout, and --json makes it a field.
//
// Three call sites decide where the contribution is announced, and each exists
// because stdout means something different on its branch. --share exists so
// that what is on screen is exactly what is safe to paste, and a path under the
// contributor's home is not; --json is a document a machine parses, so a
// human-readable paragraph in it is a parse error waiting to happen.
func TestCGE_TheAnnouncementRespectsTheOutputMode(t *testing.T) {
	t.Run("share keeps it off stdout", func(t *testing.T) {
		withCorpusConsent(t)
		out := t.TempDir()
		var stdout, stderr bytes.Buffer
		if err := run([]string{"cost", "--share", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr); err != nil {
			t.Fatalf("cost --share --contribute: %v\n%s", err, stderr.String())
		}
		if strings.Contains(stdout.String(), "carries SPEND") {
			t.Errorf("the contribution note is on the paste-safe surface:\n%s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "carries SPEND") {
			t.Errorf("the contributor is never told a file was written:\n%s", stderr.String())
		}
	})

	t.Run("json makes it a field", func(t *testing.T) {
		withCorpusConsent(t)
		out := t.TempDir()
		var stdout, stderr bytes.Buffer
		if err := run([]string{"cost", "--json", "--contribute", testCampaign, "--contribute-dir", out}, &stdout, &stderr); err != nil {
			t.Fatalf("cost --json --contribute: %v\n%s", err, stderr.String())
		}
		var doc map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
			t.Fatalf("stdout is not JSON, so the note leaked into the document: %v", err)
		}
		if doc["contribution"] == nil {
			t.Error("the JSON does not name the file that was written")
		}
		if !strings.Contains(stderr.String(), "carries SPEND") {
			t.Error("the human statement never reached a human")
		}
	})

	t.Run("no contribution, no announcement", func(t *testing.T) {
		withCorpusConsent(t)
		var stdout, stderr bytes.Buffer
		if err := run([]string{"cost", "--json"}, &stdout, &stderr); err != nil {
			t.Fatalf("cost --json: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		if _, present := doc["contribution"]; present {
			t.Error("a run that contributed nothing reports a contribution")
		}
		if strings.Contains(stderr.String(), "carries SPEND") {
			t.Error("a run that contributed nothing announced one")
		}
	})
}

// CG-F: a campaign is required, because it is the salt.
//
// LocalTag refuses an empty campaign: the campaign keys the HMAC, so without
// one every campaign's tags for a machine would be identical and therefore
// linkable across campaigns — which is the single property the per-campaign tag
// exists to provide. The corpus path had no test that reached the refusal.
func TestCGF_AnEmptyCampaignIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	consentHome(t, "corpus_opt_in = true\n", true)
	out := t.TempDir()

	_, _, err := contributeCorpus("", out, figures(), time.Now())
	if err == nil {
		t.Fatal("a submission was built with no campaign, so its tag would be linkable " +
			"to every other campaign's tag for this machine")
	}
	if !strings.Contains(err.Error(), "campaign is required") {
		t.Errorf("the refusal came from somewhere else: %v", err)
	}
	if e, _ := os.ReadDir(out); len(e) != 0 {
		t.Errorf("the refusal left %d file(s) behind", len(e))
	}
}

// CG-G: --json names what the submission supersedes.
//
// CG-E checked the JSON on a first contribution, where there is nothing to
// supersede, so the key was never populated. A machine contributing a second
// time is the case the field exists for, and a consumer pooling these files
// needs it: it is how a script knows which of two files on disk to send.
func TestCGG_TheJSONNamesWhatItSupersedes(t *testing.T) {
	withCorpusConsent(t)
	out := t.TempDir()

	var first, stderr bytes.Buffer
	if err := run([]string{"cost", "--json", "--contribute", testCampaign, "--contribute-dir", out}, &first, &stderr); err != nil {
		t.Fatalf("first contribution: %v\n%s", err, stderr.String())
	}
	var one map[string]any
	if err := json.Unmarshal(first.Bytes(), &one); err != nil {
		t.Fatal(err)
	}
	if _, present := one["supersedes"]; present {
		t.Error("a first contribution claims to supersede something")
	}

	// A second submission whose content differs, so it is a distinct file
	// rather than a refused duplicate. Editing the first file's name is enough:
	// the scan is by tag prefix, and this is what a week-old submission looks
	// like to it.
	older := filepath.Join(out, "replay-corpus-"+strings.Split(filepath.Base(one["contribution"].(string)), "-")[2]+"-000000000000.json")
	if err := os.WriteFile(older, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(one["contribution"].(string)); err != nil {
		t.Fatal(err)
	}

	var second bytes.Buffer
	stderr.Reset()
	if err := run([]string{"cost", "--json", "--contribute", testCampaign, "--contribute-dir", out}, &second, &stderr); err != nil {
		t.Fatalf("second contribution: %v\n%s", err, stderr.String())
	}
	var two map[string]any
	if err := json.Unmarshal(second.Bytes(), &two); err != nil {
		t.Fatal(err)
	}
	list, ok := two["supersedes"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("the JSON does not name the earlier submission: %v", two["supersedes"])
	}
	if list[0] != filepath.Base(older) {
		t.Errorf("supersedes names %v, want %s", list[0], filepath.Base(older))
	}
}
