package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FC-TC. Every workflow takes its Go from go.mod's toolchain line, so moving
// that one line moves CI, the release build and the reproduce job together.
//
// The toolchain has moved three times for security advisories (go1.25.13,
// go1.27.2, go1.27.3), and each time the change was the one line in go.mod.
// That is only true while no workflow names a version of its own. A
// setup-go step with `go-version: 1.27.2` would keep building and scanning
// with the old compiler after go.mod moved, and govulncheck in that job would
// go on reporting what the pinned toolchain fixed, or worse, the release job
// would ship a binary built by the compiler the advisory was about.
//
// The one exception is the go-latest job, which exists to run the newest Go
// release and says so with `go-version: stable`.
//
// PASS: every actions/setup-go step reads `go-version-file: go.mod` and names
// no version, except go-latest, which reads `go-version: stable` only.
// FAIL: any other version source on any setup-go step.
func TestFCTC_EverySetupGoReadsTheGoModToolchain(t *testing.T) {
	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	jobKey := regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*$`)
	versionKey := regexp.MustCompile(`^\s*(go-version|go-version-file):\s*(\S+)`)

	var checked int
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		lines := strings.Split(string(raw), "\n")
		job := ""
		for i, line := range lines {
			if m := jobKey.FindStringSubmatch(line); m != nil {
				job = m[1]
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "uses: actions/setup-go@") {
				continue
			}
			checked++
			// The step runs until the next line at or above its own "- " indent.
			stepIndent := len(line) - len(strings.TrimLeft(line, " "))
			got := map[string]string{}
			for _, next := range lines[i+1:] {
				nt := strings.TrimSpace(next)
				if nt == "" || strings.HasPrefix(nt, "#") {
					continue
				}
				if len(next)-len(strings.TrimLeft(next, " ")) <= stepIndent {
					break
				}
				if m := versionKey.FindStringSubmatch(next); m != nil {
					got[m[1]] = m[2]
				}
			}
			want := map[string]string{"go-version-file": "go.mod"}
			if job == "go-latest" {
				want = map[string]string{"go-version": "stable"}
			}
			if len(got) != len(want) || got["go-version-file"] != want["go-version-file"] || got["go-version"] != want["go-version"] {
				t.Errorf("%s:%d job %s: setup-go reads %v, want %v. A version named in a "+
					"workflow does not move when go.mod's toolchain line does, so a "+
					"security toolchain bump would leave this job on the old compiler.",
					e.Name(), i+1, job, got, want)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no actions/setup-go step found in any workflow; the scan is reading nothing")
	}
}
