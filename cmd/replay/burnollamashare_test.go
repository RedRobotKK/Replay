package main

import (
	"os"
	"path/filepath"
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
	if s.hasCached {
		t.Errorf("a cached share of %.2f%% was reported from %d fully-labelled requests.\n"+
			"      n_past appears only on requests whose prompt was already resident, so\n"+
			"      this is a measurement of a population selected for having been cached,\n"+
			"      not of the cache. burn's own comment says so; its condition checks\n"+
			"      whether OTHER requests lacked the field instead.\n"+
			"      See docs/evidence/ollama-cache-ceiling-2026-09-08.md.", s.cached*100, s.requests)
	}
}
