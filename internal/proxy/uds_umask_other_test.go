//go:build !unix

package proxy

// syscallUmask is a stub on platforms without a process umask.
//
// The test that calls it skips via requireUnix before reaching this, but the
// call still has to compile: uds_test.go carries no build tag, so `go vet` on
// Windows type-checks it against a helper that only existed in a unix-tagged
// file. That broke the Windows job on every push, and a permanently red job is
// one nobody reads.
func syscallUmask(int) int { return 0 }
