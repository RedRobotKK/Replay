package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// The README counts things. Counted things change.
//
// It said `replay --help` "lists all twenty-nine" while the binary listed
// thirty, and that `replay tui` puts the answers on ten screens and "Every
// screen is in docs/screens" while docs/screens held nine. Neither number was
// ever wrong when it was written; both were wrong by the time anyone read them,
// and a spelled-out number in prose has a maintenance cost with no owner.
//
// These tests give it an owner. They read the count out of the README and
// compare it to the thing being counted, so the sentence fails the build rather
// than quietly misinforming a reader who is deciding whether to trust the tool.

// numberWords covers the range a command list plausibly occupies. A count
// outside it fails loudly rather than passing unchecked.
var numberWords = map[string]int{
	"twenty-five": 25, "twenty-six": 26, "twenty-seven": 27, "twenty-eight": 28,
	"twenty-nine": 29, "thirty": 30, "thirty-one": 31, "thirty-two": 32,
	"thirty-three": 33, "thirty-four": 34, "thirty-five": 35,
	"eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
}

func readme(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("reading README: %v", err)
	}
	return string(b)
}

// RC1: "lists all <n>" is the number of commands the binary dispatches.
func TestReadmeCommandCountMatchesTheBinary(t *testing.T) {
	root := repoRoot(t)
	text := readme(t)

	m := regexp.MustCompile(`lists all ([a-z-]+)`).FindStringSubmatch(text)
	if m == nil {
		t.Fatal(`the README no longer says "lists all <n>"; this test is checking nothing`)
	}
	got, ok := numberWords[m[1]]
	if !ok {
		t.Fatalf("the README says %q commands, which is not a number this test knows. "+
			"Add it to numberWords rather than deleting the check", m[1])
	}

	// The dispatch switch is the authority, minus the spellings that are not
	// commands of their own.
	want := len(namedCommands(dispatchCommands(t, root)))
	if got != want {
		t.Errorf("the README says %s (%d) commands; the binary dispatches %d. "+
			"One of the two moved and the README is the one nobody re-runs",
			m[1], got, want)
	}
}

// namedCommands drops the flag-shaped aliases, which are spellings of a command
// rather than commands of their own.
func namedCommands(cmds map[string]bool) []string {
	var out []string
	for c := range cmds {
		switch {
		case c == "" || strings.HasPrefix(c, "-"):
			// --version, --help: spellings of a command, not commands.
			continue
		case c == "v" || c == "h":
			// Their short forms, after the leading dashes are trimmed.
			continue
		case c == "help":
			// The command that prints the list is not an entry in the list it
			// prints. `replay --help` shows no line for itself.
			continue
		}
		out = append(out, c)
	}
	return out
}

// RC2: "<n> screens" is the number of screens that exist.
func TestReadmeScreenCountMatchesTheSurface(t *testing.T) {
	text := readme(t)
	m := regexp.MustCompile(`answers on ([a-z-]+) screens`).FindStringSubmatch(text)
	if m == nil {
		t.Fatal(`the README no longer says "answers on <n> screens"; this test is checking nothing`)
	}
	got, ok := numberWords[m[1]]
	if !ok {
		t.Fatalf("the README says %q screens, which is not a number this test knows", m[1])
	}
	if want := len(tui.Shortcuts()); got != want {
		t.Errorf("the README says %s (%d) screens; Shortcuts() declares %d", m[1], got, want)
	}
}

// RC3: and it does not claim docs/screens holds all of them, because it does
// not and should not.
//
// The doctor screen reads the machine it runs on, so a committed image of it
// would be a picture of one developer's corpus presented as what the tool
// shows. It is excluded on purpose (see the unpinnable set in
// cmd/replay/screens_svg_test.go). The README's job is to say so rather than to
// promise a completeness the design deliberately declines.
func TestReadmeDoesNotOverclaimTheScreenImages(t *testing.T) {
	text := readme(t)
	if strings.Contains(text, "Every screen is in") {
		t.Error(`the README claims "Every screen is in docs/screens". The doctor ` +
			`screen is not, deliberately: it renders the reader's own machine, so ` +
			`there is nothing general to pin.`)
	}
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "docs", "screens"))
	if err != nil {
		t.Fatalf("reading docs/screens: %v", err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".svg") {
			n++
		}
	}
	if n == 0 {
		t.Fatal("docs/screens holds no images; this test is checking nothing")
	}
	if n >= len(tui.Shortcuts()) {
		t.Errorf("docs/screens holds %d images for %d screens. If doctor became "+
			"pinnable, RC3 is now the stale claim", n, len(tui.Shortcuts()))
	}
}
