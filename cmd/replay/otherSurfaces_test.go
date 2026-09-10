package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The empty state must look at the other agents before declaring nothing.
//
// `defaultTranscriptRoots` knows only about Claude Code. `codexRoots` and the
// Ollama log glob have existed for as long as `codex` and `burn` have, and the
// empty-state path has never consulted either. So somebody with hundreds of
// Codex rollout logs on disk who installs Replay and runs it is told
// "Claude Code is not installed here, or has never run" — which is true, and
// is the wrong answer to what they asked.
//
// It is the worst case to get wrong because Codex is the richest surface there
// is: it reports a cache-write field Anthropic does not, reasoning tokens
// separately, and a quota counter that actually moves.

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir reads USERPROFILE on Windows, so HOME alone leaves the
	// lookup pointed at the developer's real home.
	t.Setenv("USERPROFILE", home)
	t.Setenv(transcriptsEnv, "")
	return home
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// OS1: Codex logs on disk are named, with the command that reads them.
func TestOS1_CodexIsFoundWhenClaudeIsNot(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".codex", "sessions", "s.jsonl"), "{}\n")

	var b strings.Builder
	explainNoCorpus(home, &b)
	out := b.String()
	if !strings.Contains(strings.ToLower(out), "codex") {
		t.Errorf("Codex rollout logs are on disk and the empty state does not mention "+
			"Codex at all:\n%s", out)
	}
	if !strings.Contains(out, "replay codex") && !strings.Contains(out, "replay burn") {
		t.Errorf("the reader is not told which command reads them:\n%s", out)
	}
}

// OS2: Ollama logs are named too.
func TestOS2_OllamaIsFoundWhenClaudeIsNot(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".ollama", "logs", "server.log"), "x\n")

	var b strings.Builder
	explainNoCorpus(home, &b)
	out := strings.ToLower(b.String())
	if !strings.Contains(out, "ollama") {
		t.Errorf("Ollama logs are on disk and the empty state does not mention them:\n%s", b.String())
	}
}

// OS3: with nothing else on disk, the message is unchanged.
//
// The detour must not fire on a machine that genuinely has no agent data, or
// it becomes noise appended to every empty run.
func TestOS3_NothingElseMeansNoDetour(t *testing.T) {
	home := withHome(t)

	var b strings.Builder
	explainNoCorpus(home, &b)
	out := strings.ToLower(b.String())
	for _, absent := range []string{"codex", "ollama"} {
		if strings.Contains(out, absent) {
			t.Errorf("a machine with no %s data was told about %s:\n%s", absent, absent, b.String())
		}
	}
	if !strings.Contains(out, "looked in") {
		t.Errorf("the ordinary empty-state message was lost:\n%s", b.String())
	}
}

// OS4: an empty directory is not a corpus.
//
// `.codex/sessions` existing with nothing in it is what a machine that once
// installed Codex looks like. Pointing that reader at `replay codex` sends
// them to a second empty report.
func TestOS4_AnEmptyDirectoryDoesNotCount(t *testing.T) {
	home := withHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".codex", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	explainNoCorpus(home, &b)
	if strings.Contains(strings.ToLower(b.String()), "codex") {
		t.Errorf("an empty .codex/sessions was reported as a corpus:\n%s", b.String())
	}
}

// OS5: a surface Replay cannot read yet is named, and no command is offered.
//
// `~/.grok` and `~/.cursor` are known to `replay discover`, but Replay does
// not parse Grok's /responses wire (FD-5) and Cursor's local history carries
// structure with no usage. Naming them is honest — the reader learns their
// data was seen. Offering a command would be OS4's defect wearing a different
// hat: a second empty report, this time with the tool's word behind it.
func TestOS5_AnUnreadableSurfaceIsNamedWithoutACommand(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".grok", "updates.jsonl"), "{}\n")

	var b strings.Builder
	explainNoCorpus(home, &b)
	out := b.String()
	if !strings.Contains(strings.ToLower(out), "grok") {
		t.Errorf("Grok data is on disk and the reader is not told it was seen:\n%s", out)
	}
	if strings.Contains(out, "replay grok") || strings.Contains(out, "replay burn") {
		t.Errorf("the reader was sent to a command that cannot read Grok:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "cannot read") {
		t.Errorf("the reader is not told why no command is offered:\n%s", out)
	}
}

// OS6: readable and unreadable surfaces are distinguished, not merged.
//
// With both on disk the reader must be able to tell which one they can act on.
func TestOS6_ReadableAndUnreadableAreDistinguished(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".codex", "sessions", "s.jsonl"), "{}\n")
	mustWrite(t, filepath.Join(home, ".cursor", "x.db"), "x")

	var b strings.Builder
	explainNoCorpus(home, &b)
	out := b.String()
	low := strings.ToLower(out)
	if !strings.Contains(low, "codex") || !strings.Contains(low, "cursor") {
		t.Fatalf("both surfaces are on disk and both are not named:\n%s", out)
	}
	if !strings.Contains(out, "replay codex") {
		t.Errorf("the readable surface lost its command:\n%s", out)
	}
	if !strings.Contains(low, "cannot read") {
		t.Errorf("the unreadable surface was presented as if it were actionable:\n%s", out)
	}
}

// OS7: what the reader has comes before what they lack.
//
// Detection is only half the fix. With the Codex paragraph printed below a
// "Claude Code is not installed here" opener and a `replay serve` suggestion,
// the reader who quits after the first sentence — which is most of them —
// still learns only about a product they do not use. The found surface has to
// lead, or the routing is present and inert.
func TestOS7_TheFoundSurfaceLeads(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".codex", "sessions", "s.jsonl"), "{}\n")

	var b strings.Builder
	explainNoCorpus(home, &b)
	out := b.String()

	codex := strings.Index(out, "Codex")
	notInstalled := strings.Index(out, "not installed")
	if codex < 0 {
		t.Fatalf("Codex is not named at all:\n%s", out)
	}
	if notInstalled >= 0 && codex > notInstalled {
		t.Errorf("the reader is told what they do not have before what they do:\n%s", out)
	}
}

// OS8: with no home, nothing is looked for — and the working directory in
// particular is not looked in.
//
// os.UserHomeDir can fail. Carrying "" forward turns every probe into a
// relative path — filepath.Join("", ".codex") is ".codex" — so detection would
// resolve against whatever directory the reader ran Replay from. A repository
// that vendors a .codex fixture would make Replay announce Codex records that
// belong to the checkout, not the machine.
func TestOS8_NoHomeDoesNotSearchTheWorkingDirectory(t *testing.T) {
	// A working directory that would match every relative probe.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".codex", "sessions", "s.jsonl"), "{}\n")
	mustWrite(t, filepath.Join(dir, ".ollama", "logs", "server.log"), "x\n")
	t.Chdir(dir)

	if found := findOtherSurfaces(""); len(found) != 0 {
		t.Errorf("with no home, %d surface(s) were reported out of the working "+
			"directory: %+v", len(found), found)
	}
}
