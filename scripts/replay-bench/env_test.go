package main

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseProcLoadavg(t *testing.T) {
	got, err := parseProcLoadavg("0.52 1.10 2.25 3/812 12345\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.One != 0.52 || got.Five != 1.10 || got.Fifteen != 2.25 || !got.OK {
		t.Errorf("parseProcLoadavg = %+v", got)
	}
	if _, err := parseProcLoadavg("garbage"); err == nil {
		t.Error("a malformed /proc/loadavg must be an error, not a zero load")
	}
}

// darwinLoadavgBytes builds the 24-byte struct loadavg { fixpt_t ldavg[3];
// long fscale; } that sysctl vm.loadavg returns on a 64-bit little-endian
// Mac: three uint32, four bytes of padding, one int64.
func darwinLoadavgBytes(a, b, c uint32, fscale uint64) []byte {
	out := make([]byte, 24)
	put32 := func(off int, v uint32) {
		for i := range 4 {
			out[off+i] = byte(v >> (8 * i))
		}
	}
	put32(0, a)
	put32(4, b)
	put32(8, c)
	for i := range 8 {
		out[16+i] = byte(fscale >> (8 * i))
	}
	return out
}

func TestParseDarwinLoadavg(t *testing.T) {
	// fscale 2048: 1.5 is 3072, 0.25 is 512, 4.0 is 8192.
	raw := darwinLoadavgBytes(3072, 512, 8192, 2048)
	got, err := parseDarwinLoadavg(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.One != 1.5 || got.Five != 0.25 || got.Fifteen != 4.0 || !got.OK {
		t.Errorf("parseDarwinLoadavg = %+v", got)
	}
	// syscall.Sysctl drops ONE trailing NUL byte, and fscale's high bytes
	// are zero, so the string it hands back is 23 bytes. That must still
	// parse rather than read as "no load".
	if got, err := parseDarwinLoadavg(raw[:23]); err != nil || got.One != 1.5 {
		t.Errorf("trimmed struct: %+v, %v", got, err)
	}
	if _, err := parseDarwinLoadavg(raw[:10]); err == nil {
		t.Error("a short struct must be an error")
	}
	if _, err := parseDarwinLoadavg(darwinLoadavgBytes(1, 1, 1, 0)); err == nil {
		t.Error("fscale zero would divide by zero; it must be an error")
	}
}

func TestLoadIsNoisyAboveTheNamedThreshold(t *testing.T) {
	// The threshold is per CPU, so the same absolute load means different
	// things on a 4-core runner and a 14-core laptop.
	quiet := LoadAvg{One: NoisyLoadPerCPU*8 - 0.01, OK: true}
	busy := LoadAvg{One: NoisyLoadPerCPU*8 + 0.01, OK: true}
	if Noisy(quiet, 8) {
		t.Errorf("load %v on 8 CPUs flagged noisy", quiet.One)
	}
	if !Noisy(busy, 8) {
		t.Errorf("load %v on 8 CPUs not flagged", busy.One)
	}
	// A load that could not be read is not evidence of a quiet machine.
	if !Noisy(LoadAvg{}, 8) {
		t.Error("an unreadable load average must count as noisy, not quiet")
	}
}

func TestCaptureEnvironmentRecordsWhatTheBriefRequires(t *testing.T) {
	fixed := time.Date(2026, 10, 30, 5, 6, 7, 0, time.UTC)
	env := CaptureEnvironment(func() time.Time { return fixed }, "abc123", []string{"-repeats", "10"})
	if env.Date != "2026-10-30T05:06:07Z" {
		t.Errorf("Date = %q", env.Date)
	}
	if env.GitSHA != "abc123" {
		t.Errorf("GitSHA = %q", env.GitSHA)
	}
	if env.GoVersion != runtime.Version() || env.GOOS != runtime.GOOS || env.GOARCH != runtime.GOARCH {
		t.Errorf("toolchain fields wrong: %+v", env)
	}
	if env.NumCPU != runtime.NumCPU() || env.GOMAXPROCS != runtime.GOMAXPROCS(0) {
		t.Errorf("CPU counts wrong: %+v", env)
	}
	if strings.Join(env.Args, " ") != "-repeats 10" {
		t.Errorf("Args = %v", env.Args)
	}
	if env.CPUModel == "" {
		t.Error("CPUModel empty; an unknown model must say so in words")
	}
	// On the two platforms the harness is run on, the load is readable.
	if (runtime.GOOS == "darwin" || runtime.GOOS == "linux") && !env.LoadBefore.OK {
		t.Errorf("load average unreadable on %s", runtime.GOOS)
	}
	if !math.IsNaN(env.NoisyLoadPerCPU) && env.NoisyLoadPerCPU != NoisyLoadPerCPU {
		t.Errorf("threshold not recorded: %v", env.NoisyLoadPerCPU)
	}
}

func TestReadSidecarRefusesALocalPath(t *testing.T) {
	// The sidecar is the shell's view of the machine (ps, uptime). It ends
	// up in a committed JSON file, and AGENTS.md says no local paths.
	dir := t.TempDir()
	p := filepath.Join(dir, "side.txt")
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory to test against")
	}
	if err := os.WriteFile(p, []byte("12.0 1 "+home+"/bin/thing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readSidecar(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, home) {
		t.Errorf("sidecar kept the home directory: %q", got)
	}
	if !strings.Contains(got, "~/bin/thing") {
		t.Errorf("sidecar lost the process name: %q", got)
	}
}
