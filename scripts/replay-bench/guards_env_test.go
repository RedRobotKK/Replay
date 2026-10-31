package main

import (
	"encoding/json"
	"errors"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// The tests in the guards_*_test.go files exist because guard-reachability
// neutralised every conditional this harness added and found most of them
// unobserved. Each one names the guard it drives, so the next reader can tell
// a test that freezes behaviour from one that only adds coverage. The
// per-guard ledger is docs/evidence/harness-gate0-guards-2026-10-31.md.

// errInjected is the failure the tests hand to a seam.
var errInjected = errors.New("injected failure")

// swap sets *p to v for the rest of the test.
func swap[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func TestCPUModelFromProcReadsTheModelName(t *testing.T) {
	// Guards cpuModelFromProc's `err == nil` and its `model name` match: with
	// either neutralised, a cpuinfo that names its model reads as unknown.
	info := "processor\t: 0\nvendor_id\t: GenuineTest\nmodel name\t: Test CPU @ 3.00GHz\nflags\t\t: fpu\n"
	if got := cpuModelFromProc([]byte(info), nil); got != "Test CPU @ 3.00GHz" {
		t.Errorf("cpuModelFromProc = %q, want the model name", got)
	}
	// A key that only contains the words is not the key.
	if got := cpuModelFromProc([]byte("not model name here\n"), nil); !strings.HasPrefix(got, "unknown") {
		t.Errorf("a line without the key read as %q", got)
	}
	// A failed read is not trusted, even with bytes alongside the error.
	if got := cpuModelFromProc([]byte(info), errInjected); !strings.HasPrefix(got, "unknown") {
		t.Errorf("a failed read produced %q", got)
	}
}

func TestLoadFromProcDistrustsAFailedRead(t *testing.T) {
	// os.ReadFile returns what it read before an error. A load built from a
	// read that failed must be unreadable, not a number.
	if got := loadFromProc([]byte("0.10 0.20 0.30 1/1 1\n"), errInjected); got.OK {
		t.Errorf("failed read produced a readable load: %+v", got)
	}
	if got := loadFromProc([]byte("0.10 0.20 0.30 1/1 1\n"), nil); !got.OK || got.One != 0.10 {
		t.Errorf("good read = %+v", got)
	}
	if got := loadFromProc([]byte("garbage"), nil); got.OK {
		t.Errorf("malformed read produced a readable load: %+v", got)
	}
}

func TestCPUModelAndLoadFromSysctl(t *testing.T) {
	if got := cpuModelFromSysctl("Apple M9", nil); got != "Apple M9" {
		t.Errorf("cpuModelFromSysctl = %q", got)
	}
	for _, c := range []struct {
		s   string
		err error
	}{{"", nil}, {"Apple M9", errInjected}} {
		if got := cpuModelFromSysctl(c.s, c.err); !strings.HasPrefix(got, "unknown") {
			t.Errorf("cpuModelFromSysctl(%q, %v) = %q, want unknown", c.s, c.err, got)
		}
	}
	raw := string(darwinLoadavgBytes(3072, 512, 8192, 2048))
	if got := loadFromSysctl(raw, nil); !got.OK || got.One != 1.5 {
		t.Errorf("loadFromSysctl = %+v", got)
	}
	if got := loadFromSysctl(raw, errInjected); got.OK {
		t.Errorf("failed sysctl produced a readable load: %+v", got)
	}
	if got := loadFromSysctl("short", nil); got.OK {
		t.Errorf("malformed sysctl produced a readable load: %+v", got)
	}
}

func TestRSSBytesIsMinusOneWhenGetrusageFails(t *testing.T) {
	// A peak that could not be read must not be reported as a peak of zero
	// or of whatever the zeroed struct held.
	if got := rssBytes(7, errInjected, 1024); got != -1 {
		t.Errorf("rssBytes on error = %d, want -1", got)
	}
	if got := rssBytes(7, nil, 1024); got != 7*1024 {
		t.Errorf("rssBytes = %d, want %d", got, 7*1024)
	}
}

func TestParseProcLoadavgRefusesShortAndNonNumericInput(t *testing.T) {
	// Two fields: without the length check this indexes past the slice.
	// A non-number: without the ParseFloat check it reads as a zero load.
	for _, in := range []string{"0.10 0.20", "x 0.20 0.30", "0.10 y 0.30"} {
		got, err := parseProcLoadavg(in)
		if err == nil {
			t.Errorf("parseProcLoadavg(%q) = %+v, want an error", in, got)
		}
		// loadFromProc relies on every error return being the zero LoadAvg.
		if got.OK {
			t.Errorf("parseProcLoadavg(%q) returned OK alongside its error", in)
		}
	}
}

func TestParseDarwinLoadavgRefusesAnOversizedValue(t *testing.T) {
	// 25 bytes is not struct loadavg. Without the size check the first 24
	// would be decoded and the extra byte silently dropped.
	long := append(darwinLoadavgBytes(3072, 512, 8192, 2048), 0)
	got, err := parseDarwinLoadavg(long)
	if err == nil {
		t.Errorf("a 25-byte value parsed as %+v", got)
	}
	if got.OK {
		t.Error("parseDarwinLoadavg returned OK alongside its error")
	}
}

func TestBuildRevisionComesFromBuildInfo(t *testing.T) {
	stamped := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: "deadbeef"},
	}}
	swap(t, &readBuildInfo, func() (*debug.BuildInfo, bool) { return stamped, true })
	if got := buildRevision(); got != "deadbeef" {
		t.Errorf("buildRevision = %q, want the stamped revision", got)
	}
	swap(t, &readBuildInfo, func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{}, true })
	if got := buildRevision(); got != "unknown" {
		t.Errorf("unstamped buildRevision = %q, want unknown", got)
	}
	swap(t, &readBuildInfo, func() (*debug.BuildInfo, bool) { return nil, false })
	if got := buildRevision(); got != "unknown" {
		t.Errorf("no build info: buildRevision = %q, want unknown", got)
	}
}

func TestCaptureEnvironmentFallsBackToBuildInfoAndRecordsEmptyArgs(t *testing.T) {
	stamped := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "cafef00d"}}}
	swap(t, &readBuildInfo, func() (*debug.BuildInfo, bool) { return stamped, true })
	env := CaptureEnvironment(time.Now, "", nil)
	if env.GitSHA != "cafef00d" {
		t.Errorf("GitSHA = %q, want the build info's revision when none is given", env.GitSHA)
	}
	// No args is an empty list in the JSON, not null: a reader should not
	// have to wonder whether the field was recorded.
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"args":[]`) {
		t.Errorf("nil args not recorded as []: %s", b)
	}
}

func TestReadSidecarReportsAMissingFile(t *testing.T) {
	if _, err := readSidecar(t.TempDir() + "/missing"); err == nil {
		t.Error("a missing sidecar must be an error, not empty notes")
	}
}
