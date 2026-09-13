package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// OpenClaw is the one surface in the 2026 survey that was found on this
// machine rather than read out of a vendor's repository.
//
// Every other candidate the survey proposed (VS Code and its extension family,
// JetBrains, Zed, Windsurf, Continue, Aider, Gemini CLI, Goose, OpenCode, Amp,
// Crush, Qwen Code) is VERIFIED-ABSENT here, so their directory spellings are
// derived rather than observed and none of them ships. OpenClaw is present,
// and its session log was opened and counted before any of this was written.
//
// MEASURED, 2026-09-12, ~/.openclaw/agents/main/sessions, 2 session files,
// 707 entries, 447 of them assistant messages:
//
//   - every entry is one JSON object per line with a `type` and an `id`
//   - every assistant message carries `api`, `provider`, `model` and a `usage`
//     object with `input`, `output`, `cacheRead`, `cacheWrite`, `totalTokens`
//     and a nested `cost`
//   - 229 rows have a non-zero `input`, 113 have a non-zero `cacheRead`
//     summing to 5,208,111 tokens
//   - 0 rows have a non-zero `cacheWrite`, across both providers observed
//     (`openrouter`/`anthropic/claude-3.5-haiku`, 254 rows, and
//     `openclaw`/`delivery-mirror`, 193 rows)
//
// That last line is the whole reason OpenClaw ships without a command. The
// read side of the cache was counted in the millions and the write side was
// never counted at all, so the two halves of a cache bill cannot both be
// recovered from this file. It is the Cursor finding one step along: there the
// counter existed and was zero everywhere, here one counter moves and its
// partner does not, which is a narrower and more useful thing to tell a reader
// because it says which half is missing.
//
// WHY NO TEST FUNCTION BELOW IS NAMED AFTER THE PRODUCT.
//
// The first draft of these tests was. `t.TempDir()` builds its directory out
// of the test's own name, the empty state prints the roots it looked in, and
// so every one of those paths carried the string "OpenClaw". The presence
// check passed against the harness's own temp path rather than against
// anything the detector did, and the absence check could not pass at all. Both
// were measuring the test runner. The names avoid the word so that a match in
// the rendered output is a match on the detector's output.

// clawHome builds a home with one session under the given agent id.
func clawHome(t *testing.T, agent string) string {
	t.Helper()
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".openclaw", "agents", agent, "sessions", "s.jsonl"),
		`{"type":"session","version":3,"id":"s"}`+"\n")
	return home
}

// surfaceNamed returns the detected surface with this name, or nil.
func surfaceNamed(found []otherSurface, name string) *otherSurface {
	for i := range found {
		if found[i].name == name {
			return &found[i]
		}
	}
	return nil
}

// OS9: a session log is named, with no command, and the reason names the
// measured mechanism rather than shrugging.
//
// A `why:` string that said "no reader yet" would point a reader at writing a
// parser, and a parser recovers nothing here: the cacheWrite counter is on
// disk and it is zero. Saying which of the two it is decides whether the fix
// belongs in Replay or in the agent, and here it belongs in the agent.
func TestOS9_ASessionLogIsNamedWithItsMeasuredReason(t *testing.T) {
	home := clawHome(t, "main")

	var b strings.Builder
	explainNoCorpus(home, &b)
	out := b.String()
	low := strings.ToLower(out)

	if !strings.Contains(low, "openclaw") {
		t.Fatalf("session logs are on disk and the empty state does not name the "+
			"surface at all:\n%s", out)
	}
	if strings.Contains(out, "replay openclaw") || strings.Contains(out, "replay burn") {
		t.Errorf("the reader was sent to a command that cannot read this surface:\n%s", out)
	}
	// The mechanism, not the verdict. "cannot price" alone would leave the
	// reader guessing whether the file is unreadable, unparsed or unpriced.
	for _, want := range []string{"cachewrite", "zero", "cacheread"} {
		if !strings.Contains(low, want) {
			t.Errorf("the reason does not name %q, so the reader cannot tell which "+
				"half of the cache bill is missing:\n%s", want, out)
		}
	}
}

// OS10: a configured install with no session is not a corpus.
//
// This is the one that decides where the probe points. `~/.openclaw` holds
// openclaw.json, exec-approvals.json, update-check.json and a shelf of other
// loose files from the moment the tool is installed, so `hasEntries` on the
// root answers true on a machine that has never run a single session. Probing
// the root would announce "records are on this machine" to somebody who has
// none, which is OS4's defect with a different directory: a claim about the
// reader's data that their disk does not support.
func TestOS10_ConfigWithoutASessionIsNotACorpus(t *testing.T) {
	home := withHome(t)
	// Exactly the loose files a fresh install leaves, and nothing else.
	for _, name := range []string{"openclaw.json", "exec-approvals.json", "update-check.json"} {
		mustWrite(t, filepath.Join(home, ".openclaw", name), "{}\n")
	}
	// The agent tree exists but has never been written to, which is what an
	// install that was configured and never run looks like.
	if err := os.MkdirAll(filepath.Join(home, ".openclaw", "agents", "main", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := surfaceNamed(findOtherSurfaces(home), "OpenClaw"); got != nil {
		t.Errorf("an install with configuration and no session was reported as a "+
			"corpus, pointing the reader at %q", got.dir)
	}
}

// OS11: the agent id is not "main", it is whatever the reader named it.
//
// `~/.openclaw/agents/<agentId>/sessions` is the shape on this machine and
// `main` is the default agent, not the only one: the same tree carries
// `subagents/` and a `cron/jobs.json` that schedules work, and a reader who
// adds a second agent gets a second directory beside it. A probe hardcoded to
// `agents/main/sessions` would find the default install and silently report
// nothing for everyone who renamed or added an agent, which is the blind spot
// this whole file exists to refuse.
func TestOS11_TheAgentIdIsNotAlwaysTheDefault(t *testing.T) {
	home := clawHome(t, "scheduler")

	found := findOtherSurfaces(home)
	got := surfaceNamed(found, "OpenClaw")
	if got == nil {
		t.Fatalf("a session under agents/scheduler was not found; the probe is "+
			"hardcoded to the default agent id and goes blind for every other "+
			"one. found: %+v", found)
	}
	if want := filepath.Join(home, ".openclaw", "agents", "scheduler", "sessions"); got.dir != want {
		t.Errorf("the reader is pointed at %q, not at the directory their session "+
			"is actually in (%q)", got.dir, want)
	}
	if got.cmd != "" {
		t.Errorf("this surface was offered the command %q, and this build cannot "+
			"read it: the cacheWrite counter on every assistant row is zero", got.cmd)
	}
}
