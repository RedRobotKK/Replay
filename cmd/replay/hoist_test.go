package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"
)

// hoistFlags lets flags appear after the path, which the flag package will not
// do on its own. Doing that safely means knowing which flags take a value:
// moving `--to` to the front while leaving `claude-fable-5-1` behind turns the
// value into a path and the command fails on a file that was never named.
func TestHoistKeepsAValueWithItsFlag(t *testing.T) {
	newFS := func() *flag.FlagSet {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.String("to", "", "a string flag")
		fs.Int("top", 0, "an int flag")
		fs.Bool("json", false, "a bool flag")
		return fs
	}
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"value flag after the path", []string{"dir", "--to", "m2"}, []string{"--to", "m2", "dir"}},
		{"single dash", []string{"dir", "-to", "m2"}, []string{"-to", "m2", "dir"}},
		{"equals form is self-contained", []string{"dir", "--to=m2"}, []string{"--to=m2", "dir"}},
		{"bool takes no value", []string{"dir", "--json"}, []string{"--json", "dir"}},
		{"bool then path then value flag", []string{"--json", "dir", "--top", "5"}, []string{"--json", "--top", "5", "dir"}},
		{"already in order", []string{"--to", "m2", "dir"}, []string{"--to", "m2", "dir"}},
		{"two paths", []string{"a", "--top", "3", "b"}, []string{"--top", "3", "a", "b"}},
		{"everything after -- is a path", []string{"--json", "--", "-weird-name"}, []string{"--json", "--", "-weird-name"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hoistFlagsFor(newFS(), tc.args); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("hoist %v\n got %v\nwant %v", tc.args, got, tc.want)
			}
		})
	}
}

// The end-to-end shape of the bug: parse what hoisting produced and check the
// value actually reached the flag and not the argument list.
func TestValueFlagAfterPathSurvivesParsing(t *testing.T) {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	to := fs.String("to", "", "")
	if err := fs.Parse(hoistFlagsFor(fs, []string{"/some/dir", "--to", "claude-fable-5-1"})); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if *to != "claude-fable-5-1" {
		t.Fatalf("--to did not receive its value: %q", *to)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"/some/dir"}) {
		t.Fatalf("the model name leaked into the paths: %v", got)
	}
}

// Dispatch must strip the subcommand name before the command parses its own
// arguments. Getting that wrong makes "route" itself look like a path, and the
// failure surfaces as a confusing stat error rather than a usage message.
func TestRouteDispatchStripsItsOwnName(t *testing.T) {
	var out, errBuf bytes.Buffer
	err := run([]string{"route", "--to", "m2"}, &out, &errBuf)
	if err == nil {
		t.Fatal("route with no path must fail")
	}
	if strings.Contains(err.Error(), "stat route") {
		t.Fatalf("the subcommand name reached the path list: %v", err)
	}
	if !errors.Is(err, errUsage) {
		t.Fatalf("a missing path is a usage error, got %v", err)
	}
}
