package regression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// MC1: some job runs the catalogue.
func TestMC1_CIRunsTheFrozenMutantCatalogue(t *testing.T) {
	ci := ciWorkflow(t)
	if !strings.Contains(ci, "-tags mutation") {
		t.Error("no job in ci.yml passes -tags mutation, so internal/mutation is excluded " +
			"from every automated run and its 72 frozen mutants are never asked whether " +
			"the guards that killed them still work")
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
	ci := ciWorkflow(t)
	idx := strings.Index(ci, "-tags mutation")
	if idx < 0 {
		t.Skip("MC1 covers the absent case")
	}
	line := ci[idx:]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	if !strings.Contains(line, "-timeout") {
		t.Errorf("the mutation run sets no -timeout, so it inherits Go's 10-minute default "+
			"and dies around mutant 66 of 72:\n  %s", line)
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
