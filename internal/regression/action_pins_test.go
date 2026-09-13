package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FC-PIN. Every action in every workflow is pinned to a commit SHA.
//
// This repository's policy was prose. release.yml pins all five of its actions
// and carries two comments about why floating refs burned the project twice.
// publish-shims.yml, added later, pinned none of them, and its own header said
// the SHAs "should be filled in before the first tag that runs this". That tag
// was hours away when a reviewer found it.
//
// One of the five was `pypa/gh-action-pypi-publish@release/v1`, a MOVING BRANCH
// in the job that holds the PyPI Trusted Publishing OIDC token. Whoever can move
// that branch can publish under this project's name.
//
// Nothing caught it, and the reason is worth keeping: actionlint checks syntax
// and expressions, not supply chain, and ci.yml runs on push to main and pull
// requests, so on a TAG PUSH the only workflow that executes is release.yml. A
// policy that lives in a comment is a policy nothing applies.
//
// PASS: every `uses:` names a 40-character hex SHA.
// FAIL: a tag or a branch, which is a reference somebody else can move after
// review and before it runs.
func TestFCPIN_EveryActionIsPinnedToASHA(t *testing.T) {
	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	// `uses: owner/repo@ref` and `uses: ./local/action`. A local path is the
	// repository's own code and needs no pin.
	uses := regexp.MustCompile(`uses:\s*([^\s#]+)`)
	sha := regexp.MustCompile(`^[0-9a-f]{40}$`)

	var checked int
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			m := uses.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			ref := m[1]
			if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
				continue
			}
			at := strings.LastIndex(ref, "@")
			if at < 0 {
				t.Errorf("%s:%d uses %q with no ref at all", e.Name(), i+1, ref)
				continue
			}
			checked++
			if !sha.MatchString(ref[at+1:]) {
				t.Errorf("%s:%d uses %q. A tag or a branch is a reference somebody else "+
					"can move after you reviewed it and before it runs. Pin the 40-character "+
					"commit SHA and put the version in a trailing comment, as release.yml "+
					"does.", e.Name(), i+1, ref)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no action reference was checked, so this test asserts nothing")
	}
	t.Logf("checked %d action reference(s)", checked)
}
