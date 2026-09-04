// Package masking replaces secrets in outbound request bodies with
// placeholders derived by HMAC, keeps the mapping in an encrypted vault,
// and reports what it masked. Detection is a named pattern set; the
// README lists the names and never says "all secrets" (ADR-0004).
//
// The transform edits only the matched bytes inside JSON string values
// and leaves every other byte of the body untouched, so a body without a
// secret is forwarded exactly as sent and a body with one changes only
// where the secret was. Thinking blocks and signatures are never read or
// changed.
package masking

import (
	"regexp"
)

// Pattern is one named secret format. When Group is non-zero the secret
// is that capture group and the surrounding context stays in place.
type Pattern struct {
	Name  string
	re    *regexp.Regexp
	Group int
}

// Patterns is the maintained set. Order matters where formats overlap:
// a more specific prefix comes before a general one.
var Patterns = []Pattern{
	{Name: "anthropic-api-key", re: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{Name: "openai-api-key", re: regexp.MustCompile(`sk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{20,}`)},
	{Name: "aws-access-key-id", re: regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{Name: "github-token", re: regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,})\b`)},
	{Name: "gitlab-token", re: regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`)},
	{Name: "slack-token", re: regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`)},
	{Name: "google-api-key", re: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{Name: "stripe-key", re: regexp.MustCompile(`\b[sr]k_(?:live|test)_[A-Za-z0-9]{16,}\b`)},
	{Name: "private-key-block", re: regexp.MustCompile(`-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----[\s\S]*?-----END (?:[A-Z ]+ )?PRIVATE KEY-----`)},
	{Name: "jwt", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)},
	{Name: "bearer-token", re: regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9._~+/-]{20,}=*)`), Group: 1},
	{Name: "url-credential", re: regexp.MustCompile(`\b[a-z][a-z0-9+.-]*://[^\s/:@]+:([^\s/@]{8,})@`), Group: 1},
	// Hex and lowercase credentials cannot be caught by shape: a 32-character
	// Twilio token and a 40-character git SHA are the same string to any
	// entropy test, and the SHA actually scores lower. Masking every hex run
	// would corrupt the diffs and tool output a coding agent reads all day.
	// So catch them by the company they keep instead: a name that says the
	// value is a credential, immediately followed by the value.
	{Name: "credential-assignment", re: regexp.MustCompile(
		`(?i)\b[a-z0-9_.-]*(?:token|secret|passwd|password|api[_-]?key|auth|credential)[a-z0-9_.-]*` +
			`[\\"']{0,2}\s*[:=]\s*[\\"']{0,2}([A-Za-z0-9._~+/=-]{16,})`), Group: 1},
}

// Match is one detected secret in a text: its byte range and pattern.
type Match struct {
	Start, End int
	Pattern    string
}

// Find returns the secrets in a text, earliest first, without overlaps.
// A later pattern never matches inside an earlier pattern's match.
func Find(text []byte, patterns []Pattern) []Match {
	out, _ := find(text, patterns)
	return out
}

// find is Find returning the bytes its matches cover, so a further
// detector can avoid them.
func find(text []byte, patterns []Pattern) ([]Match, []bool) {
	var out []Match
	taken := make([]bool, len(text))
	for _, p := range patterns {
		for _, loc := range p.re.FindAllSubmatchIndex(text, -1) {
			start, end := loc[0], loc[1]
			if p.Group > 0 && loc[2*p.Group] >= 0 {
				start, end = loc[2*p.Group], loc[2*p.Group+1]
			}
			if end-start < 1 || overlaps(taken, start, end) {
				continue
			}
			for i := start; i < end; i++ {
				taken[i] = true
			}
			out = append(out, Match{Start: start, End: end, Pattern: p.Name})
		}
	}
	sortMatches(out)
	return out, taken
}

func overlaps(taken []bool, start, end int) bool {
	for i := start; i < end; i++ {
		if taken[i] {
			return true
		}
	}
	return false
}

func sortMatches(ms []Match) {
	for i := 1; i < len(ms); i++ {
		for j := i; j > 0 && ms[j].Start < ms[j-1].Start; j-- {
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
}

// ParseUserPatterns reads user-defined patterns, one per line as
// "name<TAB>regexp"; blank lines and lines starting with # are skipped.
// A user pattern masks its whole match.
func ParseUserPatterns(text string) ([]Pattern, error) {
	var out []Pattern
	for _, line := range splitLines(text) {
		if line == "" || line[0] == '#' {
			continue
		}
		name, expr, ok := cutTab(line)
		if !ok || name == "" || expr == "" {
			return nil, &PatternError{Line: line, Reason: "expected name, a tab, and a regular expression"}
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, &PatternError{Line: name, Reason: err.Error()}
		}
		out = append(out, Pattern{Name: "user:" + name, re: re})
	}
	return out, nil
}

// PatternError reports a user pattern that could not be used. It names
// the line's pattern name, never the expression's matches.
type PatternError struct {
	Line   string
	Reason string
}

func (e *PatternError) Error() string { return "pattern " + e.Line + ": " + e.Reason }

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, trimCR(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, trimCR(s[start:]))
	}
	return out
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func cutTab(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
