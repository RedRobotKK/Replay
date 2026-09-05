package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/learn"
)

// `replay learn -out <path>` wrote wherever it was pointed, truncating whatever
// was there, at exit 0 with no backup and no warning. It fired even when no
// session could be scored. The discipline for this already existed in the code
// that edits settings.json, and had simply not been applied here.
func TestWritePolicyFileRefusesToTruncateSomethingElse(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(victim, []byte("IMPORTANT DATA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePolicyFile(victim, learn.Result{Schema: learn.PolicyFileSchema}); err == nil {
		t.Fatal("expected a refusal to overwrite a file this tool did not write")
	}
	if got, _ := os.ReadFile(victim); string(got) != "IMPORTANT DATA\n" {
		t.Fatalf("the file was modified anyway: %q", got)
	}

	// Overwriting a policy file it did write is the normal case and allowed.
	policy := filepath.Join(dir, "policy.json")
	if err := writePolicyFile(policy, learn.Result{Schema: learn.PolicyFileSchema}); err != nil {
		t.Fatal(err)
	}
	if err := writePolicyFile(policy, learn.Result{Schema: learn.PolicyFileSchema}); err != nil {
		t.Fatalf("rewriting its own policy file must work: %v", err)
	}
}
