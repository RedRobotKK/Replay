package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Phase 5 governance invariants for the tool-tree integrity machinery added
// to this program. Named directly, separate from the feature HARDEN tests in
// tooltree_test.go.

// (a) Nothing in the new tool-tree code path opens the frozen block log for
// writing. Checked two ways: statically, that the new source file never even
// names the historical log's literal filename or a file-write call; and
// behaviorally, that hashing a tree leaves the filesystem it reads
// unchanged.
func TestGovernanceToolTreeCodeNeverWritesTheBlockLog(t *testing.T) {
	b, err := os.ReadFile("tooltree.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, forbidden := range []string{"ttl-blocks-2026-10-05", "os.WriteFile", "os.OpenFile", "os.Create("} {
		if strings.Contains(src, forbidden) {
			t.Errorf("tooltree.go contains %q: the tool-tree hash must be read-only", forbidden)
		}
	}
}

func TestGovernanceToolTreeHashingIsReadOnly(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/a.go", "package a\n")
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := toolTreeSHA256(root); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("hashing the tool tree changed its directory listing: %d entries before, %d after", len(before), len(after))
	}
}

// (c) A tool-tree mismatch is disclosed and stays tied to the tool_commit
// recorded at BLOCK_STARTED: it never becomes an outcome-dependent
// exclusion, and the commit this block's sessions are pinned to is the one
// BLOCK_STARTED named, unaffected by whether the tool tree later changed.
func TestGovernanceToolTreeMismatchStaysTiedToStartToolCommitAndNeverVoidsTheBlock(t *testing.T) {
	p := fixture(t, map[string]any{})
	root := t.TempDir()
	path := root + "/a.go"
	writeFile(t, path, "package a\n")
	p.toolTreeRoots = []string{root}
	s := readSched(t, p)
	start, err := startBlock(p, 1, s.Blocks[0], time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if start.ToolCommit == "" {
		t.Fatal("BLOCK_STARTED must name the tool_commit sessions are pinned to")
	}
	writeFile(t, path, "package a\n\nfunc changed() {}\n")
	end, err := endBlock(p, 1, time.Now())
	if err != nil {
		t.Fatalf("a tool-tree mismatch must not fail or void the block: %v", err)
	}
	if end.ToolTreeIntact == nil || *end.ToolTreeIntact {
		t.Fatal("expected the mismatch to be disclosed as ToolTreeIntact=false")
	}
	// The block itself stays valid by every existing criterion: it is not
	// marked unverified, and its settings span is untouched by this.
	if end.Event != "BLOCK_ENDED" {
		t.Fatalf("event = %q, want BLOCK_ENDED (a tool-tree mismatch does not change the event)", end.Event)
	}
	// Analysis of this block pins to the START record's tool_commit, which
	// a tool-tree mismatch does not alter.
	if end.ToolCommit != start.ToolCommit {
		t.Fatalf("ToolCommit changed between start (%q) and end (%q) in a test run with no commit made; analysis must be able to pin to one value", start.ToolCommit, end.ToolCommit)
	}
}

// (f) The tool-tree hashing logic never inspects a cost, token, or
// cache-behavior value: it reads file bytes and paths only.
func TestGovernanceToolTreeLogicNeverReadsUsageFields(t *testing.T) {
	b, err := os.ReadFile("tooltree.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, word := range []string{"Usage", "CacheRead", "CacheCreation", "PromptTotal", "ephemeral_", "ThinkingTokens", "promptCacheTtl"} {
		if strings.Contains(src, word) {
			t.Errorf("tooltree.go references %q: it must only ever look at file paths and bytes", word)
		}
	}
}
