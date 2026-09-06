package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two commands, one directory, one second apart, two numbers that cannot both
// be a count of the same thing:
//
//	replay doctor  ->  "transcripts   90 sessions across 12 projects under ~/.claude/projects"
//	replay cost    ->  "Cost per task, across 1477 transcripts ..."
//
// Neither figure is wrong. They count different things and the reader is given
// no way to know that. Claude Code writes one transcript per session at
//
//	~/.claude/projects/<project>/<sessionId>.jsonl
//
// and one more per sub-agent lane, nested underneath it at
//
//	~/.claude/projects/<project>/<sessionId>/subagents/agent-*.jsonl
//
// Measured on the machine that produced the two lines above: 91 files at the
// first level, 1403 at the second, 1494 in all, and every one of the 1403 sat
// under a `subagents` segment whose parent had a matching top-level transcript.
// So 91 is the session count and 1494 is the file count, and the reason doctor
// happened to report the former is that its glob was never recursive.
//
// That is an accident, not a design, and it is the kind of accident this
// repository has already paid for once: the share card said "sessions" when it
// meant "transcripts" and overstated the corpus roughly twentyfold. Here the
// error runs the other way — doctor's number is right and describes a fifth of
// one percent of what the next command will read.

// projectsWithLanes builds a projects directory shaped the way Claude Code
// shapes one: main-lane transcripts at the top of each project, sub-agent lanes
// nested under the session they belong to, and a dot directory that is a cache
// rather than a corpus.
func projectsWithLanes(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, ".claude", "projects")
	write := func(parts ...string) {
		p := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("alpha", "a.jsonl")
	write("alpha", "b.jsonl")
	write("alpha", "a", "subagents", "x.jsonl")
	write("alpha", "a", "subagents", "y.jsonl")
	write("beta", "c.jsonl")
	write("beta", "c", "subagents", "z.jsonl")
	// Skipped by transcriptFiles, so it must be skipped here too or the two
	// numbers this whole exercise is about would disagree again.
	write(".cache", "ignored.jsonl")
	return home
}

// TC-1: the two things the word "transcript" was covering are counted apart.
//
// PASS: 3 sessions, 3 sub-agent lanes, 2 projects.
// FAIL: any count that folds a lane into the session total, which is the shape
// of the original defect.
func TestTC1_SessionsAndAgentLanesAreCountedSeparately(t *testing.T) {
	root := filepath.Join(projectsWithLanes(t), ".claude", "projects")
	c := countTranscripts(root)
	if c.sessions != 3 {
		t.Errorf("sessions: %d, want 3 (the main-lane transcript of each session)", c.sessions)
	}
	if c.lanes != 3 {
		t.Errorf("sub-agent lanes: %d, want 3", c.lanes)
	}
	if c.projects != 2 {
		t.Errorf("projects: %d, want 2 (the dot directory is a cache, not a project)", c.projects)
	}
}

// TC-2: doctor prints both numbers and says why they differ.
//
// One number alone is what produced the contradiction: whichever it is, the
// reader has to reconcile it against the other command unaided.
func TestTC2_DoctorReconcilesItsCountWithWhatTheOtherCommandsRead(t *testing.T) {
	home := projectsWithLanes(t)
	isolateHome(t, home)
	t.Setenv(envBaseURL, "")
	var out, errOut bytes.Buffer
	if err := run([]string{"doctor"}, &out, &errOut); err != nil {
		t.Fatalf("doctor: %v (stderr: %s)", err, errOut.String())
	}
	got := out.String()
	for _, want := range []string{
		"3 sessions across 2 projects",
		"6 transcript files",
		"one per agent lane",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor must say %q so the two counts reconcile:\n%s", want, got)
		}
	}
}

// TC-3: doctor's file total is the corpus the other commands will actually read.
//
// Asserted against transcriptFiles rather than against a literal, because the
// number is only worth printing if it tracks the walker it is describing. Two
// hand-maintained counts that agree today is how this went wrong the first
// time.
func TestTC3_DoctorsFileTotalMatchesWhatTranscriptFilesWalks(t *testing.T) {
	root := filepath.Join(projectsWithLanes(t), ".claude", "projects")
	c := countTranscripts(root)
	files, err := transcriptFiles([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.sessions+c.lanes, len(files); got != want {
		t.Errorf("doctor counts %d transcript files, the reader will get %d: the number "+
			"is only honest if it tracks the same walk", got, want)
	}
}

// TC-4: a second number is never invented where there is only one.
//
// A corpus with no sub-agent lanes has a file count equal to its session count.
// Printing "3 transcript files in all" beside "3 sessions" is noise that reads
// as a distinction, which is the same failure as hiding a real one.
func TestTC4_NoReconciliationLineWhenThereIsNothingToReconcile(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".claude", "projects", "solo")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a.jsonl", "b.jsonl"} {
		if err := os.WriteFile(filepath.Join(p, n), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	isolateHome(t, home)
	t.Setenv(envBaseURL, "")
	var out, errOut bytes.Buffer
	if err := run([]string{"doctor"}, &out, &errOut); err != nil {
		t.Fatalf("doctor: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "2 sessions across 1 projects") {
		t.Errorf("doctor must still report the session count:\n%s", got)
	}
	if strings.Contains(got, "transcript files") {
		t.Errorf("with no sub-agent lanes there is no second number to print:\n%s", got)
	}
}

// TC-5: the cost report uses one word for one thing.
//
// The header has said "transcripts" since 2026-09-06, for the reason recorded
// on costHeaderLine. The two lines beneath it still said "sessions" about the
// very same files, so the command contradicted itself inside one page of
// output.
func TestTC5_CostCallsUnpricedFilesTranscriptsLikeItsOwnHeadline(t *testing.T) {
	priced := renderCost(summarise([]costUnit{{CostUSD: 1}}), 7, io.Discard, "")
	if !strings.Contains(priced, "7 further transcripts") {
		t.Errorf("the unpriced note must count the same unit the headline counts:\n%s", priced)
	}
	none := renderCost(summarise(nil), 7, io.Discard, "")
	if strings.Contains(none, "session") {
		t.Errorf("nothing priced is still a count of transcripts, not sessions:\n%s", none)
	}
}
