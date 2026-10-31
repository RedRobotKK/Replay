package main

import (
	"os"
	"syscall"
)

// linuxRUsageUnit is the unit of Linux ru_maxrss: kilobytes.
const linuxRUsageUnit = 1024

// The readers below hold no branch of their own: each hands what the kernel
// returned to a helper in env.go, which every platform builds and tests. A
// conditional left here would be scored only on Linux and reported unbuilt
// everywhere else.

func cpuModel() string { return cpuModelFromProc(os.ReadFile("/proc/cpuinfo")) }

func loadAverage() LoadAvg { return loadFromProc(os.ReadFile("/proc/loadavg")) }

func maxRSS() int64 {
	var ru syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return rssBytes(int64(ru.Maxrss), err, linuxRUsageUnit)
}
