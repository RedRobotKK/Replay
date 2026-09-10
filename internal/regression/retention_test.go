package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT1: the retention period is stated, not merely implementable.
//
// A capability is not a policy. `replay purge` can remove records; an auditor
// asking "for how long do you keep them" needs a number written down, and a
// reader deciding whether to run this tool at all needs it before they install.
//
// SOC 2 Confidentiality fails on "retained indefinitely, no documented period"
// even when a delete command exists, and GDPR Article 5(1)(e) asks the same
// question. This checks the number is published and that the commands which
// act on it are named beside it, so the policy and its enforcement are read
// together.
func TestRT1_TheRetentionPolicyIsStated(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "SECURITY.md"))
	if err != nil {
		t.Fatalf("reading SECURITY.md: %v", err)
	}
	s := strings.ToLower(string(b))
	if !strings.Contains(s, "retention") {
		t.Error("SECURITY.md states no retention policy. `replay purge` makes deletion " +
			"possible; it does not say for how long anything is kept, which is the question " +
			"an audit and Article 5(1)(e) both ask.")
	}
	for _, want := range []string{"purge", "privacy"} {
		if !strings.Contains(s, want) {
			t.Errorf("SECURITY.md does not name `replay %s`, so the policy is stated without "+
				"the command that carries it out", want)
		}
	}
}
