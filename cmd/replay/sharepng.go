package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/RedRobotKK/Replay/internal/card"
)

// The picture form of the share card.
//
// `--share` prints a block of text that is safe to paste. `--png` writes the
// same measurements as an image, because the places this actually travels
// render an image and truncate a code block. Nothing is added on the way: the
// picture carries the rate, the row count and its noun, the breaks, the median
// and the p90, and one figure the text card does not carry, discussed below.
// No path, no project name, no spend total.
//
// The privacy guarantee is structural rather than careful. internal/card.Data
// has no string field, so there is nowhere for a path or a project name to
// arrive even if someone later wires one in by accident, and the test in this
// package proves the stronger property directly: the same transcripts under two
// different project names produce byte-identical files.

// cardData is the only place the report becomes a picture, which makes it the
// only place a figure could be added to one without being added to the other.
func cardData(s costSummary, breaks, peakTokens int) card.Data {
	return card.Data{
		RebilledShare:      s.RebilledShare,
		Tasks:              s.Tasks,
		LaneUnit:           s.Unit == unitLane,
		Breaks:             breaks,
		MedianUSD:          s.MedianUSD,
		P90USD:             s.P90USD,
		PeakRebilledTokens: peakTokens,
	}
}

// peakRebilledTokens is the largest number of tokens re-billed within a single
// row — one session, or one agent lane.
//
// The corpus sum is the figure `replay cost` prints and it is the wrong one
// here. Divided by the re-billed rate, which is on the same card, it
// reconstructs an order-of-magnitude spend total: exactly the number share.go
// refuses to carry, arrived at by arithmetic instead of by printing it. A peak
// divides into nothing, because a reader has no way to know how many rows
// produced it.
//
// It is also the more honest headline for the card that shows it. "189k tokens
// billed twice, in one session" is a claim about one session, and a corpus sum
// under that sentence would be a fabrication with a real number in it.
func peakRebilledTokens(units []costUnit) int {
	peak := 0
	for _, u := range units {
		if u.RebilledTokens > peak {
			peak = u.RebilledTokens
		}
	}
	return peak
}

// pickVariant resolves --card into a design.
//
// Both designs ship and neither is an experiment. Randomising the choice was
// considered and rejected on the arithmetic: detecting a 50% difference in
// install rate needs on the order of 200 card-attributed installs and a 20%
// difference around 950, against roughly three a week. Worse, the design would
// be assigned per posting machine while the outcome is measured per reader,
// which makes it a cluster-randomised trial whose effective sample size is the
// number of distinct posters — two — and no amount of traffic through those two
// changes that.
//
// A comparison that cannot be read is not free. It would have cost a line of
// output telling the user their card was picked at random for a test, which
// makes the tool look like it is experimenting on them while learning nothing.
// So the default is simply the better card.
//
// The design letter still travels in the URL path. It costs nothing today and
// means that if the traffic ever arrives, the dimension is already in the data
// rather than needing a card redesign to introduce.
func pickVariant(choice string) (card.Variant, error) {
	switch choice {
	case "":
		// c reads better at the size this is actually seen at, a feed
		// thumbnail, and unlike b it does not depend on the hero number being
		// large: a corpus with one small cache break still produces a card
		// that says something.
		return card.VariantC, nil
	case string(card.VariantB):
		return card.VariantB, nil
	case string(card.VariantC):
		return card.VariantC, nil
	default:
		return "", fmt.Errorf("no card design %q: the designs are b (the dark "+
			"receipt) and c (the paper statement)", choice)
	}
}

// writeCard renders to memory first and writes once.
//
// A render that fails halfway must not leave a partial PNG where the user asked
// for a card: a file that exists reads as a card that worked, and the next
// thing that happens to it is being posted.
func writeCard(path string, v card.Variant, t card.Tone, d card.Data, stderr io.Writer) error {
	var buf bytes.Buffer
	if err := card.Encode(&buf, v, t, d); err != nil {
		return fmt.Errorf("rendering the card: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	_, _ = fmt.Fprintf(stderr, "\nwrote %s  (%dx%d, design %s, %s)\n",
		path, card.Width, card.Height, v, t)
	return nil
}
