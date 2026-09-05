package main

import (
	"fmt"
	"io"
	"math"
	"os"
)

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
	tipShare = 0.02

	// Buy Me a Coffee sells whole coffees at a fixed price and cannot pre-fill
	// a custom amount from a URL, so the suggestion has to be a figure the page
	// will actually offer. Checked against the live page on 2026-09-05:
	// coffee_price 5.0000 USD. Suggesting $3 named an amount that does not
	// exist there, which is a small thing that makes the whole line look
	// careless.
	tipUnitUSD = 5
	tipMinUSD  = 5.00
	tipMaxUSD  = 25.00
)

// tipLine returns the line to print under a cost report, or "" when the finding
// does not earn one. It hyperlinks the address when the destination is a
// terminal that can render one.
func tipLine(avoidableUSD float64, out io.Writer) string {
	return tipLineFor(avoidableUSD, canHyperlink(out))
}

// canHyperlink reports whether OSC 8 is worth emitting. A file, a pipe or a
// dumb terminal gets plain text: an escape sequence in `replay cost > out.txt`
// is corruption, not a convenience.
func canHyperlink(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" || os.Getenv("TERM") == "" {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func tipLineFor(avoidableUSD float64, hyperlink bool) string {
	if avoidableUSD < tipFloorUSD {
		return ""
	}
	// Round up to a whole coffee: rounding down would suggest less than the
	// share, and the page cannot sell a fraction of one anyway.
	units := int(math.Ceil(avoidableUSD * tipShare / float64(tipUnitUSD)))
	suggested := float64(units * tipUnitUSD)
	if suggested < tipMinUSD {
		suggested = tipMinUSD
	}
	if suggested > tipMaxUSD {
		suggested = tipMaxUSD
	}
	coffees := int(suggested) / tipUnitUSD
	unit := "coffees"
	if coffees == 1 {
		unit = "coffee"
	}
	// OSC 8. The visible label is the URL itself, so a terminal that ignores
	// the sequence still shows an address a reader can copy.
	link := shareCoffee
	if hyperlink {
		link = "\x1b]8;;https://" + shareCoffee + "\x1b\\" + shareCoffee + "\x1b]8;;\x1b\\"
	}
	return fmt.Sprintf(
		"\nReplay found $%.2f you had already paid for once. It is free, and the\n"+
			"measurements behind it are not. If it was worth %d %s of that back, that\n"+
			"is what keeps it maintained: %s\n",
		avoidableUSD, coffees, unit, link)
}

const shareCoffee = "buymeacoffee.com/saitodaniel"
