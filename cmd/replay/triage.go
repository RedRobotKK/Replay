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

// cacheCoversCorpus reports whether cached advice describes the corpus on disk.
//
// Coverage, not age. Reading advice.json made the advise screen instant and, on
// the machine this was written on, wrong: it showed three findings over one
// transcript while the corpus held 140 over 1,729, because an earlier run
// against a fixture had left the file behind. A timestamp would not have caught
// that — the advice was minutes old and about a different corpus.
//
// Equality is not the test either. The corpus grows while the tool runs; the
// transcript count moved 1,691 to 1,719 in an hour on 2026-09-09. So the
// question is whether the cache saw substantially what is there now.
//
// A cache LARGER than the corpus is also rejected. That means files were
// removed or the reader has pointed at a different directory, and advice about
// transcripts that are gone is advice about somebody else's machine.
func cacheCoversCorpus(cached, onDisk int) bool {
	if onDisk <= 0 || cached <= 0 {
		return false
	}
	if cached > onDisk {
		return false
	}
	// Within a tenth. Wide enough to survive a session's growth, narrow enough
	// that a fixture run against one file cannot pass.
	return float64(cached) >= float64(onDisk)*0.9
}

// appliedIDs reads which findings the reader has marked applied.
//
// The advice file is the only record of a decision anybody actually made. A
// missing or unreadable file means nobody has marked anything, which is
// Pending everywhere rather than an error: not having triaged is the normal
// state, not a fault.
func appliedIDs() map[string]bool {
	b, err := os.ReadFile(filepath.Join(tipStateDir(), adviceFileName))
	if err != nil {
		return nil
	}
	var f adviceFile
	if json.Unmarshal(b, &f) != nil {
		return nil
	}
	out := map[string]bool{}
	for _, s := range f.Suggestions {
		if s.Status == advisor.Applied || s.Status == advisor.Verified ||
			s.Status == advisor.NotVerified {
			out[s.ID] = true
		}
	}
	return out
}
