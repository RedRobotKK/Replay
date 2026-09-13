package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every refusal in this file was UNREACHED on 2026-09-13.
//
// `guard reachability` neutralised fourteen of this package's nineteen
// conditionals and the suite stayed green. None of the survivors were in
// render, which the tests above cover directly; all of them were in the layer
// that finds the module root, reads two files and writes one. That layer is
// the part a contributor actually runs, and it was the part nothing watched.
//
// The fix was half restructure and half test. main had a branch no unit test
// can observe because main calls os.Exit, so the branch moved into cli, which
// returns the status instead. moduleRoot resolved the working directory and
// walked it in one function, so the walk could not be handed a directory; it
// takes one now. What was left after that is below.

// fixture builds a module root: a go.mod, a manifest and a README with the
// markers in it. Each test then removes or corrupts one thing, so each test
// moves exactly one.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module example.test\n\ngo 1.24\n")
	if err := os.MkdirAll(filepath.Join(root, "distribution"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "distribution", "channels.json"),
		`{"channels":[{"name":"Installer","command":"curl -sSf https://replay.doctor/install.sh | sh","status":"live"}]}`)
	write(t, filepath.Join(root, "README.md"),
		"# Example\n\n"+beginMarker+"\n"+endMarker+"\n\nprose below\n")
	return root
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// RI1: a manifest that is not there is a refusal, not an empty install block.
//
// PASS: generate returns the read error and README.md is untouched.
// FAIL: the generator would be free to decide that no manifest means no
// channels, and rewrite the block of a repository it could not read into a
// section telling the reader there is no way to install anything.
func TestRI1_MissingManifestRefusesAndLeavesTheReadmeAlone(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "distribution", "channels.json")); err != nil {
		t.Fatal(err)
	}
	before := read(t, filepath.Join(root, "README.md"))

	err := generate(root)
	if err == nil {
		t.Fatal("a missing manifest was accepted")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("want a not-exist error, got %v", err)
	}
	if after := read(t, filepath.Join(root, "README.md")); after != before {
		t.Error("the README was rewritten from a manifest that could not be read")
	}
}

// RI2: a manifest that is not JSON is named as a parse failure.
//
// The message matters here. render wraps with "parsing manifest", and a
// contributor who has just hand-edited channels.json needs to be told that
// rather than handed a bare syntax error with no file in it.
func TestRI2_UnparseableManifestSaysWhatFailed(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "distribution", "channels.json"), "{not json")

	err := generate(root)
	if err == nil {
		t.Fatal("a manifest that is not JSON was accepted")
	}
	if !strings.Contains(err.Error(), "parsing manifest") {
		t.Errorf("the error does not say the manifest failed to parse: %v", err)
	}
}

// RI3: a README that is not there is a refusal.
//
// Distinct from RI1 because they fail at different reads and a single test
// asserting "some error" would pass with either one deleted.
func TestRI3_MissingReadmeRefuses(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	if err := generate(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("want a not-exist error for the README, got %v", err)
	}
}

// RI4: a README with no markers is a refusal that names the marker.
//
// This is the case a fork hits: someone copies the project, rewrites the
// README, and runs the generator. Appending the block, or writing it at the
// top, would both be worse than saying what is missing.
func TestRI4_ReadmeWithoutMarkersRefusesAndNamesTheMarker(t *testing.T) {
	// Both cases, because they are two different guards in splice and a test
	// that only deletes both markers never reaches the second. Found by
	// neutralising them: the end-marker branch survived until this table had
	// its second row.
	for _, tc := range []struct {
		name, readme, want string
	}{
		{"neither marker", "# Example\n\nno markers here\n", beginMarker},
		{"begin but no end", "# Example\n\n" + beginMarker + "\nbody\n", endMarker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			write(t, filepath.Join(root, "README.md"), tc.readme)

			err := generate(root)
			if err == nil {
				t.Fatal("a README the generator cannot splice was accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %s: %v", tc.want, err)
			}
		})
	}
}

// RI5: a README that already matches is not rewritten.
//
// The generator is run by CI and by contributors, and a write that always
// happens turns "nothing to do" into a dirty working tree and a spurious diff
// in every pull request that ran it.
//
// The assertion is on the bytes AND on the modification time, because a write
// of identical bytes is invisible to a content check and very visible to git
// and to make.
func TestRI5_AnUpToDateReadmeIsNotWritten(t *testing.T) {
	root := fixture(t)
	readme := filepath.Join(root, "README.md")

	if err := generate(root); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := read(t, readme)
	if !strings.Contains(first, "curl -sSf") {
		t.Fatalf("the first run did not write the install block:\n%s", first)
	}
	stat, err := os.Stat(readme)
	if err != nil {
		t.Fatal(err)
	}

	if err := generate(root); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second := read(t, readme); second != first {
		t.Error("running the generator twice changed the README the second time")
	}
	again, err := os.Stat(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !again.ModTime().Equal(stat.ModTime()) {
		t.Error("the generator rewrote a README that already matched the manifest, " +
			"which dirties the working tree of everyone who runs it")
	}
}

// RI6: a README that cannot be written is a refusal, and the old one survives.
//
// A half-written README is worse than a stale one. os.WriteFile truncates
// before it writes, so the failure mode this guards is a truncated install
// section shipped as documentation.
func TestRI6_AReadmeThatCannotBeWrittenIsNotDestroyed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, so the read-only file below is still writable")
	}
	root := fixture(t)
	readme := filepath.Join(root, "README.md")
	before := read(t, readme)
	if err := os.Chmod(readme, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readme, 0o644) })

	if err := generate(root); err == nil {
		t.Fatal("a README that could not be written was reported as rewritten")
	}
	if after := read(t, readme); after != before {
		t.Error("the README changed even though the write failed")
	}
}

// RI7: the walk gives up rather than climbing out of the filesystem.
//
// moduleRoot recurses towards the root directory. Without the parent == dir
// stop it spins there forever, because filepath.Dir("/") is "/".
func TestRI7_ModuleRootGivesUpAtTheFilesystemRoot(t *testing.T) {
	dir := t.TempDir()
	// t.TempDir sits under the OS temp directory, which has no go.mod above it.
	root, err := moduleRoot(dir)
	if err == nil {
		t.Fatalf("moduleRoot found a module root at %q walking up from a temp directory", root)
	}
	if !strings.Contains(err.Error(), "no go.mod") {
		t.Errorf("the error does not say what was not found: %v", err)
	}
}

// RI8: and it finds the root when there is one, from a subdirectory.
//
// The counterpart to RI7. Without it, moduleRoot returning an error
// unconditionally would still pass RI7.
func TestRI8_ModuleRootFindsTheRootFromBelow(t *testing.T) {
	root := fixture(t)
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := moduleRoot(deep)
	if err != nil {
		t.Fatalf("moduleRoot did not find go.mod three directories up: %v", err)
	}
	// t.TempDir can hand back a path through a symlink (/var on macOS), so
	// compare what the filesystem resolves rather than the strings.
	if resolve(t, got) != resolve(t, root) {
		t.Errorf("moduleRoot returned %q, want %q", got, root)
	}
}

// RI9: a working directory that cannot be resolved is a refusal.
//
// os.Getwd fails when the directory the process is in has been removed under
// it, which a test cannot arrange for itself without taking the rest of the
// suite with it. The seam is how this branch is watched at all.
func TestRI9_AnUnresolvableWorkingDirectoryRefuses(t *testing.T) {
	sentinel := errors.New("the working directory is gone")
	restore := getwd
	getwd = func() (string, error) { return "", sentinel }
	t.Cleanup(func() { getwd = restore })

	if err := run(); !errors.Is(err, sentinel) {
		t.Errorf("run did not report the working-directory failure, got %v", err)
	}
}

// RI10: cli turns a failure into status 1 and says so on stderr.
//
// This is the branch that used to live in main, where nothing could see it.
// The generator is run from a Makefile and from CI; a failure that exits 0 is
// a failure nobody notices.
func TestRI10_CliReportsFailureOnStderrAndExitsNonZero(t *testing.T) {
	restore := getwd
	getwd = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { getwd = restore })

	var stderr bytes.Buffer
	if got := cli(&stderr); got != 1 {
		t.Errorf("cli returned %d for a failed run, want 1", got)
	}
	// The message, not just the prefix. Neutralising the moduleRoot check in
	// run let execution fall through to generate(""), which failed too, with
	// the same status and the same "readme-install:" prefix: a test asserting
	// on the prefix alone passed with the guard removed.
	if !strings.Contains(stderr.String(), "readme-install:") ||
		!strings.Contains(stderr.String(), "no go.mod") {
		t.Errorf("stderr does not name the failure that actually happened, got %q", stderr.String())
	}
}

// RI10b: and run reports that failure rather than one further down.
//
// The direct form of the same assertion, on the error instead of on the text
// cli printed from it.
func TestRI10b_RunStopsAtTheModuleRootFailure(t *testing.T) {
	restore := getwd
	getwd = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { getwd = restore })

	err := run()
	if err == nil {
		t.Fatal("run succeeded from a directory with no go.mod above it")
	}
	if !strings.Contains(err.Error(), "no go.mod") {
		t.Errorf("run continued past the module-root failure and reported %v instead", err)
	}
}

// RI11: and a run that works exits 0 and writes nothing to stderr.
//
// The counterpart to RI10, and the one that keeps `cli` from being a function
// that always returns 1.
func TestRI11_CliSucceedsQuietly(t *testing.T) {
	root := fixture(t)
	restore := getwd
	getwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { getwd = restore })

	var stderr bytes.Buffer
	if got := cli(&stderr); got != 0 {
		t.Errorf("cli returned %d for a run that worked, want 0", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("a successful run wrote to stderr: %q", stderr.String())
	}
}

// RI12: extract refuses the same two READMEs splice refuses.
//
// extract and splice each find the markers independently. A README that
// splice accepts and extract does not would make the drift check in
// TestReadmeBlockMatchesGenerator compare against nothing.
func TestRI12_ExtractRefusesAReadmeWithoutEitherMarker(t *testing.T) {
	for _, tc := range []struct {
		name, readme, want string
	}{
		{"no begin marker", "# Example\n" + endMarker + "\n", beginMarker},
		{"no end marker", "# Example\n" + beginMarker + "\nbody\n", endMarker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extract(tc.readme)
			if err == nil {
				t.Fatalf("extract accepted a README with no %s and returned %q", tc.want, got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %s: %v", tc.want, err)
			}
		})
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func resolve(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// RI13: the generated headings sit exactly one level below the section that
// encloses the block.
//
// Two markdownlint failures in a row came from this one fact, and neither was
// about the words. The generator first emitted **bold** on its own line, which
// is MD036 (emphasis used as a heading); that became `####`, which is MD001
// (a heading level skipped) because the block sits under an `##`. A third
// spelling picked by hand would have been a third guess.
//
// So the test reads the README rather than a constant: it finds the heading
// that encloses the install block and requires the generated ones to be one
// deeper. Moving the block under a different section, or renumbering the
// headings above it, now fails here rather than in CI's linter.
func TestRI13_GeneratedHeadingsAreOneLevelBelowTheEnclosingSection(t *testing.T) {
	readme := readReadme(t)
	start := strings.Index(readme, beginMarker)
	if start == -1 {
		t.Fatal("README.md has no install-matrix begin marker")
	}

	// The nearest heading above the block is the section it belongs to.
	enclosing := 0
	for _, line := range strings.Split(readme[:start], "\n") {
		if h := headingLevel(line); h > 0 {
			enclosing = h
		}
	}
	if enclosing == 0 {
		t.Fatal("the install block sits under no heading at all")
	}

	block, err := render(readManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, line := range strings.Split(block, "\n") {
		h := headingLevel(line)
		if h == 0 {
			continue
		}
		seen++
		if h != enclosing+1 {
			t.Errorf("the generator emits %q at level %d under a level %d section.\n"+
				"markdownlint MD001 fails on a skipped level and MD036 fails on bold "+
				"used instead of a heading, so the level has to be derived, not chosen.",
				strings.TrimSpace(line), h, enclosing)
		}
	}
	if seen == 0 {
		t.Error("no headings in the generated block, so this test proved nothing")
	}
}

// headingLevel returns the ATX heading level of a line, or 0 if it is not a
// heading. A line inside a fenced code block is not a heading, and the install
// block is full of shell comments starting with #, so the caller must not feed
// this fenced content. render only emits fences around commands, and no
// command in the manifest starts a line with #.
func headingLevel(line string) int {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(line) || line[n] != ' ' {
		return 0
	}
	return n
}

// RI14: a status the generator does not know is a refusal, not a deletion.
//
// The switch in render knew live, building and blocked. The manifest carries
// planned and skipped too, so those rows fell through and printed nothing.
// That was the intended outcome for them and the wrong mechanism for it: the
// same fall-through swallows a typo.
//
// The case that matters is the third row below. "liv" is one keystroke from
// "live", and before this refusal it removed a working install route from the
// README while every check in the repository stayed green.
func TestRI14_AnUnknownStatusRefusesInsteadOfDroppingTheChannel(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		wantErr      bool
	}{
		{"live", "live", false},
		{"planned is omitted on purpose", "planned", false},
		{"skipped is omitted on purpose", "skipped", false},
		{"a typo for live", "liv", true},
		{"empty", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"channels":[{"name":"Example","command":"example install","status":"` + tc.status + `"}]}`)
			out, err := render(raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("status %q was accepted and the channel rendered as %q", tc.status, out)
				}
				if !strings.Contains(err.Error(), tc.status) || !strings.Contains(err.Error(), "Example") {
					t.Errorf("the refusal names neither the status nor the channel: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("status %q was refused: %v", tc.status, err)
			}
			if got := strings.Contains(out, "example install"); got != (tc.status == "live") {
				t.Errorf("status %q: command present = %v, want %v", tc.status, got, tc.status == "live")
			}
		})
	}
}

// RI15: every status the manifest actually uses is one the generator knows.
//
// RI14 pins the mechanism with invented statuses. This points the same
// question at the real file, so adding a status to distribution/channels.json
// without teaching the generator fails here, at the place the manifest is
// edited, rather than in whatever release first notices a missing route.
func TestRI15_TheRealManifestUsesNoStatusTheGeneratorRejects(t *testing.T) {
	if _, err := render(readManifest(t)); err != nil {
		t.Errorf("distribution/channels.json does not render: %v", err)
	}
}
