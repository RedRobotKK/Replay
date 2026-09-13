package regression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-PX. hooks.json execs the script by path, so the script needs the bit.
//
// Found by another session's smoke run on 2026-09-13, and the way it was missed
// is the point. This hook was verified end to end earlier the same day and
// printed the tip line correctly. The verification ran:
//
//	sh plugins/replay/hooks/session-end.sh
//
// and `sh <path>` does not need the execute bit. hooks.json runs
// "${CLAUDE_PLUGIN_ROOT}/hooks/session-end.sh" directly, which does. So the
// check passed, the artifact was broken, and the difference was entirely in how
// the test invoked it versus how production does.
//
// The file was mode 100644 in the tree. Every SessionEnd on every machine that
// installed the plugin would have failed with a permission error, and the tip
// line is described in its own header as the whole distribution mechanism of
// this project.
//
// PASS: every hook command hooks.json execs by path is executable in the tree.
// FAIL: the plugin is installed and does nothing, silently, for everyone.
func TestFCPX_EveryHookScriptIsExecutable(t *testing.T) {
	root := repoRoot(t)
	hooksJSON := filepath.Join(root, "plugins", "replay", "hooks", "hooks.json")
	raw, err := os.ReadFile(hooksJSON)
	if err != nil {
		t.Skipf("no plugin hooks.json: %v", err)
	}

	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("hooks.json does not parse, so no hook can run at all: %v", err)
	}

	var checked int
	for event, groups := range doc.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if h.Type != "command" {
					continue
				}
				// "${CLAUDE_PLUGIN_ROOT}/hooks/session-end.sh" with quotes.
				cmd := strings.Trim(h.Command, `"`)
				const marker = "${CLAUDE_PLUGIN_ROOT}/"
				i := strings.Index(cmd, marker)
				if i == -1 {
					continue // not a path into the plugin; nothing to check here
				}
				rel := cmd[i+len(marker):]
				// A bare path, not `sh <path>`: that is the case that needs the bit.
				if strings.ContainsAny(rel, " \t") {
					continue
				}
				checked++
				p := filepath.Join(root, "plugins", "replay", rel)
				info, err := os.Stat(p)
				if err != nil {
					t.Errorf("%s hook points at %s, which does not exist", event, rel)
					continue
				}
				if info.Mode().Perm()&0o111 == 0 {
					t.Errorf("%s hook execs %s by path and it is mode %o. Every session on "+
						"every machine that installs this plugin fails with a permission "+
						"error, and the tip line this prints is described in its own header "+
						"as the whole distribution mechanism of this project.",
						event, rel, info.Mode().Perm())
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no hook command was checked, so this test proves nothing")
	}
	t.Logf("checked %d hook command(s)", checked)
}
