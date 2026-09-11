package guardcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A conditional in a file this host does not compile is not scored, and the
// run must say so.
//
// guard-reachability neutralises a condition and re-runs the suite. It runs one
// build, for the host's GOOS/GOARCH. A conditional inside a file the host does
// not compile is never neutralised and never scored — and it is reported as
// neither survived nor unchecked. It is not seen at all, and the pass still
// prints "0 survived, 0 unchecked".
//
// This repository has ten such files. internal/tui/term_linux.go and both
// *_windows.go pairs are invisible on darwin, and internal/tui/term_unix.go is
// also the one place `unsafe` is used. A guard added there lands on main with
// the same green line as a guard that was actually exercised, and that line is
// what CI asserts on.
//
// That is ADR-0014 one level up: the check that cannot fail is the pass itself,
// for those files.

// GC29: a file gated to another platform is reported as not built here.
func TestGC29_AFileGatedToAnotherPlatformIsNotSilentlySkipped(t *testing.T) {
	other := "linux"
	if runtime.GOOS == "linux" {
		other = "darwin"
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gated.go")
	body := "//go:build " + other + "\n\npackage p\n\nfunc g(a bool) int {\n\tif a {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	built, why := BuildableHere(path)
	if built {
		t.Fatalf("a file gated to %s reports as buildable on %s, so its conditionals "+
			"are scored against a build that never happened", other, runtime.GOOS)
	}
	if why == "" {
		t.Error("the file is skipped with no reason given, so the run says a file " +
			"vanished and not why")
	}
}

// GC30: a file this host does compile is not mistaken for a gated one.
//
// The cost of getting this backwards is worse than the gap it closes: every
// ordinary conditional would be reported as unscored, and a report that says
// everything is unscored says nothing.
func TestGC30_AnOrdinaryFileIsBuildableHere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.go")
	if err := os.WriteFile(path, []byte("package p\n\nfunc g() int { return 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if built, why := BuildableHere(path); !built {
		t.Errorf("an unconstrained file reports as not buildable here (%q)", why)
	}
}

// GC31: the filename suffix counts too, not only the //go:build line.
//
// Go decides this two ways and a check that reads only one of them is blind to
// the other. internal/ownerdir/mode_windows.go carries both; a file with only
// the suffix carries neither line to read.
func TestGC31_TheFilenameSuffixIsAConstraint(t *testing.T) {
	other := "linux"
	if runtime.GOOS == "linux" {
		other = "darwin"
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "thing_"+other+".go")
	if err := os.WriteFile(path, []byte("package p\n\nfunc g() int { return 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if built, _ := BuildableHere(path); built {
		t.Errorf("thing_%s.go reports as buildable on %s; the filename is a build "+
			"constraint and reading only //go:build lines misses it", other, runtime.GOOS)
	}
}

// GC32: a build-excluded tool file is not confused with a platform-gated one.
//
// //go:build ignore means "no build, anywhere" and is already handled; this
// new report is about "not THIS host". Collapsing them would put the
// reachability tool itself in a list of things the host cannot compile.
func TestGC32_IgnoreIsNotThesameAsAnotherPlatform(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool.go")
	if err := os.WriteFile(path, []byte("//go:build ignore\n\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	excluded, _ := ExcludedFromBuild(path)
	if !excluded {
		t.Error("a //go:build ignore file is no longer reported as excluded from every build")
	}
	built, _ := BuildableHere(path)
	if built {
		t.Error("an ignore file reports as buildable here")
	}
}

// GC33: a bare filename resolves against the working directory.
//
// filepath.Split returns "" for a path with no separator, and go/build reads
// "" as "no directory" rather than "here". Passing it through unchanged makes
// MatchFile fail on every relative filename, which would report an ordinary
// file as one this host cannot build — the inversion GC30 exists to prevent,
// arriving through a different door.
func TestGC33_ABareFilenameResolvesAgainstTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain.go"),
		[]byte("package p\n\nfunc g() int { return 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	built, why := BuildableHere("plain.go")
	if !built {
		t.Errorf("a bare filename in the working directory reported as not buildable (%q); "+
			"every relative path would be filed under another platform's", why)
	}
}

// GC34: a file that cannot be read is not filed under another platform.
//
// "not built on darwin/arm64" says the code is fine and belongs elsewhere. A
// file that will not parse is not fine anywhere, and reporting it under that
// heading sends a reader to a machine where it will fail in the same way.
func TestGC34_AnUnreadableFileIsNotFiledUnderAnotherPlatform(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go")
	// A //go:build line that is not a constraint: MatchFile reads the header
	// and reports the error rather than a verdict.
	if err := os.WriteFile(path, []byte("//go:build ((\n\npackage p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	built, why := BuildableHere(path)
	if built {
		t.Fatal("a file with a malformed build constraint reported as buildable")
	}
	if why == "" || strings.Contains(why, "not built on") {
		t.Errorf("an unreadable file was filed under another platform (%q), which tells a "+
			"reader to go and run it somewhere it will fail the same way", why)
	}
}

// GC35: built-nowhere and built-elsewhere are different answers.
//
// This is the distinction the whole report rests on. ExcludedFromBuild
// evaluates with every tag false, under which `//go:build ignore` and
// `//go:build linux` both come back excluded — so used alone it skips a
// Linux-only file silently, and that file's guards ship to Linux users
// unreviewed.
//
// An ignore file is built nowhere and is correct to skip: it is a standalone
// tool and its conditionals are in no binary. A linux file is built on Linux.
func TestGC35_BuiltNowhereIsNotBuiltElsewhere(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	for _, tc := range []struct {
		name      string
		path      string
		somewhere bool
	}{
		{"ignore is built nowhere", write("tool.go", "//go:build ignore\n\npackage main\n"), false},
		{"linux is built on linux", write("gated.go", "//go:build linux\n\npackage p\n"), true},
		{"windows is built on windows", write("win.go", "//go:build windows\n\npackage p\n"), true},
		{"an unconstrained file is built everywhere", write("plain.go", "package p\n"), true},
		// The suffix again: mode_windows.go in this repo carries both, but a
		// file with only the suffix carries no line to read.
		{"the filename suffix counts", write("thing_windows.go", "package p\n"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuiltOnSomePlatform(tc.path); got != tc.somewhere {
				t.Errorf("BuiltOnSomePlatform = %v, want %v. Getting this wrong either "+
					"skips a file that ships (false when it should be true) or files the "+
					"reachability tool itself under things this host cannot compile "+
					"(true when it should be false)", got, tc.somewhere)
			}
		})
	}
}
