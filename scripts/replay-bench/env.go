package main

import (
	"errors"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// NoisyLoadPerCPU is the one-minute load average per logical CPU above which
// a repeat is flagged noisy. 0.25 means a quarter of the machine is busy with
// something other than this harness before a repeat starts. It is a flag,
// not a filter: flagged repeats stay in the result and the reader decides.
const NoisyLoadPerCPU = 0.25

// darwinLoadavgSize is sizeof(struct loadavg) on 64-bit Darwin: three
// uint32 fixed-point loads, four bytes of padding, one int64 scale.
const darwinLoadavgSize = 24

// LoadAvg is the 1, 5 and 15 minute load average. OK is false when the
// platform did not let it be read, which is not the same as zero.
type LoadAvg struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
	OK      bool    `json:"ok"`
}

// Environment is written with every result, so a figure can be traced to
// the machine, toolchain and load it was taken under.
type Environment struct {
	Date            string   `json:"date"`
	GitSHA          string   `json:"git_sha"`
	GoVersion       string   `json:"go_version"`
	GOOS            string   `json:"goos"`
	GOARCH          string   `json:"goarch"`
	CPUModel        string   `json:"cpu_model"`
	NumCPU          int      `json:"num_cpu"`
	GOMAXPROCS      int      `json:"gomaxprocs"`
	LoadBefore      LoadAvg  `json:"load_before"`
	LoadAfter       LoadAvg  `json:"load_after"`
	NoisyLoadPerCPU float64  `json:"noisy_load_per_cpu"`
	Args            []string `json:"args"`
	// Sidecar is the shell wrapper's view of the machine: uptime and the
	// top CPU consumers before and after, which Go cannot read without
	// os/exec. Home-directory paths are replaced with "~".
	Sidecar string `json:"sidecar,omitempty"`
	// MaxRSSBytes is the process's peak resident set at the end of the run.
	MaxRSSBytes int64 `json:"max_rss_bytes"`
}

// CaptureEnvironment records the toolchain, machine and load. gitSHA comes
// from the caller because `go run` does not always stamp VCS information;
// the build info is the fallback.
func CaptureEnvironment(now func() time.Time, gitSHA string, args []string) Environment {
	if gitSHA == "" {
		gitSHA = buildRevision()
	}
	if args == nil {
		args = []string{}
	}
	return Environment{
		Date:            now().UTC().Format(time.RFC3339),
		GitSHA:          gitSHA,
		GoVersion:       runtime.Version(),
		GOOS:            runtime.GOOS,
		GOARCH:          runtime.GOARCH,
		CPUModel:        cpuModel(),
		NumCPU:          runtime.NumCPU(),
		GOMAXPROCS:      runtime.GOMAXPROCS(0),
		LoadBefore:      loadAverage(),
		NoisyLoadPerCPU: NoisyLoadPerCPU,
		Args:            args,
	}
}

func buildRevision() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return "unknown"
}

// Noisy reports whether a load average is above the threshold for this many
// CPUs. An unreadable load counts as noisy: absence of evidence of load is
// not evidence of a quiet machine.
func Noisy(l LoadAvg, numCPU int) bool {
	if !l.OK || numCPU <= 0 {
		return true
	}
	return l.One/float64(numCPU) > NoisyLoadPerCPU
}

func parseProcLoadavg(s string) (LoadAvg, error) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return LoadAvg{}, errors.New("loadavg: fewer than three fields")
	}
	var v [3]float64
	for i := range v {
		x, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return LoadAvg{}, err
		}
		v[i] = x
	}
	return LoadAvg{One: v[0], Five: v[1], Fifteen: v[2], OK: true}, nil
}

// parseDarwinLoadavg decodes struct loadavg as sysctl vm.loadavg returns it.
// syscall.Sysctl drops one trailing NUL, so a 23-byte value is padded back.
func parseDarwinLoadavg(b []byte) (LoadAvg, error) {
	if len(b) < darwinLoadavgSize-1 || len(b) > darwinLoadavgSize {
		return LoadAvg{}, errors.New("vm.loadavg: unexpected size")
	}
	buf := make([]byte, darwinLoadavgSize)
	copy(buf, b)
	u32 := func(off int) uint32 {
		return uint32(buf[off]) | uint32(buf[off+1])<<8 | uint32(buf[off+2])<<16 | uint32(buf[off+3])<<24
	}
	var scale uint64
	for i := range 8 {
		scale |= uint64(buf[16+i]) << (8 * i)
	}
	if scale == 0 {
		return LoadAvg{}, errors.New("vm.loadavg: zero fscale")
	}
	f := float64(scale)
	return LoadAvg{One: float64(u32(0)) / f, Five: float64(u32(4)) / f, Fifteen: float64(u32(8)) / f, OK: true}, nil
}

// readSidecar reads the shell wrapper's environment notes and replaces the
// home directory with "~", because the result file is committed.
func readSidecar(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := string(b)
	if home, herr := os.UserHomeDir(); herr == nil && home != "" {
		s = strings.ReplaceAll(s, home, "~")
	}
	return s, nil
}
