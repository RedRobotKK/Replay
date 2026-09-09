package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Finding the other agents on this machine.
//
// `replay doctor` probed three hardcoded locations: Claude Code transcripts,
// the proxy environment variable, and its own ledger. It never asked what else
// was installed. A survey on 2026-09-09 found five agent CLIs on this machine
// that nothing in the repository recorded — Codex, Grok, Cursor, Oracle and
// OpenClaw — and two of them write usage data Replay can already read.
//
// So the tool measured one agent on a desk running six, and the reason was not
// that the data was hard to find. Nobody looked.
//
// The rule these tests exist to hold: **a tool is reported only when a file or
// binary was actually found.** A discovery that guesses is worse than none,
// because "found Codex" sends a reader to a directory that may not exist, and
// the next thing they conclude is that the whole report is decorative.

// fakeHome builds a home directory containing exactly the agent stores named,
// and nothing else.
func fakeHome(t *testing.T, stores map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for rel, content := range stores {
		full := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// DS1: an agent whose store exists is found, and counted.
func TestDS1_FindsAgentsThatArePresent(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".codex/sessions/2026/09/09/rollout-a.jsonl": "{}\n{}\n",
		".grok/sessions/abc/updates.jsonl":           `{"inputTokens":10,"cachedReadTokens":5}` + "\n",
	})
	found := discoverAgents(home)

	byName := map[string]agentFinding{}
	for _, f := range found {
		byName[f.Name] = f
	}
	for _, want := range []string{"Codex", "Grok"} {
		f, ok := byName[want]
		if !ok {
			t.Errorf("%s has a store on disk and was not found; found %v", want, names(found))
			continue
		}
		if f.Files == 0 {
			t.Errorf("%s was found but counted 0 files, so the reader cannot tell "+
				"a real store from an empty directory", want)
		}
		if f.Path == "" || !strings.Contains(f.Path, home) {
			t.Errorf("%s reports path %q, which does not name where it looked", want, f.Path)
		}
	}
}

// DS2: an agent that is not installed is not reported. This is the one.
//
// The failure mode that would make the whole feature worthless: naming a tool
// the reader does not have. A reader who is told "found Cursor" and finds no
// Cursor stops believing the transcript counts too.
func TestDS2_NeverInventsAnAgent(t *testing.T) {
	empty := t.TempDir()
	if found := discoverAgents(empty); len(found) != 0 {
		t.Fatalf("an empty home reported %d agents: %v.\nDiscovery must report what "+
			"it found, never what it expected to find.", len(found), names(found))
	}

	// And with one present, only that one.
	home := fakeHome(t, map[string]string{".codex/sessions/x/rollout-a.jsonl": "{}\n"})
	found := discoverAgents(home)
	if len(found) != 1 || found[0].Name != "Codex" {
		t.Fatalf("a home with only Codex reported %v", names(found))
	}
}

// DS3: an empty store directory is not a found agent.
//
// The directory outliving the tool is the common case: someone tries an agent,
// removes it, and the dotfile stays. Counting that as "found" reports a tool
// nobody has, which is DS2 wearing a different hat.
func TestDS3_AnEmptyStoreIsNotAnAgent(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if found := discoverAgents(home); len(found) != 0 {
		t.Errorf("an empty ~/.codex/sessions was reported as a found agent: %v", names(found))
	}
}

// DS4: every finding says what to run next, and what Replay can do with it.
//
// A list of names is not a discovery. The reader's next question is always
// "so what do I type", and a finding that cannot answer it has moved the
// problem rather than solved it.
func TestDS4_EveryFindingIsActionable(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".codex/sessions/x/rollout-a.jsonl": "{}\n",
		".grok/sessions/y/updates.jsonl":    "{}\n",
		".ollama/logs/server.log":           "line\n",
	})
	for _, f := range discoverAgents(home) {
		if f.Next == "" {
			t.Errorf("%s was found with no next step; the reader is told a tool exists "+
				"and not what to do about it", f.Name)
		}
		if f.Reads == "" {
			t.Errorf("%s says nothing about what Replay can read from it, so the reader "+
				"cannot tell a cost surface from a conversation log", f.Name)
		}
	}
}

// DS5: a surface Replay cannot price says so, rather than implying it can.
//
// Cursor has 118 transcripts on the author's machine and zero usage fields,
// twice confirmed. Listing it beside Codex with no distinction would promise a
// cost measurement its files cannot support — the exact overclaim
// docs/SURFACES.md was corrected for on 2026-09-09.
func TestDS5_ASurfaceWithoutUsageSaysSo(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".cursor/chats/a/store.db": "sqlite",
	})
	found := discoverAgents(home)
	if len(found) != 1 {
		t.Fatalf("expected Cursor only, got %v", names(found))
	}
	c := found[0]
	if c.Priceable {
		t.Error("Cursor is marked priceable; its transcripts carry no usage fields")
	}
	low := strings.ToLower(c.Reads + " " + c.Next)
	if !strings.Contains(low, "no usage") && !strings.Contains(low, "not a spend") &&
		!strings.Contains(low, "conversation") {
		t.Errorf("Cursor's entry does not tell the reader it carries no cost data: "+
			"reads=%q next=%q", c.Reads, c.Next)
	}
}

func names(fs []agentFinding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Name)
	}
	return out
}

// DS6: discovery counts what the command it recommends would read.
//
// The defect this exists to stop, found by a fresh reader within minutes of
// the feature existing: discovery scanned `~/.codex/sessions` only, while
// `codexRoots` — the function behind `replay codex`, the command discovery
// prints on the very next line — also reads `archived_sessions`. On this
// machine that was 29 files against 150. Doctor and the command it recommends
// disagreed about the same disk by 81%.
//
// `codexRoots` already carries a comment about this: `codex archive` moves a
// session between the two directories "without changing its format or its
// relevance to a bill", and a reader that knows only the first reports "a
// total that is wrong and looks right, which is the failure this project fixed
// in its own discovery once already". It was fixed there and reintroduced here.
//
// So the invariant is not "Codex has two directories". It is that **discovery
// must not undercount a surface relative to the reader it points at**, which
// is what a user checks first and what destroys trust in every other line.
func TestDS6_DiscoveryAgreesWithTheReaderItRecommends(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".codex/sessions/2026/09/09/rollout-live.jsonl":           "{}\n",
		".codex/archived_sessions/2026/08/rollout-archived.jsonl": "{}\n",
		".codex/archived_sessions/2026/07/rollout-older.jsonl":    "{}\n",
	})

	var codex *agentFinding
	for i, f := range discoverAgents(home) {
		if f.Name == "Codex" {
			codex = &discoverAgents(home)[i]
		}
	}
	if codex == nil {
		t.Fatal("Codex not found at all")
	}

	// What `replay codex` would actually read, from its own root list.
	want := len(findCodexRollouts(codexRoots(home)))
	if want != 3 {
		t.Fatalf("fixture wrong: findCodexRollouts saw %d files, expected 3", want)
	}
	if codex.Files != want {
		t.Errorf("doctor reports %d Codex files; `replay codex`, which doctor prints as "+
			"the next step, reads %d. A user checks that first, and two commands "+
			"disagreeing about their own disk discredits every other line in the report.",
			codex.Files, want)
	}
}

// DS7: a next step that names a replay command must name one that exists.
//
// The first draft of the Grok entry read `replay doctor --agents`, a flag
// nobody had built. That is the overpromise class catalogued in
// docs/design/unwired-3-branches-and-docs.md — documentation writing cheques
// the code cannot cash — written into the tool during the same session that
// catalogued it, and no test noticed. Mutating it back left every test green.
//
// So this parses the next steps for anything shaped like a replay invocation
// and checks it against the dispatch. Prose is allowed; a command that does
// not exist is not.
func TestDS7_NextStepsNameRealCommands(t *testing.T) {
	home := fakeHome(t, map[string]string{
		".codex/sessions/x/rollout-a.jsonl": "{}\n",
		".grok/sessions/y/updates.jsonl":    "{}\n",
		".ollama/logs/server.log":           "line\n",
		".cursor/chats/a/store.db":          "sqlite",
	})
	found := discoverAgents(home)
	if len(found) < 4 {
		t.Fatalf("fixture should find four agents, found %v", names(found))
	}
	for _, f := range found {
		idx := strings.Index(f.Next, "replay ")
		if idx < 0 {
			continue // prose, which is allowed when no reader exists
		}
		fields := strings.Fields(f.Next[idx:])
		if len(fields) < 2 {
			t.Errorf("%s: next step says %q but names no command", f.Name, f.Next)
			continue
		}
		cmd := strings.Trim(fields[1], ".,;:")
		var out, errb bytes.Buffer
		err := run([]string{cmd, "--help"}, &out, &errb)
		combined := out.String() + errb.String()
		if err != nil && strings.Contains(combined, "unknown command") {
			t.Errorf("%s: next step names `replay %s`, which the dispatch does not know. "+
				"A doctor that prints a command the binary rejects is worse than one that "+
				"prints nothing.", f.Name, cmd)
		}
		// And any flag it names must exist too, which is how --agents escaped.
		for _, w := range fields[2:] {
			if !strings.HasPrefix(w, "--") {
				continue
			}
			flag := strings.Trim(w, ".,;:")
			if !strings.Contains(combined, strings.TrimPrefix(flag, "-")) {
				t.Errorf("%s: next step names flag %q on `replay %s`, and --help does not "+
					"mention it", f.Name, flag, cmd)
			}
		}
	}
}
