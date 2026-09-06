package consent

import (
	"os"
	"path/filepath"
	"testing"
)

// Corpus consent, read at last.
//
// `install.sh --corpus-opt-in` has been writing
// ~/.config/replay/corpus-consent.toml since before this package existed, and
// nothing in the Go code ever read it. That was deliberate — the file itself
// says "nothing is sent: no command in this release transmits it" — and it
// stays true here: reading the file grants permission to BUILD a submission,
// never to send one.
//
//	K1  no file means undecided, and undecided sends nothing
//	K2  the affirmative the installer writes is the one that grants
//	K3  a refusal is remembered and not re-asked
//	K4  a symlink or a world-writable file is refused

func writeCorpus(t *testing.T, dir, body string) string {
	t.Helper()
	cfg := filepath.Join(dir, "replay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(cfg, CorpusFileName)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestK1_NoCorpusFileIsUndecided(t *testing.T) {
	// PASS: Unset, nothing permitted, and asking is appropriate.
	// FAIL: treating absence as consent, which would make every install a
	// contributor without anyone choosing it.
	d, err := ReadCorpusConsent(t.TempDir())
	if err != nil {
		t.Fatalf("a missing file is the normal case: %v", err)
	}
	if d.State != Unset || d.Allowed() {
		t.Errorf("state = %v allowed = %v, want Unset and false", d.State, d.Allowed())
	}
}

func TestK2_TheInstallersAffirmativeGrants(t *testing.T) {
	// The installer writes `corpus_opt_in = true` followed by comment lines.
	// PASS: that exact file grants.
	// FAIL: refusing what the shipped installer writes, which would make the
	// opt-in silently do nothing — the failure this whole file is fixing.
	dir := t.TempDir()
	writeCorpus(t, dir, "corpus_opt_in = true\n"+
		"# Written by install.sh --corpus-opt-in on 2026-09-05T10:00:00Z\n"+
		"# Delete this file to withdraw. Nothing is sent: no command in this release transmits it.\n")
	d, err := ReadCorpusConsent(dir)
	if err != nil {
		t.Fatalf("the installer's own output must parse: %v", err)
	}
	if !d.Allowed() {
		t.Error("the file the installer writes must grant")
	}

	for _, body := range []string{"corpus_opt_in = TRUE\n", "corpus_opt_in = yes\n", "corpus_opt_in=true\n", "corpus_opt_in = true junk\n"} {
		dir := t.TempDir()
		writeCorpus(t, dir, body)
		if d, err := ReadCorpusConsent(dir); err == nil && d.Allowed() {
			t.Errorf("%q was read as consent; only the exact affirmative may grant", body)
		}
	}
}

func TestK3_ARefusalIsRemembered(t *testing.T) {
	// PASS: Declined, nothing permitted, and no further asking.
	// FAIL: merging a considered no with never having been asked.
	dir := t.TempDir()
	writeCorpus(t, dir, "corpus_opt_in = false\n")
	d, err := ReadCorpusConsent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != Declined || d.Allowed() || d.ShouldAsk() {
		t.Errorf("state = %v allowed = %v shouldAsk = %v, want Declined, false, false", d.State, d.Allowed(), d.ShouldAsk())
	}
}

func TestK4_ASymlinkOrWorldWritableFileIsRefused(t *testing.T) {
	// PASS: refused. A grant any process could have written, or one whose
	// location someone else chose, is not this user's decision.
	dir := t.TempDir()
	p := writeCorpus(t, dir, "corpus_opt_in = true\n")
	if err := os.Chmod(p, 0o666); err == nil {
		if d, err := ReadCorpusConsent(dir); err == nil || d.Allowed() {
			t.Error("a world-writable consent file must be refused")
		}
	}

	dir2 := t.TempDir()
	other := filepath.Join(dir2, "elsewhere.toml")
	_ = os.WriteFile(other, []byte("corpus_opt_in = true\n"), 0o600)
	cfg := filepath.Join(dir2, "replay")
	_ = os.MkdirAll(cfg, 0o700)
	if err := os.Symlink(other, filepath.Join(cfg, CorpusFileName)); err == nil {
		if d, err := ReadCorpusConsent(dir2); err == nil || d.Allowed() {
			t.Error("a symlink at the consent path must be refused")
		}
	}
}
