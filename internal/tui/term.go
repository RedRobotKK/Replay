package tui

import "errors"

// Per-key input is not available in this build, and the reason is a boundary
// worth more than the convenience.
//
// Raw mode needs a termios ioctl, which needs unsafe.Pointer to hand the struct
// to syscall.Syscall. TestX402_NoSigningCapability keeps unsafe out of the
// shipped binary's import tree, because a tool that sits in front of your API
// traffic holding a token should not be able to reach past the type system.
// Shelling out to stty is blocked by the same family of guard, which keeps
// os/exec out.
//
// The alternatives were weighed and none is mine to take:
//
//	allowlist unsafe        widens a security boundary for a UI feature
//	add golang.org/x/term   ends a dependency-free go.mod, which is a real
//	                        claim for a binary holding a credential
//	drop per-key input      what this does
//
// So keys need Enter. That is a worse surface and an honest one, and the loop
// is written so swapping this out later changes nothing above it: readKeys
// already handles both, and its raw path is tested.
type termState struct{}

var errNoRawMode = errors.New(
	"per-key input needs a termios ioctl, which needs unsafe, which this binary " +
		"deliberately cannot import")

func rawMode() (*termState, error) { return nil, errNoRawMode }

func (t *termState) restore() {}
