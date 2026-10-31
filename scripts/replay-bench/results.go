package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ResultSchema versions the JSON. Bump it when a field changes meaning.
const ResultSchema = 1

// Arm names. The direct arm talks to the mock upstream; the proxied arm
// talks to the same upstream through the real proxy handler.
const (
	ArmDirect  = "direct"
	ArmProxied = "proxied"
)

// resultFilePerm keeps result files owner-writable and world-readable; they
// hold measurements, not secrets.
const resultFilePerm = 0o644

// maxResultSuffix bounds the search for a free file name within one second.
const maxResultSuffix = 1000

// createExclusive creates a result file that must not already exist. It is a
// variable so a test can hand WriteResult a file whose write or close fails,
// which a real file on a working disk does not do on demand.
var createExclusive = func(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, resultFilePerm)
}

// Run is one arm of one scenario in one repeat. SamplesNS holds every raw
// per-request duration so any statistic can be recomputed later.
type Run struct {
	Repeat    int           `json:"repeat"`
	Arm       string        `json:"arm"`
	Phase     string        `json:"phase"`
	SamplesNS []int64       `json:"samples_ns,omitempty"`
	TTFBNS    []int64       `json:"ttfb_ns,omitempty"`
	TailNS    []int64       `json:"tail_ns,omitempty"`
	Latency   Summary       `json:"latency"`
	WallNS    int64         `json:"wall_ns,omitempty"`
	Runtime   *RuntimeDelta `json:"runtime,omitempty"`
}

// Scenario is one measured condition. Metrics are per-repeat values reduced
// across repeats; Runs keep the raw material.
type Scenario struct {
	Name    string            `json:"name"`
	Params  map[string]int64  `json:"params,omitempty"`
	Notes   string            `json:"notes,omitempty"`
	Runs    []Run             `json:"runs"`
	Metrics map[string]Spread `json:"metrics"`
	// PerRepeat is the value of each metric in each repeat, in repeat order,
	// so a flagged-noisy repeat can be excluded by a reader.
	PerRepeat map[string][]float64 `json:"per_repeat,omitempty"`
}

// RepeatMeta records the load a repeat started under.
type RepeatMeta struct {
	Index     int     `json:"index"`
	StartedAt string  `json:"started_at"`
	Load      LoadAvg `json:"load"`
	Noisy     bool    `json:"noisy"`
}

// Result is everything one invocation measured.
type Result struct {
	Schema      int          `json:"schema"`
	Noisy       bool         `json:"noisy"`
	NoisyReason string       `json:"noisy_reason,omitempty"`
	Environment Environment  `json:"environment"`
	Config      Config       `json:"config"`
	Repeats     []RepeatMeta `json:"repeats"`
	Scenarios   []Scenario   `json:"scenarios"`
	DurationNS  int64        `json:"duration_ns"`
}

// WriteResult writes r as JSON, plus a CSV of its metrics beside it, under a
// timestamped name that is never reused: a second run in the same second
// gets a numeric suffix rather than replacing the first.
func WriteResult(dir string, now time.Time, r Result) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := now.UTC().Format("20060102T150405Z")
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	for i := range maxResultSuffix {
		name := "replay-bench-" + stamp
		if i > 0 {
			name += fmt.Sprintf("-%d", i)
		}
		jsonPath := filepath.Join(dir, name+".json")
		f, err := createExclusive(jsonPath)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.Write(append(b, '\n'))
		cerr := f.Close()
		if werr != nil {
			return "", werr
		}
		if cerr != nil {
			return "", cerr
		}
		csvPath := filepath.Join(dir, name+".csv")
		if err := os.WriteFile(csvPath, []byte(metricsCSV(r)), resultFilePerm); err != nil {
			return "", err
		}
		return jsonPath, nil
	}
	return "", errors.New("no free result file name in this second")
}

func metricsCSV(r Result) string {
	var b strings.Builder
	b.WriteString("scenario,metric,n,median,min,max,cv\n")
	for _, s := range r.Scenarios {
		for _, k := range sortedKeys(s.Metrics) {
			v := s.Metrics[k]
			fmt.Fprintf(&b, "%s,%s,%d,%s,%s,%s,%s\n", s.Name, k, v.N, num(v.Median), num(v.Min), num(v.Max), num(v.CV))
		}
	}
	return b.String()
}

func sortedKeys(m map[string]Spread) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func num(x float64) string {
	if math.IsNaN(x) {
		return "undefined"
	}
	return fmt.Sprintf("%.3f", x)
}

// Render writes a Markdown summary of r. The noise verdict is in the first
// lines, because a figure read without it is the failure this exists to stop.
func Render(w io.Writer, r Result) error {
	var b strings.Builder
	b.WriteString("# replay-bench summary\n\n")
	if r.Noisy {
		reason := r.NoisyReason
		if reason == "" {
			reason = "load above threshold"
		}
		fmt.Fprintf(&b, "**NOISY: %s.** Do not quote these figures as a clean measurement.\n\n", reason)
	} else {
		fmt.Fprintf(&b, "No repeat was flagged noisy (threshold: 1-minute load above %.2f per CPU).\n\n", NoisyLoadPerCPU)
	}
	e := r.Environment
	fmt.Fprintf(&b, "%s, %s %s/%s, %s, %d CPUs, GOMAXPROCS %d, git %s\n\n",
		e.Date, e.GoVersion, e.GOOS, e.GOARCH, e.CPUModel, e.NumCPU, e.GOMAXPROCS, e.GitSHA)
	for _, s := range r.Scenarios {
		fmt.Fprintf(&b, "## %s\n\n", s.Name)
		if s.Notes != "" {
			fmt.Fprintf(&b, "%s\n\n", s.Notes)
		}
		b.WriteString("| metric | n | median | min | max | CV |\n|---|---|---|---|---|---|\n")
		for _, k := range sortedKeys(s.Metrics) {
			v := s.Metrics[k]
			fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %s |\n", k, v.N, figure(v.Median), figure(v.Min), figure(v.Max), cv(v.CV))
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func figure(x float64) string {
	if math.IsNaN(x) {
		return "undefined"
	}
	if math.Abs(x) >= 100 {
		return fmt.Sprintf("%.0f", x)
	}
	return fmt.Sprintf("%.2f", x)
}

func cv(x float64) string {
	if math.IsNaN(x) {
		return "undefined"
	}
	return fmt.Sprintf("%.3f", x)
}
