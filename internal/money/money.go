// Package money shows a dollar figure in the reader's own currency, beside it.
//
// The tool measures spend against a first-party price table denominated in US
// dollars, and that is what the provider bills. A reader in Tokyo or Warsaw
// still has to know whether a number is large, and making them do the
// arithmetic is a worse answer than doing it for them.
//
// What this package refuses to do is swap the unit. A card issuer converts at
// its own rate on its own settlement date and adds a foreign transaction fee,
// commonly 1.5 to 3 percent, so a figure converted at a reference rate is not
// what anybody paid. Printed instead of the dollars it would be a number the
// reader was never charged, presented as their cost. Printed beside them, with
// the rate and its date, it is what it is: an indication of size.
//
// Nothing here reaches the network. The locale comes from the environment, the
// rates are compiled in from a dated ECB snapshot, and a reader who wants a
// rate the tool does not have supplies it themselves.
package money

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaxRateAge is how old a compiled rate may be before it stops being used.
//
// The price table carries an age note and keeps working, because model prices
// move a few times a year. Exchange rates move every day: a year-old rate can
// be twenty percent out, which is larger than any effect this tool measures,
// so past this the conversion stops rather than degrading quietly. A wrong
// number in the reader's own currency is more convincing than a wrong number
// in a foreign one, which is exactly what makes it worse.
const MaxRateAge = 365 * 24 * time.Hour

// StaleRateAge is when a rate starts saying its age out loud while still
// being used.
const StaleRateAge = 30 * 24 * time.Hour

// Display is the decision about how to render money, made once.
//
// The zero value is dollars only, which is the right answer whenever anything
// is missing or unreadable: no locale, no rate, an unparseable override.
type Display struct {
	// Code is the ISO 4217 code to convert to, empty for dollars only.
	Code string
	// Rate is US dollars to one unit of Code.
	Rate float64
	// Date is the rate's publication date, empty when the user supplied it.
	Date string
	// Source names where the rate came from, for the note.
	Source string
	// note carries a refusal the reader should see: a rate too old to use, or
	// an override that could not be read. Kept rather than logged, because a
	// setting that looks applied and is not is the failure this avoids.
	note string
}

// Detect resolves the display from the environment and the current time.
//
// look has os.LookupEnv's shape, reporting whether the variable was SET as
// well as what it holds, and that is load bearing rather than tidy. A variable
// set to empty is not a variable that is absent: REPLAY_FX_JPY=$RATE with an
// empty RATE is a rate the reader believes they supplied, and answering it
// with the built-in table is the failure this whole codebase keeps finding,
// where a missing value is served as if it were a measured one.
//
// It is passed in rather than read so the decision is testable without
// mutating process state, and so the precedence below is visible.
func Detect(look func(string) (string, bool), now time.Time) Display {
	if look == nil {
		return Display{}
	}
	getenv := func(k string) string { v, _ := look(k); return v }
	code := currencyFor(localeOf(getenv))
	if code == "" || code == "USD" {
		return Display{}
	}

	// An explicit rate wins, and is refused rather than ignored when it cannot
	// be used. Silently falling back to the table would price the report at a
	// rate the reader did not ask for while their own setting sat there
	// looking applied.
	key := "REPLAY_FX_" + code
	if raw, ok := look(key); ok {
		v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil || v <= 0 {
			return Display{note: key + " is not a usable rate, so figures are in US dollars only."}
		}
		return Display{Code: code, Rate: v, Source: "your " + key}
	}

	rate, ok := rates[code]
	if !ok {
		return Display{}
	}
	d, err := time.Parse("2006-01-02", RatesDate)
	if err != nil {
		return Display{}
	}
	age := now.Sub(d)
	if age > MaxRateAge {
		return Display{note: fmt.Sprintf(
			"The built-in exchange rates are dated %s, which is too old to convert with, so figures are in US dollars only. Set REPLAY_FX_%s to a rate you trust.",
			RatesDate, code)}
	}
	out := Display{Code: code, Rate: rate, Date: RatesDate, Source: RatesSource}
	if age > StaleRateAge {
		out.note = fmt.Sprintf("The exchange rate below is %d days old.", int(age.Hours()/24))
	}
	return out
}

// localeOf reads the POSIX precedence: LC_ALL, then LC_MONETARY, then LANG.
//
// The middle one is the reason this is not just LANG. A developer commonly
// runs an English interface with a local monetary locale, and reading LANG
// alone would tell that person their money is dollars.
func localeOf(getenv func(string) string) string {
	for _, k := range []string{"LC_ALL", "LC_MONETARY", "LANG"} {
		if v := getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// Line renders a dollar figure, with the local equivalent beside it when there
// is one to show.
//
// The dollars are formatted exactly as they were before this package existed
// and always come first, because they are the figure the provider billed.
func (d Display) Line(usd float64) string {
	base := fmt.Sprintf("$%.2f", usd)
	if d.Code == "" || d.Rate <= 0 {
		return base
	}
	return base + " (about " + d.Amount(usd) + ")"
}

// Short renders only the converted figure, for a column.
//
// No rate and no date: repeating them on every row of a table is noise that
// readers learn to skip, and a caveat nobody reads is not a caveat. They
// appear once, in Note, which is printed once per report.
func (d Display) Short(usd float64) string {
	if d.Code == "" || d.Rate <= 0 {
		return ""
	}
	return Symbol(d.Code) + group(strconv.FormatFloat(usd*d.Rate, 'f', minorUnits[d.Code], 64))
}

// Amount renders the converted figure with its rate, for a single figure
// standing on its own where there is no note to carry them.
func (d Display) Amount(usd float64) string {
	if d.Code == "" || d.Rate <= 0 {
		return ""
	}
	v := usd * d.Rate
	digits := minorUnits[d.Code]
	s := Symbol(d.Code) + group(strconv.FormatFloat(v, 'f', digits, 64))
	out := s + " " + d.Code
	if d.Date != "" {
		out += fmt.Sprintf(" at %s/USD, %s", trimRate(d.Rate), d.Date)
	} else {
		out += fmt.Sprintf(" at %s/USD", trimRate(d.Rate))
	}
	return out
}

// Note is the caveat and any refusal, for printing once per report rather than
// once per figure.
func (d Display) Note() string {
	if d.note != "" && d.Code == "" {
		return d.note
	}
	if d.Code == "" {
		return ""
	}
	at := fmt.Sprintf("%s %s/USD", trimRate(d.Rate), d.Code)
	if d.Date != "" {
		at += ", " + d.Source + " dated " + d.Date
	} else {
		at += ", from " + d.Source
	}
	n := fmt.Sprintf(
		"Converted at %s. You are billed in US dollars: your card issuer uses its own rate on its settlement date and usually adds a foreign transaction fee, so treat the local figure as an indication of size rather than as your bill.",
		at)
	if d.note != "" {
		n = d.note + " " + n
	}
	return n
}

// trimRate prints a rate at a useful precision without trailing zeros.
func trimRate(r float64) string {
	s := strconv.FormatFloat(r, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// group inserts thousands separators into an already-formatted number.
//
// Western grouping only, and that is a stated limitation rather than an
// oversight: de_DE writes 1.234,56 and hi_IN groups as 12,34,567. Getting
// those right needs a locale database this binary will not carry, and printing
// a number in the reader's currency with another region's punctuation is a
// smaller wrong than printing no separators at all in a figure with six
// digits.
func group(s string) string {
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")

	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String() + frac
	if neg {
		out = "-" + out
	}
	return out
}
