package cachemodel

// Model-id matching, shared by the compiled table and any loaded rules
// document so the two cannot answer differently for the same id.

// matchesModel reports whether a model id belongs to a row whose match string
// it contains.
//
// Containment alone is the rule everywhere else in this package, and it prices
// the row before the one you asked for as soon as a family gains a version. The
// id `claude-opus-4-9` contains `opus-4`, so it took the Opus 4 row at $15/$75
// while every member of that family since 4.5 is $5/$25 — a documented list
// price, three times over, for a model whose price nobody has read.
//
// What separates the two cases is what follows the match. A version component
// is a short digit run (`-9`, `-10`, `-5-1`); a build date is `-20250514`. So a
// short digit run means the id names a version this row is not for, and the row
// is skipped; anything else — end of string, a date, a word — still matches, and
// the ids the table is actually written for keep their prices.
//
// versionDigits is where the two are cut apart. YYYYMMDD is eight digits and
// published version components have never reached four, so four is the floor a
// date can be and the ceiling a version can be, with room on both sides.
//
// This does not make matching correct. It fixes one failure mode — the next
// version of a family already in the table — and leaves the rest of what
// substring matching gets wrong: `opus-5-preview` still prices as Opus 5,
// another provider's id carrying an Anthropic family name still matches, and
// neither is a version-number collision. The end state is ids matched whole,
// which is a rules-document format change and not this.
func matchesModel(lowerID, lowerMatch string) bool {
	if lowerMatch == "" {
		return false
	}
	// Every occurrence, not only the first: an id may carry the family name
	// twice (a vendor prefix, say), and one of them may be followed by a
	// version and the other not.
	for i := 0; i+len(lowerMatch) <= len(lowerID); i++ {
		if lowerID[i:i+len(lowerMatch)] != lowerMatch {
			continue
		}
		if !continuesWithVersion(lowerID[i+len(lowerMatch):]) {
			return true
		}
	}
	return false
}

// versionDigits is the longest digit run after a match that still reads as a
// version component rather than a build date.
const versionDigits = 3

// continuesWithVersion reports that the text after a match names a version the
// match does not cover.
//
// Two shapes. A digit straight after the match means the number itself
// continues: `opus-4-1` sits inside `opus-4-10`, and Opus 4.1's price is not
// Opus 4.10's. A hyphen and a SHORT run of digits means a further version
// component: `opus-4` inside `opus-4-9`. The length of that run is the whole
// decision — `opus-4-20250514` walks eight digits, which is a build date and
// still the Opus 4 row.
func continuesWithVersion(rest string) bool {
	if rest == "" {
		return false
	}
	if rest[0] >= '0' && rest[0] <= '9' {
		return true
	}
	if len(rest) < 2 || rest[0] != '-' {
		return false
	}
	n := 1
	for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
		n++
	}
	digits := n - 1
	return digits > 0 && digits <= versionDigits
}
