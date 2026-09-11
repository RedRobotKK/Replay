package tui

import "strings"

// Most people will never type a flag.
//
// Replay has 75 of them across 13 commands, and the flag-surface design
// classified all 75 into six kinds of screen element. That was the right map
// of the wrong territory: it assumed the person at the keyboard is choosing.
// They are not. A TUI, or an agent, runs the command for them and their whole
// experience is whatever lands on screen afterwards.
//
// So the unit of design is not the flag. It is the question, and every screen
// answers exactly one:
//
//	what did this cost me
//	why was it expensive
//	what is filling my context
//	what should I change
//	am I about to blow a budget
//	which model should I be using
//	is my setup actually safe
//	is anything broken
//	is any of this worth posting
//
// A flag is then an implementation detail of answering one of those, chosen by
// the tool rather than the user.
//
// One thing does not get hidden. Every screen prints the command that produced
// it. A surface that runs commands on your behalf without showing them is
// asking for trust it has not earned, and this project's whole argument is that
// you should not have to take its word for anything. Showing the command also
// means the screen teaches: a user who wants the flag can read it, copy it, and
// stop needing the screen.

// Budget is the terminal this is designed for. Eighty columns by twenty-four
// rows is the floor, not the target: every screen must be readable there, and
// anything taller has to earn the scroll.
const (
	BudgetRows = 24

	// bodyRows is how many of those rows a screen's body may use.
	//
	// The loop appends the footer, so a body that fills BudgetRows leaves it
	// nothing to add and one of the two has to give. It used to be the body:
	// every pad helper produced BudgetRows lines, the loop cut to
	// BudgetRows-1, and every screen silently lost its last line — the "copy
	// it and you never need this screen again" tagline under `ran replay
	// <cmd>`. Down a pipe, --once does not trim, so the same screen came out
	// one row taller with the line intact. The committed screen images are
	// made through the pipe, so they showed a line no live reader ever saw.
	bodyRows = BudgetRows - 1
)

// Shortcut is one question, the key that asks it, and the command that answers.
type Shortcut struct {
	// Key is the single keystroke. One key, because a shortcut needing two is
	// a menu with extra steps.
	Key rune
	// Question is what the user actually wants to know, in their words.
	Question string
	// Command is the subcommand run on their behalf.
	Command string
	// Flags are chosen by the tool. They appear on screen so the user can see
	// what was run, never as controls they must set.
	Flags []string
	// Answers is the single sentence the screen leads with. If a screen cannot
	// state its answer in one line, it is answering more than one question.
	Answers string
	// Label is the word on the key strip. A strip of bare letters is a strip
	// only somebody who already knows the tool can read, which is the audience
	// this design is explicitly not for.
	Label string
	// JSON records whether Command accepts --json, because the help overlay
	// offers that flag and four of these screens cannot honour it. It is a
	// fact about another package's flag set, so it is declared here and
	// checked against the binary by cmd/replay TestJC1 rather than trusted.
	JSON bool
}

// shareKey opens the share screen.
//
// p, not s: s is the safety screen and was there first. The card is a picture
// somebody posts, so p reads as post, and it is the same kind of borrowing as
// x for context and m for model — the letter is a handle, not an abbreviation.
const shareKey = 'p'

// Shortcuts is the whole surface: nine questions, nine keys.
//
// Nine is the ceiling, and it is a measured one rather than a preference. The
// key strip is one line of eighty columns, every entry costs three columns plus
// its label, and the nine labels come to exactly eighty with the quit hint on
// the end. A tenth does not fit, and a second row of hints is a menu.
//
// Every command Replay has is reachable from one of these or from the command
// line; not every command deserves a key.
func Shortcuts() []Shortcut {
	return []Shortcut{
		{'c', "What did this cost me?", "cost", []string{"--per-task"},
			"Cost per task, newest first, at list prices.", "cost", true},
		{'w', "Why was it expensive?", "blame", nil,
			"Where the prompt cache broke, and what each break re-billed.", "why", false},
		{'x', "What is filling my context?", "context", []string{"--top", "12"},
			"What entered this context, largest first.", "context", true},
		{'a', "What should I change?", "advise", []string{"--guards"},
			"Changes worth making, with the evidence behind each.", "advise", true},
		{'g', "Am I about to blow a budget?", "serve", []string{"--max-day-usd"},
			"Every guard, whether it is armed, and whether it can fire.", "guards", false},
		{'m', "Which model should I use?", "route", []string{"--to"},
			"What the same work would cost on another model, with error bars.", "model", true},
		{'s', "Is my setup safe?", "privacy", nil,
			"Everything Replay has written here, and what a purge will not reach.", "safe", true},
		{'d', "Is anything broken?", "doctor", nil,
			"What Replay can see on this machine, and what it cannot.", "doctor", false},
		{'l', "What is flowing right now?", "serve", nil,
			"What the proxy is seeing as it happens, or why it is seeing nothing.", "live", false},
		{shareKey, "Is any of this worth posting?", "cost", []string{"--share", "--png"},
			"What a card of these figures would say, and what it would not carry.", "share", true},
	}
}

// Ran renders the provenance line: the command this screen came from.
//
// Two lines of the budget, and the cheapest honesty in the design.
func Ran(s Shortcut) []string {
	cmd := "replay " + s.Command
	if len(s.Flags) > 0 {
		cmd += " " + strings.Join(s.Flags, " ")
	}
	return []string{
		"  ran   " + cmd,
		"  " + Dim("copy it and you never need this screen again."),
	}
}

// Dim marks text the renderer should draw at lower contrast.
//
// It returned the text unchanged from the day it was written, as a seam for a
// colour layer that had not been built. The eight call sites were already in
// the right places; only the body was missing.
//
// The property its old comment claimed, that the layout is identical with or
// without colour, is now a thing that can be got wrong rather than a thing
// that is true by construction, so it is asserted instead of assumed:
// TestCL1 strips the escapes from every screen and requires the uncoloured
// render back byte for byte. Pass text that is already padded to its column.
func Dim(s string) string { return paint(Faint, s) }

// The key strip is gone, and its absence is the design rather than an omission.
//
// Hints() rendered one line naming all ten screens. It had no callers, ever:
// the loop appends Footer(key) and nothing else, so no reader has seen it. That
// alone would make it dead code. What made it worth removing rather than
// wiring is what it was costing while dead.
//
// The strip came to exactly eighty columns at ten questions, and
// TestHintsFitTheBudget held that as a hard budget. So a line nobody rendered
// was refusing an eleventh screen on behalf of a reader who could not see the
// tenth. A ceiling on the whole surface, enforced for a layout that does not
// exist.
//
// It was also superseded and not deleted. outcomes.go records that progressive
// disclosure "moved [the eight-key strip] behind ?", which is where the index
// now lives: Help() names every question, fits the same twenty-four rows, and
// is one keystroke the footer advertises on every screen. TestHelpCarriesEveryQuestion
// holds that every question stays reachable there, and TestFooterNamesTheCurrentScreen
// holds that each screen says which one it is. Both properties the strip
// claimed are held by surfaces a reader actually sees.
