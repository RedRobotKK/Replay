package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every surface is integrated the whole way, or the suite says which half is
// missing.
//
// This build describes its agent surfaces in TWO places, and they drifted
// within a day of each other. `knownStores` in discover.go drives
// `replay doctor` and carries the next step a reader should take.
// `knownSurfaces` in othersurfaces.go drives the note under `replay cost` and
// carries either the command that reads a surface or a `why` explaining that
// none does. On 2026-09-17 `replay grok` shipped, discover.go was updated to
// name it, and othersurfaces.go went on saying "Replay cannot read Grok yet",
// which is a sentence the tool prints to a user whose data it had just read.
//
// A surface is not integrated because someone wrote a parser. It is integrated
// when the reader can find it: detected on disk, readable by a command that
// actually dispatches, and written down where a person looks. Half of that is
// worse than none, because a named surface with a stale refusal tells the
// reader a false thing in the tool's own voice.
//
// The checks below are deliberately about AGREEMENT and REACHABILITY rather
// than wording. A test that pinned sentences would fail on every edit and be
// deleted within a week.

// surfaceCommand pulls the bare command word out of a `next` or `cmd` string.
// Both carry prose around it ("replay grok  (tokens only: ...)"), and the word
// is what has to be dispatched.
func surfaceCommand(s string) string {
	m := regexp.MustCompile(`replay ([a-z]+)`).FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// TestSI1: the two registries agree about whether a surface can be read.
//
// This is the check that would have caught the Grok drift the hour it
// happened.
func TestSI1_BothRegistriesAgreeOnWhetherASurfaceHasAReader(t *testing.T) {
	home := t.TempDir()
	stores := map[string]agentStore{}
	for _, s := range knownStores(home) {
		stores[s.name] = s
	}
	for _, s := range knownSurfaces(home) {
		store, ok := stores[s.name]
		if !ok {
			continue // discover.go need not list every surface cost mentions
		}
		discoverReads := surfaceCommand(store.next) != ""
		surfaceReads := surfaceCommand(s.cmd) != ""
		if discoverReads != surfaceReads {
			t.Errorf("%s: discover.go %s, othersurfaces.go %s.\n"+
				"  discover next: %q\n  surfaces cmd:  %q\n  surfaces why:  %q\n"+
				"One of the two is telling the reader something the other contradicts.",
				s.name,
				map[bool]string{true: "names a reader", false: "names none"}[discoverReads],
				map[bool]string{true: "names a reader", false: "names none"}[surfaceReads],
				store.next, s.cmd, s.why)
		}
	}
}

// TestSI2: a surface that claims a reader names one the binary answers to.
//
// TestDS7 already asks this of discover.go. It is asked here of the other
// registry too, because that is the one that went stale.
func TestSI2_EverySurfaceReaderIsDispatched(t *testing.T) {
	// command_table_test.go owns this helper; it reads the switch in main.go,
	// which is the only authority on what the binary answers to.
	dispatched := map[string]bool{}
	for _, c := range dispatchedCommands(t) {
		dispatched[c] = true
	}
	for _, s := range knownSurfaces(t.TempDir()) {
		cmd := surfaceCommand(s.cmd)
		if cmd == "" {
			continue
		}
		if !dispatched[cmd] {
			t.Errorf("%s names `replay %s`, which main.go does not dispatch", s.name, cmd)
		}
	}
}

// TestSI3: a surface with no reader says why, and a surface with one does not.
//
// The `why` string is the whole value of naming an unreadable surface: it tells
// the reader their data was seen and what specifically is missing. A surface
// that has gained a reader and kept its `why` is the stale-refusal case.
func TestSI3_UnreadableSurfacesExplainAndReadableOnesDoNot(t *testing.T) {
	for _, s := range knownSurfaces(t.TempDir()) {
		hasReader := surfaceCommand(s.cmd) != ""
		switch {
		case !hasReader && strings.TrimSpace(s.why) == "":
			t.Errorf("%s has no reader and no reason; a named surface with neither tells the reader nothing they can act on", s.name)
		case hasReader && strings.Contains(strings.ToLower(s.why), "cannot read"):
			t.Errorf("%s is read by %q and still carries a cannot-read reason:\n  %s",
				s.name, s.cmd, s.why)
		}
	}
}

// TestSI4: every surface is written down where a person looks.
//
// A surface the binary detects and the documentation never names is one the
// reader finds by accident. Both files are checked because they answer
// different questions: the README is what a person reads before installing,
// the guide is what they read when a command surprised them.
func TestSI4_EverySurfaceIsDocumented(t *testing.T) {
	docs := map[string]string{
		"README.md":              filepath.Join("..", "..", "README.md"),
		"docs/guide/commands.md": filepath.Join("..", "..", "docs", "guide", "commands.md"),
	}
	text := map[string]string{}
	for label, path := range docs {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", label, err)
		}
		text[label] = strings.ToLower(string(b))
	}
	for _, s := range knownSurfaces(t.TempDir()) {
		name := strings.ToLower(s.name)
		for label := range docs {
			if !strings.Contains(text[label], name) {
				t.Errorf("%s: detected by the binary and never named in %s", s.name, label)
			}
		}
	}
}
