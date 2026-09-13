//go:build !windows

package main

// platformRefusal is empty everywhere the ownership promise is kept.
//
// Every platform this builds for other than Windows has Unix mode bits, which
// is what internal/ownerdir checks. There is nothing to refuse.
func platformRefusal() string { return "" }
