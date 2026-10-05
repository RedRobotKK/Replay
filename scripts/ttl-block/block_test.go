package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, settings map[string]any) paths {
	t.Helper()
	dir := t.TempDir()
	p := paths{settings: filepath.Join(dir, "claude", "settings.json"), register: filepath.Join(dir, "register.md"),
		schedule: filepath.Join(dir, "schedule.json"), log: filepath.Join(dir, "blocks.jsonl")}
	if err := os.WriteFile(p.register, []byte("# register\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sha, err := fileSHA256(p.register)
	if err != nil {
		t.Fatal(err)
	}
	s, err := makeSchedule(sha, 6)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s)
	if err := os.WriteFile(p.schedule, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if settings != nil {
		if err := os.MkdirAll(filepath.Dir(p.settings), 0o700); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(settings)
		if err := os.WriteFile(p.settings, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func readSched(t *testing.T, p paths) schedule {
	t.Helper()
	b, err := os.ReadFile(p.schedule)
	if err != nil {
		t.Fatal(err)
	}
	var s schedule
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// The schedule is a function of the register's hash and nothing else.
func TestScheduleIsDerivedFromTheRegisterHashAndAlternates(t *testing.T) {
	even := "00" + strings.Repeat("ab", 31)
	odd := "01" + strings.Repeat("ab", 31)
	s, err := makeSchedule(even, 6)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.Blocks, ",") != "treatment,control,treatment,control,treatment,control" {
		t.Errorf("even first byte starts with treatment and alternates: %v", s.Blocks)
	}
	s, _ = makeSchedule(odd, 6)
	if strings.Join(s.Blocks, ",") != "control,treatment,control,treatment,control,treatment" {
		t.Errorf("odd first byte starts with control: %v", s.Blocks)
	}
	if _, err := makeSchedule(even, 5); err == nil {
		t.Error("fewer than six blocks is refused")
	}
	if _, err := makeSchedule("zz", 6); err == nil {
		t.Error("a non-hash is refused")
	}
}

// Starting a block writes the arm's value, reads it back, and records it.
func TestStartWritesTheArmAndRecordsItOnlyWhenReadBack(t *testing.T) {
	p := fixture(t, map[string]any{"promptCacheTtl": "1h", "env": map[string]any{"K": "v"}})
	s := readSched(t, p)
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	rec, err := startBlock(p, 1, s.Blocks[0], now)
	if err != nil {
		t.Fatal(err)
	}
	want := "5m"
	if s.Blocks[0] == armControl {
		want = "UNSET"
	}
	if rec.Event != "BLOCK_STARTED" || rec.StateChange != "VERIFIED" || rec.ConfiguredValue != want || rec.RequestedValue != want || rec.PriorValue != "1h" {
		t.Errorf("record: %+v", rec)
	}
	got, _ := readBack(p.settings)
	if got != want {
		t.Errorf("settings file holds %q, want %q", got, want)
	}
	var m map[string]any
	b, _ := os.ReadFile(p.settings)
	_ = json.Unmarshal(b, &m)
	if _, ok := m["env"]; !ok {
		t.Error("an unrelated key was dropped")
	}
	if _, err := os.Stat(rec.BackupPath); err != nil {
		t.Errorf("the backup named in the record does not exist: %v", err)
	}
	log, _ := readLog(p.log)
	if len(log) != 1 || log[0].RegisterSHA256 != s.RegisterSHA256 || log[0].SettingsSHA256 == "" {
		t.Errorf("the log carries the register hash and the settings hash: %+v", log)
	}
}

// The schedule decides the arm; the caller may not.
func TestStartRefusesAnArmTheScheduleDidNotName(t *testing.T) {
	p := fixture(t, map[string]any{})
	s := readSched(t, p)
	if _, err := startBlock(p, 1, other(s.Blocks[0]), time.Now()); err == nil || !strings.Contains(err.Error(), "the schedule names block 1") {
		t.Errorf("want a schedule refusal, got %v", err)
	}
	if _, err := startBlock(p, 7, armTreat, time.Now()); err == nil {
		t.Error("a block outside the schedule is refused")
	}
	if _, err := os.Stat(p.log); !os.IsNotExist(err) {
		t.Error("a refused start writes no record")
	}
}

// A write the file does not confirm is recorded as not started.
func TestStartRecordsUnverifiedWhenTheReadBackDisagrees(t *testing.T) {
	p := fixture(t, map[string]any{"promptCacheTtl": "1h"})
	s := readSched(t, p)
	old := readBack
	defer func() { readBack = old }()
	calls := 0
	readBack = func(_ string) (string, bool) {
		calls++
		if calls == 1 {
			return "1h", true // the prior
		}
		return "1h", true // the read-back disagrees with every arm's request
	}
	// Force a request that differs from "1h" whatever the schedule says.
	arm := s.Blocks[0]
	rec, err := startBlock(p, 1, arm, time.Now())
	if err == nil {
		t.Fatal("a block whose read-back disagrees has not started")
	}
	if rec.Event != "BLOCK_NOT_STARTED" || rec.StateChange != "UNVERIFIED" {
		t.Errorf("record: %+v", rec)
	}
	log, _ := readLog(p.log)
	if len(log) != 1 || log[0].Event != "BLOCK_NOT_STARTED" {
		t.Errorf("the failure is recorded as what it was: %+v", log)
	}
	if openBlock(log) != 0 {
		t.Error("a block that did not start is not open")
	}
}

// Blocks run one at a time, in order, and never twice.
func TestBlocksDoNotOverlapOrRepeat(t *testing.T) {
	p := fixture(t, map[string]any{})
	s := readSched(t, p)
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	if _, err := startBlock(p, 1, s.Blocks[0], now); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 2, s.Blocks[1], now.Add(time.Hour)); err == nil || !strings.Contains(err.Error(), "still open") {
		t.Errorf("block 2 cannot start while block 1 is open: %v", err)
	}
	if _, err := endBlock(p, 2, now.Add(time.Hour)); err == nil {
		t.Error("only the open block can end")
	}
	rec, err := endBlock(p, 1, now.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if rec.SpanIntact == nil || !*rec.SpanIntact {
		t.Errorf("an untouched settings file keeps the span intact: %+v", rec)
	}
	if _, err := startBlock(p, 1, s.Blocks[0], now.Add(72*time.Hour)); err == nil || !strings.Contains(err.Error(), "already started") {
		t.Errorf("a block does not start twice: %v", err)
	}
	if _, err := startBlock(p, 2, s.Blocks[1], now.Add(72*time.Hour)); err != nil {
		t.Errorf("block 2 starts once block 1 ended: %v", err)
	}
}

// A settings file that changed during a block is recorded as a broken span.
func TestEndRecordsABrokenSpan(t *testing.T) {
	p := fixture(t, map[string]any{})
	s := readSched(t, p)
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	if _, err := startBlock(p, 1, s.Blocks[0], now); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.settings, []byte(`{"promptCacheTtl":"1h","other":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, err := endBlock(p, 1, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if rec.SpanIntact == nil || *rec.SpanIntact || !strings.Contains(rec.Reason, "changed during the block") {
		t.Errorf("a changed file is a broken span, said so: %+v", rec)
	}
}

// An amended register voids the schedule until it is re-derived.
func TestAnAmendedRegisterRefusesEveryOperation(t *testing.T) {
	p := fixture(t, map[string]any{})
	s := readSched(t, p)
	if err := os.WriteFile(p.register, []byte("# register, amended\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 1, s.Blocks[0], time.Now()); err == nil || !strings.Contains(err.Error(), "amended register") {
		t.Errorf("want a refusal naming the amended register, got %v", err)
	}
}

// TestMain isolates HOME and USERPROFILE for the whole run: main.go resolves
// the home directory to find the settings file, and a test that reached the
// reader's real home could write to the file the study itself changes.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "replay-ttl-block-home-")
	if err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	if err := os.Setenv("HOME", dir); err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	if err := os.Setenv("USERPROFILE", dir); err != nil {
		panic("isolating USERPROFILE for the test suite: " + err.Error())
	}
	if err := os.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, ".claude")); err != nil {
		panic("isolating CLAUDE_CONFIG_DIR for the test suite: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// Every way the storage can fail is named, and none of them starts a block.
func TestStartNamesEveryStorageFailureAndStartsNothing(t *testing.T) {
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)

	// Settings that are not JSON: refused before any write.
	p := fixture(t, nil)
	if err := os.MkdirAll(filepath.Dir(p.settings), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.settings, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := readSched(t, p)
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("invalid settings JSON: %v", err)
	}
	if b, _ := os.ReadFile(p.settings); string(b) != "{not json" {
		t.Error("the unparseable file was modified")
	}

	// The settings path is a directory: not "does not exist", so an error.
	p = fixture(t, nil)
	if err := os.MkdirAll(p.settings, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil {
		t.Error("a directory at the settings path is an error, not an absent file")
	}

	// The settings directory cannot be created because a file sits in its path.
	p = fixture(t, nil)
	if err := os.WriteFile(filepath.Dir(p.settings), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil {
		t.Error("a file where the settings directory must be is an error")
	}

	// The settings directory cannot be created: its parent is read-only.
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.MkdirAll(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	p = fixture(t, nil)
	p.settings = filepath.Join(ro, "claude", "settings.json")
	if _, err := startBlock(p, 1, s.Blocks[0], now); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("an uncreatable settings directory is a permission error: %v", err)
	}
	_ = os.Chmod(ro, 0o700)

	// The backup cannot be written: the directory is read-only. The settings
	// file is left untouched.
	p = fixture(t, map[string]any{"promptCacheTtl": "1h"})
	if err := os.Chmod(filepath.Dir(p.settings), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(p.settings), 0o700) })
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil || !strings.Contains(err.Error(), "backup not written") {
		t.Errorf("read-only directory: %v", err)
	}
	if got, _ := readBack(p.settings); got != "1h" {
		t.Errorf("the settings file was touched although the backup failed: %q", got)
	}
	_ = os.Chmod(filepath.Dir(p.settings), 0o700)

	// A settings file that exists but cannot be read (write-only): refused as
	// unreadable, and not overwritten, which is what the write would do.
	p = fixture(t, map[string]any{"promptCacheTtl": "1h", "keep": true})
	if err := os.Chmod(p.settings, 0o200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p.settings, 0o600) })
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil {
		t.Error("an unreadable settings file is refused")
	}
	_ = os.Chmod(p.settings, 0o600)
	if got, _ := readBack(p.settings); got != "1h" {
		t.Errorf("an unreadable settings file was overwritten: %q", got)
	}

	// The backup is written but the settings file itself is read-only.
	p = fixture(t, map[string]any{"promptCacheTtl": "1h"})
	if err := os.Chmod(p.settings, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p.settings, 0o600) })
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil {
		t.Error("a read-only settings file cannot be written")
	}
	if _, err := os.Stat(p.log); !os.IsNotExist(err) {
		t.Error("a write that failed is not a block that started or failed to start; no record is written")
	}
	_ = os.Chmod(p.settings, 0o600)

	// The block log cannot be read because a directory sits at its path:
	// refused before anything is written, so the settings file is untouched.
	p = fixture(t, map[string]any{"promptCacheTtl": "1h"})
	if err := os.MkdirAll(p.log, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil {
		t.Error("a log path that is a directory cannot be read")
	}
	if got, _ := readBack(p.settings); got != "1h" {
		t.Errorf("the settings file was written although the log could not be read: %q", got)
	}

	// The block log can be read but not appended (read-only): the write
	// happened, the record did not, and the error says so.
	p = fixture(t, map[string]any{})
	if err := os.WriteFile(p.log, nil, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p.log, 0o600) })
	if _, err := startBlock(p, 1, s.Blocks[0], now); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("a read-only log cannot be appended, a permission error: %v", err)
	}
	_ = os.Chmod(p.log, 0o600)

	// The read-back cannot read the file after the write: UNREADABLE.
	p = fixture(t, map[string]any{})
	old := readBack
	readBack = func(_ string) (string, bool) { return "", false }
	rec, err := startBlock(p, 1, s.Blocks[0], now)
	readBack = old
	if err == nil || rec.Event != "BLOCK_NOT_STARTED" || rec.ConfiguredValue != "UNREADABLE" || rec.StateChange != "UNVERIFIED" {
		t.Errorf("unreadable after write: %v %+v", err, rec)
	}

	// No schedule: nothing runs.
	p = fixture(t, map[string]any{})
	if err := os.Remove(p.schedule); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 1, armTreat, now); err == nil || !strings.Contains(err.Error(), "reading the schedule") {
		t.Errorf("a missing schedule refuses every operation, as a missing schedule: %v", err)
	}
	if _, err := endBlock(p, 1, now); err == nil || !strings.Contains(err.Error(), "reading the schedule") {
		t.Errorf("a missing schedule refuses end too: %v", err)
	}
	// No register, a schedule that is not JSON: refused, each for its own
	// reason and not for the next guard's.
	p = fixture(t, map[string]any{})
	if err := os.Remove(p.register); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 1, armTreat, now); err == nil || !strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "amended") {
		t.Errorf("a missing register is refused as missing: %v", err)
	}
	p = fixture(t, map[string]any{})
	if err := os.WriteFile(p.schedule, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := startBlock(p, 1, armTreat, now); err == nil || !strings.Contains(err.Error(), "invalid character") || strings.Contains(err.Error(), "amended") {
		t.Errorf("a schedule that is not JSON is refused as not JSON: %v", err)
	}
}

// Ending a block is refused for the frozen artefacts' sake before anything
// is written, and a log that cannot be read at the end is an error.
func TestEndRefusesBeforeWritingWhenTheArtefactsAreGone(t *testing.T) {
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	p := fixture(t, map[string]any{})
	s := readSched(t, p)
	if _, err := startBlock(p, 1, s.Blocks[0], now); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.schedule); err != nil {
		t.Fatal(err)
	}
	if _, err := endBlock(p, 1, now.Add(time.Hour)); err == nil {
		t.Error("no schedule: end refused")
	}
	log, _ := readLog(p.log)
	if len(log) != 1 || openBlock(log) != 1 {
		t.Errorf("a refused end writes nothing; block 1 stays open: %+v", log)
	}
	// Restore the schedule, replace the log with a directory: readable? No.
	b, _ := json.Marshal(s)
	if err := os.WriteFile(p.schedule, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.log); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.log, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := endBlock(p, 1, now.Add(time.Hour)); err == nil || !strings.Contains(err.Error(), "reading the block log") {
		t.Errorf("a log that cannot be read refuses end, as unreadable: %v", err)
	}
}

// The encoder cannot refuse what this program hands it, and the refusal
// paths are still seen: nothing is written when it does.
func TestAnEncoderRefusalWritesNothing(t *testing.T) {
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	p := fixture(t, map[string]any{"promptCacheTtl": "1h"})
	s := readSched(t, p)
	oldIndent, oldJSON := marshalIndent, marshalJSON
	defer func() { marshalIndent, marshalJSON = oldIndent, oldJSON }()
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("refused") }
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil || !strings.Contains(err.Error(), "encoding the settings") {
		t.Errorf("settings encoding refused: %v", err)
	}
	if got, _ := readBack(p.settings); got != "1h" {
		t.Errorf("the settings file was written although encoding failed: %q", got)
	}
	var out, errb strings.Builder
	if code := run([]string{"schedule", "-register", p.register, "-schedule", filepath.Join(t.TempDir(), "s.json")}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "encoding the schedule") {
		t.Errorf("schedule encoding refused: code %d, %q", code, errb.String())
	}
	marshalIndent = oldIndent
	marshalJSON = func(any) ([]byte, error) { return nil, errors.New("refused") }
	if _, err := startBlock(p, 1, s.Blocks[0], now); err == nil || !strings.Contains(err.Error(), "encoding the block record") {
		t.Errorf("record encoding refused: %v", err)
	}
	if _, err := os.Stat(p.log); !os.IsNotExist(err) {
		t.Error("no record was appended when encoding failed")
	}
}

// gitHead reads HEAD only from a repository found on the walk, never from
// a file named HEAD in the working directory.
func TestGitHeadDoesNotReadAStrayHEADFromTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte(strings.Repeat("e", 40)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if got := gitHead(filepath.Join(dir, "no", "repo")); got != "UNAVAILABLE" {
		t.Errorf("a stray HEAD in the working directory is not a commit: %q", got)
	}
}

// The block log is read strictly, and a prior value is what the file held.
func TestLogAndPriorAreReadAsTheyAre(t *testing.T) {
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	p := fixture(t, nil)
	s := readSched(t, p)
	// No settings file at all: the prior is UNSET, and the file is created.
	rec, err := startBlock(p, 1, s.Blocks[0], now)
	if err != nil {
		t.Fatal(err)
	}
	if rec.PriorValue != "UNSET" || rec.BackupPath != "NONE" || rec.ToolCommit == "" {
		t.Errorf("absent file: %+v", rec)
	}
	// A log with a line that is not JSON is refused, not skipped.
	if err := os.WriteFile(p.log, []byte("{\"schema\":\"x\"}\nnot json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLog(p.log); err == nil || !strings.Contains(err.Error(), "one JSON object per line") {
		t.Errorf("malformed log: %v", err)
	}
	// A log path that is a directory is an error, not an empty log.
	if err := os.Remove(p.log); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.log, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := readLog(p.log); err == nil {
		t.Error("a directory at the log path is an error")
	}
	// A last line without a newline still counts.
	if err := os.RemoveAll(p.log); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.log, []byte(`{"schema":"a","event":"BLOCK_STARTED","block":1}`+"\n"+`{"schema":"a","event":"BLOCK_ENDED","block":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	log, err := readLog(p.log)
	if err != nil || len(log) != 2 || openBlock(log) != 0 {
		t.Errorf("two records, the last without a newline, the block closed: %v %d", err, len(log))
	}
	// Ending a block whose settings file has gone is an error, not a record.
	p = fixture(t, map[string]any{})
	if _, err := startBlock(p, 1, s.Blocks[0], now); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.settings); err != nil {
		t.Fatal(err)
	}
	if _, err := endBlock(p, 1, now.Add(time.Hour)); err == nil {
		t.Error("a settings file that vanished during the block cannot be hashed")
	}
	// Hash helpers refuse what they cannot read, and a 62-character hash is not a digest.
	if _, err := fileSHA256(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("a missing file has no hash")
	}
	if _, err := makeSchedule(strings.Repeat("ab", 31), 6); err == nil {
		t.Error("31 bytes is not a SHA-256 digest")
	}
	if v, ok := readBack(filepath.Join(t.TempDir(), "absent")); ok || v != "" {
		t.Error("a missing settings file does not read back")
	}
	bad := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, ok := readBack(bad); ok || v != "" {
		t.Error("settings that are not JSON do not read back")
	}
	if got := short("abc"); got != "abc" {
		t.Errorf("short of a short string is itself: %q", got)
	}
	if got := short(strings.Repeat("f", 64)); got != strings.Repeat("f", 12) {
		t.Errorf("short of a digest is its first twelve characters: %q", got)
	}
}

// The command line dispatches, prints the record, and exits by the outcome.
func TestRunDispatchesAndExitsByOutcome(t *testing.T) {
	var out, errb strings.Builder
	if code := run(nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), "usage") {
		t.Errorf("no command: code %d, %q", code, errb.String())
	}
	errb.Reset()
	if code := run([]string{"frobnicate"}, &out, &errb); code != 2 {
		t.Errorf("unknown command: code %d", code)
	}
	errb.Reset()
	if code := run([]string{"start", "-no-such-flag"}, &out, &errb); code != 2 {
		t.Errorf("a bad flag is a usage error: code %d", code)
	}
	p := fixture(t, map[string]any{})
	if err := os.Remove(p.schedule); err != nil {
		t.Fatal(err)
	}
	args := func(cmd string, more ...string) []string {
		return append([]string{cmd, "-settings", p.settings, "-register", p.register, "-schedule", p.schedule, "-log", p.log}, more...)
	}
	if code := run(args("schedule"), &out, &errb); code != 0 {
		t.Fatalf("schedule: code %d, %s", code, errb.String())
	}
	s := readSched(t, p)
	out.Reset()
	if code := run(args("start", "-block", "1", "-arm", s.Blocks[0]), &out, &errb); code != 0 || !strings.Contains(out.String(), `"event":"BLOCK_STARTED"`) {
		t.Errorf("start: code %d, out %q, err %q", code, out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := run(args("start", "-block", "2", "-arm", s.Blocks[1]), &out, &errb); code != 1 || !strings.Contains(errb.String(), "still open") {
		t.Errorf("an overlap exits 1 with the reason: code %d, %q", code, errb.String())
	}
	out.Reset()
	if code := run(args("end", "-block", "1"), &out, &errb); code != 0 || !strings.Contains(out.String(), `"event":"BLOCK_ENDED"`) {
		t.Errorf("end: code %d, %q", code, out.String())
	}
	errb.Reset()
	if code := run(args("schedule", "-blocks", "2"), &out, &errb); code != 1 {
		t.Errorf("a schedule under six blocks is refused: code %d", code)
	}
	errb.Reset()
	if code := run([]string{"schedule", "-register", filepath.Join(t.TempDir(), "absent"), "-schedule", p.schedule}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "no such file") {
		t.Errorf("a missing register cannot be scheduled, and the error names the file: code %d, %q", code, errb.String())
	}
	// The default settings path follows CLAUDE_CONFIG_DIR, then the home.
	t.Setenv("CLAUDE_CONFIG_DIR", "/x/cfg")
	if got := defaultSettings(); got != filepath.Join("/x/cfg", "settings.json") {
		t.Errorf("CLAUDE_CONFIG_DIR wins: %q", got)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if got := defaultSettings(); !strings.HasSuffix(got, filepath.Join(".claude", "settings.json")) {
		t.Errorf("home fallback: %q", got)
	}
}

// The tool names its commit from the git files, for a plain repository, a
// worktree, a packed ref, a detached HEAD, and nothing at all.
func TestGitHeadReadsEveryLayoutWithoutRunningGit(t *testing.T) {
	hash := strings.Repeat("a", 40)
	// Plain repository with a loose ref.
	repo := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(repo, ".git", "refs", "heads"), 0o700))
	must(os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o600))
	must(os.WriteFile(filepath.Join(repo, ".git", "refs", "heads", "main"), []byte(hash+"\n"), 0o600))
	nested := filepath.Join(repo, "docs", "evidence")
	must(os.MkdirAll(nested, 0o700))
	if got := gitHead(nested); got != hash {
		t.Errorf("loose ref from a subdirectory: %q", got)
	}
	// Packed ref only.
	must(os.Remove(filepath.Join(repo, ".git", "refs", "heads", "main")))
	must(os.WriteFile(filepath.Join(repo, ".git", "packed-refs"), []byte("# pack-refs\n"+strings.Repeat("b", 40)+" refs/heads/other\n"+hash+" refs/heads/main\n"), 0o600))
	if got := gitHead(repo); got != hash {
		t.Errorf("packed ref: %q", got)
	}
	// A ref in neither place.
	must(os.WriteFile(filepath.Join(repo, ".git", "packed-refs"), []byte("# pack-refs\n"), 0o600))
	if got := gitHead(repo); got != "UNAVAILABLE" {
		t.Errorf("unresolvable ref: %q", got)
	}
	must(os.Remove(filepath.Join(repo, ".git", "packed-refs")))
	if got := gitHead(repo); got != "UNAVAILABLE" {
		t.Errorf("no packed-refs: %q", got)
	}
	// Detached HEAD.
	must(os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte(hash+"\n"), 0o600))
	if got := gitHead(repo); got != hash {
		t.Errorf("detached: %q", got)
	}
	// A worktree: .git is a file naming its gitdir, whose commondir holds the refs.
	wt := t.TempDir()
	wtGit := filepath.Join(repo, ".git", "worktrees", "wt")
	must(os.MkdirAll(wtGit, 0o700))
	must(os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+wtGit+"\n"), 0o600))
	must(os.WriteFile(filepath.Join(wtGit, "commondir"), []byte("../..\n"), 0o600))
	must(os.WriteFile(filepath.Join(wtGit, "HEAD"), []byte("ref: refs/heads/feature\n"), 0o600))
	must(os.WriteFile(filepath.Join(repo, ".git", "refs", "heads", "feature"), []byte(strings.Repeat("c", 40)+"\n"), 0o600))
	if got := gitHead(wt); got != strings.Repeat("c", 40) {
		t.Errorf("worktree through commondir: %q", got)
	}
	// A gitdir file with no commondir resolves against the gitdir itself.
	must(os.Remove(filepath.Join(wtGit, "commondir")))
	must(os.WriteFile(filepath.Join(wtGit, "HEAD"), []byte(strings.Repeat("d", 40)+"\n"), 0o600))
	if got := gitHead(wt); got != strings.Repeat("d", 40) {
		t.Errorf("gitdir without commondir: %q", got)
	}
	// No repository above: UNAVAILABLE, and a HEAD that cannot be read likewise.
	if got := gitHead(filepath.Join(t.TempDir(), "x", "y")); got != "UNAVAILABLE" {
		t.Errorf("outside any repository: %q", got)
	}
	must(os.Remove(filepath.Join(repo, ".git", "HEAD")))
	if got := gitHead(repo); got != "UNAVAILABLE" {
		t.Errorf("missing HEAD: %q", got)
	}
	// A gitdir file pointing nowhere, and a .git file that cannot be read.
	broken := t.TempDir()
	must(os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: /nonexistent/x\n"), 0o600))
	if got := gitHead(broken); got != "UNAVAILABLE" {
		t.Errorf("gitdir pointing nowhere: %q", got)
	}
	unreadable := t.TempDir()
	must(os.WriteFile(filepath.Join(unreadable, ".git"), []byte("gitdir: x\n"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(unreadable, ".git"), 0o600) })
	if got := gitHead(unreadable); got != "UNAVAILABLE" {
		t.Errorf("unreadable .git file: %q", got)
	}
	// This repository itself resolves to a 40-character hash.
	if got := toolCommit(); len(got) != 40 {
		t.Errorf("this tree's HEAD: %q", got)
	}
}
