package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/reference"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// `replay context` ranks what entered the context. Until now a reader had no
// way to tell whether their own shape was ordinary.
//
// internal/analysis/outlier.go states why the tool could not tell them: a bare
// figure is not actionable, and the pooled corpus has one member. A published
// population lifts that without anybody contributing anything — see
// docs/design/reference-distribution.md — and the system prompt is where the
// mapping is defensible, because this command's `system` row and the
// publication's "system prompt" bucket are the same cut of the same prompt.

// corpusWithSystemPrompt is the transcript this repository already ships,
// which carries a real system row at 8.0% of its context. Observed rather than
// constructed: a hand-built fixture would encode my belief about how a system
// block is labelled, and the point of the assertion is that the label the
// analyser really produces is the one the comparison really reads.
func corpusWithSystemPrompt(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRootFor(t), "internal", "transcript", "testdata", "session-redacted.jsonl")
}

// repoRootFor walks up to the module root, so the fixture path holds wherever
// the test binary runs.
func repoRootFor(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}

func contextOver(t *testing.T, args ...string) string {
	t.Helper()
	var out, errs bytes.Buffer
	if err := runContext(args, &out, &errs); err != nil {
		t.Fatalf("context failed: %v (stderr %s)", err, errs.String())
	}
	return out.String()
}

// CR1: the comparison names the population and the citation, every time.
//
// That is the safety property of the whole design. "28.8% here, 14.0% across
// 13.5M sessions (arXiv:2608.00101)" is a sentence a reader can weigh. A bare
// "roughly double the average" is not.
func TestCR1_TheComparisonCarriesItsPopulation(t *testing.T) {
	dir := corpusWithSystemPrompt(t)
	got := contextOver(t, dir)
	if !strings.Contains(got, "system prompt") {
		t.Fatalf("no reference line at all:\n%s", got)
	}
	for _, want := range []string{"across", "arXiv:2608.00101", "13.5M sessions"} {
		if !strings.Contains(got, want) {
			t.Errorf("the reference line omits %q:\n%s", want, got)
		}
	}
}

// CR2: it describes a difference and never judges one.
//
// A provider's published minimum is refuted by one counterexample. A
// population's published median is not: one machine differing from it is one
// draw, and language implying otherwise would be a claim this tool cannot
// support.
func TestCR2_TheComparisonDoesNotJudge(t *testing.T) {
	got := strings.ToLower(contextOver(t, corpusWithSystemPrompt(t)))
	for _, banned := range []string{"average", "typical", "too high", "worse", "better than", "should be"} {
		if strings.Contains(got, banned) {
			t.Errorf("the output judges rather than describes (%q):\n%s", banned, got)
		}
	}
}

// CR3: once per run, not once per session.
//
// `replay context <dir>` walks every transcript. A comparison under each of
// 1,800 tables is a comparison the reader learns to skip, which is the
// objection internal/analysis/outlier.go raises about printing one on every
// run.
func TestCR3_TheComparisonIsPrintedOnce(t *testing.T) {
	got := contextOver(t, corpusWithSystemPrompt(t))
	if n := strings.Count(got, "arXiv:2608.00101"); n != 1 {
		t.Errorf("the reference line appears %d times, want 1:\n%s", n, got)
	}
}

// CR4: --json carries no reference line.
//
// A sentence inside machine-readable output is corruption, and this command
// already refuses to put its funding line there for the same reason.
func TestCR4_JSONCarriesNoProse(t *testing.T) {
	got := contextOver(t, "--json", corpusWithSystemPrompt(t))
	if strings.Contains(got, "arXiv") {
		t.Errorf("a citation reached --json output:\n%s", got)
	}
}

// CR5: nothing to place means no line, rather than a zero share.
//
// Asserted on the function rather than through the command, because the state
// cannot be produced by a Claude Code transcript: every one observed carries a
// system row, since a request with a cache read implies a prefix the transcript
// cannot see. A test that tried to build that corpus would have been asserting
// against a fixture that does not exist, which is how three tests in this
// repository passed against the defects they named today.
//
// The guard is still real. The ledger and the Codex reader are not bound by the
// Claude Code parser's shape, and a zero share of a system prompt is a
// different claim from a session that carried none.
func TestCR5_NothingToPlaceMeansNoLine(t *testing.T) {
	if got := systemPromptLine(0, 1000); got != "" {
		t.Errorf("a corpus with no system tokens produced %q", got)
	}
	if got := systemPromptLine(100, 0); got != "" {
		t.Errorf("a corpus with no tokens at all produced %q", got)
	}
	got := systemPromptLine(288, 1000)
	if !strings.Contains(got, "28.8% here") {
		t.Errorf("a real share did not render: %q", got)
	}
	// And a metric nothing publishes places nothing, rather than comparing
	// against a zero-valued reference with no citation behind it.
	if got := referenceLine("whatever", "no-such-metric", 288, 1000); got != "" {
		t.Errorf("an unpublished metric was compared anyway: %q", got)
	}
}

// CR6: the label this reads is a constant, not a string somebody typed twice.
//
// The comparison finds its local figure by matching the context row against
// transcript.RoleSystem. If that label is ever renamed, this fails here rather
// than the feature silently disappearing from the report.
func TestCR6_TheSystemLabelIsTheOneTheAnalyserProduces(t *testing.T) {
	if transcript.RoleSystem != "system" {
		t.Fatalf("RoleSystem is %q; the reference lookup and the analyser must agree",
			transcript.RoleSystem)
	}
	if _, ok := reference.For("systemPromptShare"); !ok {
		t.Fatal("no compiled reference for systemPromptShare, so nothing can be compared")
	}
}
