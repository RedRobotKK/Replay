package regression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FC-CB. "building" means not yet published, so a channel this repository
// publishes automatically cannot be building.
//
// distribution/channels.json defines its own vocabulary, and the definition is
// unambiguous: "building = code exists, not yet published". Six rows carried
// that status while the automation in this tree publishes every one of them on
// every tag. scripts/readme-install/gen.go renders building rows under
// "Coming", so README told a reader that .deb, .rpm, .apk, npm and PyPI were
// on their way, eight lines above a paragraph saying the packages are on the
// releases page. Both sentences were generated from the same file.
//
// The evidence is in the repository, which is why this test needs no network:
// .goreleaser.yaml's nfpms block emits deb, rpm, apk and archlinux on tag, and
// .github/workflows/publish-shims.yml has npm and pypi publish jobs. A status
// claiming those artifacts do not exist yet contradicts the code that makes
// them.
//
// PASS: no building row is published by this repository's own automation.
// FAIL: the README promises as "Coming" something the last tag already shipped.
func TestFCCB_BuildingChannelsAreNotAlreadyPublished(t *testing.T) {
	root := filepath.Join("..", "..")

	raw, err := os.ReadFile(filepath.Join(root, "distribution", "channels.json"))
	if err != nil {
		t.Fatalf("reading channels.json: %v", err)
	}
	var m struct {
		Channels []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Automation string `json:"automation"`
		} `json:"channels"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("channels.json does not parse: %v", err)
	}
	if len(m.Channels) == 0 {
		t.Fatal("no channels were read, so this test asserts nothing")
	}

	// What goreleaser emits on a tag.
	gr, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("reading .goreleaser.yaml: %v", err)
	}
	formats := map[string]bool{}
	if i := strings.Index(string(gr), "nfpms:"); i >= 0 {
		blk := string(gr)[i:]
		if j := strings.Index(blk, "formats:"); j >= 0 {
			for _, line := range strings.Split(blk[j:], "\n")[1:] {
				s := strings.TrimSpace(line)
				if !strings.HasPrefix(s, "- ") {
					break
				}
				formats[strings.TrimPrefix(s, "- ")] = true
			}
		}
	}
	if len(formats) == 0 {
		t.Fatal("no nfpms formats were parsed, so half this test asserts nothing")
	}

	// What the shim workflow publishes on a tag.
	wf, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "publish-shims.yml"))
	if err != nil {
		t.Fatalf("reading publish-shims.yml: %v", err)
	}
	jobs := map[string]bool{}
	for _, mm := range regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*):$`).FindAllStringSubmatch(string(wf), -1) {
		jobs[mm[1]] = true
	}
	if !jobs["npm"] && !jobs["pypi"] {
		t.Fatal("no publish jobs were parsed, so half this test asserts nothing")
	}

	for _, c := range m.Channels {
		if c.Status != "building" {
			continue
		}
		// deb-release -> deb, rpm-release -> rpm, apk-release -> apk.
		if f := strings.TrimSuffix(c.ID, "-release"); formats[f] {
			t.Errorf("channel %q is %q, but .goreleaser.yaml emits %q on every tag; "+
				"README renders building rows as \"Coming\" for something already shipped",
				c.ID, c.Status, f)
		}
		if jobs[c.ID] {
			t.Errorf("channel %q is %q, but .github/workflows/publish-shims.yml has a %q "+
				"publish job that runs on every tag", c.ID, c.Status, c.ID)
		}
	}
}

// FC-CB2. Every published package format has a row.
//
// goreleaser emits four formats and the inventory carried three. The archlinux
// package has been attached to every release since the nfpms block was added
// and appears in no row, so nothing checks it and the README cannot offer it.
func TestFCCB2_EveryBuiltPackageFormatHasAChannel(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "distribution", "channels.json"))
	if err != nil {
		t.Fatal(err)
	}
	gr, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var formats []string
	if i := strings.Index(string(gr), "nfpms:"); i >= 0 {
		blk := string(gr)[i:]
		if j := strings.Index(blk, "formats:"); j >= 0 {
			for _, line := range strings.Split(blk[j:], "\n")[1:] {
				s := strings.TrimSpace(line)
				if !strings.HasPrefix(s, "- ") {
					break
				}
				formats = append(formats, strings.TrimPrefix(s, "- "))
			}
		}
	}
	if len(formats) == 0 {
		t.Fatal("no nfpms formats were parsed, so this test asserts nothing")
	}
	body := string(raw)
	for _, f := range formats {
		if !strings.Contains(body, `"`+f+`-release"`) {
			t.Errorf("goreleaser builds a %q package on every tag and distribution/channels.json "+
				"has no %q row, so nothing verifies it and the README cannot offer it", f, f+"-release")
		}
	}
}
