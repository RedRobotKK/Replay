package guardcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GC15: a file excluded from every build is skipped, with the reason.
//
// Found by running the reviewer on the pull request that added it: it analysed
// its own source, found 28 conditionals, and tried to test a package carrying
// //go:build ignore. The toolchain reports "build constraints exclude all Go
// files", the baseline is red, and ADR-0014 refuses to run against a red
// baseline — so the refusal worked, but the analysis should never have got
// that far.
func TestGC15_BuildExcludedFilesAreSkipped(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		excluded bool
	}{
		{"ignore tag", "//go:build ignore\n\npackage main\n", true},
		{"unset tag", "//go:build mutation\n\npackage p\n", true},
		{"ordinary file", "package p\n", false},
		{"negated tag is satisfied", "//go:build !windows\n\npackage p\n", false},
		// A constraint below the package clause is not a build constraint.
		// Treating it as one would exclude a file that ships.
		{"constraint after package", "package p\n\n//go:build ignore\n", false},
		// A comment above the constraint is the ordinary shape of a file
		// carrying a licence header or a doc comment. The scan has to walk
		// past lines that are not constraints to reach the one that is —
		// stopping at the first line that will not parse would let an
		// excluded file through as if it shipped.
		{"comment before the constraint",
			"// Package p does a thing.\n//go:build ignore\n\npackage p\n", true},
		// A //go:build line that is not valid syntax is not a constraint
		// anyone can evaluate. Excluding on it would drop a file from the
		// analysis on the strength of a typo.
		{"malformed constraint", "//go:build ((\n\npackage p\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "f.go")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			got, why := ExcludedFromBuild(path)
			if got != tc.excluded {
				t.Errorf("excluded = %v, want %v (%q)", got, tc.excluded, tc.body)
			}
			if got && why == "" {
				t.Error("a file was skipped with no reason given, so the run says a " +
					"file vanished and not why")
			}
		})
	}
}

// GC16: a file that cannot be read is not reported as excluded.
//
// Excluding on a read error would silently drop a real source file from the
// analysis, which is the failure mode this whole tool exists to prevent.
func TestGC16_AnUnreadableFileIsNotTreatedAsExcluded(t *testing.T) {
	if excluded, _ := ExcludedFromBuild(filepath.Join(t.TempDir(), "absent.go")); excluded {
		t.Error("a file that could not be opened was reported as build-excluded, " +
			"which drops it from the analysis without saying so")
	}
}

// GC17: only conditionals on changed lines are collected.
//
// The reviewer is diff-scoped so it costs seconds rather than the eleven
// minutes the full catalogue needs. A collector that ignored the line set
// would put every guard in the file to the suite on every run.
func TestGC17_OnlyChangedLinesAreCollected(t *testing.T) {
	src := `package p

func g(a, b bool) int {
	if a {
		return 1
	}
	if b {
		return 2
	}
	return 0
}
`
	path := filepath.Join(t.TempDir(), "s.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	gs, err := Conditionals(path, map[int]bool{7: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 1 {
		t.Fatalf("expected only the guard on line 7, got %d: %+v", len(gs), gs)
	}
	if gs[0].Line != 7 {
		t.Errorf("collected line %d, want 7", gs[0].Line)
	}
}

// GC18: a file that is not Go is an error, not an empty result.
//
// Returning no conditionals for an unparseable file would report a clean run
// over a change the tool could not read.
func TestGC18_AnUnparseableFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.go")
	if err := os.WriteFile(path, []byte("this is not go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Conditionals(path, map[int]bool{1: true}); err == nil {
		t.Error("a file that is not Go produced no error, so an unreadable change " +
			"would be reported as having no guards")
	}
}

// GC19: Neutralise refuses offsets that do not fit the file.
//
// A Guard can outlive the file it was read from — the reviewer restores
// between mutations, and a bad restore or a concurrent edit would leave the
// offsets pointing into the middle of something else. Splicing there would
// write corrupted source into somebody's working tree.
func TestGC19_ImpossibleOffsetsAreRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.go")
	body := "package p\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, g := range []Guard{
		{File: path, CondStart: 0, CondEnd: 5},
		{File: path, CondStart: 3, CondEnd: len(body) + 100},
		{File: path, CondStart: 5, CondEnd: 5},
	} {
		if _, err := Neutralise(g); err == nil {
			t.Errorf("offsets [%d,%d) over a %d-byte file were accepted",
				g.CondStart, g.CondEnd, len(body))
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Errorf("a refused mutation still wrote to the file:\n%s", after)
	}
}

// GC20: a file that has gone is an error, not a silent pass.
func TestGC20_AMissingFileIsAnError(t *testing.T) {
	g := Guard{File: filepath.Join(t.TempDir(), "gone.go"), CondStart: 1, CondEnd: 2}
	_, err := Neutralise(g)
	if err == nil {
		t.Fatal("neutralising a file that does not exist reported success")
	}
	// The message is the point. Without the read guard the offset check below
	// it catches the same case and reports "offsets do not fit a 0-byte file"
	// — true, and the wrong diagnosis to hand someone.
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("a missing file was diagnosed as something else: %v", err)
	}
}

// GC21: an already-neutralised condition is refused rather than nested.
//
// Nesting would produce `false && (false && (cond))`, which is harmless, but
// the refusal is what catches the reviewer running twice over the same tree
// without restoring — and that leaves a repository quietly modified.
func TestGC21_ADoubleNeutralisationIsRefused(t *testing.T) {
	src := "package p\n\nfunc g(a bool) int {\n\tif a {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	path := filepath.Join(t.TempDir(), "s.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	gs, err := Conditionals(path, map[int]bool{4: true})
	if err != nil || len(gs) != 1 {
		t.Fatalf("Conditionals: %v (%d)", err, len(gs))
	}
	if _, err := Neutralise(gs[0]); err != nil {
		t.Fatal(err)
	}
	// The file on disk is now mutated; the same Guard describes a condition
	// that already reads `false && (a)`.
	again, err := Conditionals(path, map[int]bool{4: true})
	if err != nil || len(again) != 1 {
		t.Fatalf("re-reading the mutant: %v (%d)", err, len(again))
	}
	if _, err := Neutralise(again[0]); err == nil {
		t.Error("an already-neutralised condition was neutralised again")
	}
}

// GC22: a write that fails leaves the mutation reported, not assumed.
//
// The reviewer writes a mutant into somebody's working tree. If that write
// fails and the failure is swallowed, it runs the suite against unmutated
// code, sees green, and reports SURVIVED for a guard it never disabled —
// the exact false green the whole tool exists to prevent, produced by the
// tool itself.
//
// The refusal has to come from the filesystem, and not every filesystem will
// produce it: root ignores the permission bits, and Windows does not carry
// them in this form. Where the refusal cannot be produced the test skips and
// says so, rather than asserting on a failure that never happened.
func TestGC22_AFailedWriteIsReported(t *testing.T) {
	src := "package p\n\nfunc g(a bool) int {\n\tif a {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "s.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	gs, err := Conditionals(path, map[int]bool{4: true})
	if err != nil || len(gs) != 1 {
		t.Fatalf("Conditionals: %v (%d)", err, len(gs))
	}

	// Read-only file: the write must fail, the read must still succeed.
	if err := os.Chmod(path, 0o400); err != nil {
		t.Skipf("cannot make the file read-only here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if f, err := os.OpenFile(path, os.O_WRONLY, 0o400); err == nil {
		_ = f.Close()
		t.Skip("this filesystem writes to a read-only file, so the refusal this " +
			"test needs cannot be produced here")
	}

	if _, err := Neutralise(gs[0]); err == nil {
		t.Error("a write that the filesystem refused was reported as a successful " +
			"mutation, so the suite would run against unmutated code and every " +
			"verdict from that run would be a false green")
	}
}

// GC23: a source file that cannot be read is an error, not an empty analysis.
//
// Conditionals returning no guards for a file it could not open would report a
// clean run over a change nobody looked at — a green tick implying somebody
// checked, which is worse than no reviewer at all.
func TestGC23_AnUnreadableSourceFileIsAnError(t *testing.T) {
	_, err := Conditionals(filepath.Join(t.TempDir(), "absent.go"), map[int]bool{1: true})
	if err == nil {
		t.Fatal("a file that could not be read produced no error, so the reviewer " +
			"would report no guards over a change it never saw")
	}
	// The message is the observable part. Nil source makes the parser open the
	// path itself, so an error arrives either way — phrased as a parse
	// failure, which sends a reader hunting for a syntax problem in a file
	// nothing ever read.
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("an unreadable file was diagnosed as something else: %v", err)
	}
}
