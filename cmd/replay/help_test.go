package main

import (
	"bytes"
	"strings"
	"testing"
)

// `<tool> <subcommand> --help` is the first thing anyone does with a CLI they
// do not know — a person, and every coding agent. On 2026-09-05 it failed on
// all sixteen subcommands: exit 1, usage on stderr, three of them printing
// "Usage of report:" for a command not called report, and `redact --help`
// trying to open a file named --help.
//
// The contract, asserted for every subcommand:
//
//	H1  exit 0. Asking for help is not a mistake
//	H2  the text goes to stdout, so `replay cost --help | head` works
//	H3  nothing goes to stderr
//	H4  it names the command actually invoked
//
// Genuine misuse keeps the old behaviour: usage on stderr, non-zero exit. A
// help request and a mistake are different events and must stay
// distinguishable by a script.
var subcommands = []string{
	"replay", "blame", "diff", "corpus", "advise", "learn", "doctor", "rules",
	"statusline", "cost", "context", "route", "trim", "redact", "serve", "version",
}

func TestHelpOnEverySubcommand(t *testing.T) {
	for _, cmd := range subcommands {
		for _, flagName := range []string{"--help", "-h"} {
			t.Run(cmd+" "+flagName, func(t *testing.T) {
				var out, errb bytes.Buffer
				err := run([]string{cmd, flagName}, &out, &errb)

				if err != nil {
					t.Errorf("H1 %s %s returned %v; asking for help is not a mistake",
						cmd, flagName, err)
				}
				if out.Len() == 0 {
					t.Errorf("H2 %s %s wrote nothing to stdout (stderr had %q)",
						cmd, flagName, truncate(errb.String()))
				}
				if errb.Len() != 0 {
					t.Errorf("H3 %s %s wrote to stderr: %q", cmd, flagName, truncate(errb.String()))
				}
				if got := out.String(); !strings.Contains(got, cmd) {
					t.Errorf("H4 %s %s does not name the command it was asked about:\n%s",
						cmd, flagName, truncate(got))
				}
			})
		}
	}
}

// A mistake is still a mistake. If help and misuse become indistinguishable,
// a script cannot tell "you typed it wrong" from "here is how to type it".
func TestMisuseStillFails(t *testing.T) {
	var out, errb bytes.Buffer
	err := run([]string{"cost", "--no-such-flag"}, &out, &errb)
	if err == nil {
		t.Error("an unknown flag returned no error; misuse must stay distinguishable from help")
	}
	if errb.Len() == 0 {
		t.Error("an unknown flag wrote nothing to stderr; diagnostics belong there")
	}
}

// The bare command with no arguments still explains itself rather than failing
// silently, and the top-level listing is on stdout.
func TestBareInvocationExplainsItself(t *testing.T) {
	var out, errb bytes.Buffer
	_ = run([]string{"--help"}, &out, &errb)
	if out.Len() == 0 {
		t.Fatal("replay --help wrote nothing to stdout")
	}
	for _, cmd := range []string{"cost", "serve", "blame", "redact"} {
		if !strings.Contains(out.String(), cmd) {
			t.Errorf("the top-level help does not list %q", cmd)
		}
	}
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
