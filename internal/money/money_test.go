package money

import (
	"strings"
	"testing"
	"time"
)

// A converted figure is a presentation, not a measurement, and the tests below
// are mostly about keeping that distinction visible.
//
// The bill is in US dollars. A card issuer converts at its own rate on its own
// settlement date and adds a foreign transaction fee, commonly 1.5 to 3
// percent, so a figure converted at a reference rate is not what anybody paid.
// It is worth showing anyway, because a reader in Tokyo should not have to do
// arithmetic to know whether a number is large. It is not worth showing in
// place of the dollars, because then the tool would be stating a figure nobody
// was billed and calling it the cost.

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

var refDate, _ = time.Parse("2006-01-02", RatesDate)

// MN-1: a dollar reader sees exactly what they saw before.
//
// The feature has to be invisible to the people it is not for. A US locale, or
// any locale whose currency is the dollar, gets no second figure, no rate and
// no caveat.
func TestMN1_DollarLocalesAreUnchanged(t *testing.T) {
	for _, lang := range []string{"en_US.UTF-8", "en_US", "C", "POSIX"} {
		d := Detect(env(map[string]string{"LANG": lang}), refDate)
		got := d.Line(3244.67)
		if got != "$3244.67" {
			t.Errorf("LANG=%s rendered %q; a dollar reader must see the dollars and nothing else", lang, got)
		}
		if d.Note() != "" {
			t.Errorf("LANG=%s produced a conversion note %q", lang, d.Note())
		}
	}
}

// MN-2: a yen reader sees both, and the yen has no decimal places.
//
// JPY has zero minor units. Printing 502,145.00 yen is not a rounding choice,
// it is a currency that does not exist.
func TestMN2_YenShowsBothAndHasNoDecimals(t *testing.T) {
	d := Detect(env(map[string]string{"LANG": "ja_JP.UTF-8"}), refDate)
	got := d.Line(3244.67)
	if !strings.HasPrefix(got, "$3244.67") {
		t.Errorf("the dollar figure must lead, and survive: %q", got)
	}
	if !strings.Contains(got, "\u00a5") {
		t.Errorf("the reader's own symbol is what they scan for, and %q has none", got)
	}
	if !strings.Contains(got, "JPY") {
		t.Errorf("the ISO code has to appear too: a symbol alone is ambiguous, and %q has none", got)
	}
	if strings.Contains(got, ".00") || strings.Contains(got, ".0 ") {
		t.Errorf("JPY has no minor units and %q prints some", got)
	}
	if !strings.Contains(got, RatesDate) {
		t.Errorf("a converted figure must carry the rate's date: %q", got)
	}
}

// MN-3: LC_ALL beats LC_MONETARY beats LANG.
//
// POSIX order, and it is not academic: a developer commonly runs an English
// interface with a local monetary locale, and reading LANG alone would tell
// that person their money is dollars.
func TestMN3_LocalePrecedence(t *testing.T) {
	d := Detect(env(map[string]string{
		"LANG": "en_US.UTF-8", "LC_MONETARY": "ja_JP.UTF-8", "LC_ALL": "de_DE.UTF-8",
	}), refDate)
	if d.Code != "EUR" {
		t.Errorf("LC_ALL should win, got %q", d.Code)
	}
	d = Detect(env(map[string]string{"LANG": "en_US.UTF-8", "LC_MONETARY": "ja_JP.UTF-8"}), refDate)
	if d.Code != "JPY" {
		t.Errorf("LC_MONETARY should beat LANG, got %q", d.Code)
	}
}

// MN-4: a territory with no rate in the table converts nothing.
//
// The table holds what the ECB publishes and no more. A locale outside it must
// fall back to dollars rather than reach for a nearby number, which is the
// failure this project keeps finding: an absent value standing in for a
// measured one.
func TestMN4_NoRateMeansNoConversion(t *testing.T) {
	d := Detect(env(map[string]string{"LANG": "vi_VN.UTF-8"}), refDate)
	if got := d.Line(10); got != "$10.00" {
		t.Errorf("a currency with no published rate must not be converted: %q", got)
	}
}

// MN-5: an unset environment is not an error and not a guess.
func TestMN5_NoLocaleIsDollars(t *testing.T) {
	d := Detect(env(nil), refDate)
	if got := d.Line(10); got != "$10.00" {
		t.Errorf("with no locale set the answer is dollars, got %q", got)
	}
}

// MN-6: a stale table stops converting rather than converting quietly.
//
// Model prices move a few times a year and the price table carries an age
// note. Exchange rates move every day, and a year-old rate can be twenty
// percent out, which is larger than everything this tool measures. Past the
// limit the dollars stand alone and the reason is said out loud.
func TestMN6_AStaleTableRefusesToConvert(t *testing.T) {
	old := refDate.AddDate(1, 0, 1)
	d := Detect(env(map[string]string{"LANG": "ja_JP.UTF-8"}), old)
	if got := d.Line(3244.67); got != "$3244.67" {
		t.Errorf("a year-old rate must not be used: %q", got)
	}
	if !strings.Contains(d.Note(), "too old") {
		t.Errorf("the refusal has to say why, got %q", d.Note())
	}
}

// MN-7: a rate the user supplies wins, and is labelled as theirs.
//
// Someone who knows the rate their card settled at can say so, and the output
// must not then claim the ECB said it.
func TestMN7_AUserRateWinsAndIsAttributed(t *testing.T) {
	d := Detect(env(map[string]string{"LANG": "ja_JP.UTF-8", "REPLAY_FX_JPY": "160.5"}), refDate)
	if d.Rate != 160.5 {
		t.Errorf("user rate ignored, got %v", d.Rate)
	}
	got := d.Line(100)
	if !strings.Contains(got, "\u00a516,050") {
		t.Errorf("user rate not applied: %q", got)
	}
	if strings.Contains(got, RatesDate) || strings.Contains(d.Note(), RatesSource) {
		t.Errorf("a user-supplied rate must not be attributed to the ECB: %q / %q", got, d.Note())
	}
}

// MN-8: an unreadable user rate is refused, not silently ignored.
//
// Falling back to the table would price the report at a rate the user did not
// ask for while their own setting sat there looking applied.
func TestMN8_AnUnusableUserRateIsRefused(t *testing.T) {
	for _, bad := range []string{"abc", "0", "-3", ""} {
		d := Detect(env(map[string]string{"LANG": "ja_JP.UTF-8", "REPLAY_FX_JPY": bad}), refDate)
		if got := d.Line(100); got != "$100.00" {
			t.Errorf("REPLAY_FX_JPY=%q should stop the conversion, got %q", bad, got)
		}
		if !strings.Contains(d.Note(), "REPLAY_FX_JPY") {
			t.Errorf("REPLAY_FX_JPY=%q was ignored without saying so: %q", bad, d.Note())
		}
	}
}

// MN-9: the note says the converted figure is not the bill.
//
// The whole hazard of this feature in one assertion. Without it the tool
// prints a number in the reader's own currency that they were never charged.
func TestMN9_TheCaveatIsPresentWheneverAFigureIsConverted(t *testing.T) {
	d := Detect(env(map[string]string{"LANG": "ja_JP.UTF-8"}), refDate)
	note := d.Note()
	for _, want := range []string{"billed in US dollars", "reference"} {
		if !strings.Contains(note, want) {
			t.Errorf("the conversion note must contain %q, got %q", want, note)
		}
	}
}

// MN-10: the dollar figure is never replaced, for any currency in the table.
//
// Stated over the whole table rather than for one example, because the way
// this would regress is somebody adding a currency and formatting it its own
// way.
func TestMN10_TheDollarsAlwaysSurvive(t *testing.T) {
	for code := range rates {
		d := Display{Code: code, Rate: rates[code], Date: RatesDate, Source: RatesSource}
		got := d.Line(1234.5)
		if !strings.HasPrefix(got, "$1234.50") {
			t.Errorf("%s dropped or moved the dollar figure: %q", code, got)
		}
	}
}

// MN-11: the column form carries the symbol and nothing else.
//
// A rate and a date repeated down every row of a table is noise readers learn
// to skip, and a caveat nobody reads is not a caveat. They appear once, in the
// note. What the column needs is the glyph the reader scans for.
func TestMN11_TheColumnFormIsJustTheFigure(t *testing.T) {
	d := Detect(env(map[string]string{"LANG": "ja_JP.UTF-8"}), refDate)
	got := d.Short(3244.67)
	if got != "\u00a5502,111" {
		t.Errorf("column form is %q", got)
	}
	if strings.Contains(got, RatesDate) || strings.Contains(got, "/USD") {
		t.Errorf("the column must not repeat the rate or its date: %q", got)
	}
}

// MN-12: a currency with no symbol in the table falls back to its ISO code.
//
// Guessing a glyph is how a report ends up asserting that some currency uses a
// symbol nobody checked. The code is always correct and always ASCII.
func TestMN12_AnUnknownSymbolFallsBackToTheCode(t *testing.T) {
	if got := Symbol("XTS"); got != "XTS " {
		t.Errorf("unknown currency should render its code, got %q", got)
	}
}
