package main

import (
	"encoding/json"
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
