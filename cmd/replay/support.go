package main

import (
	"fmt"
	"io"
)

// The ask, on every surface that hands the reader a result.
//
// `cost` carried it alone, because it is the command with a dollar figure and
// the line was written around that figure. Every other analysis command
// produced a finding and said nothing about who maintains the thing or how it
// is paid for. A reader who learns which tool definitions they are paying for
// on every request, and is never told the tool is free and maintained by one
// person, has been given something and told nothing.
//
// Two constraints keep it an ask rather than a nag, and both are tested:
//
//   - It fires only when a real result was produced. A refusal, an empty
//     corpus or a usage error never carries it. Asking for money directly
//     after failing to answer is the shape that gets software uninstalled.
//   - It never touches machine-readable output. A funding line inside JSON is
//     corruption, and the one thing more certain than a caller not paying is a
//     caller stripping a tool that breaks their parser.
//
// It also stays in the register the rest of the tool uses: it names what this
// command actually found, and it never quotes a figure the command did not
// compute.

// valueCommands are the subcommands that hand the reader an analysis. Derived
// here rather than typed at each call site so a command added later is covered
// by the test the day it exists.
func valueCommands() []string {
	// corpus is deliberately absent: its output is a publishable evidence
	// document in the house format, and a funding line inside one would be
	// pollution rather than an ask. redact and serve produce no analysis.
	return []string{"cost", "context", "blame", "diff", "advise", "learn", "route", "trim"}
}

// supportLine is the ask for a command with no dollar figure of its own.
//
// what should name the finding in the reader's terms — "what is filling your
// context", "where the cache broke" — because a specific sentence is both
// more useful and easier to believe than a generic plea.
func supportLine(what string, out io.Writer) string {
	return supportLineFor(what, canHyperlink(out))
}

func supportLineFor(what string, hyperlink bool) string {
	link := shareCoffee
	if hyperlink {
		link = "\x1b]8;;https://" + shareCoffee + "\x1b\\" + shareCoffee + "\x1b]8;;\x1b\\"
	}
	return fmt.Sprintf(
		"\nThat is %s, from sessions you had already paid for. Replay is free and\n"+
			"maintained by one person, and the measurements behind it are real API spend.\n"+
			"If it was useful, that is what keeps it going: %s\n",
		what, link)
}
