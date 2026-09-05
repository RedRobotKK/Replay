package main

import (
	"fmt"
	"strings"
)

// The share card.
//
// `replay cost` answers "what is this costing me". This answers a different
// question: "is it worth telling anyone". The two need different numbers, and
// the difference is the whole design.
//
// What it deliberately leaves out is a spend total. A total tells a reader the
// poster's monthly burn and lets them infer team size and runway, which is the
// one figure a company would regret having posted. It is also the least
// interesting number in the set, because it is not comparable: $3,000 means
// nothing without knowing how many engineers spent it.
//
// A rate is comparable across everyone and reveals nothing. "5% of my spend was
// paid twice" reads the same from a solo developer and from a team of fifty,
// which is exactly what makes it worth comparing — and comparison is the only
// mechanism by which a number like this travels.

const shareRepo = "github.com/RedRobotKK/Replay"

// shareCard renders a paste-ready summary, or the empty string when there is
// nothing measured enough to stand behind. Refusing beats posting zeros that
// look like a finding.
func shareCard(s costSummary, breaks int) string {
	if s.Tasks == 0 || s.MedianUSD == 0 {
		return ""
	}

	pct := s.AvoidableShare * 100
	headline := fmt.Sprintf("%.0f%% of my agent spend was paid twice.", pct)
	if pct > 0 && pct < 1 {
		// Rounding a real number to "0%" would report a finding as nothing.
		headline = "Under 1% of my agent spend was paid twice."
	}
	if pct == 0 {
		headline = "None of my agent spend was paid twice."
	}

	var b strings.Builder
	line := "  " + strings.Repeat("─", 52)

	b.WriteString(line + "\n\n")
	b.WriteString("  " + headline + "\n\n")
	fmt.Fprintf(&b, "    sessions      %-8d  median task   $%.2f\n", s.Tasks, s.MedianUSD)
	fmt.Fprintf(&b, "    cache breaks  %-8d  p90 task      $%.2f\n", breaks, s.P90USD)
	b.WriteString("\n")
	b.WriteString("  Not a forecast of savings. Tokens already billed twice\n")
	b.WriteString("  because a prompt cache broke and nothing said so.\n\n")
	b.WriteString("  Measure yours:  " + shareRepo + "\n")
	b.WriteString(line + "\n")
	return b.String()
}

// shareNote is printed under the card, to the terminal only. It never becomes
// part of what gets pasted, which is why it is separate: the card has to be
// safe to post without the poster having to edit anything out of it.
func shareNote() string {
	return "\n" + "Copy the block above. It carries no paths, no project names, and no\n" +
		"spend total — a total tells a reader your burn rate, a rate does not.\n\n" +
		"If Replay found something worth posting, a star is how it reaches the\n" +
		"next person: " + shareRepo + "\n"
}
