package main

import (
	"os"
	"testing"
)

// TestMain points HOME and USERPROFILE at a temporary directory, as
// internal/regression's TestHI1 requires of every package whose tests can
// resolve a home directory: readSidecar calls os.UserHomeDir, and a test must
// never be able to reach the reader's own files. cmd/replay/home_test.go is
// the worked example this follows.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "replay-bench-home-")
	if err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	if err := os.Setenv("HOME", dir); err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	if err := os.Setenv("USERPROFILE", dir); err != nil {
		panic("isolating USERPROFILE for the test suite: " + err.Error())
	}
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		code = 1
	}
	os.Exit(code)
}
