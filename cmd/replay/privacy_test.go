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

// `replay privacy` answers "what do you hold about me, and where".
//
// The question a subject access request asks, and the one this tool could not
// answer. A local-first tool is unusually well placed to: the answer is a
// directory listing plus an honest sentence per store, and nothing has to be
// requested from anyone.
//
// It reports rather than deletes. Every path it names is one `replay purge` can
// act on, and separating the two means reading what you hold is never one
// keystroke from removing it.

func homeWithStores(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, ".replay")
	for _, d := range []string{"ledger", "ledger-grok", "vault", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ledger", "s1.jsonl"),
		[]byte(`{"schema":1,"session_id":"keepme"}`+"\n"+`{"schema":1,"session_id":"goaway"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"advice.json", "seen.json", "tip.json", "cost-index.json"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return root
}

// PV1: every store present on disk is disclosed, with what it holds.
//
// PASS: each store found is named, with its size and a sentence saying what is
// inside.
// FAIL: a store on disk the report does not mention — which is the failure that
// makes a disclosure worse than none.
func TestPV1_EveryStoreOnDiskIsDisclosed(t *testing.T) {
	homeWithStores(t)
	var out, errb bytes.Buffer
	if err := runPrivacy(nil, &out, &errb); err != nil {
		t.Fatalf("privacy failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"ledger", "ledger-grok", "vault", "archive", "advice.json", "seen.json"} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not mention %s, which exists on disk:\n%s", want, got)
		}
	}
	lower := strings.ToLower(strings.Join(strings.Fields(got), " "))
	if !strings.Contains(lower, "never message content") && !strings.Contains(lower, "no message text") {
		t.Error("the report does not say what the ledger does NOT hold, which is the part a " +
			"reader most needs")
	}
}

// PV2: the sensitive store is called out, not listed flatly.
//
// PASS: the vault is marked as holding secrets.
// FAIL: it reads like a cache, and somebody copies it into a bug report.
func TestPV2_TheVaultIsFlaggedInTheReport(t *testing.T) {
	homeWithStores(t)
	var out, errb bytes.Buffer
	if err := runPrivacy(nil, &out, &errb); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	i := strings.Index(got, "vault")
	if i < 0 {
		t.Fatal("the vault is absent from the report")
	}
	if !strings.Contains(strings.ToLower(got), "secret") {
		t.Errorf("the vault is listed without saying it holds secrets:\n%s", got)
	}
}

// PV3: nothing is written or removed by asking.
//
// PASS: the tree is byte-identical afterwards.
// FAIL: a disclosure command with a side effect.
func TestPV3_ReportingChangesNothing(t *testing.T) {
	root := homeWithStores(t)
	before := treeSnapshot(t, root)
	var out, errb bytes.Buffer
	if err := runPrivacy(nil, &out, &errb); err != nil {
		t.Fatal(err)
	}
	if after := treeSnapshot(t, root); after != before {
		t.Errorf("running `privacy` changed the tree:\n before %s\n after  %s", before, after)
	}
}

// PV4: --json is machine-readable and carries the same stores.
//
// PASS: valid JSON naming every store the prose named.
// FAIL: a format that drifts from the human one, so two answers disagree.
func TestPV4_JSONCarriesTheSameStores(t *testing.T) {
	homeWithStores(t)
	var out, errb bytes.Buffer
	if err := runPrivacy([]string{"--json"}, &out, &errb); err != nil {
		t.Fatalf("privacy --json failed: %v", err)
	}
	var doc struct {
		Stores []struct {
			Name      string `json:"name"`
			Holds     string `json:"holds"`
			Sensitive bool   `json:"sensitive"`
			Bytes     int64  `json:"bytes"`
		} `json:"stores"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("--json is not JSON: %v\n%s", err, out.String())
	}
	if len(doc.Stores) < 6 {
		t.Fatalf("--json reports %d stores; six exist on disk in this fixture", len(doc.Stores))
	}
	for _, s := range doc.Stores {
		if strings.TrimSpace(s.Holds) == "" {
			t.Errorf("store %q is reported with no description", s.Name)
		}
	}
}

// PG7: --session removes one session's records from every store that has them.
//
// PASS: the named session's records are gone from the ledger, and other
// sessions in the same file survive.
// FAIL: the whole file removed, or the other session lost with it. This is the
// erasure request, and taking too much is as wrong as taking too little.
func TestPG7_SessionErasureRemovesOnlyThatSession(t *testing.T) {
	root := homeWithStores(t)
	var out, errb bytes.Buffer
	if err := runPurge([]string{root, "--session", "goaway", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("purge --session failed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "ledger", "s1.jsonl"))
	if err != nil {
		t.Fatalf("the ledger file was removed entirely, taking another session with it: %v", err)
	}
	body := string(b)
	if strings.Contains(body, "goaway") {
		t.Error("the erased session is still in the ledger")
	}
	if !strings.Contains(body, "keepme") {
		t.Error("erasing one session removed another from the same file")
	}
}

// PG8: --session and --older-than are not combined silently.
//
// PASS: giving both is refused.
// FAIL: one quietly wins, and the reader gets a deletion they did not describe.
func TestPG8_TheTwoModesAreNotCombined(t *testing.T) {
	root := homeWithStores(t)
	var out, errb bytes.Buffer
	if err := runPurge([]string{root, "--session", "goaway", "--older-than", "30d", "--yes"}, &out, &errb); err == nil {
		t.Error("purge accepted both --session and --older-than; one of them silently won")
	}
}

// PG9: --export writes the records before they are removed.
//
// PASS: the export file exists and contains the erased session.
// FAIL: data removed with no copy, when the reader asked for one.
func TestPG9_ExportWritesWhatItIsAboutToRemove(t *testing.T) {
	root := homeWithStores(t)
	dest := filepath.Join(t.TempDir(), "export.jsonl")
	var out, errb bytes.Buffer
	if err := runPurge([]string{root, "--session", "goaway", "--export", dest, "--yes"}, &out, &errb); err != nil {
		t.Fatalf("purge --export failed: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("--export wrote nothing: %v", err)
	}
	if !strings.Contains(string(b), "goaway") {
		t.Errorf("the export does not contain the records that were removed:\n%s", b)
	}
}

func treeSnapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		b.WriteString(p + ":" + info.ModTime().Format(time.RFC3339Nano) + ":" +
			string(rune(info.Size())) + "\n")
		return nil
	})
	return b.String()
}

// PV5: every line of the disclosure is a branch, and none of them were
// observed.
//
// `replay privacy` is the answer to "what do you have on me". Eight of its
// conditionals could be removed without a test noticing, including the mark
// that flags the one store holding secrets and the sentence saying a store
// survives a retention window. A disclosure whose distinguishing marks are
// untested discloses whatever the last edit left behind.
func TestPV5_TheDisclosureMarksAreEachObserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := filepath.Join(home, ".replay")
	if err := os.MkdirAll(filepath.Join(root, "vault"), 0o700); err != nil {
		t.Fatal(err)
	}
	// A directory store with more than one file, so the file count prints.
	led := filepath.Join(root, "ledger")
	if err := os.MkdirAll(led, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a.jsonl", "b.jsonl"} {
		if err := os.WriteFile(filepath.Join(led, n), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "vault", "vault.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := runPrivacy(nil, &out, &errb); err != nil {
		t.Fatalf("privacy failed: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "! vault") {
		t.Errorf("the sensitive store is not marked; the mark is how a reader finds the one\n"+
			"store holding secrets rather than counts about them:\n%s", s)
	}
	if !strings.Contains(s, "2 files") {
		t.Errorf("a store of several files did not report how many:\n%s", s)
	}
	if !strings.Contains(s, "Not removed by a retention window") {
		t.Errorf("a store that a window does not cover must say so, or a reader believes\n"+
			"purge --older-than reaches everything:\n%s", s)
	}
}

// PV6: nothing on disk is said plainly, not rendered as an empty table.
func TestPV6_AnEmptyMachineIsSaidPlainly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var out, errb bytes.Buffer
	if err := runPrivacy(nil, &out, &errb); err != nil {
		t.Fatalf("privacy failed: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "has written nothing to this machine") {
		t.Errorf("an empty machine must be stated, not shown as a heading over nothing:\n%s", s)
	}
	if strings.Contains(s, "Everything Replay holds") {
		t.Errorf("a heading was printed over an empty disclosure:\n%s", s)
	}
}

// PV7: with no home directory there is nothing to disclose and the command
// says why rather than reporting an empty machine.
//
// Absence, zero and unknown are three values. "I could not find your home" and
// "you have nothing" are different answers to "what do you have on me".
func TestPV7_NoHomeIsAnErrorNotAnEmptyReport(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	var out, errb bytes.Buffer
	err := runPrivacy(nil, &out, &errb)
	if err == nil {
		// Not a skip. `os.UserHomeDir` returns an error when HOME is empty on
		// unix, so a nil error here means the guard was removed, not that the
		// platform differs — and a skip would swallow exactly that regression.
		t.Fatal("privacy resolved a home directory with HOME unset, so the error " +
			"branch is gone: an unlocatable home would be reported as an empty machine")
	}
	if !strings.Contains(err.Error(), "home directory") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
	if strings.Contains(out.String(), "written nothing") {
		t.Errorf("an unlocatable home was reported as an empty machine:\n%s", out.String())
	}
}

// PV8: a store measured as a single file reports one file, and a store that
// cannot be measured reports nothing rather than guessing.
func TestPV8_MeasureStoreCountsFilesAndAbsence(t *testing.T) {
	dir := t.TempDir()
	single := filepath.Join(dir, "one.json")
	if err := os.WriteFile(single, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, n := measureStore(single); b != 5 || n != 1 {
		t.Errorf("measureStore(single file) = (%d, %d), want (5, 1)", b, n)
	}
	if b, n := measureStore(filepath.Join(dir, "absent")); b != 0 || n != 0 {
		t.Errorf("measureStore(absent) = (%d, %d), want (0, 0)", b, n)
	}
	sub := filepath.Join(dir, "many")
	if err := os.MkdirAll(filepath.Join(sub, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a", "nested/b"} {
		if err := os.WriteFile(filepath.Join(sub, p), []byte("xy"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Directories are counted as containers, never as files.
	if b, n := measureStore(sub); b != 4 || n != 2 {
		t.Errorf("measureStore(directory) = (%d, %d), want (4, 2): a directory is not a file", b, n)
	}
}
