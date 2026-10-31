package main

import (
	"math"
	"runtime/metrics"
)

// Runtime metric names, read through runtime/metrics so the harness measures
// what the Go runtime itself counts rather than a wall-clock proxy for it.
const (
	metricAllocBytes   = "/gc/heap/allocs:bytes"
	metricAllocObjects = "/gc/heap/allocs:objects"
	metricGCCycles     = "/gc/cycles/total:gc-cycles"
	metricGCPauses     = "/sched/pauses/total/gc:seconds"
	metricGoroutines   = "/sched/goroutines:goroutines"
	metricTotalMemory  = "/memory/classes/total:bytes"
)

var metricNames = []string{metricAllocBytes, metricAllocObjects, metricGCCycles, metricGCPauses, metricGoroutines, metricTotalMemory}

// snapshot is one read of the runtime counters.
type snapshot struct {
	allocBytes, allocObjects, gcCycles, totalMemory uint64
	goroutines                                      uint64
	pauses                                          *metrics.Float64Histogram
}

func readSnapshot() snapshot {
	samples := make([]metrics.Sample, len(metricNames))
	for i, n := range metricNames {
		samples[i].Name = n
	}
	metrics.Read(samples)
	var s snapshot
	for _, m := range samples {
		switch m.Name {
		case metricAllocBytes:
			s.allocBytes = uint64Of(m.Value)
		case metricAllocObjects:
			s.allocObjects = uint64Of(m.Value)
		case metricGCCycles:
			s.gcCycles = uint64Of(m.Value)
		case metricGoroutines:
			s.goroutines = uint64Of(m.Value)
		case metricTotalMemory:
			s.totalMemory = uint64Of(m.Value)
		case metricGCPauses:
			if m.Value.Kind() == metrics.KindFloat64Histogram {
				s.pauses = m.Value.Float64Histogram()
			}
		}
	}
	return s
}

// uint64Of reads a counter, treating a metric this toolchain does not
// support (KindBad) as zero rather than panicking.
func uint64Of(v metrics.Value) uint64 {
	if v.Kind() != metrics.KindUint64 {
		return 0
	}
	return v.Uint64()
}

// RuntimeDelta is what the whole process — client, mock upstream and proxy,
// which share it — did across a batch of requests. The proxy's share is the
// proxied arm minus the direct arm, because the direct arm runs the same
// client and upstream without the proxy.
type RuntimeDelta struct {
	Ops               int     `json:"ops"`
	BytesPerOp        float64 `json:"bytes_per_op"`
	AllocsPerOp       float64 `json:"allocs_per_op"`
	GCCycles          uint64  `json:"gc_cycles"`
	GCPauses          uint64  `json:"gc_pauses"`
	GCPauseTotalEstNS float64 `json:"gc_pause_total_est_ns"`
	GoroutinesAfter   uint64  `json:"goroutines_after"`
	GoroutinesPeak    int     `json:"goroutines_peak,omitempty"`
	TotalMemoryBytes  uint64  `json:"total_memory_bytes"`
}

func delta(before, after snapshot, ops int) RuntimeDelta {
	d := RuntimeDelta{
		Ops:              ops,
		GCCycles:         after.gcCycles - before.gcCycles,
		GoroutinesAfter:  after.goroutines,
		TotalMemoryBytes: after.totalMemory,
	}
	if ops > 0 {
		d.BytesPerOp = float64(after.allocBytes-before.allocBytes) / float64(ops)
		d.AllocsPerOp = float64(after.allocObjects-before.allocObjects) / float64(ops)
	}
	d.GCPauses, d.GCPauseTotalEstNS = pauseDelta(before.pauses, after.pauses)
	return d
}

// pauseDelta counts the stop-the-world GC pauses between two histogram reads
// and estimates their total from bucket midpoints. It is an estimate: the
// runtime publishes pauses only as a histogram.
func pauseDelta(before, after *metrics.Float64Histogram) (uint64, float64) {
	if before == nil || after == nil || len(before.Counts) != len(after.Counts) {
		return 0, 0
	}
	var n uint64
	var total float64
	for i := range after.Counts {
		c := after.Counts[i] - before.Counts[i]
		if c == 0 {
			continue
		}
		lo, hi := after.Buckets[i], after.Buckets[i+1]
		mid := (lo + hi) / 2
		switch {
		case math.IsInf(lo, -1):
			mid = hi
		case math.IsInf(hi, 1):
			mid = lo
		}
		n += c
		total += float64(c) * mid * float64(1e9)
	}
	return n, total
}
