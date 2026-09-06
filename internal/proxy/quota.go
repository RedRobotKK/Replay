package proxy

import (
	"net/http"
	"strings"
)

// quotaPrefixes are the response-header families that describe what a request
// spent against a rate limit, rather than what it cost in dollars.
//
// Prefix matching rather than an enumerated list: providers add limit
// dimensions (input tokens, output tokens, a separate long-context bucket)
// without notice, and an enumeration silently drops the new one - which is the
// failure mode that leaves a user's real constraint unmeasured while the file
// still looks complete.
var quotaPrefixes = []string{
	"anthropic-ratelimit-",
	"x-ratelimit-",
}

// quotaExact are single headers outside those families that belong with them.
var quotaExact = []string{
	// Not a budget but the lockout itself: the provider saying the budget is
	// gone and naming the wait. For a flat seat that is the whole event.
	"retry-after",
}

// quotaFrom extracts the quota headers from a response, or nil when it carried
// none.
//
// Values are stored as the provider sent them, keyed by the header's own
// lowercased name, and deliberately not parsed into numbers and durations here.
// The formats disagree across providers and across fields on the same provider:
// Anthropic's reset is an RFC 3339 instant, OpenAI's is a duration string like
// "6ms", and retry-after is either a count of seconds or an HTTP date. A typed
// struct has to guess one shape per field, and a wrong guess fails silently -
// leaving a zero that reads as "the provider reported none" rather than "this
// did not parse". Keeping the bytes means a parse failure happens in analysis,
// where it is visible and where the record still holds what would have been
// needed to fix it.
//
// It is an allowlist rather than a copy of the header block, because response
// headers are provider-controlled input and the ledger is append-only: a
// passthrough would put whatever the provider chose to send - a cookie, a token
// echo, an identifier - into a file the user publishes findings from.
func quotaFrom(h http.Header) map[string]string {
	var out map[string]string
	add := func(name, value string) {
		if value == "" {
			return
		}
		if out == nil {
			out = map[string]string{}
		}
		out[name] = value
	}
	for name, values := range h {
		if len(values) == 0 {
			continue
		}
		lower := strings.ToLower(name)
		for _, p := range quotaPrefixes {
			if strings.HasPrefix(lower, p) {
				add(lower, values[0])
				break
			}
		}
		for _, e := range quotaExact {
			if lower == e {
				add(lower, values[0])
				break
			}
		}
	}
	return out
}
