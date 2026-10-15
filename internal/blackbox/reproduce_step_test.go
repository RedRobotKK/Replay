//go:build mutation

package blackbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The reproduce job's step script, as release.yml holds it, run under bash
// against a stub of scripts/reproduce-release.sh.
//
// The job runs when the release job fails as well as when it succeeds (F2,
// 2026-10-14), and a failed release may have published nothing. The step must
// keep the script's status as its own, and say the release is public only when
// it is known to be: the comparison ran and differed (exit 1), or release
// succeeded. internal/regression's RR1 and RR3 pin the job's text, including
// its `if:`; whether GitHub runs the job after a failed release is GitHub's
// scheduling and is not tested here.
func reproduceStep(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var script []string
	inJob, inRun := false, false
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(trimmed, ":") {
			inJob = trimmed == "reproduce:"
			inRun = false
			continue
		}
		if !inJob {
			continue
		}
		if line == "        run: |" {
			inRun = true
			continue
		}
		if inRun {
			if trimmed != "" && !strings.HasPrefix(line, "          ") {
				break
			}
			script = append(script, strings.TrimPrefix(line, "          "))
		}
	}
	if len(script) == 0 {
		t.Fatal("release.yml's reproduce job has no `run: |` step script to test")
	}
	return strings.Join(script, "\n") + "\n"
}

func TestRS_TheStepKeepsTheStatusAndSaysPublicOnlyWhenItIsKnown(t *testing.T) {
	step := reproduceStep(t)
	if !strings.Contains(step, "./scripts/reproduce-release.sh") {
		t.Fatal("precondition: the extracted step runs scripts/reproduce-release.sh")
	}
	const public = "This release is already public"
	const unknown = "this check compared nothing"
	for _, result := range []string{"success", "failure", "cancelled", "skipped"} {
		for _, rc := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("release_%s_exit_%d", result, rc), func(t *testing.T) {
				dir := t.TempDir()
				if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
					t.Fatal(err)
				}
				stub := "#!/bin/sh\necho \"stub: $*\"\nexit \"$STUB_RC\"\n"
				if err := os.WriteFile(filepath.Join(dir, "scripts", "reproduce-release.sh"), []byte(stub), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "step.sh"), []byte(step), 0o644); err != nil {
					t.Fatal(err)
				}
				summary := filepath.Join(dir, "summary.md")
				cmd := exec.Command("bash", "step.sh")
				cmd.Dir = dir
				cmd.Env = []string{
					"PATH=" + os.Getenv("PATH"),
					"GITHUB_REF_NAME=v9.9.9",
					"PLATFORM=linux/amd64",
					"GITHUB_STEP_SUMMARY=" + summary,
					"RELEASE_RESULT=" + result,
					fmt.Sprintf("STUB_RC=%d", rc),
				}
				out, err := cmd.CombinedOutput()
				code := 0
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					code = ee.ExitCode()
				} else if err != nil {
					t.Fatalf("bash did not run the step: %v", err)
				}
				if code != rc {
					t.Fatalf("the step exited %d where the script exited %d:\n%s", code, rc, out)
				}
				if !strings.Contains(string(out), "stub: v9.9.9 linux amd64") {
					t.Fatalf("the step did not hand the script the tag and platform:\n%s", out)
				}
				sum, err := os.ReadFile(summary)
				if err != nil {
					t.Fatal(err)
				}
				wantPublic := rc == 1 || (rc != 0 && result == "success")
				wantUnknown := rc != 0 && !wantPublic
				for _, text := range []string{string(out), string(sum)} {
					if strings.Contains(text, public) != wantPublic {
						t.Errorf("%q present: %v, want %v:\n%s", public, !wantPublic, wantPublic, text)
					}
					if strings.Contains(text, unknown) != wantUnknown {
						t.Errorf("%q present: %v, want %v:\n%s", unknown, !wantUnknown, wantUnknown, text)
					}
				}
			})
		}
	}
}
