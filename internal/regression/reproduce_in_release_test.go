package regression

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// FC-REPRO. Every release tag is rebuilt and compared, by the workflow the tag
// triggers.
//
// scripts/reproduce-release.sh reproduced v0.5.4 on 2026-09-13 and v0.8.0 on
// 2026-10-08, each time because somebody ran it. RELEASE-CRITERIA.md recorded
// both as "not yet enforced": nothing ran it on a new tag, so reproducibility
// was a fact about two releases rather than a property of releases. These
// guards keep the wiring in place. internal/blackbox's TestRR_* cases prove
// what the script decides; these prove the release workflow asks it, for every
// platform goreleaser builds, in a way whose failure fails the job.

func releaseWorkflow(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("reading release.yml: %v", err)
	}
	return string(b)
}

// reproduceJob returns the executable lines of release.yml's reproduce job.
func reproduceJob(t *testing.T) []string {
	t.Helper()
	var job []string
	in := false
	for _, line := range strings.Split(releaseWorkflow(t), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// A job key sits at exactly two spaces of indent under `jobs:`.
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(trimmed, ":") {
			in = trimmed == "reproduce:"
			continue
		}
		if in {
			job = append(job, line)
		}
	}
	if len(job) == 0 {
		t.Fatal("release.yml has no `reproduce` job, so no tag is ever rebuilt and compared. " +
			"Reproducibility stays a fact about the releases somebody checked by hand.")
	}
	return job
}

// goreleaserPlatforms reads goos x goarch from .goreleaser.yaml's build.
func goreleaserPlatforms(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	list := func(key string) []string {
		m := regexp.MustCompile(`(?m)^\s*` + key + `:\s*\[([^\]]*)\]`).FindStringSubmatch(string(b))
		if m == nil {
			t.Fatalf(".goreleaser.yaml has no inline %s list; this guard cannot tell what is built", key)
		}
		var out []string
		for _, v := range strings.Split(m[1], ",") {
			out = append(out, strings.TrimSpace(v))
		}
		return out
	}
	var out []string
	for _, goos := range list("goos") {
		for _, arch := range list("goarch") {
			out = append(out, goos+"/"+arch)
		}
	}
	sort.Strings(out)
	return out
}

// RR1: the job exists, runs after the release, reads only, and runs the script.
func TestRR1_EveryTagIsRebuiltAfterRelease(t *testing.T) {
	job := reproduceJob(t)
	var needs, readOnly, runs, noCreds bool
	for _, line := range job {
		s := strings.TrimSpace(line)
		switch {
		case s == "needs: release":
			needs = true
		case s == "contents: read":
			readOnly = true
		case s == "persist-credentials: false":
			noCreds = true
		case strings.Contains(s, "scripts/reproduce-release.sh"):
			runs = true
		case strings.Contains(s, "continue-on-error"), strings.Contains(s, "|| true"):
			t.Errorf("the reproduce job tolerates failure (%q), so a release that does not "+
				"reproduce is reported and passed", s)
		// Each of these leaves RR1-RR4 green while the comparison is skipped,
		// narrowed or pointed at another revision. Found by mutation on
		// 2026-10-13: `if: false` on the step and an `exclude:` of one platform
		// both survived the first version of these guards.
		case strings.HasPrefix(s, "if:"), strings.HasPrefix(s, "- if:"):
			t.Errorf("the reproduce job carries a condition (%q); it must run on every tag", s)
		case strings.HasPrefix(s, "exclude:"), strings.HasPrefix(s, "include:"):
			t.Errorf("the reproduce matrix is edited by %q, so the platform list RR2 reads is "+
				"not the list that runs", s)
		case strings.HasPrefix(s, "ref:"), strings.HasPrefix(s, "repository:"):
			t.Errorf("the reproduce job checks out %q instead of the tag that triggered it", s)
		}
	}
	if !noCreds {
		t.Error("the reproduce job's checkout persists the job token into .git/config; it " +
			"needs no credentials there (persist-credentials: false)")
	}
	if !needs {
		t.Error("the reproduce job does not `needs: release`. It has to compare against the " +
			"assets that release published, which do not exist before that job ends.")
	}
	if !readOnly {
		t.Error("the reproduce job does not set `contents: read`. It inherits the workflow's " +
			"contents: write, and a comparison has no reason to be able to change a release.")
	}
	if !runs {
		t.Error("no executable line in the reproduce job runs scripts/reproduce-release.sh")
	}
}

// RR2: one comparison per platform goreleaser builds, no more and no fewer.
func TestRR2_EveryBuiltPlatformIsCompared(t *testing.T) {
	var got []string
	for _, line := range reproduceJob(t) {
		s := strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(s, "platform: ["); ok {
			for _, p := range strings.Split(strings.TrimSuffix(v, "]"), ",") {
				got = append(got, strings.TrimSpace(p))
			}
		}
	}
	sort.Strings(got)
	want := goreleaserPlatforms(t)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the reproduce matrix compares %v, but goreleaser builds %v. A platform that is "+
			"published and never rebuilt is a reproducibility claim nobody checks.", got, want)
	}
}

// RR3: the reproduce job is the reviewed job, whole.
//
// Every executable line, indentation included, is compared with the version
// that was reviewed. Piecemeal checks were defeated four ways by mutation on
// 2026-10-13, each leaving the guards green while the comparison was skipped
// or narrowed: `rc=0` between capture and exit; PLATFORM set to a constant, so
// all four matrix jobs checked linux/amd64; a step inserted ahead that rewrote
// the script; and `shell:` changed. The step pins `shell: bash` because a
// workflow-level `defaults: run: shell:` would otherwise choose the shell that
// runs it, and a shell is free to ignore the script.
//
// The output passes through ::stop-commands:: so a published binary's build
// info, which the script prints on a mismatch, cannot be read by the runner as
// a workflow command.
//
// Changing this job means changing this text too. That is the point.
const reviewedReproduceJob = `  reproduce:
    needs: release
    runs-on: ubuntu-latest
    permissions:
      contents: read
    strategy:
      fail-fast: false
      matrix:
        platform: [linux/amd64, linux/arm64, darwin/amd64, darwin/arm64]
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7
        with:
          go-version-file: go.mod
          cache: false
      - name: Rebuild ${{ matrix.platform }} and compare the bytes
        shell: bash
        env:
          GH_TOKEN: ${{ github.token }}
          PLATFORM: ${{ matrix.platform }}
        run: |
          rc=0
          ./scripts/reproduce-release.sh "$GITHUB_REF_NAME" "${PLATFORM%/*}" "${PLATFORM#*/}" > reproduce.log 2>&1 || rc=$?
          stop="$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
          echo "::stop-commands::$stop"
          cat reproduce.log
          echo "::$stop::"
          { echo "### $PLATFORM: exit $rc"; echo '` + "`" + `` + "`" + `` + "`" + `'; cat reproduce.log; echo '` + "`" + `` + "`" + `` + "`" + `'; } >> "$GITHUB_STEP_SUMMARY"
          if [ "$rc" != 0 ]; then
            echo "This release is already public. This check ran after publication and did not prevent it: pull or supersede ${GITHUB_REF_NAME}." | tee -a "$GITHUB_STEP_SUMMARY"
          fi
          exit "$rc"`

func TestRR3_TheReproduceJobIsTheReviewedJob(t *testing.T) {
	var got []string
	in := false
	for _, line := range strings.Split(releaseWorkflow(t), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(trimmed, ":") {
			in = trimmed == "reproduce:"
		}
		if in {
			got = append(got, strings.TrimRight(line, " "))
		}
	}
	if strings.Join(got, "\n") != reviewedReproduceJob {
		want := strings.Split(reviewedReproduceJob, "\n")
		for i := 0; i < len(got) || i < len(want); i++ {
			var g, w string
			if i < len(got) {
				g = got[i]
			}
			if i < len(want) {
				w = want[i]
			}
			if g != w {
				t.Fatalf("release.yml's reproduce job differs from the reviewed job at line %d of the job:\n got: %q\nwant: %q", i+1, g, w)
			}
		}
	}
}

// RR4: the rebuild uses goreleaser's flags.
//
// The script rebuilds with `go build` rather than goreleaser. That is a second
// statement of the build, and the place it can drift: a new -X in
// .goreleaser.yaml that the script lacks makes every release fail to
// reproduce, and the cheap reaction to that is to switch the check off.
func TestRR4_TheRebuildUsesGoreleasersFlags(t *testing.T) {
	gr, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sh, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "reproduce-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	xs := regexp.MustCompile(`-X (\S+)=`).FindAllStringSubmatch(string(gr), -1)
	if len(xs) == 0 {
		t.Fatal("no -X in .goreleaser.yaml; this guard compares nothing")
	}
	for _, x := range xs {
		if !strings.Contains(string(sh), "-X "+x[1]+"=") {
			t.Errorf("goreleaser sets -X %s and scripts/reproduce-release.sh does not", x[1])
		}
	}
	for _, flag := range []string{"-trimpath", "-s -w", "CGO_ENABLED=0"} {
		if !strings.Contains(string(gr), flag) {
			t.Errorf(".goreleaser.yaml no longer carries %q; update this guard with it", flag)
		}
		if !strings.Contains(string(sh), flag) {
			t.Errorf("goreleaser builds with %q and the rebuild does not", flag)
		}
	}
}

// RR5: goreleaser's build entry holds only keys whose effect the rebuild
// reproduces.
//
// RR4 compares flags as strings. A key it does not know about, such as goamd64,
// tags, gcflags or a second env entry, changes the bytes goreleaser produces
// without changing any string RR4 reads, and the first tag after that edit fails
// to reproduce after it is already public. This fails before the tag instead.
func TestRR5_GoreleasersBuildHoldsOnlyReproducedKeys(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{"id": true, "main": true, "binary": true, "env": true, "goos": true,
		"goarch": true, "flags": true, "ldflags": true, "mod_timestamp": true}
	var in bool
	var keys, env []string
	inEnv := false
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			in = trimmed == "builds:"
			continue
		}
		if !in {
			continue
		}
		k := strings.TrimPrefix(trimmed, "- ")
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if (indent == 4 || (indent == 2 && strings.HasPrefix(trimmed, "- "))) && strings.Contains(k, ":") {
			name := strings.TrimSpace(strings.SplitN(k, ":", 2)[0])
			keys = append(keys, name)
			inEnv = name == "env"
			continue
		}
		if inEnv && strings.HasPrefix(trimmed, "- ") {
			env = append(env, strings.TrimPrefix(trimmed, "- "))
		}
	}
	if len(keys) == 0 {
		t.Fatal("found no keys under builds:, so this guard checks nothing")
	}
	for _, k := range keys {
		if !known[k] {
			t.Errorf(".goreleaser.yaml's build sets %q, which scripts/reproduce-release.sh does not "+
				"reproduce. Teach the script and this test about it, or every tag after this edit "+
				"fails to reproduce once it is already public.", k)
		}
	}
	if strings.Join(env, ",") != "CGO_ENABLED=0" {
		t.Errorf("goreleaser builds with env %v; the rebuild sets exactly CGO_ENABLED=0", env)
	}
	if !regexp.MustCompile(`(?m)^\s*main:\s*\./cmd/replay\s*$`).Match(b) ||
		!regexp.MustCompile(`(?m)^\s*binary:\s*replay\s*$`).Match(b) {
		t.Error("goreleaser builds something other than ./cmd/replay as replay, which is what the rebuild builds")
	}
}

// RR6: release.yml is in a form this guard can read completely, and nothing
// outside the reproduce job can reach it.
//
// The first RR6 matched lines starting "env:" or "defaults:". "\"env\":",
// "env :", "'defaults':", an explicit "? env" key, a flow mapping and a merge
// key all put an env block at the workflow level past it, which a YAML parser
// confirms (Ruby's YAML, 2026-10-13; GitHub's own parser was not run). This
// module has no YAML library and adds none, so this does not parse YAML: it
// refuses every construct it cannot read and checks the structure it can.
// It is a canonical-form check, stated as one. RR3 still pins the job itself.
func validateReleaseWorkflow(text string) []string {
	var problems []string
	bad := func(n int, why string) { problems = append(problems, fmt.Sprintf("line %d: %s", n, why)) }
	var (
		quoted       = regexp.MustCompile(`"[^"]*"|'[^']*'`)
		quotedKey    = regexp.MustCompile(`^\s*(-\s+)?["'][^"']*["']\s*:`)
		spacedKey    = regexp.MustCompile(`^\s*(-\s+)?[A-Za-z0-9_.-]+\s+:(\s|$)`)
		anchorAlias  = regexp.MustCompile(`(^|[\s\[{,])[&*][A-Za-z_]`)
		flowMapping  = regexp.MustCompile(`(^|:\s+|-\s+)\{`)
		blockScalar  = regexp.MustCompile(`:\s*[|>][-+0-9]*\s*(#.*)?$`)
		topKey       = regexp.MustCompile(`^([A-Za-z0-9_-]+):(\s|$)`)
		jobKey       = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*(#.*)?$`)
		allowedTop   = map[string]bool{"name": true, "on": true, "permissions": true, "jobs": true}
		allowedJobs  = map[string]bool{"release": true, "reproduce": true}
		topSeen      = map[string]int{}
		jobsSeen     = map[string]int{}
		inJobs       bool
		scalarIndent = -1
	)
	for i, line := range strings.Split(text, "\n") {
		n := i + 1
		if strings.Contains(line, "\t") {
			bad(n, "a tab character; YAML indentation here is spaces only")
		}
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if scalarIndent >= 0 {
			if trimmed == "" || indent > scalarIndent {
				continue // block scalar content: a run script, not YAML
			}
			scalarIndent = -1
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// A document marker ("---", "...") can only stand at column 0, where
		// it fails the plain-key read below; a separate check for it was
		// redundant and could never fail on its own (mutation, 2026-10-13).
		if strings.HasPrefix(trimmed, "?") || strings.HasPrefix(trimmed, ": ") || trimmed == ":" {
			bad(n, "an explicit YAML key")
		}
		if strings.HasPrefix(trimmed, "<<") {
			bad(n, "a YAML merge key")
		}
		unquoted := quoted.ReplaceAllString(trimmed, `""`)
		if anchorAlias.MatchString(unquoted) {
			bad(n, "a YAML anchor or alias")
		}
		if flowMapping.MatchString(unquoted) {
			bad(n, "a flow mapping")
		}
		if quotedKey.MatchString(line) {
			bad(n, "a quoted key")
		}
		if spacedKey.MatchString(line) {
			bad(n, "a space before a key's colon")
		}
		switch {
		case indent == 0:
			m := topKey.FindStringSubmatch(line)
			if m == nil {
				bad(n, "a top-level line this guard cannot read as a plain key")
				inJobs = false
				break
			}
			topSeen[m[1]]++
			if !allowedTop[m[1]] {
				bad(n, fmt.Sprintf("top-level key %q; release.yml may set only name, on, permissions and jobs", m[1]))
			}
			inJobs = m[1] == "jobs"
		case inJobs && indent == 2:
			m := jobKey.FindStringSubmatch(line)
			if m == nil {
				bad(n, "a jobs entry this guard cannot read as a plain job name")
				break
			}
			jobsSeen[m[1]]++
			if !allowedJobs[m[1]] {
				bad(n, fmt.Sprintf("job %q; release.yml may hold only release and reproduce", m[1]))
			}
		}
		if blockScalar.MatchString(line) {
			scalarIndent = indent
		}
	}
	for k := range allowedTop {
		if topSeen[k] != 1 {
			problems = append(problems, fmt.Sprintf("top-level key %q appears %d times, want once", k, topSeen[k]))
		}
	}
	for k := range allowedJobs {
		if jobsSeen[k] != 1 {
			problems = append(problems, fmt.Sprintf("job %q appears %d times, want once", k, jobsSeen[k]))
		}
	}
	return problems
}

func TestRR6_TheReleaseWorkflowIsInAFormThisGuardCanRead(t *testing.T) {
	committed := releaseWorkflow(t)
	if p := validateReleaseWorkflow(committed); len(p) > 0 {
		t.Fatalf("release.yml as committed fails its own guard:\n  %s", strings.Join(p, "\n  "))
	}
	top := "\npermissions:\n  contents: write"
	if strings.Count(committed, top) != 1 {
		t.Fatal("the workflow-level permissions block moved; update the anchor this test edits")
	}
	at := func(insert string) string { return strings.Replace(committed, top, "\n"+insert+top, 1) }
	cases := map[string]string{
		"plain env":                  at("env:\n  GOCACHEPROG: /opt/x\n"),
		"quoted env key":             at("\"env\":\n  GOCACHEPROG: /opt/x\n"),
		"single-quoted defaults key": at("'defaults':\n  run:\n    shell: true {0}\n"),
		"space before the colon":     at("env :\n  GOCACHEPROG: /opt/x\n"),
		"explicit key":               at("? env\n: {GOCACHEPROG: /opt/x}\n"),
		"flow mapping value":         at("env: {GOCACHEPROG: /opt/x}\n"),
		"anchor and merge key":       at("x-env: &hostile\n  GOCACHEPROG: /opt/x\nenv:\n  <<: *hostile\n"),
		"unknown top-level key":      at("concurrency:\n  group: x\n  cancel-in-progress: true\n"),
		"tab indentation":            at("env:\n\tGOCACHEPROG: /opt/x\n"),
		"second document":            committed + "\n---\nenv:\n  GOCACHEPROG: /opt/x\n",
		"duplicate reproduce job":    strings.Replace(committed, "\n  reproduce:\n", "\n  reproduce:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo REPRODUCES\n  reproduce-moved:\n", 1),
		"extra job":                  committed + "\n  after:\n    needs: reproduce\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo ok\n",
		"duplicate jobs key":         committed + "\njobs:\n  reproduce:\n    runs-on: ubuntu-latest\n",
		"top-level sequence item":    at("- env\n"),
		"duplicate name key":         at("name: Release again\n"),
		"second reproduce, one jobs": committed + "\n  reproduce:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo REPRODUCES\n",
		"missing reproduce job":      strings.Replace(committed, "\n  reproduce:\n", "\n  reproduce_off:\n", 1),
	}
	// The same constructs inside the release job, which RR3 does not pin. None
	// reaches the reproduce job from there, but the guard's claim is that the
	// whole file is in a form it can read, so each is refused anywhere.
	job := "\n  release:\n    runs-on: ubuntu-latest\n"
	if strings.Count(committed, job) != 1 {
		t.Fatal("the release job's opening lines moved; update the anchor this test edits")
	}
	in := func(insert string) string { return strings.Replace(committed, job, job+insert, 1) }
	for name, text := range map[string]string{
		"nested quoted key":      in("    \"timeout-minutes\": 5\n"),
		"nested spaced key":      in("    timeout-minutes : 5\n"),
		"nested flow mapping":    in("    env: {X: y}\n"),
		"nested anchor":          in("    env: &shared\n      X: y\n"),
		"nested alias":           in("    env: *shared\n"),
		"nested merge key":       in("    env:\n      <<: {X: y}\n"),
		"nested explicit key":    in("    ? env\n    : y\n"),
		"nested tab":             in("    env:\n    \tX: y\n"),
		"document marker inside": in("---\n"),
		"nested block merge key": in("    env:\n      <<:\n        X: y\n"),
	} {
		cases[name] = text
	}
	for name, text := range cases {
		if text == committed {
			t.Fatalf("%s: the edit did not apply, so this case tests nothing", name)
		}
		if len(validateReleaseWorkflow(text)) == 0 {
			t.Errorf("%s: release.yml with this edit passes RR6", name)
		}
	}
}
