// Package workflowread is a minimal reader for this repository's GitHub
// workflow style, shared by internal/regression's release_channel_test.go and
// internal/blackbox's mock run of the publish jobs (FC-RC). It is test
// tooling and is not part of the binary.
//
// It is a leaf on purpose: it imports regexp and strings and nothing else, so
// a test package that imports it inherits no other package's reach (see
// internal/regression home_isolation_test.go, which follows imports).
//
// The module has no third-party dependencies on purpose, so there is no YAML
// parser. This reads the shape these workflows are written in: jobs at two
// spaces, step items at six, step keys at eight, `run: |` and `with:` blocks
// deeper. It is not a YAML parser and does not claim to be; actionlint in CI is
// what proves the files are valid YAML.
package workflowread

import (
	"regexp"
	"strings"
)

// ChannelScript classifies a release tag as stable or prerelease, or refuses
// it. Every registry-publishing job runs it before publishing.
const ChannelScript = "scripts/release-channel.sh"

// StableStepCond and StableJobCond are the conditions a publishing step and a
// publishing job must carry.
const (
	StableStepCond = "steps.channel.outputs.channel == 'stable'"
	StableJobCond  = "needs.channel.outputs.channel == 'stable'"
)

// WorkflowStep is one step of a job, as far as FC-RC needs to see it.
type WorkflowStep struct {
	Index    int
	Name, ID string
	If, Uses string
	Run      string
	With     map[string]string
}

// WorkflowJob is one job: its name, job-level if and needs, and steps.
type WorkflowJob struct {
	Name  string
	If    string
	Needs string
	Steps []WorkflowStep
}

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

func inlineValue(s string) string {
	v := strings.TrimSpace(s)
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		v = v[1 : len(v)-1]
	}
	return v
}

// ParseWorkflow reads the jobs of a workflow file's text.
func ParseWorkflow(src string) []WorkflowJob {
	lines := strings.Split(src, "\n")
	var jobs []WorkflowJob
	var job *WorkflowJob
	var step *WorkflowStep
	inJobs, inSteps := false, false
	block := ""      // key whose block is being read: "run" or "with"
	blockIndent := 0 // indent of the key that opened the block
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		ind := indentOf(line)
		if block != "" {
			if trimmed == "" || ind > blockIndent {
				switch block {
				case "run":
					step.Run += strings.TrimPrefix(line, strings.Repeat(" ", blockIndent+2)) + "\n"
				case "with":
					if k, v, ok := strings.Cut(trimmed, ":"); ok && !strings.HasPrefix(trimmed, "#") {
						step.With[strings.TrimSpace(k)] = inlineValue(v)
					}
				}
				continue
			}
			block = ""
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if ind == 0 {
			inJobs = trimmed == "jobs:"
			continue
		}
		if !inJobs {
			continue
		}
		if ind == 2 && strings.HasSuffix(trimmed, ":") {
			jobs = append(jobs, WorkflowJob{Name: strings.TrimSuffix(trimmed, ":")})
			job, step, inSteps = &jobs[len(jobs)-1], nil, false
			continue
		}
		if job == nil {
			continue
		}
		if ind == 4 {
			inSteps = trimmed == "steps:"
			k, v, _ := strings.Cut(trimmed, ":")
			switch k {
			case "if":
				job.If = strings.TrimSpace(v)
			case "needs":
				job.Needs = strings.TrimSpace(v)
			}
			continue
		}
		if !inSteps {
			continue
		}
		kv := trimmed
		if ind == 6 && strings.HasPrefix(trimmed, "- ") {
			job.Steps = append(job.Steps, WorkflowStep{Index: len(job.Steps), With: map[string]string{}})
			step = &job.Steps[len(job.Steps)-1]
			kv = strings.TrimPrefix(trimmed, "- ")
			ind = 8
		}
		if step == nil || ind != 8 {
			continue
		}
		k, v, _ := strings.Cut(kv, ":")
		v = strings.TrimSpace(v)
		switch k {
		case "name":
			step.Name = inlineValue(v)
		case "id":
			step.ID = inlineValue(v)
		case "if":
			step.If = v
		case "uses":
			step.Uses = strings.TrimSpace(strings.SplitN(v, "#", 2)[0])
		case "run":
			if v == "|" || v == "|-" {
				block, blockIndent = "run", 8
			} else {
				step.Run = v + "\n"
			}
		case "with":
			if v == "" {
				block, blockIndent = "with", 8
			} else {
				inner := strings.TrimSuffix(strings.TrimPrefix(v, "{"), "}")
				for _, part := range strings.Split(inner, ",") {
					if a, b, ok := strings.Cut(part, ":"); ok {
						step.With[strings.TrimSpace(a)] = inlineValue(b)
					}
				}
			}
		}
	}
	return jobs
}

// registryRun and RegistryUses match a step that uploads to a package
// registry. GitHub releases are not in them: those are matched by the
// githubRelease pair, because a prerelease is allowed to become a GitHub
// prerelease.
var registryRun = regexp.MustCompile(`\bnpm\s+publish\b|\byarn\s+publish\b|\bpnpm\s+publish\b|\btwine\s+upload\b|\bdocker\s+push\b|\bcargo\s+publish\b|\bgem\s+push\b|\bpoetry\s+publish\b|\buv\s+publish\b|\bflit\s+publish\b`)

// RegistryUses matches a `uses:` action that publishes to a package registry.
var RegistryUses = regexp.MustCompile(`^(pypa/gh-action-pypi-publish|docker/build-push-action|JS-DevTools/npm-publish)@`)
var githubReleaseRun = regexp.MustCompile(`\bgh\s+release\s+(create|upload|edit)\b|\bgoreleaser\s+release\b`)
var githubReleaseUses = regexp.MustCompile(`^(goreleaser/goreleaser-action|softprops/action-gh-release|ncipollo/release-action)@`)

// PublishesToRegistry reports whether a step uploads to a package registry.
func PublishesToRegistry(s WorkflowStep) bool {
	return registryRun.MatchString(s.Run) || RegistryUses.MatchString(s.Uses)
}

// PublishesGitHubRelease reports whether a step creates or uploads to a GitHub release.
func PublishesGitHubRelease(s WorkflowStep) bool {
	return githubReleaseRun.MatchString(s.Run) || githubReleaseUses.MatchString(s.Uses)
}

// IsChannelGuard reports whether a step is the channel guard: `id: channel`,
// running ChannelScript, with no ${{ }} expression in its shell text.
func IsChannelGuard(s WorkflowStep) bool {
	return s.ID == "channel" && strings.Contains(s.Run, ChannelScript) && !strings.Contains(s.Run, "${{")
}
