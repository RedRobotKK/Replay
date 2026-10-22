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

// shape renders what the reader found as one comparable string: each job's
// name and its steps' names, in order. The tests below compare it whole, so a
// job or step that appears, disappears or moves is a failure, not a count that
// happens to match.
func shape(jobs []WorkflowJob) string {
	var b strings.Builder
	for _, j := range jobs {
		b.WriteString(j.Name + "[")
		for i, s := range j.Steps {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(s.Name)
		}
		b.WriteString("] ")
	}
	return strings.TrimSpace(b.String())
}

// A comment or blank line at column zero inside jobs: is not a top-level key.
// If the reader took it for one it would leave jobs: there, and every job after
// it, a publishing job included, would be invisible to FC-RC, which would then
// pass on a workflow it never read.
func TestColumnZeroCommentInsideJobsHidesNoJob(t *testing.T) {
	src := "on: push\njobs:\n  channel:\n    steps:\n      - name: guard\n        run: echo\n" +
		"# a column-zero comment between jobs\n\n" +
		"  npm:\n    steps:\n      - name: Publish\n        run: npm publish\n"
	jobs := ParseWorkflow(src)
	if got, want := shape(jobs), "channel[guard] npm[Publish]"; got != want {
		t.Fatalf("jobs = %q, want %q", got, want)
	}
	if !PublishesToRegistry(jobs[1].Steps[0]) {
		t.Errorf("publish step not seen: %+v", jobs[1].Steps[0])
	}
}

// Keys under on: sit at two spaces and end in a colon, exactly like jobs. Only
// the jobs: section holds jobs.
func TestKeysUnderOnAreNotJobs(t *testing.T) {
	src := "on:\n  push:\n    tags:\n      - 'v*'\n  workflow_dispatch:\n" +
		"permissions:\n  contents:\n" +
		"jobs:\n  release:\n    steps:\n      - name: Build\n        run: make\n"
	if got, want := shape(ParseWorkflow(src)), "release[Build]"; got != want {
		t.Fatalf("jobs = %q, want %q", got, want)
	}
}

// A line deeper than a job key before any job has been opened belongs to no
// job. It is dropped, not attached to a job that does not exist yet.
func TestLineBeforeTheFirstJobBelongsToNoJob(t *testing.T) {
	src := "jobs:\n    if: stray\n  a:\n    if: real\n    steps:\n      - name: s\n        run: x\n"
	jobs := ParseWorkflow(src)
	if got, want := shape(jobs), "a[s]"; got != want {
		t.Fatalf("jobs = %q, want %q", got, want)
	}
	if jobs[0].If != "real" {
		t.Errorf("job a if = %q, want %q", jobs[0].If, "real")
	}
}

// List items under a job key other than steps: (a block-form needs:, here) are
// not steps. A phantom step would be one FC-RC checks and the runner never
// runs.
func TestListItemsOutsideStepsAreNotSteps(t *testing.T) {
	src := "jobs:\n  npm:\n    needs:\n      - channel\n      - build\n" +
		"    steps:\n      - name: Publish\n        run: npm publish\n"
	if got, want := shape(ParseWorkflow(src)), "npm[Publish]"; got != want {
		t.Fatalf("jobs = %q, want %q", got, want)
	}
}

// Only keys at a step's own indent are the step's keys. A key nested deeper
// (an env: mapping's entry) must not overwrite the step's name or if, and a
// step-depth line before the first "- " item belongs to no step.
func TestOnlyStepDepthKeysAreStepKeys(t *testing.T) {
	src := "jobs:\n  npm:\n    steps:\n        name: stray\n" +
		"      - name: Publish\n        if: steps.channel.outputs.channel == 'stable'\n" +
		"        env:\n          name: nested\n          if: always()\n        run: npm publish\n"
	jobs := ParseWorkflow(src)
	if got, want := shape(jobs), "npm[Publish]"; got != want {
		t.Fatalf("jobs = %q, want %q", got, want)
	}
	if s := jobs[0].Steps[0]; s.If != StableStepCond {
		t.Errorf("step if = %q, want %q", s.If, StableStepCond)
	}
}
