package regression

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// An assertion on an operating system's wording must be gated on that system.
//
// A test that says the refusal contained "no such file or directory" is
// testing libc. Unix says that; Windows says "The system cannot find the file
// specified"; the program under test said neither, and behaved identically on
// both. TestBG7 failed that way on Windows CI over a refusal that had worked
// perfectly — the fifth defect of this shape in this repository.
//
// The rule is not "never name an OS message". Sometimes that is exactly the
// right assertion: internal/proxy's UR2 checks that a socket's parent was
// refused by the package rather than by the kernel, and the kernel's own
// wording is the thing being excluded. What makes UR2 correct is that it runs
// `requireUnix(t)` first, so it only ever asserts where that wording exists.
//
// So: gate it, or do not write it.
//
// PASS: every test asserting on an OS phrase is gated on the platform.
// FAIL: one is not, and it passes here and fails on somebody else's machine.
func TestOSPhrasingAssertionsAreGatedOnThePlatform(t *testing.T) {
	// Messages produced by an operating system, not by this program. Written
	// as the OS spells them.
	// "file exists" is deliberately absent. It is the wording of EEXIST and
	// also of ordinary English — "the defect this file exists to prevent"
	// appears four times in this tree — and a guard that cannot tell those
	// apart reports six findings and is right about none of them. That was
	// this test's first output.
	osPhrases := []string{
		"no such file or directory",
		"The system cannot find",
		"permission denied",
		"directory not empty",
		"not a directory",
		"too many open files",
		"bad file descriptor",
	}
	// A test may assert on any of the above once it has established which
	// platform it is running on.
	// A bare t.Skip is NOT a gate. Every gate here has to name a platform,
	// because a test that skips for some unrelated reason has established
	// nothing about which operating system it is on. The first version of
	// this list included "t.Skip" and therefore excused the defect planted
	// to prove the guard could fail — a check that cannot fail, in the guard
	// written against checks that cannot fail.
	gates := []string{
		"requireUnix(", "requireNotWindows(", "skipOnWindows(",
		"runtime.GOOS",
	}

	// The defect has one shape, and matching it exactly is what keeps this
	// guard honest: a containment check against an error's own text, on one
	// line. Anything looser reads comments and prose — the first version
	// flagged a doc comment, a sentence about what a file exists to prevent,
	// and a string that belonged to the test below the one it accused.
	assertion := regexp.MustCompile(
		`strings\.(Contains|HasPrefix|HasSuffix|EqualFold)\([^)]*\.Error\(\)\s*,\s*"([^"]*)"`)

	var offences []string
	for path, body := range textFiles(t, ".go") {
		if !strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, fn := range testFunctions(body) {
			_, gated := containsAny(fn.body, gates...)
			for i, line := range strings.Split(fn.body, "\n") {
				code := line
				// Comments describe the defect; they do not commit it.
				if j := strings.Index(code, "//"); j >= 0 {
					code = code[:j]
				}
				m := assertion.FindStringSubmatch(code)
				if m == nil {
					continue
				}
				phrase, ok := containsAny(m[2], osPhrases...)
				if !ok || gated {
					continue
				}
				offences = append(offences,
					fmt.Sprintf("%s:%d  %s  asserts %q", path, i+1, fn.name, phrase))
			}
		}
	}
	sort.Strings(offences)

	if len(offences) > 0 {
		t.Errorf("these assert on an operating system's wording without establishing which "+
			"operating system they are on, so they pass here and fail on somebody else's "+
			"machine:\n  %s\n\nAssert on the message this program wrote, or gate the test on "+
			"the platform whose wording it quotes.", strings.Join(offences, "\n  "))
	}
}

// testFunction is one Test… function and the source between its braces.
type testFunction struct {
	name string
	body string
}

// testFunctions splits a file into its test functions.
//
// Function scope, not file scope: a gate in one test says nothing about
// another in the same file, and file scope would let a single requireUnix
// excuse every assertion beside it.
func testFunctions(body string) []testFunction {
	decl := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)
	locs := decl.FindAllStringSubmatchIndex(body, -1)
	var out []testFunction
	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, testFunction{name: body[loc[2]:loc[3]], body: body[loc[0]:end]})
	}
	return out
}
