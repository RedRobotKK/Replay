package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `replay advise` told a reader holding Ollama logs that they had no data.
//
// Pointed straight at a directory of server*.log files it answered
// "no .jsonl transcripts found" and exited 1. The files are right there, the
// parser for them has been in the tree and tested since it was written, and
// `replay burn` reads them. Only advise did not look.
//
// It is FD-11 one command over: the reader is told, accurately, about a file
// format they do not have, and nothing about the one they do.
//
// What advise can honestly say about that surface is the substance here, and
// most of it is a refusal. docs/evidence/ollama-cache-ceiling-2026-09-08.md
// establishes that n_past appears in an Ollama log ONLY when the whole prompt
// was already resident: all 586 requests carrying one on that corpus are
// back-off cases, where llama.cpp finds every token cached and re-evaluates
// one because it must evaluate at least one per slot. A cache figure computed
// over them is a measurement of a population defined by having been cached.
//
// So advise names the surface, points at the command that does read it, and
// says why it will not turn that log into cache advice.

func ollamaDir(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "transcript", "ollamadata", "server.log")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("no Ollama fixture in this tree: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "server.log"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// adviseSaw is everything the reader sees, which is not everything the buffers
// hold.
//
// main() writes the returned error to stderr itself:
//
//	if err := run(...); err != nil { fmt.Fprintln(os.Stderr, "replay:", err) }
//
// so a test reading only the two buffers is reading a strict subset of the
// output and can fail an assertion the shipped binary satisfies. These three
// first did exactly that. The helper reproduces main's line rather than
// paraphrasing it, so if that ever stops being how errors reach the user, this
// stops being how the test reads them.
func adviseSaw(t *testing.T, dir string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := run([]string{"advise", dir}, &stdout, &stderr)
	if err != nil {
		fmt.Fprintln(&stderr, "replay:", err)
	}
	return stdout.String() + stderr.String()
}

// AO1: a directory of Ollama logs is recognised, not called empty.
func TestAO1_AdviseNamesOllamaRatherThanReportingNothing(t *testing.T) {
	dir := ollamaDir(t)
	out := adviseSaw(t, dir)
	if strings.Contains(out, "no .jsonl transcripts found") && !strings.Contains(strings.ToLower(out), "ollama") {
		t.Fatalf("a directory of Ollama server logs was reported as holding no data at "+
			"all, with no mention of the surface the reader is actually on:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "ollama") {
		t.Errorf("advise did not name Ollama:\n%s", out)
	}
}

// AO2: the reader is pointed at the command that does read them.
func TestAO2_ItNamesTheCommandThatReadsThem(t *testing.T) {
	dir := ollamaDir(t)
	out := adviseSaw(t, dir)
	if !strings.Contains(out, "replay burn") {
		t.Errorf("Ollama logs were found and the reader is not told which command reads "+
			"them:\n%s", out)
	}
}

// AO3: it does not offer cache advice it cannot support.
//
// The refusal is the finding. An advisor that produced a hit rate here would
// be reporting a number computed over requests selected for having been
// cached — which is the shape of every measurement this repository has had to
// retract.
func TestAO3_ItRefusesCacheAdviceAndSaysWhy(t *testing.T) {
	dir := ollamaDir(t)
	out := adviseSaw(t, dir)
	low := strings.ToLower(out)
	if !strings.Contains(low, "n_past") {
		t.Errorf("the refusal does not name the field it is about:\n%s", out)
	}
	if !strings.Contains(low, "already") && !strings.Contains(low, "selected") {
		t.Errorf("the refusal does not say WHY the log cannot support a cache figure, so "+
			"it reads as a missing feature rather than a measurement that would be "+
			"wrong:\n%s", out)
	}
}

// AO4: a .log that is not an Ollama log does not trigger the pointer.
//
// ollamaLogsUnder counts by parsing rather than by filename, and this is what
// says so. A mutation replacing the parse with `if true` — counting every
// *.log — passed AO1 through AO3 untouched, because the fixture directory
// contains only real Ollama logs. The comment claiming the distinction
// mattered was the only thing asserting it.
//
// It matters because the message is a referral. Sending a reader to
// `replay burn` over a Caddy access log gets them told a second time, by a
// second command, that there is nothing here — which is worse than the
// original refusal, not better, because now they have followed an instruction
// to get it.
func TestAO4_ANonOllamaLogDoesNotSendTheReaderToBurn(t *testing.T) {
	dir := t.TempDir()
	// Shaped to be tempting: the right extension, plausible server output,
	// even the word "model". Nothing the Ollama parser can open a block with.
	if err := os.WriteFile(filepath.Join(dir, "server.log"), []byte(
		"2026-09-11 10:00:01 INF request served path=/v1/model status=200\n"+
			"2026-09-11 10:00:02 INF request served path=/health status=200\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := adviseSaw(t, dir)
	if strings.Contains(out, "replay burn") {
		t.Errorf("a log with no Ollama request blocks in it sent the reader to "+
			"`replay burn`, which will tell them the same nothing a second time:\n%s", out)
	}
	if !strings.Contains(out, "no .jsonl transcripts found") {
		t.Errorf("the plain refusal is the right answer here and was not given:\n%s", out)
	}
}

// AO5: other files beside the log do not change the count.
//
// This began as a test for a .log suffix filter, written because
// guard-reachability reported that filter UNREACHED. Forcing the condition
// showed the filter was INERT as well: neutralise it and the count is
// identical, because the parser already returns nothing for a README. The
// filter went, and this stayed — it asserts the property the filter was
// supposed to provide, which the parser provides on its own.
//
// That is the AO4 principle applied to its own implementation. A function
// whose whole point is to count by parsing rather than by name had a name
// check in front of it.
func TestAO5_NonLogFilesAreSkippedByName(t *testing.T) {
	dir := ollamaDir(t)
	// Two files the parser opens and finds no request block in.
	for name, body := range map[string]string{
		"README.md":  "# notes\n",
		"config.yml": "model: llama3\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := adviseSaw(t, dir)
	if !strings.Contains(out, "1 Ollama server log(s)") {
		t.Errorf("the count changed when non-log files were added beside the log, so "+
			"something other than an Ollama request block is being counted as one:\n%s", out)
	}
}
