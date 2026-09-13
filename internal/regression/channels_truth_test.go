package regression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-CH. A channel that claims goreleaser publishes it must have a stanza.
//
// distribution/channels.json is the source of truth for the install page. Five
// rows claimed `"automation": "goreleaser"` for stanzas that do not exist in
// .goreleaser.yaml, two of them at status "building", which the file's own
// notes define as "code exists". No code existed.
//
// homebrew-tap was the worst of them: its own cta says "add a `brews:` stanza",
// so the row simultaneously claimed goreleaser automates it and that somebody
// still has to write the thing that would. A manifest that contradicts itself
// in adjacent fields is worse than one that is merely out of date, because a
// reader checks one field and stops.
//
// PASS: every goreleaser-automated channel maps to a stanza that is present.
// FAIL: the install page promises a route nothing builds.
func TestFCCH_GoreleaserChannelsHaveAStanza(t *testing.T) {
	root := repoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, "distribution", "channels.json"))
	if err != nil {
		t.Skipf("no channel manifest: %v", err)
	}
	var doc struct {
		Channels []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Automation string `json:"automation"`
		} `json:"channels"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("channels.json does not parse: %v", err)
	}
	if len(doc.Channels) == 0 {
		t.Fatal("no channels were read, so this test asserts nothing")
	}

	cfg, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("reading .goreleaser.yaml: %v", err)
	}
	has := func(stanza string) bool {
		return strings.Contains(string(cfg), "\n"+stanza+":")
	}

	// The stanza each channel needs. A channel absent from this map is one
	// whose claim nobody has checked, which is how the five got through.
	needs := map[string]string{
		"homebrew-tap": "brews",
		"aur-bin":      "aurs",
		"nur":          "nix",
		"snap":         "snapcrafts",
		"ghcr":         "dockers",
		"dockerhub":    "dockers",
		"deb-release":  "nfpms",
		"rpm-release":  "nfpms",
		"apk-release":  "nfpms",
	}

	for _, c := range doc.Channels {
		if c.Automation != "goreleaser" {
			continue
		}
		stanza, known := needs[c.ID]
		if !known {
			continue // github-releases, self-update and the like need no stanza
		}
		present := has(stanza)
		switch {
		case present:
			// Fine. The route exists.
		case c.Status == "planned" || c.Status == "skipped" || c.Status == "blocked":
			// Also fine: the row says the work has not been done.
		default:
			t.Errorf("channel %q is status %q and claims goreleaser automates it, but "+
				"there is no `%s:` stanza in .goreleaser.yaml. The install page would "+
				"promise a route nothing builds.", c.ID, c.Status, stanza)
		}
	}
}
