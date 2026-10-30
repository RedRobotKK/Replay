package main

import "syscall"

func cpuModel() string {
	if s, err := syscall.Sysctl("machdep.cpu.brand_string"); err == nil && s != "" {
		return s
	}
	return "unknown (sysctl machdep.cpu.brand_string unreadable)"
}

func loadAverage() LoadAvg {
	s, err := syscall.Sysctl("vm.loadavg")
	if err != nil {
		return LoadAvg{}
	}
	l, err := parseDarwinLoadavg([]byte(s))
	if err != nil {
		return LoadAvg{}
	}
	return l
}

// maxRSS is the peak resident set. Darwin's ru_maxrss is in bytes.
func maxRSS() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return -1
	}
	return ru.Maxrss
}
