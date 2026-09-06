package tui

import "strings"

// Most people will never type a flag.
//
// Replay has 72 of them across 11 commands, and the flag-surface design
// classified all 72 into six kinds of screen element. That was the right map
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
	BudgetCols = 80
	BudgetRows = 24
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
}

// Shortcuts is the whole surface: eight questions, eight keys.
//
// Eight because that is what fits on one line of hints inside the budget, and
// because a ninth would mean two of them overlap. Every command Replay has is
// reachable from one of these or from the command line; not every command
// deserves a key.
func Shortcuts() []Shortcut {
	return []Shortcut{
		{'c', "What did this cost me?", "cost", []string{"--per-task"},
			"Cost per task, newest first, at list prices.", "cost"},
		{'w', "Why was it expensive?", "blame", nil,
			"Where the prompt cache broke, and what each break re-billed.", "why"},
		{'x', "What is filling my context?", "context", []string{"--top", "12"},
			"What entered this context, largest first.", "context"},
		{'a', "What should I change?", "advise", []string{"--guards"},
			"Changes worth making, with the evidence behind each.", "advise"},
		{'g', "Am I about to blow a budget?", "serve", []string{"--max-day-usd"},
			"Every guard, whether it is armed, and whether it can fire.", "guards"},
		{'m', "Which model should I use?", "route", []string{"--to"},
			"What the same work would cost on another model, with error bars.", "model"},
		{'s', "Is my setup safe?", "serve", []string{"--mask", "--mask-patterns"},
			"What masking covers, and the paths it does not reach.", "safe"},
		{'d', "Is anything broken?", "doctor", nil,
			"What Replay can see on this machine, and what it cannot.", "doctor"},
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

// Dim marks text the renderer should draw at lower contrast. It returns the
// text unchanged so the layout is identical with or without colour, which is
// what keeps a piped run and a live run the same width.
func Dim(s string) string { return s }

// Hints renders the one-line key strip. Every question is reachable from every
// screen, because a surface where the answer you want is three screens away is
// a surface people stop using.
func Hints(cur rune) string {
	var b strings.Builder
	for _, s := range Shortcuts() {
		if s.Key == cur {
			b.WriteString("[" + string(s.Key) + "]" + s.Label)
			continue
		}
		b.WriteString(" " + string(s.Key) + " " + s.Label)
	}
	b.WriteString("  q quit")
	return b.String()
}
