package main

import (
	"io/fs"
	"os"
	"path/filepath"
)

// defaultTranscriptRoots returns the transcript directory to use when the
// caller named none, or nothing when there is no usable one.
//
// The binary already located this in `replay doctor` a second earlier. Making
// every value command demand it again as an argument is the funnel dying at
// step one: `curl | sh` then `replay cost` returned "one or more transcript
// directories are required" to a person who had months of transcripts sitting
// on disk before they installed anything.
//
// It returns nothing rather than a guess when the directory is missing or
// empty, so the caller still prints its usage error. A path that does not exist
// turns a clear "tell me where" into an obscure "no such file", and an empty
// one produces a report about nothing, which reads as "this tool is broken"
// rather than "there is nothing here yet".
func defaultTranscriptRoots(home string) []string {
	if home == "" {
		return nil
	}
	root := filepath.Join(claudeConfigDir(home), "projects")
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return nil
	}
	found := false
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".jsonl" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if !found {
		return nil
	}
	return []string{root}
}
