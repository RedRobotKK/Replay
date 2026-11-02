package analysis

// Guard-reachability contract tests for classifyResult. Each test makes a
// branch true and asserts the classification it must produce, so neutralising
// the guard changes the asserted class.

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A huge non-error result is content, not an overflow notice, and must not be
// classified as overflow even if it mentions an overflow marker
// (errors.go:175).
func TestClassifyResultHugeNonErrorIsNotOverflow(t *testing.T) {
	text := strings.Repeat("x", overflowMarkerMaxBytes) + " context window"
	class, ok := classifyResult(transcript.Block{Text: text}, false)
	if ok || class != "" {
		t.Fatalf("huge non-error result classified as %q (ok=%v); it is content, not a notice", class, ok)
	}
}

// An error result carrying an edit-anchor marker is a failed edit
// (errors.go:180), distinct from a plain tool failure.
func TestClassifyResultEditAnchor(t *testing.T) {
	class, ok := classifyResult(transcript.Block{Text: "string to replace not found"}, true)
	if !ok || class != ErrorEditAnchor {
		t.Fatalf("edit-anchor error classified as %q (ok=%v), want ErrorEditAnchor", class, ok)
	}
}
