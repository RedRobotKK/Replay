package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/workflowread"
)

// FC-RC. A prerelease tag never reaches a package registry.
//
// release.yml runs on every pushed v* tag and goreleaser marks a tag with a
// -suffix as a GitHub prerelease (`prerelease: auto`). publish-shims.yml runs
// when that Release workflow completes and, until 2026-10-20, published to
// npm and PyPI with no look at the tag at all. npm publish without --tag puts
// the version under the dist-tag `latest`, so v1.0.0-rc.1 would have become
// what `npx replay-doctor` installs for everyone, from a release GitHub itself
// labels as not ready.
//
// The policy these tests hold: scripts/release-channel.sh classifies a tag as
// stable or prerelease, or refuses it (the same grammar as
// selfupdate.ValidTag and install.sh). Every job that publishes to a package
// registry runs it before its first publishing command and conditions every
// later step on `stable`; a job-level `if:` repeats the decision from a
// separate `channel` job. There is no opt-in path for prereleases. Adding one
// means an explicit non-latest dist-tag, and TestFCRC5 refuses `--tag latest`
// and a bare publish of a prerelease alike.
//
// What these tests cannot show: how GitHub evaluates an `if:` expression, and
// what head_branch holds on a real workflow_run. The mock run below evaluates
// the one expression form the workflow uses, by this file's reading of it.
// Only a real prerelease tag shows the runner agreeing.
//
// The tests that execute anything (the script under sh, dash and bash, and
// the mock run of the publish jobs against shims) live in
// internal/blackbox/release_channel_test.go, behind the mutation build tag,
// because os/exec is confined to it (cmd/replay TestX402_ExecIsConfinedTo...).

func readWorkflow(t *testing.T, name string) []workflowread.WorkflowJob {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return workflowread.ParseWorkflow(string(raw))
}

func workflowFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), ".github", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml") {
			out = append(out, e.Name())
		}
	}
	return out
}

// FC-RC3: in every workflow, every job that publishes to a package registry
// runs the channel guard first and conditions every later step on `stable`,
// at step level and at job level.
func TestFCRC3_EveryRegistryPublishIsBehindTheChannelGuard(t *testing.T) {
	var publishingJobs int
	for _, file := range workflowFiles(t) {
		jobs := readWorkflow(t, file)
		hasChannelJob := false
		for _, j := range jobs {
			if j.Name == "channel" {
				for _, s := range j.Steps {
					if workflowread.IsChannelGuard(s) {
						hasChannelJob = true
					}
				}
			}
		}
		for _, j := range jobs {
			first := -1
			for _, s := range j.Steps {
				if workflowread.PublishesToRegistry(s) {
					first = s.Index
					break
				}
			}
			if first < 0 {
				continue
			}
			publishingJobs++
			where := file + " job " + j.Name
			guard := -1
			for _, s := range j.Steps {
				if workflowread.IsChannelGuard(s) {
					guard = s.Index
					break
				}
			}
			if guard < 0 || guard > first {
				t.Errorf("%s: no step `id: channel` running %s (with the tag from env, not a ${{ }} "+
					"expression) before its first registry publish (step %d). A prerelease or malformed "+
					"tag reaches the registry.", where, workflowread.ChannelScript, first)
				continue
			}
			if j.Steps[guard].If != "" {
				t.Errorf("%s: the guard step itself is conditional (%q), so it can be skipped", where, j.Steps[guard].If)
			}
			for _, s := range j.Steps[guard+1:] {
				if !strings.Contains(s.If, workflowread.StableStepCond) {
					t.Errorf("%s: step %d %q runs after the guard without `if: %s` (has %q)",
						where, s.Index, s.Name+s.Uses, workflowread.StableStepCond, s.If)
				}
				if strings.Contains(s.If, "||") || regexp.MustCompile(`\b(always|failure|cancelled)\(`).MatchString(s.If) {
					t.Errorf("%s: step %d condition %q can be true without the guard saying stable", where, s.Index, s.If)
				}
			}
			for _, s := range j.Steps[:guard] {
				if workflowread.PublishesToRegistry(s) || workflowread.PublishesGitHubRelease(s) {
					t.Errorf("%s: step %d publishes before the guard", where, s.Index)
				}
			}
			if !hasChannelJob {
				t.Errorf("%s: the workflow has no `channel` job running the guard, so there is no job-level decision", file)
			}
			if !regexp.MustCompile(`(^|[\s\[,])channel([\s\],]|$)`).MatchString(j.Needs) {
				t.Errorf("%s: needs %q does not include the channel job", where, j.Needs)
			}
			if !strings.Contains(j.If, workflowread.StableJobCond) || strings.Contains(strings.SplitN(j.If, workflowread.StableJobCond, 2)[0], "||") {
				t.Errorf("%s: job-level if %q does not require %s ahead of any ||", where, j.If, workflowread.StableJobCond)
			}
		}
	}
	if publishingJobs < 2 {
		t.Fatalf("found %d registry-publishing jobs; publish-shims.yml has npm and pypi, so this reader is not seeing them", publishingJobs)
	}
}

// FC-RC4: the job that creates the GitHub release refuses a malformed tag
// before goreleaser runs. A prerelease is allowed through: goreleaser marks it
// a prerelease, and /releases/latest, which install.sh and replay upgrade
// read, does not return one.
func TestFCRC4_TheReleaseJobRefusesAMalformedTagBeforeGoreleaser(t *testing.T) {
	var seen int
	for _, file := range workflowFiles(t) {
		for _, j := range readWorkflow(t, file) {
			first := -1
			for _, s := range j.Steps {
				if workflowread.PublishesGitHubRelease(s) {
					first = s.Index
					break
				}
			}
			if first < 0 {
				continue
			}
			seen++
			ok := false
			for _, s := range j.Steps[:first] {
				if strings.Contains(s.Run, workflowread.ChannelScript) && s.If == "" && !strings.Contains(s.Run, "${{") {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s job %s: step %d publishes a GitHub release with no unconditional %s step before it, "+
					"so a tag like v1.0.0+meta is released and then handed to publish-shims", file, j.Name, first, workflowread.ChannelScript)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no job publishes a GitHub release; this reader is not seeing release.yml")
	}
}

// FC-RC5: no npm publish names the dist-tag latest explicitly. If a
// prerelease opt-in is ever added, it carries a non-latest --tag; this keeps
// `--tag latest` from being the way it is spelled.
func TestFCRC5_NoPublishNamesLatest(t *testing.T) {
	re := regexp.MustCompile(`\bnpm\s+publish\b[^\n;&|]*--tag[ =]+latest\b`)
	for _, file := range workflowFiles(t) {
		for _, j := range readWorkflow(t, file) {
			for _, s := range j.Steps {
				if re.MatchString(s.Run) {
					t.Errorf("%s job %s step %d publishes with --tag latest", file, j.Name, s.Index)
				}
			}
		}
	}
}

// FC-RC6: .goreleaser.yaml publishes only to the GitHub release. Every one of
// these sections would push a prerelease tag to a registry or a tap that
// release.yml's guard does not stop, because the guard lets prereleases
// through to goreleaser.
func TestFCRC6_GoreleaserPublishesNowhereButTheRelease(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^(brews|homebrew_casks|scoops|winget|aurs|aur_sources|nix|krews|snapcrafts|dockers|docker_manifests|dockers_v2|kos|publishers|blobs|uploads|artifactories|chocolateys|npms|fury|gemfury|cloudsmiths|mcp):`)
	for _, m := range re.FindAllString(string(raw), -1) {
		t.Errorf(".goreleaser.yaml has %q. A prerelease tag reaches goreleaser by design, so this would "+
			"publish it; add a prerelease skip (skip_upload: auto or an equivalent) and extend this test.", m)
	}
}
