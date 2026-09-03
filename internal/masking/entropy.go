package masking

import (
	"bytes"
	"math"
)

// EntropyPattern names matches of the entropy heuristic in reports and
// scope specs.
const EntropyPattern = "entropy"

// Entropy heuristic bounds. A candidate is a maximal run of token
// characters of at least EntropyMinLength; longer than EntropyMaxLength
// it is treated as encoded data rather than a credential. It must mix
// upper case, lower case, and digits, which rules out hashes, commit ids,
// and uuids; its Shannon entropy per byte must reach EntropyMinBits,
// which rules out words and paths; and the share of positions where the
// character class changes must reach EntropyMinTransitions, which rules
// out camel-case identifiers, whose classes change only at word
// boundaries. The corpus test records where each line falls.
const (
	EntropyMinLength      = 32
	EntropyMaxLength      = 256
	EntropyMinBits        = 4.0
	EntropyMinTransitions = 0.4
)

// benignPrefixes open runs that are random by construction but never a
// credential: subresource integrity hashes.
var benignPrefixes = []string{"sha256-", "sha384-", "sha512-"}

// isTokenByte reports whether a byte can be part of a credential: the
// base64 and URL-safe base64 alphabets.
func isTokenByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '+', c == '/', c == '=', c == '_', c == '-':
		return true
	}
	return false
}

// FindEntropy returns runs that look like credentials by shape and
// entropy, skipping bytes already taken by a pattern match.
func FindEntropy(text []byte, taken []bool) []Match {
	var out []Match
	for i := 0; i < len(text); {
		if !isTokenByte(text[i]) {
			i++
			continue
		}
		j := i
		for j < len(text) && isTokenByte(text[j]) {
			j++
		}
		if looksLikeCredential(text[i:j]) && !overlaps(taken, i, j) {
			out = append(out, Match{Start: i, End: j, Pattern: EntropyPattern})
		}
		i = j
	}
	return out
}

func looksLikeCredential(run []byte) bool {
	if len(run) < EntropyMinLength || len(run) > EntropyMaxLength {
		return false
	}
	for _, p := range benignPrefixes {
		if bytes.HasPrefix(run, []byte(p)) {
			return false
		}
	}
	if pathLike(run) {
		return false
	}
	var seen [4]bool
	transitions := 0
	for i, c := range run {
		k := class(c)
		seen[k] = true
		if i > 0 && class(run[i-1]) != k {
			transitions++
		}
	}
	if !seen[classLower] || !seen[classUpper] || !seen[classDigit] {
		return false
	}
	if float64(transitions)/float64(len(run)-1) < EntropyMinTransitions {
		return false
	}
	return shannonBits(run) >= EntropyMinBits
}

// pathLike reports a run with a separator and a segment of one character
// class of two or more characters ("com", "pull", "15"): a path or URL
// tail, which a base64 credential is very unlikely to contain.
func pathLike(run []byte) bool {
	if !bytes.ContainsRune(run, '/') {
		return false
	}
	for _, seg := range bytes.Split(run, []byte("/")) {
		if len(seg) < 2 {
			continue
		}
		k := class(seg[0])
		uniform := k != classOther
		for _, c := range seg[1:] {
			if class(c) != k {
				uniform = false
				break
			}
		}
		if uniform {
			return true
		}
	}
	return false
}

// Character classes for the transition measure.
const (
	classLower = iota
	classUpper
	classDigit
	classOther
)

func class(c byte) int {
	switch {
	case c >= 'a' && c <= 'z':
		return classLower
	case c >= 'A' && c <= 'Z':
		return classUpper
	case c >= '0' && c <= '9':
		return classDigit
	}
	return classOther
}

// shannonBits is the entropy of the byte distribution in bits per byte.
func shannonBits(b []byte) float64 {
	var counts [256]int
	for _, c := range b {
		counts[c]++
	}
	n := float64(len(b))
	bits := 0.0
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		bits -= p * math.Log2(p)
	}
	return bits
}
