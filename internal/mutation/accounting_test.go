//go:build mutation

package mutation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The harness's own arithmetic, put to the same standard it applies to
// everything else.
//
// Every number this package reports rests on two claims it never checked
// about itself: that a mutant the compiler rejected is classified stillborn
// rather than killed, and that a non-zero stillborn count fails the run. Both
// were true by inspection and neither was observable. The first is a
// two-stage detection — `go build` for the tree, a string match for the test
// files — and the string match is one phrase away from reporting a mutant no
// test ever saw as caught. The second was a `t.Logf` at the end of a run,
// which says a number and asserts nothing.
//
// These tests build a throwaway module, break it three different ways, and
// read the verdict back. Four small compiles, not seventy-five.

// tinyModule writes a module with one function and one test that passes.
func tinyModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module tiny\n\ngo 1.24\n")
	write("double.go", "package tiny\n\nfunc Double(n int) int { return n * 2 }\n")
	write("double_test.go", `package tiny

import "testing"

func TestDouble(t *testing.T) {
	if got := Double(3); got != 6 {
		t.Fatalf("Double(3) = %d, want 6", got)
	}
}
`)
	return dir
}

// editTiny rewrites one file of the throwaway module.
func editTiny(t *testing.T, dir, name, from, to string) {
	t.Helper()
	path := filepath.Join(dir, name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), from) {
		t.Fatalf("%s does not contain %q, so this case mutates nothing", name, from)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(body), from, to, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTheClassifierTellsARefusalFromAFailure is the check behind every number
// this package prints.
//
// `go test` exits non-zero for a build failure exactly as it does for a test
// failure, so the naive reading of that exit code reports a kill for a mutant
// the compiler threw out. Two stages stop that, and they are not
// interchangeable:
//
//   - `go build ./...` answers for package files by exit code, which no
//     wording can defeat. The "package file" case below pins it by asserting
//     the refusal arrives WITHOUT the test runner's "[build failed]" marker;
//     drop the pre-check and the same mutant is still classified stillborn,
//     but by the string match, and the assertion on the output goes red.
//   - buildFailed reads the test runner's output, because `go build` does not
//     compile _test.go files at all. That stage is the phrase-dependent one
//     and it is the only thing standing behind the "test file" case: with
//     "[build failed]" removed from its list, that case is classified KILLED —
//     a mutant no test ever saw, counted as caught.
//
// Both were verified by making each change and watching the case named above
// fail.
func TestTheClassifierTellsARefusalFromAFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		from string
		to   string
		want outcome
		// refusedBeforeTheRunner asserts the compiler answered first, so the
		// output carries no test-runner marker.
		refusedBeforeTheRunner bool
	}{
		{
			name: "untouched",
			want: survived,
		},
		{
			name: "behaviour changed",
			file: "double.go",
			from: "n * 2",
			to:   "n * 3",
			want: killed,
		},
		{
			name:                   "package file does not compile",
			file:                   "double.go",
			from:                   "n * 2",
			to:                     "nope * 2",
			want:                   stillborn,
			refusedBeforeTheRunner: true,
		},
		{
			name: "only the test file does not compile",
			file: "double_test.go",
			from: "got != 6",
			to:   `got != "six"`,
			want: stillborn,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tinyModule(t)
			if tc.file != "" {
				editTiny(t, dir, tc.file, tc.from, tc.to)
			}
			got, failed, out := runNamed(t, dir, []string{"TestDouble"})
			if got != tc.want {
				t.Fatalf("classified %s, want %s.\n%s", got, tc.want, tail(out))
			}
			if tc.want == killed && !failed["TestDouble"] {
				t.Errorf("killed, but TestDouble is not among the failures: %v", failed)
			}
			if tc.refusedBeforeTheRunner && strings.Contains(out, "[build failed]") {
				t.Errorf("the refusal came from the test runner, not the compiler: the "+
					"pre-check that answers by exit code is gone, and the verdict now "+
					"rests entirely on a phrase in someone else's output.\n%s", tail(out))
			}
		})
	}
}

// TestTheStillbornCountIsWhatFailsTheRun holds the second claim: that the
// number is load-bearing rather than decorative.
//
// It was not. The count lived in a `t.Logf` that only prints under -v, and the
// run went red because each stillborn mutant called t.Fatalf in its own
// subtest. Softening any one of those Fatalfs to a Logf — the obvious edit
// when a mutant is temporarily awkward — would have left the tally printing a
// non-zero number under a green run.
func TestTheStillbornCountIsWhatFailsTheRun(t *testing.T) {
	clean := verdict{killed: 74, survived: 1}
	if err := clean.evidence(); err != nil {
		t.Errorf("a run with nothing stillborn is evidence, but it reported: %v", err)
	}

	v := verdict{
		killed: 73,
		stillborn: []stillbirth{
			{id: "M9", name: "carry-count-is-one", out: "undefined: carried"},
		},
	}
	err := v.evidence()
	if err == nil {
		t.Fatal("one stillborn mutant and the run still calls itself evidence; " +
			"that is the whole defect this accounting exists to make visible")
	}
	for _, want := range []string{"M9", "carry-count-is-one", "stillborn"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the failure does not name %q, so the reader cannot act on it:\n%s", want, err)
		}
	}

	if got, want := v.total(), 74; got != want {
		t.Errorf("total is %d, want %d: a stillborn mutant is still a catalogue entry, "+
			"and dropping it from the denominator is how a score reads 100%%", got, want)
	}
	if s := v.String(); !strings.Contains(s, "1 stillborn") {
		t.Errorf("the one-line score does not say how many were stillborn: %s", s)
	}
}
