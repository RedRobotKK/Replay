package money

import "strings"

// currencyFor maps a POSIX locale to the currency its territory bills in.
//
// The territory is the part after the underscore: "ja_JP.UTF-8" is JP, and JP
// bills in yen. Language is not enough on its own, which is the point of using
// the territory instead: de_DE and de_CH share a language and not a currency,
// and en_US, en_GB, en_AU and en_IN share four.
//
// This covers the territories whose currency the compiled rate table actually
// carries. A locale outside it returns empty and the reader sees dollars,
// which is the honest answer: the alternative is naming a currency and then
// having no rate for it, or worse, reaching for a nearby one.
//
// Deliberately not IP geolocation. That would be a network call this binary
// does not make, it would tell a third party where the user is, and it answers
// the wrong question anyway: where the machine is sitting today is not the
// currency the card settles in. The locale is already on the machine, costs
// nothing, and is the setting the user chose.
func currencyFor(locale string) string {
	t := territoryOf(locale)
	if t == "" {
		return ""
	}
	if c, ok := territories[t]; ok {
		return c
	}
	return ""
}

// territoryOf pulls the territory out of a POSIX locale string.
//
// Handles the shapes these actually arrive in: "ja_JP", "ja_JP.UTF-8",
// "de_DE.UTF-8@euro", "en_US.iso88591". "C" and "POSIX" have no territory and
// are not an error: they mean the environment has not said, and the answer to
// that is dollars.
func territoryOf(locale string) string {
	s := locale
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	i := strings.IndexByte(s, '_')
	if i < 0 || i+1 >= len(s) {
		return ""
	}
	t := strings.ToUpper(s[i+1:])
	if len(t) != 2 {
		return ""
	}
	return t
}

// territories maps ISO 3166 territory to ISO 4217 currency.
//
// Only currencies the ECB publishes, plus the euro area. USD territories are
// present and map to USD so the lookup succeeds and the caller can see that
// the answer is "no conversion needed" rather than "no idea".
var territories = map[string]string{
	// Dollar. Named rather than defaulted, so "unknown" stays distinguishable
	// from "dollars".
	"US": "USD", "PR": "USD", "GU": "USD", "VI": "USD", "AS": "USD",
	"EC": "USD", "SV": "USD", "PA": "USD", "TL": "USD", "ZW": "USD",

	// Euro area.
	"AT": "EUR", "BE": "EUR", "CY": "EUR", "DE": "EUR", "EE": "EUR",
	"ES": "EUR", "FI": "EUR", "FR": "EUR", "GR": "EUR", "HR": "EUR",
	"IE": "EUR", "IT": "EUR", "LT": "EUR", "LU": "EUR", "LV": "EUR",
	"MT": "EUR", "NL": "EUR", "PT": "EUR", "SI": "EUR", "SK": "EUR",
	"MC": "EUR", "AD": "EUR", "SM": "EUR", "VA": "EUR", "ME": "EUR",

	// Everything else the rate table carries.
	"AU": "AUD",
	"BR": "BRL",
	"CA": "CAD",
	"CH": "CHF", "LI": "CHF",
	"CN": "CNY",
	"CZ": "CZK",
	"DK": "DKK", "GL": "DKK", "FO": "DKK",
	"GB": "GBP", "UK": "GBP", "IM": "GBP", "JE": "GBP", "GG": "GBP",
	"HK": "HKD",
	"HU": "HUF",
	"ID": "IDR",
	"IL": "ILS",
	"IN": "INR", "BT": "INR",
	"IS": "ISK",
	"JP": "JPY",
	"KR": "KRW",
	"MX": "MXN",
	"MY": "MYR",
	"NO": "NOK", "SJ": "NOK",
	"NZ": "NZD",
	"PH": "PHP",
	"PL": "PLN",
	"RO": "RON",
	"SE": "SEK",
	"SG": "SGD",
	"TH": "THB",
	"TR": "TRY",
	"ZA": "ZAR", "LS": "ZAR", "NA": "ZAR",
}

// symbols is what a reader in each place actually expects to see.
//
// A width caution rides with every one of these. Currency symbols are almost
// all East Asian width class AMBIGUOUS: U+00A5 YEN, U+20AC EURO and U+00A3
// POUND each occupy one cell in a Latin locale and TWO in ja_JP, zh_CN and
// ko_KR. That is the same property that keeps box drawing out of this
// project's screens.
//
// So a symbol is safe where the figure it decorates ENDS a line, and unsafe
// inside a padded column, where two cells where one was budgeted shifts
// everything after it. The cost report puts the local figure last on its line
// and uses these. The TUI keeps the ISO code, which is ASCII, and its
// allowlist test refuses anything else.
//
// Where a symbol is ambiguous between currencies it is qualified rather than
// bare: A$ and C$ and HK$ are not $, and CN¥ is not ¥. The ISO code still
// appears once, in the note under the block, so nothing rests on the glyph.
var symbols = map[string]string{
	"EUR": "€", "JPY": "¥", "GBP": "£",
	"CNY": "CN¥", "KRW": "₩", "INR": "₹",
	"AUD": "A$", "CAD": "C$", "NZD": "NZ$", "HKD": "HK$", "SGD": "S$",
	"MXN": "MX$", "BRL": "R$",
	"SEK": "kr", "NOK": "kr", "DKK": "kr", "ISK": "kr",
	"PLN": "zł", "CZK": "Kč", "HUF": "Ft", "RON": "lei",
	"ILS": "₪", "TRY": "₺", "ZAR": "R", "THB": "฿",
	"PHP": "₱", "MYR": "RM", "IDR": "Rp", "CHF": "CHF",
}

// Symbol is what to print before the figure, falling back to the ISO code.
//
// The fallback is a space-separated code rather than a guessed glyph: a
// currency this table does not know is one whose symbol nobody here has
// checked, and an invented one is worse than a correct three-letter code.
func Symbol(code string) string {
	if s, ok := symbols[code]; ok {
		return s
	}
	return code + " "
}
