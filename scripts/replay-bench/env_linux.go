package main

import (
	"os"
	"strings"
	"syscall"
)

// linuxRUsageUnit is the unit of Linux ru_maxrss: kilobytes.
const linuxRUsageUnit = 1024

func cpuModel() string {
	b, err := os.ReadFile("/proc/cpuinfo")
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

func loadAverage() LoadAvg {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return LoadAvg{}
	}
	l, err := parseProcLoadavg(string(b))
	if err != nil {
		return LoadAvg{}
	}
	return l
}

func maxRSS() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return -1
	}
	return ru.Maxrss * linuxRUsageUnit
}
