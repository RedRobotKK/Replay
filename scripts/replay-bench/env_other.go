//go:build !darwin && !linux

package main

func cpuModel() string { return "unknown (not read on this platform)" }

// loadAverage is unreadable here, so every repeat is flagged noisy.
func loadAverage() LoadAvg { return LoadAvg{} }

func maxRSS() int64 { return -1 }
