package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// burnOllama withholds the cached share for the wrong reason.
//
// Its own comment says why the share is disqualified, and the reason is what
// n_past MEANS: the field appears only when the whole prompt was already
// resident, so any share computed over the requests carrying one measures a
// population selected for having been cached, pinned near (n-1)/n by the
// back-off rule. That is a property of the field, true of every Ollama log
// ever written.
//
// The suppression, though, is conditional on `unmeasured > 0` — on some OTHER
// request in the same log lacking an n_past line. On the corpus this was
// written against, 2,682 of 3,294 requests lack one, so the condition held and
// the share stayed hidden. It is not the stated reason, and a log where every
// block carries n_past satisfies neither.
//
// That log is not exotic. It is what a short session looks like: a handful of
// turns on one slot, every one of them a continuation. The reader gets a
// cached share of about 99.98% — the disqualified quantity, printed as a
// measurement, at its most flattering.
func TestBO1_TheCachedShareIsWithheldOnAFullyLabelledLog(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "ollama")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	// Three requests, each carrying n_past: the back-off shape the evidence
	// file describes, with nothing else in the file to trip the condition.
	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, []byte(
			"level=INFO msg=\"processing task\" id 0 | task 0\n"+
				"msg=llm model=registry.ollama.ai/library/llama3:8b\n"+
				"slot context shift n_past was set to 4095\n"+
				"prompt eval time =  12.00 ms /  1 tokens\n"+
				"eval time = 100.00 ms /  20 tokens\n"+
				"total time = 112.00 ms /  21 tokens | id 0 | task 0\n")...)
	}
	if err := os.WriteFile(filepath.Join(logs, "server.log"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	s := burnOllama("", dir)
	if s.requests == 0 {
		t.Fatal("the fixture parsed to no requests at all; this test asserts nothing")
	}
	// The counts are the report now, so they are what gets asserted.
	//
	// guard-reachability called the measured/unmeasured split INERT: the
	// branch ran and nothing depended on which way it went. It was right —
	// BO1 checked only that no share came back, which stays true if both
	// counters are wired to the same variable, or to nothing.
	want := "3 of 3 requests were served entirely from cache"
	if len(s.problems) != 1 || !strings.Contains(s.problems[0], want) {
		t.Errorf("expected a note reading %q; got %q", want, s.problems)
	}
	if !strings.Contains(strings.Join(s.problems, " "), "the other 0 log no reuse figure") {
		t.Errorf("the unmeasured count is not reported, so a reader cannot tell how much "+
			"of the log the first number is out of: %q", s.problems)
	}
	if s.hasCached {
		t.Errorf("a cached share of %.2f%% was reported from %d fully-labelled requests.\n"+
			"      n_past appears only on requests whose prompt was already resident, so\n"+
			"      this is a measurement of a population selected for having been cached,\n"+
			"      not of the cache. burn's own comment says so; its condition checks\n"+
			"      whether OTHER requests lacked the field instead.\n"+
			"      See docs/evidence/ollama-cache-ceiling-2026-09-08.md.", s.cached*100, s.requests)
	}
}

// BO2: a surface with no requests emits no note.
//
// `if s.requests > 0` was INERT — nothing asserted that an Ollama-less machine
// stays quiet. Without it the note reads "0 of 0 requests were served entirely
// from cache", which is a sentence about a cache on a machine that has no
// local model.
func TestBO2_NoOllamaLogsMeansNoNote(t *testing.T) {
	s := burnOllama("", t.TempDir())
	if s.requests != 0 {
		t.Fatalf("the empty fixture parsed to %d requests; this test asserts nothing", s.requests)
	}
	if len(s.problems) != 0 {
		t.Errorf("a machine with no Ollama logs was told something about its cache: %q", s.problems)
	}
}

// BO3: a log holding both kinds reports both counts.
//
// BO1's fixture is entirely measured, so it cannot tell the two counters
// apart: replace the whole if/else with `measured++` and BO1 still reads
// "3 of 3 ... the other 0", because that is what both versions produce. The
// mutant survived four tests.
//
// It matters because the ratio is the note's entire content. "612 of 3,294"
// says the reuse figure is missing from four requests in five; a build that
// counted every request as measured would say "3,294 of 3,294" and turn a
// statement about sparse logging into a statement about a well-cached machine.
func TestBO3_AMixedLogReportsBothCounts(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "ollama")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	block := func(withNPast bool) string {
		b := "level=INFO msg=\"processing task\" id 0 | task 0\n" +
			"msg=llm model=registry.ollama.ai/library/llama3:8b\n"
		if withNPast {
			b += "slot context shift n_past was set to 4095\n"
		}
		return b +
			"prompt eval time =  12.00 ms /  1 tokens\n" +
			"eval time = 100.00 ms /  20 tokens\n" +
			"total time = 112.00 ms /  21 tokens | id 0 | task 0\n"
	}
	// One of each, so the two counters cannot be the same variable.
	body := block(true) + block(false)
	if err := os.WriteFile(filepath.Join(logs, "server.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	s := burnOllama("", dir)
	if s.requests != 2 {
		t.Fatalf("fixture parsed to %d requests, wanted 2; the test asserts nothing", s.requests)
	}
	note := strings.Join(s.problems, " ")
	if !strings.Contains(note, "1 of 2 requests") {
		t.Errorf("the measured count is wrong or is counting every request:\n  %s", note)
	}
	if !strings.Contains(note, "the other 1 log no reuse figure") {
		t.Errorf("the unmeasured count is wrong; a request with no n_past line was counted "+
			"as one that had it:\n  %s", note)
	}
}
