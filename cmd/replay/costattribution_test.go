package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A total has to say whose it is.
//
// `replay cost` reads Claude Code transcripts and prints one figure. On a
// machine that also runs Codex it printed "$10,783.57" and named no agent, so
// the reader had no way to tell whether that was everything they spend or one
// surface of three. On the corpus this was written against, 610,551,532 Codex
// tokens sat outside it — unpriced, and unmentioned.
//
// `replay burn` already attributes per surface. The defect is that the command
// a reader runs FIRST does not, and silence reads as completeness.

func costAttribution(t *testing.T, args ...string) string {
	t.Helper()
	var out, errs strings.Builder
	if err := runCost(args, &out, &errs); err != nil {
		t.Fatalf("cost failed: %v (%s)", err, errs.String())
	}
	return out.String()
}

// withCodexOnDisk gives the test a HOME carrying both a Claude Code corpus and
// a Codex store, which is the machine the defect was found on.
func withCodexOnDisk(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REPLAY_TRANSCRIPTS", "")

	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTranscript(t, filepath.Join(proj, "aaaa1111-x.jsonl"), "aaaa1111-x", "req-1")

	// A Codex store with a rollout in it. findOtherSurfaces requires entries,
	// because a directory that exists and is empty is what a machine which once
	// installed an agent looks like.
	codex := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(codex, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codex, "rollout.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return proj
}

// CA1: the total names the agent it priced.
func TestCA1_TheTotalNamesTheAgent(t *testing.T) {
	dir := withCodexOnDisk(t)
	got := costAttribution(t, dir)
	// On the HEADER line, not anywhere in the output. The caveat below also
	// says "Claude Code", so a whole-output Contains passed with the header
	// stripped — the same defect this repository found four times today, in a
	// test written to catch a version of it.
	header := ""
	for _, l := range strings.Split(got, "\n") {
		if strings.HasPrefix(l, "Cost per ") {
			header = l
			break
		}
	}
	if header == "" {
		t.Fatalf("no cost header line at all:\n%s", got)
	}
	if !strings.Contains(header, "Claude Code") {
		t.Errorf("the headline never says whose spend this is: %q", header)
	}
}

// CA2: and it names the surface it did NOT price, when one is on the machine.
//
// This is the half that matters. A reader with Codex traffic seeing one figure
// and no mention of Codex concludes the figure is everything.
func TestCA2_ASurfaceOutsideTheTotalIsNamed(t *testing.T) {
	dir := withCodexOnDisk(t)
	got := costAttribution(t, dir)
	if !strings.Contains(got, "Codex") {
		t.Errorf("Codex records are on this machine and the report does not "+
			"mention them, so its total reads as everything:\n%s", got)
	}
	if !strings.Contains(got, "replay burn") {
		t.Errorf("the report does not name the command that shows every surface:\n%s", got)
	}
}

// CA3: a machine running only Claude Code is not told about surfaces it does
// not have.
//
// A caveat printed unconditionally is one a reader learns to skip, and this one
// would be false: on a single-agent machine the total IS everything.
func TestCA3_NoOtherSurfaceMeansNoCaveat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REPLAY_TRANSCRIPTS", "")
	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTranscript(t, filepath.Join(proj, "bbbb2222-y.jsonl"), "bbbb2222-y", "req-2")

	got := costAttribution(t, proj)
	if strings.Contains(got, "Codex") {
		t.Errorf("a machine with no Codex records was told about Codex:\n%s", got)
	}
}

// CA4: --json carries no prose caveat.
func TestCA4_JSONCarriesNoAttributionProse(t *testing.T) {
	dir := withCodexOnDisk(t)
	got := costAttribution(t, "--json", dir)
	if strings.Contains(got, "replay burn") || strings.Contains(got, "Codex") {
		t.Errorf("a prose caveat reached --json output:\n%s", got)
	}
}

// CA5: the sentence is grammatical for one surface, two, and several.
//
// The first version joined with " and " unconditionally and produced "Codex
// and Ollama and Grok and Cursor" on a real machine.
func TestCA5_TheListReadsLikeASentence(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{[]string{"Codex"}, "Codex also has"},
		{[]string{"Codex", "Ollama"}, "Codex and Ollama also have"},
		{[]string{"Codex", "Ollama", "Grok"}, "Codex, Ollama and Grok also have"},
	} {
		var others []otherSurface
		for _, n := range c.in {
			others = append(others, otherSurface{name: n})
		}
		got := otherSurfacesNote(others)
		if !strings.Contains(got, c.want) {
			t.Errorf("for %v the note reads %q, want it to contain %q", c.in, got, c.want)
		}
	}
	if got := otherSurfacesNote(nil); got != "" {
		t.Errorf("no other surfaces produced %q", got)
	}
}

// CA6: it does not call local work spend.
//
// Ollama runs on the machine and nobody invoices for it. `replay burn` gives it
// its own column for that reason; a sentence sweeping it into "spend" is the
// same category error one report over.
func TestCA6_LocalWorkIsNotCalledSpend(t *testing.T) {
	got := otherSurfacesNote([]otherSurface{{name: "Ollama"}})
	if strings.Contains(strings.ToLower(got), "spend") {
		t.Errorf("a surface with no bill is described as spend: %q", got)
	}
}

// CA7: a write that fails is reported, not swallowed.
//
// The note is written with io.WriteString and its error was unobserved. A
// report that cannot reach its destination must say so: this repository has
// spent the day finding places where a failure looked like a success, and a
// silently truncated cost report is one a reader would act on.
func TestCA7_AFailedWriteIsNotSwallowed(t *testing.T) {
	dir := withCodexOnDisk(t)
	// Fails only once the table has been written, so the run reaches the note.
	w := &failAfter{n: 400}
	err := runCost([]string{dir}, w, &strings.Builder{})
	if err == nil {
		t.Fatalf("a failing writer produced no error; the report was truncated "+
			"silently after %d bytes", w.written)
	}
	if !strings.Contains(err.Error(), "no space") {
		t.Errorf("the error does not carry the cause: %v", err)
	}
}

// failAfter accepts n bytes and then refuses, the way a full disk or a closed
// pipe does partway through a report.
type failAfter struct {
	n       int
	written int
}

func (f *failAfter) Write(p []byte) (int, error) {
	if f.written >= f.n {
		return 0, errNoSpace
	}
	f.written += len(p)
	return len(p), nil
}

var errNoSpace = errors.New("no space left on device")
