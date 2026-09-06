package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// A shortcut must not promise a command or flag the binary does not have.
//
// The TUI runs commands for people who will never type one, and prints what it
// ran so they can check it. That line is worthless the moment it names
// something that does not exist: it would be a screen inventing provenance,
// which is worse than having none, because it reads as evidence.
//
// Same rule as the surface registry, one layer up. A status cannot outrun its
// evidence; a shortcut cannot outrun the CLI.
func TestShortcuts_NameOnlyRealCommandsAndFlags(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "cmd", "replay")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var src strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		src.Write(b)
	}
	all := src.String()

	flags := map[string]bool{}
	for _, m := range regexp.MustCompile(
		`\.(?:String|Int|Int64|Bool|Duration|Float64)\("([a-z0-9-]+)"`).FindAllStringSubmatch(all, -1) {
		flags[m[1]] = true
	}
	cmds := map[string]bool{}
	for _, m := range regexp.MustCompile(`case "([a-z-]+)"`).FindAllStringSubmatch(all, -1) {
		cmds[m[1]] = true
	}
	if len(flags) == 0 || len(cmds) == 0 {
		t.Fatal("found no commands or flags to check against, so this test would pass over " +
			"nothing, which is the shape of a check that cannot fail")
	}

	for _, s := range tui.Shortcuts() {
		if !cmds[s.Command] {
			t.Errorf("shortcut %q runs %q, which is not a command this binary has. The "+
				"screen would print a command line that does nothing", string(s.Key), s.Command)
		}
		for _, f := range s.Flags {
			if !strings.HasPrefix(f, "--") {
				continue
			}
			if name := strings.TrimPrefix(f, "--"); !flags[name] {
				t.Errorf("shortcut %q passes %s to %s, and no such flag exists",
					string(s.Key), f, s.Command)
			}
		}
	}
}
