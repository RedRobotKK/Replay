//go:build mutation

package blackbox

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/selfupdate"
	"github.com/RedRobotKK/Replay/internal/workflowread"
)

// FC-RC, the executing half. internal/regression/release_channel_test.go
// holds the policy and the workflow shape; this runs scripts/release-channel.sh
// under each POSIX shell available, checks it against selfupdate.ValidTag, and
// runs publish-shims.yml's npm and pypi jobs against shims for stable,
// prerelease and malformed tags.
//
// This is a mock run, not a real tag. It evaluates the one `if:` form the
// workflow uses by this file's reading of it; how GitHub evaluates it, and
// what workflow_run's head_branch holds, only a real prerelease tag shows.

var channelCases = []struct {
	tag, want string // want "" means refused
}{
	{"v1.0.0", "stable"},
	{"v0.9.0", "stable"},
	{"v10.20.30", "stable"},
	{"v1.0.0-rc.1", "prerelease"},
	{"v0.10.0-beta", "prerelease"},
	{"v1.0.0-0", "prerelease"},
	{"V1.0.0", ""},
	{"v1.0", ""},
	{"v1.0.0+meta", ""},
	{"v1.0.0;x", ""},
	{"", ""},
	{"1.0.0", ""},
	{"v1.0.0-", ""},
	{"v1.0.0 ", ""},
	{" v1.0.0", ""},
	{"v1.0.0\n", ""},
	{"v1.0.0\nv1.0.0", ""},
	{"v1.0.0-rc.1\nx", ""},
	{"v1.0.0-rc_1", ""},
	{"v1.0.0$(id)", ""},
	{"--help", ""},
}

// posixShells are the interpreters the script is run under. dash is the
// strict one; bash in posix mode is what an ubuntu runner's sh is not, but a
// laptop's may be.
func posixShells(t *testing.T) [][]string {
	t.Helper()
	var out [][]string
	for _, s := range [][]string{{"sh"}, {"dash"}, {"bash", "--posix"}} {
		if _, err := exec.LookPath(s[0]); err == nil {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		t.Fatal("no POSIX shell found; the script cannot be exercised")
	}
	return out
}

func runChannel(t *testing.T, shell []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	script := filepath.Join(repoRoot(t), workflowread.ChannelScript)
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s does not exist, so no publishing job can ask which channel a tag is on: %v", workflowread.ChannelScript, err)
	}
	argv := append(append(append([]string{}, shell[1:]...), script), args...)
	cmd := exec.Command(shell[0], argv...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		code = ee.ExitCode()
	case err != nil:
		t.Fatalf("running %v: %v", shell, err)
	}
	return o.String(), e.String(), code
}

// FC-RC1: the classifier prints stable or prerelease, and refuses every tag
// outside the release grammar with a non-zero exit and nothing on stdout.
func TestFCRC1_TheChannelScriptClassifiesAndRefuses(t *testing.T) {
	for _, sh := range posixShells(t) {
		for _, c := range channelCases {
			out, errOut, code := runChannel(t, sh, c.tag)
			if c.want == "" {
				if code == 0 || out != "" {
					t.Errorf("%v %q: exit %d stdout %q; want a refusal with nothing on stdout", sh, c.tag, code, out)
				}
				if errOut == "" {
					t.Errorf("%v %q: refused without saying why on stderr", sh, c.tag)
				}
				continue
			}
			if code != 0 || out != c.want+"\n" {
				t.Errorf("%v %q: exit %d stdout %q stderr %q; want %q", sh, c.tag, code, out, errOut, c.want)
			}
		}
		// No argument and two arguments are refused too: a workflow that
		// lost its TAG must not read as an empty, and so refused, tag by luck.
		if _, _, code := runChannel(t, sh); code == 0 {
			t.Errorf("%v: no argument was accepted", sh)
		}
		if _, _, code := runChannel(t, sh, "v1.0.0", "v1.0.0"); code == 0 {
			t.Errorf("%v: two arguments were accepted", sh)
		}
	}
}

// FC-RC2: the script and selfupdate.ValidTag accept exactly the same tags.
// Two copies of one grammar drift; this is the tripwire.
func TestFCRC2_TheChannelScriptAgreesWithValidTag(t *testing.T) {
	sh := posixShells(t)[0]
	for _, c := range channelCases {
		_, _, code := runChannel(t, sh, c.tag)
		if (code == 0) != selfupdate.ValidTag(c.tag) {
			t.Errorf("%q: script accepted=%v, selfupdate.ValidTag=%v", c.tag, code == 0, selfupdate.ValidTag(c.tag))
		}
		if (c.want != "") != selfupdate.ValidTag(c.tag) {
			t.Errorf("%q: this table says valid=%v, selfupdate.ValidTag says %v", c.tag, c.want != "", selfupdate.ValidTag(c.tag))
		}
	}
}

func readWorkflow(t *testing.T, name string) []workflowread.WorkflowJob {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return workflowread.ParseWorkflow(string(raw))
}

// ---- mock execution ---------------------------------------------------------

var stepCondRe = regexp.MustCompile(`^(?:\$\{\{\s*)?steps\.([A-Za-z0-9_-]+)\.outputs\.([A-Za-z0-9_-]+) == '([^']*)'(?:\s*\}\})?$`)

// simulateJob runs a job's steps the way this file reads them: `run:` under
// bash -e with npm, node, gh, python3 and twine replaced by shims that log
// their arguments, `uses:` registry actions logged as ACTION lines, and the
// one `if:` form the workflows use evaluated against GITHUB_OUTPUT. Any other
// condition stops the test rather than being guessed at. The job-level `if:`
// is treated as true on purpose: this is the case where it failed open.
func simulateJob(t *testing.T, job workflowread.WorkflowJob, tag string) (log string, jobFailed bool) {
	t.Helper()
	dir := t.TempDir()
	shims := filepath.Join(dir, ".shims")
	logPath := filepath.Join(dir, ".shim.log")
	for _, d := range []string{".shims", "packaging/npm/build/replay-doctor-linux-x64", "packaging/pypi", "scripts", ".release-guard/scripts"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"npm", "node", "gh", "python3", "twine", "pip"} {
		shim := "#!/bin/sh\nprintf '%s %s\\n' \"" + name + "\" \"$*\" >> \"$SHIM_LOG\"\n"
		if err := os.WriteFile(filepath.Join(shims, name), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The guard script as the workflow would check it out, if it exists yet.
	if b, err := os.ReadFile(filepath.Join(repoRoot(t), workflowread.ChannelScript)); err == nil {
		for _, p := range []string{workflowread.ChannelScript, filepath.Join(".release-guard", workflowread.ChannelScript)} {
			if err := os.WriteFile(filepath.Join(dir, p), b, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	appendLog := func(s string) {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		// A lost line would hide a publish call from the assertions below, so
		// a failed write fails the test rather than passing it.
		if _, err := f.WriteString(s + "\n"); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	outputs := map[string]map[string]string{}
	for _, s := range job.Steps {
		if s.If != "" {
			m := stepCondRe.FindStringSubmatch(strings.TrimSpace(s.If))
			if m == nil {
				t.Fatalf("job %s step %d: the simulator does not evaluate `if: %s`", job.Name, s.Index, s.If)
			}
			if outputs[m[1]][m[2]] != m[3] {
				continue
			}
		}
		if s.Uses != "" {
			if workflowread.RegistryUses.MatchString(s.Uses) {
				appendLog("ACTION " + s.Uses)
			}
			continue
		}
		if s.Run == "" {
			continue
		}
		if strings.Contains(s.Run, "${{") {
			t.Fatalf("job %s step %d: run text holds a ${{ }} expression the simulator cannot expand", job.Name, s.Index)
		}
		stepFile := filepath.Join(dir, ".step.sh")
		outFile := filepath.Join(dir, ".github_output")
		_ = os.Remove(outFile)
		if err := os.WriteFile(stepFile, []byte(s.Run), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "-e", stepFile)
		cmd.Dir = dir
		cmd.Env = []string{
			"PATH=" + shims + ":/usr/bin:/bin",
			"HOME=" + dir,
			"TAG=" + tag,
			"GITHUB_OUTPUT=" + outFile,
			"SHIM_LOG=" + logPath,
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			appendLog("FAILED step " + s.Name + ": " + strings.TrimSpace(string(out)))
			b, _ := os.ReadFile(logPath)
			return string(b), true
		}
		if s.ID != "" {
			outputs[s.ID] = map[string]string{}
			if b, err := os.ReadFile(outFile); err == nil {
				for _, l := range strings.Split(string(b), "\n") {
					if k, v, ok := strings.Cut(l, "="); ok {
						outputs[s.ID][k] = v
					}
				}
			}
		}
	}
	b, _ := os.ReadFile(logPath)
	return string(b), false
}

var npmPublishLog = regexp.MustCompile(`(?m)^npm publish\b.*$`)

// FC-RC7: the publish jobs, run against shims, publish for a stable tag and
// for nothing else.
func TestFCRC7_MockRunPublishesOnlyForAStableTag(t *testing.T) {
	jobs := readWorkflow(t, "publish-shims.yml")
	byName := map[string]workflowread.WorkflowJob{}
	for _, j := range jobs {
		byName[j.Name] = j
	}
	npmJob, ok1 := byName["npm"]
	pypiJob, ok2 := byName["pypi"]
	if !ok1 || !ok2 {
		t.Fatalf("publish-shims.yml has no npm and pypi jobs (found %d jobs)", len(jobs))
	}

	t.Run("stable", func(t *testing.T) {
		log, failed := simulateJob(t, npmJob, "v1.2.3")
		if failed {
			t.Fatalf("npm job failed on a stable tag:\n%s", log)
		}
		pubs := npmPublishLog.FindAllString(log, -1)
		if len(pubs) != 2 {
			t.Fatalf("stable v1.2.3: want 2 npm publish calls (platform package, then replay-doctor), got %d:\n%s", len(pubs), log)
		}
		for _, p := range pubs {
			if m := regexp.MustCompile(`--tag[ =]+(\S+)`).FindStringSubmatch(p); m != nil && m[1] != "latest" {
				t.Errorf("stable publish moved off the default dist-tag: %q", p)
			}
		}
		log, failed = simulateJob(t, pypiJob, "v1.2.3")
		if failed || !strings.Contains(log, "ACTION pypa/gh-action-pypi-publish@") {
			t.Fatalf("stable v1.2.3: the PyPI publish action did not run (failed=%v):\n%s", failed, log)
		}
	})

	for _, tag := range []string{"v1.0.0-rc.1", "v0.10.0-beta"} {
		t.Run("prerelease "+tag, func(t *testing.T) {
			log, failed := simulateJob(t, npmJob, tag)
			// A prerelease is skipped, not refused: the guard says
			// prerelease and the job ends green with nothing published.
			if failed && strings.Contains(log, "FAILED step Refuse") {
				t.Errorf("%s was refused by the guard; a prerelease is a valid tag and must skip cleanly:\n%s", tag, log)
			}
			for _, p := range npmPublishLog.FindAllString(log, -1) {
				m := regexp.MustCompile(`--tag[ =]+(\S+)`).FindStringSubmatch(p)
				if m == nil || m[1] == "latest" {
					t.Errorf("%s reached npm under the default dist-tag latest: %q", tag, p)
				}
			}
			log, _ = simulateJob(t, pypiJob, tag)
			if strings.Contains(log, "ACTION pypa/gh-action-pypi-publish@") || strings.Contains(log, "twine ") {
				t.Errorf("%s reached PyPI:\n%s", tag, log)
			}
		})
	}

	for _, tag := range []string{"V1.0.0", "v1.0", "v1.0.0+meta", "v1.0.0;x", ""} {
		t.Run("malformed "+tag, func(t *testing.T) {
			for _, j := range []workflowread.WorkflowJob{npmJob, pypiJob} {
				log, failed := simulateJob(t, j, tag)
				if npmPublishLog.MatchString(log) || strings.Contains(log, "ACTION ") || strings.Contains(log, "twine ") {
					t.Errorf("job %s published for malformed tag %q:\n%s", j.Name, tag, log)
				}
				if !failed {
					t.Errorf("job %s did not fail on malformed tag %q; a refusal must be loud:\n%s", j.Name, tag, log)
				}
				if strings.Contains(log, "gh release download") {
					t.Errorf("job %s acted on malformed tag %q before refusing it:\n%s", j.Name, tag, log)
				}
			}
		})
	}
}
