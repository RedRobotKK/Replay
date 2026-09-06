//go:build unix

package proxy

import "syscall"

// syscallUmask sets the process umask and returns the previous value, so a
// test can prove the socket mode is set explicitly rather than inherited.
func syscallUmask(mask int) int { return syscall.Umask(mask) }
