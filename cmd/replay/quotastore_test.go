package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func lim(p float64, mins int) *window {
	return &window{UsedPercentage: p, ResetsAt: time.Now().Add(time.Duration(mins) * time.Minute).Unix()}
}

// TestQS1: a reading round-trips, and carries when it was taken.
//
// A quota figure without its age is the defect this whole area keeps producing:
// a reading from three hours ago renders identically to a live one.
func TestQS1(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "quota.json")
	in := statusInput{RateLimits: &rateLimits{FiveHour: lim(23.5, 40), SevenDay: lim(88, 3000)}}
	if err := saveQuota(p, in, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := loadQuota(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.RateLimits == nil || got.RateLimits.SevenDay == nil || got.RateLimits.SevenDay.UsedPercentage != 88 {
		t.Fatalf("reading did not survive the round trip: %+v", got)
	}
	if got.TakenAt.IsZero() {
		t.Error("a stored reading with no timestamp cannot be aged, so it will be shown as current forever")
	}
}

// TestQS2: the file is owner-only. It describes an account's consumption.
//
// Skipped on Windows, and the reason is worth stating rather than hiding
// behind a build tag. Go's file mode is a POSIX concept; on Windows the only
// bit the os package actually carries through is read-only, so a file written
// with 0600 stats as 0666 and this assertion fails for a reason that has
// nothing to do with the code under test. Access there is governed by the
// ACL the file inherits from the user profile directory, which this test
// cannot read and saveQuota does not set.
//
// So on Windows the owner-only property is NOT checked by anything. That is a
// real gap, recorded here rather than papered over by a skip with no comment,
// which would have read as coverage.
func TestQS2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are POSIX; on Windows 0600 stats as 0666 and access comes from the profile ACL instead")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "quota.json")
	if err := saveQuota(p, statusInput{RateLimits: &rateLimits{FiveHour: lim(10, 60)}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if mode := st.Mode().Perm(); mode&0o077 != 0 {
		t.Errorf("mode is %04o; group and other must have nothing", mode)
	}
}

// TestQS3: an absent store is not an error and not a zero reading.
//
// Most machines will never have run the status line. That must render as "no
// reading", never as a full window.
func TestQS3(t *testing.T) {
	got, err := loadQuota(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("a missing store must not be an error: %v", err)
	}
	if got.RateLimits != nil {
		t.Errorf("missing store produced a reading: %+v", got)
	}
}

// TestQS4: nothing is written when there is nothing to record.
//
// The status line runs on a 300ms debounce. A metered account has no windows at
// all, and writing an empty file on every tick would be a disk write per
// keystroke for a reading that does not exist.
func TestQS4(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "quota.json")
	if err := saveQuota(p, statusInput{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("a status input with no rate_limits wrote a file anyway")
	}
}

// TestQS5: a corrupt store reads as no reading, not as a crash and not as a
// zero. The status line writes this on a hot path; a partial write must not
// take out every other command.
func TestQS5(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "quota.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadQuota(p)
	if err != nil {
		t.Fatalf("corrupt store must not be an error to the caller: %v", err)
	}
	if got.RateLimits != nil {
		t.Error("corrupt store produced a reading")
	}
}
