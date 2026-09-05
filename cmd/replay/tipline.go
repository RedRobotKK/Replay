package main

import "fmt"

// The ask, at the one moment it is relevant.
//
// `replay cost` has just told you how much you already paid twice. That is the
// only point in the whole tool where money is on the reader's mind and the tool
// has just done something for them, so it is the only place this belongs.
//
// It is deliberately NOT a browser tab that opens itself. Replay's standing
// claim is that it makes no network request except your own traffic to your own
// provider, and a binary that launches a browser is the kind of thing people
// uninstall — rightly. It prints a line. Opening it is the reader's move.
//
// The amount is not passed to the payment page either, and that is a fact about
// Buy Me a Coffee rather than a choice: `?amount=` is echoed into its canonical
// URL and the payment input's value attribute stays empty, checked against the
// live page on 2026-09-05. A link that promised a pre-filled amount would be a
// promise broken on arrival, so the suggestion is made in the text where it is
// honest about being a suggestion.
const (
	// Below this the tool found something too small to mention twice. Asking
	// for money over a rounding error reads as a shakedown and costs more
	// goodwill than it collects.
	tipFloorUSD = 5.00

	// The suggestion is a small share of what was found, and capped. Somebody
	// who discovers four thousand dollars of re-billed tokens is not being
	// asked for four hundred of them.
	tipShare  = 0.02
	tipMinUSD = 3.00
	tipMaxUSD = 25.00
)

// tipLine returns the line to print under a cost report, or "" when the finding
// does not earn one.
func tipLine(avoidableUSD float64) string {
	if avoidableUSD < tipFloorUSD {
		return ""
	}
	suggested := avoidableUSD * tipShare
	if suggested < tipMinUSD {
		suggested = tipMinUSD
	}
	if suggested > tipMaxUSD {
		suggested = tipMaxUSD
	}
	return fmt.Sprintf(
		"\nReplay found $%.2f you had already paid for once. It is free and funded by\n"+
			"nobody; if it was worth about $%.0f of that back, %s\n",
		avoidableUSD, suggested, shareCoffee)
}

const shareCoffee = "buymeacoffee.com/saitodaniel"
