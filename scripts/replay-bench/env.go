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

// readBuildInfo is debug.ReadBuildInfo, a variable so a test can hand
// buildRevision a binary that was, or was not, stamped with VCS information:
// `go test` never stamps it, so the real one cannot reach either branch.
var readBuildInfo = debug.ReadBuildInfo

func buildRevision() string {
	if bi, ok := readBuildInfo(); ok {
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

// cpuModelFromProc reads the model name out of /proc/cpuinfo as os.ReadFile
// returned it.
func cpuModelFromProc(b []byte, err error) string {
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := strings.Cut(line, ":")
			if ok && strings.TrimSpace(k) == "model name" {
				return strings.TrimSpace(v)
			}
		}
	}
	return "unknown (/proc/cpuinfo has no model name)"
}

// loadFromProc decodes /proc/loadavg as os.ReadFile returned it. A read that
// failed is unreadable even if it returned bytes: os.ReadFile hands back what
// it read before the error.
//
// The parse error needs no branch of its own. Every error return of
// parseProcLoadavg is the zero LoadAvg, OK false, which is exactly how this
// reports "unreadable"; TestParseProcLoadavg pins that, so the error is
// dropped here for that reason and no other.
func loadFromProc(b []byte, err error) LoadAvg {
	if err != nil {
		return LoadAvg{}
	}
	l, _ := parseProcLoadavg(string(b)) // zero LoadAvg (OK false) on error; see above
	return l
}

// cpuModelFromSysctl is Darwin's machdep.cpu.brand_string, or says it is
// unknown.
func cpuModelFromSysctl(s string, err error) string {
	if err == nil && s != "" {
		return s
	}
	return "unknown (sysctl machdep.cpu.brand_string unreadable)"
}

// loadFromSysctl decodes Darwin's vm.loadavg as syscall.Sysctl returned it.
// As in loadFromProc, a parse error is already the zero LoadAvg.
func loadFromSysctl(s string, err error) LoadAvg {
	if err != nil {
		return LoadAvg{}
	}
	l, _ := parseDarwinLoadavg([]byte(s)) // zero LoadAvg (OK false) on error; TestParseDarwinLoadavg pins it
	return l
}

// rssBytes converts ru_maxrss to bytes, or -1 when getrusage failed: a peak
// that could not be read is not a peak of zero.
func rssBytes(maxrss int64, err error, unit int64) int64 {
	if err != nil {
		return -1
	}
	return maxrss * unit
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
