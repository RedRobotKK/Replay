package main

import (
	"flag"
	"reflect"
	"testing"
)

// HB. A guard that is correct and untested, found by mutation rather than by
// reading.
//
// `hoistFlagsFor` moves flags ahead of paths so that `replay purge <dir>
// --older-than 30d` works, which is the form the README shows and the form
// people reach for. A value-taking flag must carry its value with it, because
// hoisting the flag and leaving the value behind turns the value into a path
// and the command fails on a file nobody named.
//
// The loop guards that with `i+1 < len(args)`. A mutation run on 2026-09-13
// changed it to `i+1 <= len(args)` and no test noticed, which means nothing
// exercised a value-taking flag as the LAST argument. The mutated binary
// indexes one past the end and panics.
//
// The shipped binary is correct: `replay purge --older-than` prints "flag
// needs an argument" and exits cleanly. So this is not a bug report, it is the
// stronger thing. By this repository's own standard an untested guard is
// indistinguishable from an absent one, and the next edit to this loop has
// nothing holding it. That is what ADR-0014 means by a check that cannot fail.
//
// The whole class is one line: a value-taking flag at the end of the argument
// list, which is exactly what a user types when they forget the value.

func hoistTestSet() *flag.FlagSet {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.String("older-than", "", "takes a value")
	fs.Bool("yes", false, "takes none")
	return fs
}

// HB1: a value-taking flag with no value does not read past the end.
//
// PASS: the flag is hoisted alone and nothing panics.
// FAIL, as the mutant does: index out of range on a command line a user types
// whenever they forget an argument.
func TestHB1_AValueFlagAtTheEndDoesNotReadPastTheEnd(t *testing.T) {
	for _, args := range [][]string{
		{"--older-than"},
		{"dir", "--older-than"},
		{"dir", "--yes", "--older-than"},
		{"--older-than", "30d", "dir", "--older-than"},
	} {
		got := hoistFlagsFor(hoistTestSet(), args)
		if len(got) != len(args) {
			t.Errorf("hoistFlagsFor(%q) returned %d arguments from %d; hoisting must not "+
				"invent or drop one", args, len(got), len(args))
		}
	}
}

// HB2: when the value IS there, it travels with its flag.
//
// The companion property. HB1 alone would pass on a function that hoisted
// nothing at all, so this pins the behaviour the guard is protecting rather
// than only the crash it prevents.
func TestHB2_AValueTravelsWithItsFlag(t *testing.T) {
	got := hoistFlagsFor(hoistTestSet(), []string{"dir", "--older-than", "30d"})
	want := []string{"--older-than", "30d", "dir"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q.\nHoisting a flag while leaving its value behind turns the "+
			"value into a path, and the command fails on a file nobody named.", got, want)
	}
}

// HB3: a boolean does not swallow the next argument.
//
// `--yes dir` is two things, not one flag with the value "dir". The FlagSet is
// asked rather than a list kept here, and this is what keeps that true.
func TestHB3_ABooleanDoesNotSwallowThePathAfterIt(t *testing.T) {
	got := hoistFlagsFor(hoistTestSet(), []string{"--yes", "dir"})
	want := []string{"--yes", "dir"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q: a boolean took the path after it as its value", got, want)
	}
}
