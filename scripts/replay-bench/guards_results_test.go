package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime/metrics"
	"strings"
	"testing"
	"time"
)

var resultTime = time.Date(2026, 10, 31, 1, 2, 3, 0, time.UTC)

func TestWriteResultFailsWhenTheDirectoryCannotBeMade(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// The error is the directory's, not a later one from trying to create
	// a file inside a directory that does not exist.
	var pe *fs.PathError
	if p, err := WriteResult(filepath.Join(file, "sub"), resultTime, Result{}); !errors.As(err, &pe) || pe.Op != "mkdir" {
		t.Errorf("WriteResult under a regular file returned %q, %v; want the mkdir failure", p, err)
	}
}

func TestWriteResultRefusesAResultJSONCannotHold(t *testing.T) {
	// PerRepeat is a plain []float64, and a ratio of two zero throughputs
	// is NaN, which JSON cannot hold. The write must fail rather than leave
	// a file behind that claims to be the result.
	dir := t.TempDir()
	r := Result{Scenarios: []Scenario{{Name: "x", PerRepeat: map[string][]float64{"ratio": {math.NaN()}}}}}
	if p, err := WriteResult(dir, resultTime, r); err == nil {
		t.Errorf("a NaN in PerRepeat was written to %q", p)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a refused result left files behind: %v", entries)
	}
}

// scriptedFile is a result file whose write or close fails on demand.
type scriptedFile struct{ writeErr, closeErr error }

func (f scriptedFile) Write(b []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(b), nil
}
func (f scriptedFile) Close() error { return f.closeErr }

func TestWriteResultReportsOpenWriteAndCloseFailures(t *testing.T) {
	for _, c := range []struct {
		name string
		open func(string) (io.WriteCloser, error)
	}{
		{"open", func(string) (io.WriteCloser, error) { return nil, fs.ErrPermission }},
		{"write", func(string) (io.WriteCloser, error) { return scriptedFile{writeErr: errInjected}, nil }},
		{"close", func(string) (io.WriteCloser, error) { return scriptedFile{closeErr: errInjected}, nil }},
	} {
		t.Run(c.name, func(t *testing.T) {
			swap(t, &createExclusive, c.open)
			dir := t.TempDir()
			if p, err := WriteResult(dir, resultTime, Result{}); err == nil {
				t.Errorf("a failed %s returned %q and no error", c.name, p)
			}
			// No CSV is written beside a JSON that did not make it.
			if m, _ := filepath.Glob(filepath.Join(dir, "*.csv")); len(m) != 0 { // Glob errs only on a bad pattern
				t.Errorf("a failed %s still wrote %v", c.name, m)
			}
		})
	}
}

func TestWriteResultFailsWhenTheCSVCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	// A directory where the CSV belongs makes the CSV write fail.
	if err := os.Mkdir(filepath.Join(dir, "replay-bench-20261031T010203Z.csv"), 0o700); err != nil {
		t.Fatal(err)
	}
	if p, err := WriteResult(dir, resultTime, Result{}); err == nil {
		t.Errorf("WriteResult returned %q with no CSV beside it", p)
	}
}

func TestUndefinedStatisticsReadAsUndefinedInTheCSVAndTheSummary(t *testing.T) {
	r := Result{Noisy: true, Scenarios: []Scenario{{
		Name:  "s",
		Notes: "the scenario's caveat",
		Metrics: map[string]Spread{
			"big":       {N: 2, Median: 2500, Min: 100, Max: 99.5, CV: math.NaN()},
			"undefined": {N: 0, Median: math.NaN(), Min: math.NaN(), Max: math.NaN(), CV: math.NaN()},
		},
	}}}
	csv := metricsCSV(r)
	if !strings.Contains(csv, "s,big,2,2500.000,100.000,99.500,undefined\n") {
		t.Errorf("CSV does not write an undefined CV as undefined:\n%s", csv)
	}
	var buf bytes.Buffer
	if err := Render(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		// A figure of 100 or more is whole; below it, two places.
		"| big | 2 | 2500 | 100 | 99.50 | undefined |",
		"| undefined | 0 | undefined | undefined | undefined | undefined |",
		"the scenario's caveat",
		// Noisy with no reason still says why.
		"**NOISY: load above threshold.**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "NaN") || strings.Contains(csv, "NaN") {
		t.Errorf("NaN leaked into the output:\n%s\n%s", out, csv)
	}
}

func TestSummarizeOfNothingIsTheZeroSummary(t *testing.T) {
	if got := Summarize(nil); got != (Summary{}) {
		t.Errorf("Summarize(nil) = %+v", got)
	}
}

func TestSpreadUnmarshalRefusesMalformedJSON(t *testing.T) {
	var s Spread
	if err := s.UnmarshalJSON([]byte(`{"median":"fast"}`)); err == nil {
		t.Errorf("a string median unmarshalled as %+v", s)
	}
}

func TestUint64OfAnUnsupportedMetricIsZero(t *testing.T) {
	// A zero metrics.Value is KindBad, which is what a metric this toolchain
	// does not publish reads as. Uint64() on it panics.
	if got := uint64Of(metrics.Value{}); got != 0 {
		t.Errorf("uint64Of(KindBad) = %d", got)
	}
}

func TestReadSnapshotCarriesTheGCPauseHistogram(t *testing.T) {
	if readSnapshot().pauses == nil {
		t.Error("the GC pause histogram was not read; every pause figure would be zero")
	}
}

func TestPauseDeltaRefusesHistogramsItCannotSubtract(t *testing.T) {
	h := &metrics.Float64Histogram{Counts: []uint64{1}, Buckets: []float64{0, 1}}
	wider := &metrics.Float64Histogram{Counts: []uint64{1, 1}, Buckets: []float64{0, 1, 2}}
	for _, c := range []struct{ before, after *metrics.Float64Histogram }{{nil, h}, {h, nil}, {h, wider}} {
		if n, total := pauseDelta(c.before, c.after); n != 0 || total != 0 {
			t.Errorf("pauseDelta(%v, %v) = %d, %v; want 0, 0", c.before, c.after, n, total)
		}
	}
}

func TestPauseDeltaEstimatesOpenBucketsFromTheirFiniteEdge(t *testing.T) {
	// Buckets (-Inf,1] (1,3] (3,+Inf), one new pause in each. A midpoint of
	// an open bucket is infinite, so the finite edge stands in: 1, 2 and 3
	// seconds, 6e9 ns in all.
	before := &metrics.Float64Histogram{Counts: []uint64{0, 0, 0}, Buckets: []float64{math.Inf(-1), 1, 3, math.Inf(1)}}
	after := &metrics.Float64Histogram{Counts: []uint64{1, 1, 1}, Buckets: before.Buckets}
	n, total := pauseDelta(before, after)
	if n != 3 || total != 6e9 {
		t.Errorf("pauseDelta = %d, %v; want 3, 6e9", n, total)
	}
}

func TestPauseDeltaSkipsEmptyBucketsRatherThanMultiplyingByInfinity(t *testing.T) {
	// A bucket open at both ends has an infinite stand-in midpoint, and
	// 0 * Inf is NaN. A bucket with no new pauses contributes nothing.
	h := &metrics.Float64Histogram{Counts: []uint64{4}, Buckets: []float64{math.Inf(-1), math.Inf(1)}}
	n, total := pauseDelta(h, h)
	if n != 0 || total != 0 {
		t.Errorf("pauseDelta over an unchanged open bucket = %d, %v; want 0, 0", n, total)
	}
}
