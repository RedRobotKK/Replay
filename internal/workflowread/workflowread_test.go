package workflowread

import (
	"strings"
	"testing"
)

const sample = `name: x
on: push
env:
  TAG: v1
jobs:
  channel:
    runs-on: ubuntu-latest
    steps:
      - name: guard
        id: channel
        run: |
          channel="$(./scripts/release-channel.sh "$TAG")" || exit 1

          echo "channel=$channel" >> "$GITHUB_OUTPUT"
  npm:
    needs: channel
    # a comment at job level
    if: needs.channel.outputs.channel == 'stable'
    steps:
      - uses: actions/checkout@abc # v7
        with:
          path: .release-guard
          persist-credentials: false
      # a comment between steps
      - name: Publish
        if: steps.channel.outputs.channel == 'stable'
        run: npm publish --access public
        env: { A: "b" }
      - uses: pypa/gh-action-pypi-publish@def # v1
        with: { packages-dir: wheelhouse/, x: 'y' }
      - uses: goreleaser/goreleaser-action@123
`

// The reader sees what the workflows hold: job keys, step keys, both block
// forms, and the predicates FC-RC relies on. If it misreads, FC-RC's tests
// pass on a file they never read, so this pins it.
func TestParseWorkflowReadsTheShapeTheWorkflowsUse(t *testing.T) {
	jobs := ParseWorkflow(sample)
	if len(jobs) != 2 || jobs[0].Name != "channel" || jobs[1].Name != "npm" {
		t.Fatalf("jobs = %+v", jobs)
	}
	g := jobs[0].Steps
	if len(g) != 1 || g[0].ID != "channel" || !IsChannelGuard(g[0]) {
		t.Fatalf("guard step = %+v", g)
	}
	if !strings.Contains(g[0].Run, "\n\necho \"channel=$channel\"") {
		t.Errorf("run block lost its blank line or indentation: %q", g[0].Run)
	}
	n := jobs[1]
	if n.Needs != "channel" || n.If != StableJobCond {
		t.Errorf("job keys: needs %q if %q", n.Needs, n.If)
	}
	if len(n.Steps) != 4 {
		t.Fatalf("npm steps = %d, want 4: %+v", len(n.Steps), n.Steps)
	}
	if s := n.Steps[0]; s.Uses != "actions/checkout@abc" || s.With["path"] != ".release-guard" || s.With["persist-credentials"] != "false" {
		t.Errorf("checkout step = %+v", s)
	}
	if s := n.Steps[1]; s.If != StableStepCond || !PublishesToRegistry(s) || IsChannelGuard(s) {
		t.Errorf("publish step = %+v", s)
	}
	if s := n.Steps[2]; !PublishesToRegistry(s) || s.With["packages-dir"] != "wheelhouse/" || s.With["x"] != "y" {
		t.Errorf("pypi step = %+v", s)
	}
	if s := n.Steps[3]; PublishesToRegistry(s) || !PublishesGitHubRelease(s) {
		t.Errorf("goreleaser step = %+v", s)
	}
	for i, s := range n.Steps {
		if s.Index != i {
			t.Errorf("step %d has Index %d", i, s.Index)
		}
	}
}

// A guard that takes the tag from a ${{ }} expression is not the guard: the
// expression is pasted into the shell text before bash sees it.
func TestIsChannelGuardRefusesAnInterpolatedTag(t *testing.T) {
	s := WorkflowStep{ID: "channel", Run: `./scripts/release-channel.sh "${{ env.TAG }}"`}
	if IsChannelGuard(s) {
		t.Error("an interpolated tag was accepted as the guard")
	}
	if IsChannelGuard(WorkflowStep{ID: "other", Run: "./scripts/release-channel.sh \"$TAG\""}) {
		t.Error("a step without id channel was accepted as the guard")
	}
}

// Over-matching is the safe direction: a step wrongly read as publishing must
// carry the guard condition, which costs nothing. So "echo npm publish-ish"
// counts as publishing, and only commands that cannot publish are listed here.
func TestDownloadIsNotPublishing(t *testing.T) {
	if !PublishesToRegistry(WorkflowStep{Run: "echo npm publish-ish"}) {
		t.Error("the matcher stopped over-matching; check it still sees `npm publish --tag x`")
	}
	for _, run := range []string{"gh release download v1", "npm test", "npm view x", "npm pack"} {
		s := WorkflowStep{Run: run}
		if PublishesToRegistry(s) || PublishesGitHubRelease(s) {
			t.Errorf("%q read as publishing", run)
		}
	}
}
