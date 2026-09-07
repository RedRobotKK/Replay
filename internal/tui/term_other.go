//go:build !unix

package tui

import "errors"

type termState struct{}

// rawMode is unavailable here and says so rather than pretending.
//
// The caller falls back to line mode, which needs Enter after each key and
// works everywhere. Windows has its own console API for this and no
// implementation until somebody can test it on Windows, which is the same
// standard the rest of this project holds itself to.
func rawMode() (*termState, error) {
	return nil, errors.New("per-key input needs a terminal API this build does not use")
}

func (t *termState) restore() {}
