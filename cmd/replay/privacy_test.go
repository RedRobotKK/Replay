package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

// PV10: an unreadable ~/.replay is not reported as an empty one.
//
// resolveStores returned nil for both, and privacy printed "Replay has written
// nothing to this machine" — a false absence claim in the command that answers
// "what do you hold about me". A subject access request answered with silence
// about a directory nobody could read is worse than an error.
func TestPV10_AnUnreadableStoreIsNotReportedAsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix mode bits on this platform; a directory cannot be made unreadable here")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read anything")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := filepath.Join(home, ".replay")
	if err := os.MkdirAll(filepath.Join(root, "ledger"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Skipf("cannot remove directory permissions: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

	var out, errb bytes.Buffer
	err := runPrivacy(nil, &out, &errb)
	if err == nil {
		t.Fatal("an unreadable ~/.replay was accepted; it must not be answered at all " +
			"rather than answered wrongly")
	}
	if strings.Contains(out.String(), "written nothing to this machine") {
		t.Errorf("an unreadable store was reported as an empty machine:\n%s", out.String())
	}
}

// PV11: a store with an entry nobody could walk is not reported at its readable size.
//
// measureStore totals what the walk could read. A directory inside a store that
// cannot be listed contributes nothing, so the store prints smaller than it is —
// and an entire store that cannot be opened prints as "0 B", which reads as a
// store known to be empty. The disclosure has to bound its own claim.
func TestPV11_AStoreWithAnEntryItCouldNotWalkSaysSo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix mode bits on this platform; a directory cannot be made unreadable here")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read anything")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	locked := filepath.Join(home, ".replay", "ledger", "locked")
	if err := os.MkdirAll(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "s1.jsonl"), []byte(`{"schema":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skipf("cannot remove directory permissions: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	var out, errb bytes.Buffer
	if err := runPrivacy(nil, &out, &errb); err != nil {
		t.Fatalf("privacy failed on a readable ~/.replay holding one unreadable entry: %v", err)
	}
	if !strings.Contains(out.String(), "could not be measured") {
		t.Errorf("a store holding an entry nobody could walk was printed as a plain "+
			"size, so the reader cannot tell it from one measured whole:\n%s", out.String())
	}
}

// PV12: and a store measured whole claims nothing was missed.
//
// The other half of PV11. A disclosure that always hedges is a disclosure the
// reader learns to ignore, which costs exactly as much as never hedging.
func TestPV12_AStoreMeasuredWholeSaysNothingWasMissed(t *testing.T) {
	homeWithStores(t)
	var out, errb bytes.Buffer
	if err := runPrivacy(nil, &out, &errb); err != nil {
		t.Fatalf("privacy failed: %v", err)
	}
	if strings.Contains(out.String(), "could not be measured") {
		t.Errorf("a fully readable ~/.replay reported entries it could not measure:\n%s",
			out.String())
	}
}

// measureStore counts files, and the directories holding them are not files.
//
// Walk visits the store root and every subdirectory under it. Counting those as
// entries would inflate the file count `replay privacy` prints and add the
// directories' own sizes to a total the reader reads as bytes of their data.
func TestMeasureStoreCountsFilesAndNotTheDirectoriesHoldingThem(t *testing.T) {
	store := filepath.Join(t.TempDir(), "ledger")
	if err := os.MkdirAll(filepath.Join(store, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, "a.jsonl"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, "nested", "b.jsonl"), []byte("01234567890123456789"), 0o600); err != nil {
		t.Fatal(err)
	}

	total, files, unmeasured := measureStore(store)
	if files != 2 {
		t.Errorf("measureStore counted %d files under a store holding two, plus two "+
			"directories; a directory is not a record", files)
	}
	if total != 30 {
		t.Errorf("measureStore totalled %d bytes, want 30: the reader reads this as "+
			"bytes of their data, not as bytes of directory entries", total)
	}
	if unmeasured != 0 {
		t.Errorf("%d entr(ies) reported unmeasurable in a store that was readable "+
			"throughout", unmeasured)
	}
}
