package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// burn reports each surface on its own terms and refuses to add them up.
//
// The three metered surfaces do not measure the same quantity. A Claude Code
// token is billed or drawn against a subscription; a Codex token is drawn
// against a rate-limit window that the client will actually tell you about; an
// Ollama token is compute on this machine and is drawn against nothing. Their
// headline figures do not even agree on whether the cached prefix is inside
// them: Anthropic partitions it out, Codex nests it in, Ollama excludes it
// entirely and reports work rather than context.
//
// So a single grand total would be a number with no unit. This asserts the
// tool does not print one.
func TestBurnDoesNotSumSurfacesThatDoNotShareAUnit(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	for _, banned := range []string{"grand total", "combined total", "all surfaces total"} {
		if strings.Contains(strings.ToLower(got), banned) {
			t.Errorf("printed a %q across surfaces whose tokens are different quantities:\n%s",
				banned, got)
		}
	}
	// Each surface must be named with what it can answer, so a reader knows
	// which figure is a bill, which is a quota, and which is neither.
	for _, want := range []string{"claude-code", "codex", "ollama"} {
		if !strings.Contains(got, want) {
			t.Errorf("surface %q missing from the report:\n%s", want, got)
		}
	}
}

// The quota column tells the truth about which surface will answer.
//
// Only Codex reports a live quota reading. Anthropic's counter did not move
// under 3.09M tokens of titration, and Ollama has no quota at all. Printing a
// blank or a zero for the two that cannot answer would read as "nothing used".
func TestBurnSaysWhichSurfacesCannotReportQuota(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "not reported") && !strings.Contains(got, "none") {
		t.Errorf("a surface with no quota signal must say so rather than show a blank:\n%s", got)
	}
}

// Codex's credits-based limit reports no rolling window. The quota column
// used to render that as "0% of unknown", a measurement of nothing beside
// surfaces honestly marked "not reported". Written RED against that cell.
func TestBurnDoesNotRenderAMissingCodexWindowAsZero(t *testing.T) {
	dir := t.TempDir()
	codex := filepath.Join(dir, "codex")
	if err := os.MkdirAll(codex, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"timestamp":"2026-10-04T10:00:00.000Z","type":"session_meta","payload":{"id":"019d0f52-8ca6-7f00-0000-00000000burn","cli_version":"0.154.0"}}
{"timestamp":"2026-10-04T10:00:05.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100,"reasoning_output_tokens":40,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100,"reasoning_output_tokens":40,"total_tokens":1100},"model_context_window":258400},"rate_limits":{"limit_id":"premium","primary":null,"secondary":null,"credits":{"has_credits":false,"unlimited":false,"balance":null}}}}
`
	if err := os.WriteFile(filepath.Join(codex, "rollout-2026-10-04T10-00-00-burn.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", dir}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "0% of") {
		t.Errorf("an absent window is rendered as a measured zero:\n%s", got)
	}
	if !strings.Contains(got, "no window reported (limit premium)") {
		t.Errorf("the codex row does not say the window is absent:\n%s", got)
	}
}
