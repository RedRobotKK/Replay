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

func TestResultFilesAreTimestampedAndNeverOverwritten(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 30, 14, 23, 5, 0, time.UTC)
	r := Result{Schema: ResultSchema}

	first, err := WriteResult(dir, now, r)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first) != "replay-bench-20261030T142305Z.json" {
		t.Errorf("first name = %s", filepath.Base(first))
	}
	// Same second, second run: a new file, not a replacement.
	second, err := WriteResult(dir, now, r)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatalf("second write reused %s", first)
	}
	for _, p := range []string{first, second} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s missing: %v", p, err)
		}
		csv := strings.TrimSuffix(p, ".json") + ".csv"
		if _, err := os.Stat(csv); err != nil {
			t.Errorf("CSV beside %s missing: %v", p, err)
		}
	}
}

func TestResultRoundTripsThroughJSON(t *testing.T) {
	r := Result{
		Schema: ResultSchema,
		Noisy:  true,
		Scenarios: []Scenario{{
			Name: "latency/45KB/mask-off",
			Runs: []Run{{Repeat: 0, Arm: ArmDirect, SamplesNS: []int64{1, 2, 3}}},
		}},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back Result
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Noisy || len(back.Scenarios) != 1 || len(back.Scenarios[0].Runs[0].SamplesNS) != 3 {
		t.Errorf("round trip lost data: %+v", back)
	}
}

func TestRenderFlagsANoisyResultInItsFirstLines(t *testing.T) {
	r := Result{
		Schema:      ResultSchema,
		Noisy:       true,
		NoisyReason: "noisy — concurrent workloads present",
		Environment: Environment{GoVersion: "go1.27.3", NumCPU: 8},
		Scenarios: []Scenario{{
			Name:    "latency/45KB/mask-off",
			Metrics: map[string]Spread{"overhead_paired_p50_us": {N: 10, Median: 2500, Min: 2000, Max: 3000, CV: 0.1}},
		}},
	}
	var buf bytes.Buffer
	if err := Render(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(out, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	head := strings.Join(lines, "\n")
	if !strings.Contains(head, "NOISY") {
		t.Errorf("noise flag not in the first lines:\n%s", head)
	}
	for _, want := range []string{"latency/45KB/mask-off", "overhead_paired_p50_us", "2500", "2000", "3000", "0.100"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSaysQuietOnlyWhenNothingWasFlagged(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, Result{Schema: ResultSchema}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "NOISY") {
		t.Errorf("a clean result rendered as noisy:\n%s", buf.String())
	}
}

func TestUndefinedStatisticsRoundTripAsNullNotASentinel(t *testing.T) {
	// A sentinel such as -1 could be a real overhead in microseconds. JSON
	// has no NaN, so undefined is null on the wire and NaN in memory.
	b, err := json.Marshal(SpreadOf([]float64{7}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"cv":null`) {
		t.Errorf("undefined CV not written as null: %s", b)
	}
	var back Spread
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Median != 7 || back.CV == back.CV { // NaN != NaN
		t.Errorf("round trip = %+v", back)
	}
}

func TestProxyCloseIsIdempotent(t *testing.T) {
	// A second Close once waited thirty seconds for an exit it had already
	// received, which made every session repeat cost half a minute.
	up := newUpstream()
	defer up.Close()
	px, err := startProxy(up.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := px.Close(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := px.Close(); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el > time.Second {
		t.Errorf("second Close took %v", el)
	}
}

func TestFortyFiveKilobyteBodyIsTheExistingBenchmarkBody(t *testing.T) {
	// internal/proxy's benchRequest is 44,981 bytes. The harness must send
	// the same request, or its figure is not comparable to the benchmark's.
	if n := len(requestBody(size45KB, false)); n != 44981 {
		t.Errorf("45KB body is %d bytes, want 44981", n)
	}
	if n := len(requestBody(size1MB, false)); n < size1MB {
		t.Errorf("1MB body is %d bytes", n)
	}
}
