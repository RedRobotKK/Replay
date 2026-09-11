// Package mergeguard is the machinery behind the merge-result reviewer.
//
// Every CI job in this repo runs against a branch head. Nothing runs against
// the result of merging that branch into main, so two PRs that are each
// green can produce a main that does not build. That has happened twice:
// #157 with #160, where a row budget each respected alone was exceeded
// together, and #184 with #185, where one carried a file to main and the
// other added the same file renamed.
//
// It lives here, rather than inside the script, for the reason guardcheck
// does: a tool that judges whether a merge compiles should be able to say
// so about itself, and a //go:build ignore script cannot be tested.
package mergeguard

import (
	"strings"
)

// Checks are the commands run against the merged tree, in order.
//
// `go build ./...` is not among them, and its absence is the whole finding.
// Build does not type-check test files, and the merge that broke main broke
// it in a _test.go: two declarations of BenchmarkParseRealTranscript, one
// from each branch. `go build ./...` on that tree printed nothing at all.
//
// `go vet` type-checks tests, which is what catches a collision between two
// files that each compile alone. It is also cheap: it does not run anything.
func Checks() [][]string {
	return [][]string{
		{"go", "vet", "./..."},
	}
}

// Conflicted reports the tree a `git merge-tree --write-tree` run wrote, and
// the paths it could not merge.
//
// The distinction that matters: a conflict is the EASY case. git reports it,
// a person expects it, and nobody merges through it. The case this tool
// exists for produces no conflict at all — the merge of #184 and #185 was
// clean, wrote a tree, exited zero, and did not compile. So a caller must
// not treat "no conflict" as "safe"; it means "now go build it".
//
// It reads the output alone and not the exit status. The two say the same
// thing — git exits non-zero exactly when it prints a conflict section — and
// taking both meant a branch that could not change an answer, which the
// reviewer duly reported as inert. A command that failed for some other
// reason prints no tree, and the caller checks for that.
func Conflicted(out string) (tree string, paths []string) {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// strings.Split never returns an empty slice, so there is always a first
	// line to read: for empty input it is the empty string, which is not a
	// tree and which the caller refuses.
	tree = strings.TrimSpace(lines[0])
	for _, l := range lines[1:] {
		l = strings.TrimSpace(l)
		// Informational lines are "<mode> <oid> <stage>\t<path>". git also
		// prints a blank line either side of the conflict section; those
		// trim to empty and are dropped by the check below, which the
		// reviewer pointed out makes a skip here redundant.
		if i := strings.IndexByte(l, '\t'); i >= 0 {
			l = l[i+1:]
		}
		if l != "" && !contains(paths, l) {
			paths = append(paths, l)
		}
	}
	return tree, paths
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
