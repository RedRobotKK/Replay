package guardcheck

import (
	"strings"
	"testing"
)

// ParseDiff decides what the reviewer looks at, so everything it drops is a
// guard nobody checks. Every branch below drops something.

// GC8: the lines a hunk names are the lines that get analysed.
func TestGC8_HunkLinesAreCollected(t *testing.T) {
	got := ParseDiff(`diff --git a/pkg/a.go b/pkg/a.go
--- a/pkg/a.go
+++ b/pkg/a.go
@@ -10,0 +11,3 @@
+	if x {
+		return
+	}
`)
	lines, ok := got["pkg/a.go"]
	if !ok {
		t.Fatalf("the changed file was not collected: %v", got)
	}
	for _, want := range []int{11, 12, 13} {
		if !lines[want] {
			t.Errorf("line %d was in the hunk and is not in the set: %v", want, lines)
		}
	}
	if lines[14] {
		t.Errorf("line 14 is past the hunk and was collected anyway: %v", lines)
	}
}

// GC9: a single-line hunk has no comma, and still covers its one line.
//
// `@@ -1 +5 @@` is the shape git emits for a one-line change. Read as
// "start=5, count missing", it would cover nothing.
func TestGC9_ASingleLineHunkCoversItsLine(t *testing.T) {
	got := ParseDiff("+++ b/pkg/a.go\n@@ -1 +5 @@\n+\tif x {\n")
	if !got["pkg/a.go"][5] {
		t.Errorf("a one-line hunk covered no lines: %v", got["pkg/a.go"])
	}
}

// GC10: test files and non-Go files are not analysed.
//
// A guard inside a _test.go file is test code; neutralising it measures the
// test, not the thing tested. And a changed .md file has no conditionals at
// all — collecting it sends the tool to parse Markdown as Go.
func TestGC10_TestAndNonGoFilesAreDropped(t *testing.T) {
	got := ParseDiff(`+++ b/pkg/a_test.go
@@ -0,0 +1,2 @@
+++ b/README.md
@@ -0,0 +1,2 @@
+++ b/pkg/a.go
@@ -0,0 +1,2 @@
`)
	for _, dropped := range []string{"pkg/a_test.go", "README.md"} {
		if _, ok := got[dropped]; ok {
			t.Errorf("%s was collected for analysis", dropped)
		}
	}
	if _, ok := got["pkg/a.go"]; !ok {
		t.Errorf("the real source file was dropped along with them: %v", got)
	}
}

// GC11: a hunk header the parser cannot read is skipped, not guessed.
//
// A malformed or unexpected header must not become line 0, or a bogus set of
// lines: the reviewer would analyse a region the diff never touched and
// report guards the change did not introduce.
func TestGC11_AnUnreadableHunkIsSkipped(t *testing.T) {
	for _, bad := range []string{
		"+++ b/pkg/a.go\n@@ -1\n",         // too few fields
		"+++ b/pkg/a.go\n@@ -1 +x @@\n",   // start is not a number
		"+++ b/pkg/a.go\n@@ -1 +5,y @@\n", // count is not a number
	} {
		got := ParseDiff(bad)
		if lines, ok := got["pkg/a.go"]; ok && len(lines) > 0 {
			t.Errorf("a header the parser cannot read produced lines %v\ninput: %q",
				lines, bad)
		}
	}
}

// GC12: hunks before any +++ header belong to no file and are dropped.
//
// git emits the header first, but a truncated or concatenated diff may not.
// Attributing those lines to whatever file came last would analyse the wrong
// source entirely.
func TestGC12_AHunkWithNoFileIsDropped(t *testing.T) {
	if got := ParseDiff("@@ -1 +5 @@\n+\tif x {\n"); len(got) != 0 {
		t.Errorf("a hunk with no file header was attributed to something: %v", got)
	}
}

// GC13: an empty diff is an empty set, not an error and not a nil map.
//
// The reviewer ranges over the result. A nil map ranges fine; the guard here
// is that the empty case is distinguishable from "everything changed".
func TestGC13_AnEmptyDiffCollectsNothing(t *testing.T) {
	if got := ParseDiff(""); len(got) != 0 {
		t.Errorf("an empty diff collected %v", got)
	}
}

// GC14: a diff far larger than one scanner buffer is read whole.
//
// bufio.Scanner refuses a token past its buffer and stops. A silently
// truncated diff means the guards below the cut are never analysed, and the
// tool reports a clean run over a change it only half read.
func TestGC14_ALongLineDoesNotTruncateTheDiff(t *testing.T) {
	huge := "+++ b/pkg/big.go\n@@ -1 +1 @@\n+" + strings.Repeat("x", 200_000) + "\n" +
		"+++ b/pkg/after.go\n@@ -1 +9 @@\n+\tif x {\n"
	got := ParseDiff(huge)
	if !got["pkg/after.go"][9] {
		t.Errorf("the file after a very long line was not collected, so the diff was "+
			"read only up to the cut: %v", got)
	}
}
