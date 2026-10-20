package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A runbook is an instruction to someone who is not the author, on a machine
// that is not the author's. A flag the binary does not define is refused at
// step one, and the reader cannot tell a documentation fault from their own.
// docs/evidence/second-provider-calibration-runbook-2026-10-09.md told the
// independent operator to run `replay corpus ... --json`; corpus defines only
// --contribute and --contribute-dir, so the documented command exited 1 with
// "flag provided but not defined: -json".
//
// PASS: every flag on every `replay <cmd> ...` line inside a fenced block of a
// docs/evidence/*runbook*.md is one `replay <cmd> --help` lists.
// FAIL: a flag the subcommand does not define.
func TestRunbookCommandsUseDefinedFlags(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "docs", "evidence", "*runbook*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no runbooks found (err %v); the glob has drifted, not the docs", err)
	}

	var topOut, topErr bytes.Buffer
	_ = run([]string{"--help"}, &topOut, &topErr)
	known := map[string]bool{}
	for _, line := range strings.Split(topOut.String(), "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) >= 2 && f[0] == "replay" && !strings.HasPrefix(f[1], "<") && !strings.HasPrefix(f[1], "-") {
			known[f[1]] = true
		}
	}
	if len(known) < 10 {
		t.Fatalf("parsed only %d commands from --help; the parser has drifted", len(known))
	}

	flagsOf := map[string]map[string]bool{}
	defined := func(cmd string) map[string]bool {
		if m, ok := flagsOf[cmd]; ok {
			return m
		}
		var h, he bytes.Buffer
		_ = run([]string{cmd, "--help"}, &h, &he)
		m := map[string]bool{}
		for _, line := range strings.Split(h.String()+he.String(), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "-") {
				m[strings.TrimLeft(strings.Fields(line)[0], "-")] = true
			}
		}
		flagsOf[cmd] = m
		return m
	}

	checked := 0
	var bad []string
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		inFence := false
		for n, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			if !inFence {
				continue
			}
			// Drop trailing comments, then redirections and pipes.
			if i := strings.Index(line, " #"); i >= 0 {
				line = line[:i]
			}
			line = regexp.MustCompile(`\s[>|<]\s.*$`).ReplaceAllString(line, "")
			f := strings.Fields(line)
			if len(f) < 2 || f[0] != "replay" || !known[f[1]] {
				continue
			}
			checked++
			for _, tok := range f[2:] {
				if !strings.HasPrefix(tok, "-") || tok == "-" || tok == "--" {
					continue
				}
				name := strings.TrimLeft(tok, "-")
				if i := strings.Index(name, "="); i >= 0 {
					name = name[:i]
				}
				if name != "help" && name != "h" && !defined(f[1])[name] {
					bad = append(bad, filepath.Base(p)+":"+strconv.Itoa(n+1)+": replay "+f[1]+" "+tok+" (flag provided but not defined)")
				}
			}
		}
	}
	if checked < 3 {
		t.Fatalf("only %d runbook commands were checked; the extractor has drifted", checked)
	}
	if len(bad) > 0 {
		t.Errorf("runbook commands the binary would refuse:\n  %s", strings.Join(bad, "\n  "))
	}
}
