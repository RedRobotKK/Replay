package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RedRobotKK/Replay/internal/advisor"
)

// markAdvice records a triage decision against one finding.
//
// The screen let a reader rank findings and do nothing about them, which is a
// report with a cursor. This is the other half: what the reader decided
// outlives the keystroke, in the file `replay advise` already writes and
// already reads back.
//
// It refuses rather than no-ops on an id it cannot find or a file that is not
// there. A silent success is how somebody comes to trust a screen that is
// recording nothing — the failure this repository has spent the day cataloguing
// in other forms.
func markAdvice(id string, status advisor.Status) error {
	path := filepath.Join(tipStateDir(), adviceFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no advice to mark: run `replay advise` first (%w)", err)
	}
	var f adviceFile
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("advice file is not readable by this build: %w", err)
	}
	found := false
	for i := range f.Suggestions {
		if f.Suggestions[i].ID == id {
			f.Suggestions[i].Status = status
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no finding with id %q in %s", id, path)
	}

	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	// Written through a sibling and renamed, like every other file this tool
	// owns: a half-written advice file read on the next keystroke would lose
	// every decision the reader had made, not just the one in flight.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
