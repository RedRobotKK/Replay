package card

import (
	"fmt"
	"strings"
)

// Tone is the register the card is written in.
//
// Two, over one set of figures. The measured card states what was found; the
// rekt card is the one that travels, and it travels because the number on it
// is exact. Everything below is copy, and copy only: Say is handed the same
// Data whichever tone is asked for, and there is nowhere in it for a tone to
// compute a figure of its own.
//
// The rules of the rekt register, which are strict because the genre is:
//
//	deadpan. no exclamation, no drama, no adjectives.
//	figures exact and comma-grouped. never rounded, never abbreviated.
//	very short lines.
//	no call to action. the install line stays and reads as how I found out.
//	grievance, not self-deprecation. the loss was discovered, not chosen.
type Tone string

const (
	// ToneMeasured is the default: what was found, stated.
	ToneMeasured Tone = "measured"
	// ToneRekt is the register of cloud-bill horror screenshots.
	ToneRekt Tone = "rekt"
)

// Tones lists the registers, in a fixed order so anything iterating them sees
// the same set in the same order every time.
func Tones() []Tone { return []Tone{ToneMeasured, ToneRekt} }

// Script is every string a card prints, and the classification the leak rule
// needs.
//
// One function produces it and the layouts draw nothing else, so a preview of
// a card cannot disagree with the file that gets written. That property is the
// reason this type exists rather than the copy sitting inline in the layouts:
// a preview that can differ from what it previews is worse than no preview.
type Script struct {
	// Lines is every string the card draws, in drawing order.
	Lines []string
	// Absolute is the token figure this card carries, or "" when it carries
	// none. Rate is the percentage, or "" likewise.
	//
	// Never both. The corpus figures divide: an absolute number of re-billed
	// tokens over the rate it was a share of reconstructs an order-of-magnitude
	// spend total, which is the one figure share.go exists to refuse. Neither
	// half discloses it alone.
	Absolute string
	Rate     string
}

// Say returns everything one card says.
func Say(v Variant, t Tone, d Data) Script {
	switch v {
	case VariantB:
		return sayB(t, d).script()
	case VariantC:
		return sayC(t, d).script()
	}
	return Script{}
}

// bScript is what design b prints: a terminal frame, one figure, and the copy
// that reads it.
type bScript struct {
	Prompt  string
	Stats   []string
	Figure  string
	Legs    []string
	Install string
	Foot    string
}

func (s bScript) script() Script {
	lines := []string{s.Prompt}
	lines = append(lines, s.Stats...)
	lines = append(lines, s.Figure)
	lines = append(lines, s.Legs...)
	return Script{Lines: append(lines, s.Install, s.Foot), Absolute: s.Figure}
}

// sayB writes the receipt.
//
// The figure is the peak row's re-billed tokens in both tones, never the corpus
// sum. See peakRebilledTokens in cmd/replay/sharepng.go: a sum divides by the
// rate into a spend total, and a peak divides into nothing because a reader
// cannot know how many rows produced it. The rekt tone prints that peak exactly
// where the measured tone abbreviates it, and says which row it came from,
// because an unqualified large number reads as a corpus total and would be a
// fabrication with a real figure in it.
func sayB(t Tone, d Data) bScript {
	s := bScript{
		// The command that produced these figures, named accurately. A card
		// that shows the output of one command under the prompt of another is
		// a small fabrication, and this card's whole argument is that it is a
		// receipt.
		Prompt:  "$ replay cost --share",
		Install: InstallLine(VariantB),
		Foot:    "Reads transcripts already on your disk. Sends nothing anywhere.",
	}
	if t == ToneRekt {
		s.Stats = []string{
			fmt.Sprintf("%s %s   median $%.2f   p90 $%.2f",
				commas(d.Tasks), d.unit(), d.MedianUSD, d.P90USD),
			commas(d.Breaks) + " cache breaks",
		}
		s.Figure = commas(d.PeakRebilledTokens)
		s.Legs = []string{
			"tokens I paid for twice.",
			"One " + d.unitSingular() + ". No warning. No line item.",
		}
		if d.PeakRebilledTokens == 0 {
			// A zero under "tokens I paid for twice" is the card lying in the
			// largest type on the page.
			s.Legs = []string{"tokens were billed twice.", "Nothing was re-billed. I checked."}
		}
		return s
	}
	s.Stats = []string{
		fmt.Sprintf("%d %s   median $%.2f   p90 $%.2f",
			d.Tasks, d.unit(), d.MedianUSD, d.P90USD),
		fmt.Sprintf("%d cache breaks", d.Breaks),
	}
	s.Figure = shortTokens(d.PeakRebilledTokens)
	s.Legs = []string{"tokens billed twice,", "in one " + d.unitSingular() + ".", "Nothing told me."}
	if d.PeakRebilledTokens == 0 {
		s.Legs = []string{"tokens billed twice.", "Nothing was re-billed.", "Now I can tell."}
	}
	return s
}

// cScript is what design c prints: a statement, and the figures under a rule.
type cScript struct {
	Head    []string
	Sub     string
	Lead    string
	Segs    []string
	Install string
	// Absolute and Rate say which of the two the lines above carry. The
	// statement leads with one or the other and never both.
	Absolute string
	Rate     string
}

// script flattens the statement to one string per row of the card.
//
// The lead figure and the segments beside it are one row on the card, laid out
// by measurement rather than at fixed stops, so they are one line here. A
// preview that stacked them would show "5%" alone above "of spend paid twice",
// which is four fragments of a sentence read as four statements.
func (s cScript) script() Script {
	lines := append([]string{}, s.Head...)
	lines = append(lines, s.Sub, strings.Join(append([]string{s.Lead}, s.Segs...), "  "))
	return Script{Lines: append(lines, s.Install), Absolute: s.Absolute, Rate: s.Rate}
}

// sayC writes the confession.
//
// The measured statement leads with the rate, which is comparable across
// everyone and reveals nothing. The rekt statement leads with the absolute,
// which is what the register is for, and therefore drops the rate entirely:
// the two together are the spend total. That is why the footnote row under the
// rule changes what it leads with as well.
func sayC(t Tone, d Data) cScript {
	s := cScript{Install: InstallLine(VariantC)}
	if t == ToneRekt {
		s.Head = []string{"I paid for", commas(d.PeakRebilledTokens) + " tokens twice."}
		s.Sub = "In one " + d.unitSingular() + ". Nothing told me."
		s.Absolute = commas(d.PeakRebilledTokens)
		s.Lead = commas(d.Breaks)
		s.Segs = []string{"cache breaks", "none of them said so", commas(d.Tasks) + " " + d.unit()}
		if d.PeakRebilledTokens == 0 {
			s.Head = []string{"Nothing was", "paid for twice."}
			s.Sub = "I checked."
			s.Segs = []string{"cache breaks", commas(d.Tasks) + " " + d.unit()}
		}
		return s
	}
	// The headline is a claim, so it has to follow the measurement. Nothing
	// re-billed and "billed me twice" over it is the card lying about its own
	// figures, in the largest type on the page.
	s.Head = []string{"My coding agent", "billed me twice."}
	s.Sub = "I only found out because I measured it."
	if d.RebilledShare == 0 {
		s.Head = []string{"Nothing was", "billed twice."}
		s.Sub = "I only know that because I measured it."
	}
	s.Rate = d.rateText()
	s.Lead = s.Rate
	s.Segs = []string{
		"of spend paid twice",
		fmt.Sprintf("%d cache breaks", d.Breaks),
		fmt.Sprintf("%d %s", d.Tasks, d.unit()),
	}
	return s
}

// commas groups thousands. A seven-digit figure without them is a figure
// nobody reads at a glance, and the rekt register's whole claim is that its
// figure is one a reader can check.
func commas(n int) string {
	s := fmt.Sprint(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// ParseTone resolves a --tone value, and is the only place a tone is spelled,
// so the command line and any other caller reject the same strings with the
// same words.
func ParseTone(choice string) (Tone, error) {
	switch choice {
	case "", string(ToneMeasured):
		return ToneMeasured, nil
	case string(ToneRekt):
		return ToneRekt, nil
	default:
		return "", fmt.Errorf("no card tone %q: the tones are measured (what was "+
			"found, stated) and rekt (the same figures, exact, deadpan)", choice)
	}
}
