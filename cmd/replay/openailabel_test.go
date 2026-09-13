package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/proxy"
)

// The OpenAI-compatible path carries its label where it is offered, not only
// where it is explained.
//
// RELEASE-CRITERIA.md gates 1.0 on this family being "labelled EXPERIMENTAL,
// UNMASKED wherever it is offered", and the two flags below are where it is
// offered. -upstream is the only setting that sends traffic to an
// OpenAI-compatible provider at all, and -mask is the setting whose name makes
// a promise this build does not keep on that traffic.
//
// Prose documentation was never the gap. docs/SURFACES.md has said "NOT
// masked" since the path landed, and an operator who types `replay serve
// -mask -upstream https://api.deepseek.com` and reads the flag help has seen
// every word Replay is going to show them before their first request goes out.
// A warning that exists only in a file the reader did not open is the same
// shape as a guard that exists only when a flag is set.
func TestOL1_ServeHelpLabelsTheOpenAICompatiblePathOnTheFlagsThatOfferIt(t *testing.T) {
	var out, errOut bytes.Buffer
	_ = run([]string{"serve", "-h"}, &out, &errOut)
	help := out.String() + errOut.String()

	// The flag help is one block per flag, so the label has to be inside the
	// block it qualifies. Reading from the flag's own line to the next flag is
	// what makes this an assertion about -upstream and -mask rather than about
	// the page containing the words somewhere.
	for _, flag := range []string{"-upstream", "-mask"} {
		block := helpBlock(t, help, flag)
		if !strings.Contains(block, "EXPERIMENTAL, UNMASKED") {
			t.Errorf("%s offers the OpenAI-compatible path and does not label it:\n%s", flag, block)
		}
		if !strings.Contains(block, proxy.ChatCompletionsPath) {
			t.Errorf("%s must name the path the label applies to:\n%s", flag, block)
		}
		// UNMASKED alone does not tell a reader their API keys are in flight.
		if !strings.Contains(block, "in clear") {
			t.Errorf("%s says UNMASKED without saying what it costs the reader:\n%s", flag, block)
		}
	}
}

// helpBlock returns one flag's help text: its own line plus every line up to
// the next flag. flag.PrintDefaults writes "  -name type" then a tab-indented
// description, so a new flag is the next line starting with two spaces and a
// hyphen.
func helpBlock(t *testing.T, help, flag string) string {
	t.Helper()
	lines := strings.Split(help, "\n")
	start := -1
	for i, l := range lines {
		if f := strings.Fields(l); len(f) > 0 && f[0] == flag && strings.HasPrefix(l, "  -") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("serve help has no %s flag at all:\n%s", flag, help)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "  -") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}
