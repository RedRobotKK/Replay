package main

import (
	"os"
	"strings"
	"testing"
)

// A Codex user must be able to find the two commands that read their data.
//
// `loadSession` parses Claude Code transcripts only, so `cost`, `diff`,
// `advise`, `trim`, `route`, `ceiling` and the TUI cannot read a Codex rollout.
// The two that can are `codex` and `burn`, and neither appeared in the help
// text's opening block. Someone arriving with only ~/.codex on disk read five
// commands, none of which worked on their data, and no pointer to the two that
// did.
func TestTheHelpBlockNamesTheCommandsThatReadCodex(t *testing.T) {
	var sb strings.Builder
	if err := printUsage(&sb); err != nil {
		t.Fatal(err)
	}
	help := sb.String()
	start := strings.Index(help, "Start here")
	if start < 0 {
		t.Fatal("no 'Start here' block in the help text")
	}
	block := help[start:]
	if i := strings.Index(block, "Look closer"); i > 0 {
		block = block[:i]
	}
	for _, cmd := range []string{"replay codex", "replay burn"} {
		if !strings.Contains(block, cmd) {
			t.Errorf("the opening help block does not name %q; a Codex user has "+
				"no route from `replay --help` to the commands that read their "+
				"transcripts", cmd)
		}
	}
}

// Advice that names no file is advice nobody can follow.
//
// `replay burn` refuses to price an unpriced surface and then says
// "next: replay rules --update <file|https URL>" without naming a file or a
// URL, and no document in the repository names one either. The OpenAI rows
// that would price a Codex surface live at docs/rules/openai-2026-09-15.json,
// mentioned in exactly one evidence file and described there as an accident.
func TestTheUnpricedRemedyNamesAFile(t *testing.T) {
	b, err := os.ReadFile("burn.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	i := strings.Index(src, "rules --update")
	if i < 0 {
		t.Fatal("the unpriced remedy line is gone; this test needs rewriting")
	}
	window := src[i:min(i+600, len(src))]
	if !strings.Contains(window, "openai-2026-09-15.json") {
		t.Error("the unpriced remedy does not name a document that would work; " +
			"docs/rules/openai-2026-09-15.json carries the OpenAI rows")
	}
}

// The masking claim in `serve -h` is false, and it understates the product.
//
// maskable(path) is isMessages(path) || isResponses(path) (proxy/passthrough.go),
// and TestRES1 proves a credential on /v1/responses is masked before egress.
// /v1/responses is the path GPT-6 Astra speaks. The flag help nevertheless said
// /v1/messages was "the only shape this build masks", denying the one capability
// that works on the OpenAI path.
func TestTheUpstreamFlagDoesNotDenyResponsesMasking(t *testing.T) {
	b, err := os.ReadFile("serve.go")
	if err != nil {
		t.Fatal(err)
	}
	// Assert the truth is present, not that one wording is absent.
	//
	// A first draft banned the phrase "is the only shape", and mutation testing
	// walked straight past it: rewriting the denial as "and nothing else" left
	// every test green. A test that forbids one spelling of a false claim
	// forbids one spelling, not the claim.
	src := string(b)
	if !strings.Contains(src, "/v1/responses") {
		t.Error("serve -h does not mention /v1/responses, the path GPT-6 Astra " +
			"speaks and the one path besides /v1/messages that -mask covers")
	}
	for _, denial := range []string{"is the only shape", "and nothing else", "only shape this build masks"} {
		if strings.Contains(src, denial) {
			t.Errorf("serve -h still denies masking beyond /v1/messages via %q", denial)
		}
	}
}
