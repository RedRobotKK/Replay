package regression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The frozen mutant catalogue must actually run somewhere automatic.
//
// internal/mutation carries 72 mutants as data and thaws them one at a time to
// ask whether the check that caught each one still works. Its own doc comment
// says the point is that "a defect that was fixed once cannot return quietly".
//
// It never ran. It is behind `//go:build mutation`, and no job in ci.yml passed
// `-tags mutation`, so `go test ./...` skipped the package on every push since
// it was written. 72 mutants, zero executions — the catalogue was itself an
// instance of the defect class it exists to freeze.
//
// It was not laziness either. Run locally it takes 659 seconds, past Go's
// 10-minute default timeout, so the obvious invocation dies with
// "panic: test timed out after 10m0s" at roughly M66 of 72. Anyone who tried it
// casually would have concluded it was broken.
//
// These tests are cheap and run in the normal suite, which is the point: the
// expensive job is what proves the mutants die, and this is what proves the
// expensive job is still wired up.

func ciWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the CI workflow: %v", err)
	}
	return string(b)
}

// ciCommands returns only the lines CI would execute.
//
// The reason these guards exist at all is that a capability can sit in the
// repository looking wired and never run. Reading the workflow as one blob
// reproduced that exactly: an audit replaced the mutation step with
// `echo skipping`, left the real command one line above as a comment, and both
// MC1 and MC2 stayed green while the catalogue stopped running entirely. The
// guard whose stated job is "this is what proves the expensive job is still
// wired up" was satisfied by a note about the job.
//
// A comment is not a command. Lines whose first non-space character is # are
// dropped, and what remains is what a runner would actually do.
func ciCommands(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(ciWorkflow(t), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		t.Fatal("the workflow has no executable lines, so these guards assert nothing")
	}
	return out
}

// mutationRunLine finds the executable line that runs the catalogue.
func mutationRunLine(t *testing.T) (string, bool) {
	t.Helper()
	for _, line := range ciCommands(t) {
		if strings.Contains(line, "-tags mutation") {
			return line, true
		}
	}
	return "", false
}

// minMutationTimeout is the floor the catalogue's -timeout must clear.
//
// Two measurements, both real. 659s on a quiet laptop, and 1531s on the same
// tree under concurrent load. A CI runner is slower than either, so the floor
// is set well above the slower of the two rather than at a round number that
// happens to be bigger than the faster one.
//
// It exists because MC2 used to check that the TOKEN -timeout appeared and
// never what followed it: `-timeout 1s` passed, which is the exact failure its
// own comment describes — a job that dies on the clock, red for a reason
// unrelated to the commit, until somebody switches the check off.
const minMutationTimeout = 30 * time.Minute

// MC1: some job runs the catalogue.
func TestMC1_CIRunsTheFrozenMutantCatalogue(t *testing.T) {
	if _, ok := mutationRunLine(t); !ok {
		t.Error("no EXECUTABLE line in ci.yml passes -tags mutation, so internal/mutation " +
			"is excluded from every automated run and its frozen mutants are never asked " +
			"whether the guards that killed them still work. A commented-out command does " +
			"not count: that is how this guard was defeated.")
	}
}

// MC2: it is given longer than Go's default timeout.
//
// The failure mode this prevents is worse than not running it: a job that dies
// at 10 minutes is red for a reason that has nothing to do with the code under
// test, and a check that goes red on unrelated work is how a check gets
// switched off. installer-drift's comment in the same file makes exactly that
// argument.
func TestMC2_TheCatalogueIsGivenTimeToFinish(t *testing.T) {
	line, ok := mutationRunLine(t)
	if !ok {
		t.Skip("MC1 covers the absent case")
	}
	fields := strings.Fields(line)
	var raw string
	for i, f := range fields {
		if f == "-timeout" && i+1 < len(fields) {
			raw = fields[i+1]
			break
		}
		if v, found := strings.CutPrefix(f, "-timeout="); found {
			raw = v
			break
		}
	}
	if raw == "" {
		t.Fatalf("the mutation run sets no -timeout, so it inherits Go's 10-minute default "+
			"and dies partway through:\n  %s", strings.TrimSpace(line))
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatalf("-timeout %q is not a duration Go will accept: %v", raw, err)
	}
	// The value, not the token. `-timeout 1s` used to pass.
	if d < minMutationTimeout {
		t.Errorf("-timeout is %s; the catalogue has been measured at 659s on a quiet "+
			"machine and 1531s under load, and a CI runner is slower than either. Below %s "+
			"the job dies on the clock, which is red for a reason that has nothing to do "+
			"with the commit under test.", d, minMutationTimeout)
	}
}

// MC3: every mutant in the catalogue names the test that kills it.
//
// A mutant with no killedBy is a mutant nobody has to keep working, and the
// harness would report it as a survivor with no way to tell whether that is a
// hole or an equivalent mutant.
func TestMC3_EveryMutantNamesItsKiller(t *testing.T) {
	path := filepath.Join(repoRoot(t), "internal", "mutation", "testdata", "mutants.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the catalogue: %v", err)
	}
	var cat struct {
		Mutants []struct {
			ID       string   `json:"id"`
			Name     string   `json:"name"`
			KilledBy []string `json:"killedBy"`
		} `json:"mutants"`
	}
	if err := json.Unmarshal(b, &cat); err != nil {
		t.Fatalf("parsing the catalogue: %v", err)
	}
	// 72 is what the tree carried when this guard was written and what the
	// 659-second local run scored. A drop means mutants were deleted rather
	// than fixed, which is the one edit this file cannot let through quietly.
	if len(cat.Mutants) < 72 {
		t.Errorf("the catalogue holds %d mutants, was 72; mutants are added when a defect "+
			"is frozen and never removed", len(cat.Mutants))
	}
	for _, m := range cat.Mutants {
		if len(m.KilledBy) == 0 {
			t.Errorf("mutant %s (%s) names no killing test", m.ID, m.Name)
		}
	}
}

// MC4: main's CI runs are never cancelled.
//
// cancel-in-progress applied to every ref, so each merge killed the previous
// merge's run. Measured on 2026-09-09: of fourteen consecutive main commits,
// eleven had a CANCELLED run and two completed. The branch every release is cut
// from was the least-verified ref in the repository.
//
// "Every pull request was green" is a different claim. A pull request is tested
// against its base; main is what those merges add up to, and with six PRs
// touching the same three files that difference is where an interaction would
// show. The job most likely to be killed is the slowest — here the 25-minute
// mutation catalogue, whose entire purpose is catching a guard that stopped
// working while nobody was looking.
//
// On a pull request the cancellation is still correct, so this asserts the
// exemption rather than the absence of the setting.
// MC5: main's runs are not superseded while queued either.
//
// cancel-in-progress only governs a RUNNING job. With a shared concurrency
// group, GitHub cancels a PENDING run when a newer one queues behind it, so a
// 25-minute job and merges minutes apart still left most main runs cancelled —
// while queued rather than while running, which reads identically in the run
// list and leaves the same gap in verification. Giving each main commit its own
// group is the half that actually does the work.
func TestMC5_MainGetsItsOwnConcurrencyGroup(t *testing.T) {
	for _, l := range ciCommands(t) {
		if !strings.Contains(l, "group:") || !strings.Contains(l, "concurrency") && !strings.Contains(l, "ci-") {
			continue
		}
		if !strings.Contains(l, "group:") {
			continue
		}
		if strings.Contains(l, "github.sha") {
			return // main is keyed per commit
		}
		t.Errorf("the concurrency group does not vary by commit on main, so a queued run "+
			"is superseded by the next merge and cancel-in-progress never gets a say:\n  %s",
			strings.TrimSpace(l))
		return
	}
	t.Error("no concurrency group line found in the workflow")
}

func TestMC4_MainCIRunsAreNotCancelled(t *testing.T) {
	var line string
	for _, l := range ciCommands(t) {
		if strings.Contains(l, "cancel-in-progress") {
			line = l
			break
		}
	}
	if line == "" {
		// No setting at all means nothing is cancelled, which satisfies the
		// property this test is about.
		return
	}
	// The POLARITY is asserted, not the presence of two words. `github.ref ==
	// 'refs/heads/main'` cancels main and nothing else — the maximally wrong
	// setting — and the first version of this test passed for it, because it
	// only grepped for the substrings `github.ref` and `refs/heads/main`.
	if strings.Contains(line, "github.ref") && strings.Contains(line, "refs/heads/main") &&
		!strings.Contains(line, "!=") {
		t.Errorf("cancel-in-progress names main but not with `!=`, so it may be cancelling "+
			"exactly the ref it should protect:\n  %s", strings.TrimSpace(line))
	}
	if strings.Contains(line, "true") && !strings.Contains(line, "github.ref") {
		t.Error("cancel-in-progress is unconditionally true, so every merge to main " +
			"cancels the previous merge's run and main is the least-verified ref in the " +
			"repository. Exempt main: cancel-in-progress: ${{ github.ref != 'refs/heads/main' }}")
	}
	if strings.Contains(line, "github.ref") && !strings.Contains(line, "refs/heads/main") {
		t.Errorf("cancel-in-progress is conditional but does not name main:\n  %s",
			strings.TrimSpace(line))
	}
}
