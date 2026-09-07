package card

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"
)

// The two registers, and the rules that keep them honest.
//
// A tone is copy. It is allowed to change every word on the card and no figure
// on it, and that is not a matter of care: the words come from one function
// that is handed the same Data whichever tone is asked for, so there is nowhere
// for a tone to compute a number of its own. These tests hold that, and hold
// the two rules the rekt register lives or dies by — exact figures, and never
// an absolute beside the rate it is a share of.

// loud is a corpus with every figure far enough apart that no two can be
// mistaken for each other in an assertion.
func loud() Data {
	return Data{
		AvoidableShare:      0.03,
		Tasks:               1384,
		Breaks:              748,
		MedianUSD:           0.77,
		P90USD:              2.30,
		PeakAvoidableTokens: 32_635_820,
	}
}

// figure matches a number as it appears in copy: grouped, abbreviated, a
// percentage, or a dollar amount.
//
// The leading class is what stops "p90" contributing a 90. A figure is
// preceded by the start of the line or by something that is not part of a word
// or of another number, which is exactly how a reader tells them apart too.
var figure = regexp.MustCompile(`(?:^|[^0-9A-Za-z.,])([0-9][0-9,]*(?:\.[0-9]+)?[kM]?%?)`)

func figuresIn(lines []string) []string {
	var out []string
	for _, l := range lines {
		for _, m := range figure.FindAllStringSubmatch(l, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// allowedFigures is every string the formatters produce from one Data.
//
// Built by calling them, not by typing them out. A list of literals would pass
// while the renderer printed something else entirely, which is the shape of a
// check that cannot fail.
func allowedFigures(d Data) map[string]bool {
	out := map[string]bool{}
	for _, s := range []string{
		d.rateText(),
		strings.TrimPrefix(d.rateText(), "<"),
		shortTokens(d.PeakAvoidableTokens),
		commas(d.PeakAvoidableTokens),
		fmt.Sprint(d.PeakAvoidableTokens),
		commas(d.Tasks), fmt.Sprint(d.Tasks),
		commas(d.Breaks), fmt.Sprint(d.Breaks),
		fmt.Sprintf("%.2f", d.MedianUSD),
		fmt.Sprintf("%.2f", d.P90USD),
	} {
		out[s] = true
	}
	return out
}

// A tone changes the words and cannot change a figure.
//
// The assertion is not that the two tones print the same strings: they must
// not, because the rekt register's whole credibility is that its figure is
// exact where the measured card abbreviates. What must hold is that every
// number on either card is a formatting of the SAME Data, so there is no tone
// under which a reader is shown a quantity the other tone would have shown
// differently.
//
// PASS: every numeric token on every card is one of the formattings of the one
// Data the card was rendered from.
// FAIL: a figure that is not, which is a tone that computed something.
func TestToneCannotChangeANumber(t *testing.T) {
	d := loud()
	ok := allowedFigures(d)
	for _, v := range Variants() {
		for _, tone := range Tones() {
			got := figuresIn(Say(v, tone, d).Lines)
			if len(got) < 3 {
				t.Fatalf("card %s/%s prints %d figures. This check would pass over a "+
					"card with no numbers on it", v, tone, len(got))
			}
			for _, f := range got {
				if !ok[f] {
					t.Errorf("card %s/%s prints %q, which is not a formatting of the "+
						"Data it was given. A tone is copy and must not compute a "+
						"figure of its own", v, tone, f)
				}
			}
		}
	}
}

// And the words really do change, or there is one register with two names.
func TestTheTwoTonesAreDifferentRegisters(t *testing.T) {
	d := loud()
	for _, v := range Variants() {
		a := strings.Join(Say(v, ToneMeasured, d).Lines, "\n")
		b := strings.Join(Say(v, ToneRekt, d).Lines, "\n")
		if a == b {
			t.Errorf("card %s reads identically in both tones", v)
		}
	}
}

// A card carries an absolute or the rate, never both.
//
// The corpus figures divide. An absolute number of re-billed tokens, divided by
// the avoidable rate printed beside it, reconstructs an order-of-magnitude
// spend total: the one figure share.go exists to refuse, arrived at by
// arithmetic instead of by printing it. Neither half is a disclosure on its
// own, and the pair is.
//
// PASS: for every design and tone, the card declares one or the other and its
// words carry only that one.
// FAIL: both declared, a percent sign on a card carrying an absolute, or the
// token figure on a card carrying a rate.
func TestNoCardCarriesBothAnAbsoluteAndARate(t *testing.T) {
	d := loud()
	for _, v := range Variants() {
		for _, tone := range Tones() {
			s := Say(v, tone, d)
			if s.Absolute != "" && s.Rate != "" {
				t.Errorf("card %s/%s declares both an absolute (%q) and a rate (%q). "+
					"One divided by the other is the spend total", v, tone,
					s.Absolute, s.Rate)
			}
			if s.Absolute == "" && s.Rate == "" {
				t.Errorf("card %s/%s declares neither an absolute nor a rate, so this "+
					"check passes over a card it never looked at", v, tone)
			}
			body := strings.Join(s.Lines, "\n")
			if s.Absolute != "" {
				if strings.Contains(body, "%") {
					t.Errorf("card %s/%s carries the absolute %q and a percentage "+
						"appears in its words:\n%s", v, tone, s.Absolute, body)
				}
				if !strings.Contains(body, s.Absolute) {
					t.Errorf("card %s/%s declares the absolute %q and does not print "+
						"it", v, tone, s.Absolute)
				}
			}
			if s.Rate != "" {
				for _, abs := range []string{
					commas(d.PeakAvoidableTokens),
					shortTokens(d.PeakAvoidableTokens),
					fmt.Sprint(d.PeakAvoidableTokens),
				} {
					if strings.Contains(body, abs) {
						t.Errorf("card %s/%s carries the rate %q and the token total "+
							"%q. Dividing one by the other reconstructs the spend "+
							"total:\n%s", v, tone, s.Rate, abs, body)
					}
				}
			}
		}
	}
}

// The rekt register prints figures exactly and groups them.
//
// The genre's entire credibility is that the number is real. "32.6M" is a
// number somebody chose to round; 32,635,820 is a number somebody was billed.
//
// PASS: every count on a rekt card appears comma-grouped, and no abbreviation
// or ungrouped run of four digits survives.
// FAIL: an abbreviated figure, or a four-digit run, which is a figure that was
// printed without grouping.
func TestRektFiguresAreExactAndGrouped(t *testing.T) {
	d := loud()
	ungrouped := regexp.MustCompile(`[0-9]{4,}`)
	abbreviated := regexp.MustCompile(`[0-9](\.[0-9])?[kM]\b`)
	for _, v := range Variants() {
		body := strings.Join(Say(v, ToneRekt, d).Lines, "\n")
		if !strings.Contains(body, commas(d.Tasks)) {
			t.Errorf("card %s/rekt does not print the row count grouped (%q):\n%s",
				v, commas(d.Tasks), body)
		}
		if !strings.Contains(body, commas(d.Breaks)) {
			t.Errorf("card %s/rekt does not print the break count (%q):\n%s",
				v, commas(d.Breaks), body)
		}
		if m := ungrouped.FindString(body); m != "" {
			t.Errorf("card %s/rekt prints %q ungrouped. A four-digit run is a figure "+
				"nobody reads at a glance:\n%s", v, m, body)
		}
		if m := abbreviated.FindString(body); m != "" {
			t.Errorf("card %s/rekt abbreviates %q. The register's credibility is that "+
				"the figure is exact:\n%s", v, m, body)
		}
	}
	// And the measured card is the one that abbreviates, or this test is
	// asserting a property both tones happen to have.
	if !abbreviated.MatchString(strings.Join(Say(VariantB, ToneMeasured, d).Lines, "\n")) {
		t.Error("the measured card no longer abbreviates, so the check above " +
			"distinguishes nothing")
	}
}

// The rekt card carries no call to action.
//
// The install line stays, because it is how the poster found out. Nothing may
// ask the reader to do anything, and nothing may sell.
func TestRektAsksTheReaderForNothing(t *testing.T) {
	d := loud()
	for _, v := range Variants() {
		body := strings.Join(Say(v, ToneRekt, d).Lines, "\n")
		for _, sell := range []string{
			"Measure yours", "Try ", "Find out", "Check your", "you can", "your bill",
			"!",
		} {
			if strings.Contains(body, sell) {
				t.Errorf("card %s/rekt contains %q. The register is deadpan and asks "+
					"for nothing:\n%s", v, sell, body)
			}
		}
		if !strings.Contains(body, InstallLine(v)) {
			t.Errorf("card %s/rekt dropped the install line, which is how the poster "+
				"found out:\n%s", v, body)
		}
	}
}

// A card over a measured zero must not claim a loss.
//
// "tokens I paid for twice" in the largest type on the page, over a corpus
// where nothing was re-billed, is the card lying about its own figure. Both
// tones have to handle it, and the rekt one is the one that would be believed.
func TestNeitherToneClaimsALossOverAZero(t *testing.T) {
	zero := Data{Tasks: 1384, Breaks: 0, MedianUSD: 0.77, P90USD: 2.30}
	for _, v := range Variants() {
		for _, tone := range Tones() {
			body := strings.Join(Say(v, tone, zero).Lines, "\n")
			if !strings.Contains(body, "Nothing was") {
				t.Errorf("card %s/%s over a zero corpus does not say nothing was "+
					"re-billed:\n%s", v, tone, body)
			}
			for _, claim := range []string{"I paid for", "In one ", "billed me twice"} {
				if strings.Contains(body, claim) {
					t.Errorf("card %s/%s claims %q over a corpus where nothing was "+
						"re-billed:\n%s", v, tone, claim, body)
				}
			}
		}
	}
	// The claim is present when it is true, or the check above is satisfied by
	// a card that never makes one.
	body := strings.Join(Say(VariantC, ToneRekt, loud()).Lines, "\n")
	if !strings.Contains(body, "I paid for") {
		t.Error("the rekt statement no longer claims the loss when there was one, so " +
			"the zero-case check distinguishes nothing")
	}
}

// The tone reaches the pixels.
//
// Everything above reads strings. A renderer that computed the right words and
// drew something else would satisfy all of it, which is the defect this file
// would otherwise be one more instance of.
func TestToneReachesThePicture(t *testing.T) {
	d := loud()
	for _, v := range Variants() {
		a, err := Render(v, ToneMeasured, d)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Render(v, ToneRekt, d)
		if err != nil {
			t.Fatal(err)
		}
		var differ int
		for y := 0; y < Height; y++ {
			for x := 0; x < Width; x++ {
				if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
					differ++
				}
			}
		}
		if differ == 0 {
			t.Errorf("card %s renders identically in both tones; the tone is not "+
				"reaching the pixels", v)
		}
	}
}

// Nothing a card says is spelled inside the renderer.
//
// Say is the one place the words are decided, and it is only the one place if
// the layouts cannot spell a figure of their own. A literal in renderB is a
// number that no tone rule, no zero case and no leak check can see.
//
// PASS: no string literal in either layout contains a digit or a percent sign.
// FAIL: any, which is copy that escaped the one function that governs it.
func TestTheLayoutsSpellNoFigures(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "card.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	digits := regexp.MustCompile(`[0-9%]`)
	var checked int
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "renderB" && fn.Name.Name != "renderC") {
			return true
		}
		checked++
		ast.Inspect(fn, func(m ast.Node) bool {
			lit, ok := m.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if digits.MatchString(lit.Value) {
				t.Errorf("%s spells %s. Every figure and every word belongs in Say, "+
					"where the tone rules and the zero case can see it",
					fn.Name.Name, lit.Value)
			}
			return true
		})
		return false
	})
	if checked != 2 {
		t.Fatalf("found %d layout functions, want 2. The parser is wrong, not the code", checked)
	}
}

// Nothing runs off the right edge.
//
// A figure that overflows is clipped by the compositor rather than reported, so
// a ten-digit token count on a rekt card would simply lose its last digits and
// still look like a card. The margin is checked instead: ink in the last
// fifteen pixels is a line that did not fit.
func TestNoCardOverrunsItsRightMargin(t *testing.T) {
	big := loud()
	big.PeakAvoidableTokens = 987_654_321
	big.Tasks = 12_845
	big.Breaks = 9_999
	for _, v := range Variants() {
		for _, tone := range Tones() {
			img, err := Render(v, tone, big)
			if err != nil {
				t.Fatal(err)
			}
			bg := img.RGBAAt(600, 5)
			for y := 0; y < Height; y++ {
				for x := Width - 15; x < Width; x++ {
					if img.RGBAAt(x, y) != bg {
						t.Fatalf("card %s/%s has ink at (%d,%d), inside the right "+
							"margin. A line that overflows is clipped, not reported",
							v, tone, x, y)
					}
				}
			}
		}
	}
}
