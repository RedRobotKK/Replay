package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-GV. The govulncheck badge is a claim, and this is what enforces it.
//
// The badge says "no known vulnerabilities". That is a strong sentence to put at
// the top of a README for a binary people pipe from `curl` onto machines holding
// provider credentials, and it is true only while three things hold at once:
// the job exists, it actually runs govulncheck, and it is allowed to fail the
// build. Remove any one and the badge becomes a green shield over nothing, which
// is the exact failure mode the coverage badge test was written to prevent and
// whose SCOPE note admits it could not check.
//
// This one can check it, because the workflow is a file in the repository.
//
// WHY THE LEVEL IS "NONE" AND NOT A SCORE. There is no partial credit available
// here that would mean anything. govulncheck reports vulnerabilities reachable
// from this module's own call graph, so a finding is not a package this project
// happens to link, it is a code path this binary can execute. A badge saying
// "3 known vulnerabilities" would be an honest number and a useless one: the
// only actionable states are zero and not-zero.
//
// It earned the badge on its first run by failing. The toolchain floor was
// go1.24.7 for one commit and CI reported three reachable standard-library
// vulnerabilities at that version: a quadratic parse in net/url reached through
// the self-update client, and post-handshake message handling in crypto/tls plus
// an HTTP/2 header timeout in net/http, both reached through the proxy. The
// floor moved to go1.25.13, where all three are fixed. Zero third-party
// dependencies never meant zero dependencies.
//
// PASS: the badge is present and the job that backs it is present and armed.
// FAIL: one of them was removed and the other was not.
func TestFCGV_TheGovulncheckBadgeIsBackedByAJobThatCanFail(t *testing.T) {
	root := repoRoot(t)

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	badge := strings.Contains(string(readme), "img.shields.io/badge/govulncheck-")

	ciPath := filepath.Join(root, ".github", "workflows", "ci.yml")
	ciRaw, err := os.ReadFile(ciPath)
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	ci := string(ciRaw)

	job := strings.Contains(ci, "govulncheck:")
	runs := strings.Contains(ci, "golang.org/x/vuln/cmd/govulncheck")

	switch {
	case badge && !job:
		t.Error("README shows a govulncheck badge and ci.yml defines no govulncheck job.\n" +
			"The badge is claiming an enforcement that is not happening.")
	case badge && !runs:
		t.Error("README shows a govulncheck badge and ci.yml never invokes govulncheck.\n" +
			"A job named for a check it does not run is worse than no job: it reports success.")
	case job && !badge:
		t.Error("ci.yml runs govulncheck and the README does not say so.\n" +
			"Not a defect in the software, but the work is being done and not claimed.")
	}

	// The job must be able to fail the build. `continue-on-error: true` on this
	// job would turn a reported vulnerability into a green tick, and it is the
	// single edit that would silently void the badge.
	//
	// It matches the YAML KEY, not the words. The first version of this check
	// used strings.Contains over the whole file and failed immediately on the
	// comment above the go-latest job, which uses the phrase to explain why that
	// job deliberately does not carry the setting. A guard that fires on prose
	// about itself is a guard nobody will keep.
	if jobHasKey(ci, "govulncheck:", "continue-on-error:") {
		t.Error("the govulncheck job carries continue-on-error, so a reported vulnerability " +
			"cannot fail the build and the README badge is a decoration.")
	}

	// govulncheck reads the toolchain from go.mod, so a floor below the version
	// that fixed a known-reachable vulnerability puts the job back where it
	// started. This does not verify the floor is currently sufficient, which
	// only a real run can do; it verifies the floor is stated at all, because a
	// go.mod with no toolchain line hands the decision to whatever the runner
	// happened to ship.
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	if !strings.Contains(string(gomod), "toolchain ") {
		t.Error("go.mod states no toolchain floor, so the standard library this badge " +
			"makes a claim about is whichever one the build machine had.")
	}
}

// jobHasKey reports whether the YAML block introduced by jobHeader contains key
// as a directive rather than as text inside a comment.
//
// This is deliberately a small scanner rather than a YAML parse. The repository
// takes no third-party dependencies, and the question is narrow enough to answer
// by indentation: a job block runs from its header to the next line at the same
// indent, and a directive is a line whose first non-space character is not '#'.
func jobHasKey(ci, jobHeader, key string) bool {
	lines := strings.Split(ci, "\n")
	start := -1
	indent := 0
	for i, ln := range lines {
		if strings.TrimSpace(ln) == strings.TrimSpace(jobHeader) {
			start = i + 1
			indent = len(ln) - len(strings.TrimLeft(ln, " "))
			break
		}
	}
	if start == -1 {
		return false
	}
	for _, ln := range lines[start:] {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		got := len(ln) - len(strings.TrimLeft(ln, " "))
		if got <= indent {
			return false // the next job began
		}
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, key) || strings.HasPrefix(t, "- "+key) {
			return true
		}
	}
	return false
}
