package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The store registry is a privacy disclosure, and prose counts it.
//
// `replay privacy` walks homeStores() so its output cannot be stale. The
// sentences that describe that output can be, and were: docs/guide/commands.md
// said "Eleven stores sit under ~/.replay" and the header comment in
// stores_test.go listed eleven rows, while the registry held twelve. Both had
// been correct when written. Both stopped being correct on 2026-09-10, when
// contributor-secret and rules.json were registered after SC1 found them by
// reading the source — the registry grew, the prose did not, and nothing went
// red.
//
// An ordinary stale count is untidy. This one is a disclosure that omits two
// stores, one of which is a stable per-machine identifier, answering "what does
// this thing know about me" with a number that is short by two. So the number
// gets an owner: these tests read it out of the prose and compare it to
// len(homeStores()).
//
// Both carry a positive control. The failure mode of a scan is that the text it
// looks for moves, it matches nothing, and "no mismatches found" reads as "all
// consistent" — a check that cannot fail. Each one below fails loudly when its
// pattern matches nothing, and SD2 additionally asserts the list it found still
// contains real rows rather than an empty block.

// storeCountWords covers the range the registry plausibly occupies. A spelling
// outside it fails rather than passing unchecked, because an unrecognised word
// is the same silent pass as an unmatched pattern.
var storeCountWords = map[string]int{
	"eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
	"thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16,
	"seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20,
}

// storeCountWord resolves a spelled-out number, case-insensitively, and fails
// the test rather than returning a zero that would compare as a mismatch with a
// misleading message.
func storeCountWord(t *testing.T, word, where string) int {
	t.Helper()
	n, ok := storeCountWords[strings.ToLower(word)]
	if !ok {
		t.Fatalf("%s spells the store count %q, which is not a number this test knows. "+
			"Add it to storeCountWords rather than deleting the check", where, word)
	}
	return n
}

// SD1: the guide's store count is the registry's store count.
//
// PASS: "<n> stores sit under `~/.replay`" equals len(homeStores()).
// FAIL: the guide tells a reader asking what this tool keeps about them a
// number that omits stores the tool writes.
func TestSD1_TheGuideStoreCountMatchesTheRegistry(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "guide", "commands.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	// Positive control: the sentence must still exist. Without this the guide
	// could stop stating a count entirely and this test would report success.
	re := regexp.MustCompile("(?i)([a-z]+) stores sit under `~/\\.replay`")
	m := re.FindStringSubmatch(string(b))
	if m == nil {
		t.Fatalf("docs/guide/commands.md no longer says \"<n> stores sit under `~/.replay`\", " +
			"so this check is reading nothing. If the sentence was rewritten, point the " +
			"pattern at its replacement; do not delete it, because the count it guards is " +
			"the answer to a subject access request")
	}

	got := storeCountWord(t, m[1], "docs/guide/commands.md")
	if want := len(homeStores()); got != want {
		t.Errorf("docs/guide/commands.md says %s (%d) stores sit under ~/.replay; "+
			"homeStores() registers %d. The guide understates what this tool writes to the "+
			"reader's disk by %d store(s)", m[1], got, want, want-got)
	}
}

// SD2: the disclosure list in stores_test.go is the registry, row for row.
//
// The list is not decoration. It is the only place the stores appear together
// with what each one holds, which is what makes it the thing a reader or a
// reviewer reads instead of the registry. It drifted the same day the guide
// did, and by the same two stores.
//
// PASS: the header comment states the right number, has one row per registered
// store, and names every one of them.
// FAIL: the list describes a tool that writes fewer things than this one does.
func TestSD2_TheDisclosureListMatchesTheRegistry(t *testing.T) {
	const path = "stores_test.go"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := string(b)

	// Positive control 1: the sentence introducing the list must still exist.
	re := regexp.MustCompile(`(?i)the tool writes ([a-z]+) things:`)
	m := re.FindStringSubmatchIndex(text)
	if m == nil {
		t.Fatalf("%s no longer says \"the tool writes <n> things:\", so this check is "+
			"reading nothing. The list below that sentence is the disclosure; if it moved, "+
			"move the pattern with it", path)
	}
	got := storeCountWord(t, text[m[2]:m[3]], path)

	// The rows are the tab-indented comment lines that follow, up to the first
	// unindented comment line. A continuation line starts with whitespace after
	// the tab and is not a row of its own.
	var rows []string
	// TrimLeft drops the newline the match ends on, so the first element below
	// is the "//" line rather than an empty string the loop would break on.
	block := strings.TrimLeft(text[m[1]:], " \t\r\n")
	for _, line := range strings.Split(block, "\n") {
		if !strings.HasPrefix(line, "//") {
			break
		}
		body := strings.TrimPrefix(line, "//")
		if !strings.HasPrefix(body, "\t") {
			if strings.TrimSpace(body) == "" {
				continue // the blank comment line between sentence and list
			}
			break
		}
		row := strings.TrimPrefix(body, "\t")
		if strings.TrimSpace(row) == "" || row[0] == ' ' {
			continue // a continuation of the row above
		}
		rows = append(rows, strings.Fields(row)[0])
	}

	// Positive control 2: an empty or near-empty block means the parse above
	// stopped matching the comment's shape, not that the tool stopped writing.
	if len(rows) < 8 {
		t.Fatalf("parsed %d rows out of the disclosure list in %s; it held eleven when this "+
			"check was written, so the parser has drifted from the comment's shape and is "+
			"not reading the list it thinks it is: %v", len(rows), path, rows)
	}

	stores := homeStores()
	if got != len(stores) {
		t.Errorf("%s says the tool writes %d things; homeStores() registers %d",
			path, got, len(stores))
	}
	if len(rows) != len(stores) {
		t.Errorf("the disclosure list in %s has %d rows for %d registered stores: %v",
			path, len(rows), len(stores), rows)
	}
	for _, s := range stores {
		if !strings.Contains(block, s.Name) {
			t.Errorf("%q is registered and the disclosure list in %s does not name it. The "+
				"list is where a reviewer reads what this tool keeps; a store missing from "+
				"it is undisclosed to everyone who reads the list instead of the registry",
				s.Name, path)
		}
	}
}
