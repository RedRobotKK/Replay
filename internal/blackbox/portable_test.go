//go:build mutation

package blackbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The matrix is a golden file compared byte for byte on every machine that
// runs the gate, so a production entry may carry no path that names the
// machine it was generated on. The first CI run of the gate (run 37152865478,
// 2026-10-03) failed on exactly this: the jev entry held the generating
// machine's repository root, and the Linux runner's differed.
func TestBB_ProductionEntriesCarryNoMachinePath(t *testing.T) {
	home := t.TempDir()
	repo := repoRoot(t)
	got := relativise(home, []string{"jev", filepath.Join(repo, "internal", "x.jsonl"), filepath.Join(home, "ledger")})
	want := []string{"jev", "$REPO/internal/x.jsonl", "$HOME/ledger"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("relativise: got %q, want %q", got, want)
	}
	body, err := os.ReadFile(filepath.Join(repo, matrixPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), repo) {
		t.Errorf("%s names this machine's repository root %s", matrixPath, repo)
	}
	if h, _ := os.UserHomeDir(); h != "" && strings.Contains(string(body), h) {
		t.Errorf("%s names this machine's home directory", matrixPath)
	}
}
