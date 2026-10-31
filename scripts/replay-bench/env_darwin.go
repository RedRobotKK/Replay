package main

import "syscall"

// darwinRUsageUnit is the unit of Darwin ru_maxrss: bytes.
const darwinRUsageUnit = 1

// As in env_linux.go, the branches live in env.go's helpers, so they are
// built and tested on every platform rather than only on this one.

func cpuModel() string { return cpuModelFromSysctl(syscall.Sysctl("machdep.cpu.brand_string")) }

func loadAverage() LoadAvg { return loadFromSysctl(syscall.Sysctl("vm.loadavg")) }

// maxRSS is the peak resident set.
func maxRSS() int64 {
	var ru syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return rssBytes(ru.Maxrss, err, darwinRUsageUnit)
}
