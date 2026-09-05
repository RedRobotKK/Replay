package main

import (
	"errors"
	"flag"
	"io"
)

// Asking a command how to use it is not a misuse of it.
//
// Every subcommand builds a flag.FlagSet with ContinueOnError and did
// `if err := fs.Parse(...); err != nil { return errUsage }`. That is correct
// for a typo and wrong for -h: flag returns flag.ErrHelp after printing usage
// to the set's output, which defaulted to stderr, so the help a person or an
// agent asked for arrived on the error stream with a non-zero exit. `replay
// cost --help | head` produced nothing.
//
// errHelpShown says the text has already been printed and there is nothing
// wrong. run() maps it to a clean exit.
var errHelpShown = errors.New("help shown")

// wantsHelp reports whether the caller asked for usage, before any parsing.
// Checked ahead of Parse so it also covers commands that take positional
// arguments and no flags: `redact --help` used to try to open a file called
// --help, because redact reads its arguments as paths.
//
// Stops at "--" so a literal argument after it is never mistaken for a
// request, which matters for a tool whose arguments are file paths.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" || a == "-help" {
			return true
		}
	}
	return false
}

// parseArgs parses a subcommand's flags, routing a help request to stdout with
// a clean exit and leaving genuine misuse on stderr with a non-zero one. The
// two are different events and a script has to be able to tell them apart.
func parseArgs(fs *flag.FlagSet, args []string, stdout io.Writer) error {
	if wantsHelp(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return errHelpShown
	}
	if err := fs.Parse(hoistFlagsFor(fs, args)); err != nil {
		return errUsage
	}
	return nil
}
