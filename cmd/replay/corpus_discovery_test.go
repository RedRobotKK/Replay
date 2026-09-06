package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSession puts one transcript under root/<project>/ and returns root.
func writeSession(t *testing.T, root, project string) string {
	t.Helper()
	dir := filepath.Join(root, project)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(p, []byte(`{"type":"user"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// Discovery must look everywhere a transcript is plausibly written, not only
// the one path this project happened to be built against.
//
// It searched exactly one root: $CLAUDE_CONFIG_DIR (or ~/.claude) + /projects.
// A user whose agent writes under ~/.config/claude, or who has a second
// install, got "No transcripts found" over a machine holding thousands of
// sessions, and no way to tell that from having none.
func TestDiscovery_FindsTranscriptsOutsideTheOneKnownRoot(t *testing.T) {
	home := t.TempDir()
	// Not ~/.claude/projects: the XDG-style location.
	writeSession(t, filepath.Join(home, ".config", "claude", "projects"), "proj-a")

	roots := defaultTranscriptRoots(home)
	if len(roots) == 0 {
		t.Fatal("transcripts exist under this home and discovery found none. " +
			"An unsearched location is indistinguishable from an empty machine")
	}
}

// Every root that holds transcripts must be returned, not the first one.
//
// A user with sessions in two places was silently reported on one of them, and
// a cost total that omits half a corpus is worse than no total: it is wrong
// and looks right.
func TestDiscovery_ReturnsEveryRootThatHoldsTranscripts(t *testing.T) {
	home := t.TempDir()
	writeSession(t, filepath.Join(home, ".claude", "projects"), "proj-a")
	writeSession(t, filepath.Join(home, ".config", "claude", "projects"), "proj-b")

	roots := defaultTranscriptRoots(home)
	if len(roots) < 2 {
		t.Errorf("two roots hold transcripts, discovery returned %d: %v. Reporting on "+
			"a subset of the corpus produces a total that is wrong and looks right",
			len(roots), roots)
	}
}

// A root with no transcripts in it is not a root.
func TestDiscovery_IgnoresAnEmptyRoot(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	if roots := defaultTranscriptRoots(home); len(roots) != 0 {
		t.Errorf("an empty projects directory is not a corpus: %v", roots)
	}
}

// When nothing is found anywhere, say where was searched and ask.
//
// The old message named one directory and moved on to the command list. It
// could not distinguish "you have no transcripts" from "yours are somewhere I
// did not look", and it gave the user no way to say which.
func TestDiscovery_WhenNothingIsFoundItSaysWhereItLookedAndAsks(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer

	explainNoCorpus(home, &out)
	got := out.String()

	for _, want := range []string{".claude", "projects"} {
		if !strings.Contains(got, want) {
			t.Errorf("the message must name the places searched, or the user cannot tell "+
				"an empty machine from an unsearched one. missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "REPLAY_TRANSCRIPTS") {
		t.Errorf("the message must say how to point Replay at a corpus it did not find:\n%s", got)
	}
}

// An explicit location wins over discovery, and works when discovery fails.
func TestDiscovery_AnExplicitLocationIsHonoured(t *testing.T) {
	home := t.TempDir()
	elsewhere := writeSession(t, filepath.Join(t.TempDir(), "somewhere-else"), "proj-c")
	t.Setenv("REPLAY_TRANSCRIPTS", elsewhere)

	roots := defaultTranscriptRoots(home)
	if len(roots) != 1 || roots[0] != elsewhere {
		t.Errorf("REPLAY_TRANSCRIPTS must be honoured over discovery; got %v, want [%s]",
			roots, elsewhere)
	}
}

// The list of places searched must not repeat one.
//
// claudeConfigDir(home) resolves to ~/.claude by default, and ~/.claude is also
// a literal candidate, so the same path appeared twice. A "here is where I
// looked" message that names one directory twice reads as a bug in the tool at
// the exact moment the user is deciding whether to trust it.
func TestDiscovery_TheSearchedListHasNoDuplicates(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	explainNoCorpus(home, &out)

	seen := map[string]bool{}
	for _, ln := range strings.Split(out.String(), "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.Contains(ln, string(filepath.Separator)) || strings.Contains(ln, "=") {
			continue
		}
		if seen[ln] {
			t.Errorf("path listed twice: %s\n%s", ln, out.String())
		}
		seen[ln] = true
	}
}

// The empty state must tell apart two situations that are not the same.
//
// "You have never run Claude Code here" and "you have run it but there are no
// sessions yet" produce identical output today, and the useful next step is
// different for each. This is the installer's advertised first command, so on a
// fresh machine this branch IS the product's first impression.
func TestDiscovery_TheEmptyStateKnowsWhichKindOfEmptyItIs(t *testing.T) {
	// No agent directory at all.
	fresh := t.TempDir()
	var a bytes.Buffer
	explainNoCorpus(fresh, &a)
	if !strings.Contains(a.String(), "not installed") && !strings.Contains(a.String(), "have not run") {
		t.Errorf("a machine with no Claude Code directory should be told that is what "+
			"happened, not handed a path list:\n%s", a.String())
	}

	// Agent installed, no sessions recorded yet.
	installed := t.TempDir()
	if err := os.MkdirAll(filepath.Join(installed, ".claude", "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	explainNoCorpus(installed, &b)
	if a.String() == b.String() {
		t.Error("an installed agent with no sessions reads the same as a machine that " +
			"has never seen one. The next step is different for each")
	}
	if !strings.Contains(b.String(), "no sessions") {
		t.Errorf("say that the agent is there and the sessions are not:\n%s", b.String())
	}
}
