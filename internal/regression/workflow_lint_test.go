package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-AL. A workflow that cannot be parsed fails INVISIBLY.
//
// `publish-shims.yml` had a YAML syntax error from the day it was written and
// nobody saw it for weeks. It is the workflow that publishes the npm and PyPI
// shims when a release is cut, so two distribution channels this project
// advertises would have quietly published nothing on the first tag.
//
// The error was one character class: `env: { GH_TOKEN: ${{ github.token }} }`.
// Inside a YAML FLOW mapping a value starting with `{` opens a nested flow
// mapping, so the expression has to be quoted. Four occurrences, two files
// worth of jobs, zero runs.
//
// WHY IT WAS INVISIBLE, which is the part worth keeping. A workflow whose file
// will not parse does not produce a failed job. It produces a run with NO JOBS,
// attributed to whatever event happened to be pushed, and it registers NO CHECK
// RUN — so it never appears in a pull request's check list and never turns a
// merge button red. `gh run list` showed it failing at 0s on every single push
// and the pull request stayed green the whole time.
//
// That is the same shape as every other finding in this package: a check that
// looks present, is absent, and whose absence is reported nowhere a person
// looks. The fix is to make the parse a job that CAN go red.
//
// PASS: ci.yml runs actionlint over the workflow directory.
// FAIL: it does not, and the next unparseable workflow is found by a user who
// tagged a release and got no package.
func TestFCAL_TheWorkflowsAreLintedByAJobThatCanFail(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	ci := string(raw)

	// The COMMAND, not the word. The first version of this test looked for
	// "actionlint" anywhere in the file and was satisfied by its own explanatory
	// comment, which is the failure mode this whole package is about: a check
	// that passes because of the prose describing it.
	installs := strings.Contains(ci, "actionlint/cmd/actionlint@")
	invokes := strings.Contains(ci, "/bin/actionlint")
	if !installs || !invokes {
		t.Error("ci.yml does not install and run actionlint.\n\n" +
			"A workflow that will not parse produces a run with no jobs and NO CHECK RUN, " +
			"so it never appears in a pull request's checks and never blocks a merge. " +
			"publish-shims.yml sat unparseable for weeks exactly this way, which would " +
			"have meant no npm package and no PyPI wheel on the first release tag.\n\n" +
			"Add a step that runs actionlint over .github/workflows.")
	}
}

// FC-AL2: no workflow uses an unquoted ${{ }} inside a YAML flow mapping.
//
// The specific defect, caught locally rather than on a runner. actionlint in CI
// is the real guard; this is the one that fails on a laptop before the push,
// and it needs no YAML parser, which matters because this module has zero
// third-party dependencies on purpose.
func TestFCAL2_NoUnquotedExpressionInAFlowMapping(t *testing.T) {
	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var checked int
	for _, e := range entries {
		if e.IsDir() || !(strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		checked++
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") || !strings.Contains(line, "${{") {
				continue
			}
			// Only flow mappings are at risk: `key: { a: ${{ x }} }`. A block
			// mapping puts the expression at the end of its own line and YAML
			// reads it as a plain scalar, which is fine.
			open := strings.Index(line, "{")
			expr := strings.Index(line, "${{")
			if open == -1 || open >= expr {
				continue
			}
			// Inside a flow mapping. The expression must be quoted.
			if !strings.Contains(line, `"${{`) && !strings.Contains(line, `'${{`) {
				t.Errorf("%s:%d has an unquoted ${{ }} inside a flow mapping, which YAML "+
					"reads as a nested mapping and fails to parse. The whole workflow then "+
					"produces no jobs and no check run, so nothing anywhere goes red:\n  %s",
					e.Name(), i+1, trimmed)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no workflow files were read, so this test proves nothing")
	}
	t.Logf("checked %d workflow files", checked)
}
