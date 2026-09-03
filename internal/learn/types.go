package learn

import (
	"fmt"
	"strings"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// Session types are classified from signals a proxy has at a session's
// first request, so the type a policy was learned for is the type it can
// be applied to: the model family and the size of the first prompt. What
// a session becomes later (long, edit-heavy) is not known at its start,
// so it is not a type here.
const (
	// largePrefixTokens splits first prompts into small and large: above
	// it the session started with substantial instructions, attachments,
	// or tool definitions and caching decides its cost.
	largePrefixTokens = 20_000
)

// Model families, from the model id.
var modelFamilies = []string{"fable", "mythos", "opus", "sonnet", "haiku"}

// Type names a session's type as "<family>/<small|large>-prefix", or an
// empty string for a session with no requests.
func Type(s *transcript.Session) string {
	lane := analysis.MainLane(s)
	if lane == nil || len(lane.Requests) == 0 {
		return ""
	}
	first := lane.Requests[0]
	return TypeOf(first.Model, first.Usage.PromptTotal())
}

// TypeOf classifies from what the proxy sees at a first request, the
// same signals Type reads from a session.
func TypeOf(model string, firstPromptTokens int) string {
	family := "other"
	m := strings.ToLower(model)
	for _, f := range modelFamilies {
		if strings.Contains(m, f) {
			family = f
			break
		}
	}
	size := "small"
	if firstPromptTokens >= largePrefixTokens {
		size = "large"
	}
	return fmt.Sprintf("%s/%s-prefix", family, size)
}

// BytesPerTokenEstimate is the coarse ratio the proxy uses to classify a
// session at its first request, when the prompt's size is known in bytes
// but its token count only after the response. Learning classifies from
// measured tokens; the two agree except near the boundary, and a session
// that lands on the wrong side gets the overall selection, never a wrong
// one.
const BytesPerTokenEstimate = 4

// TypeFromBytes classifies from what the proxy has before the response.
func TypeFromBytes(model string, firstPromptBytes int) string {
	return TypeOf(model, firstPromptBytes/BytesPerTokenEstimate)
}

// TypeResult is the selection for one session type.
type TypeResult struct {
	Type     string     `json:"type"`
	Sessions int        `json:"sessions"`
	Selected *Candidate `json:"selected"`
	Reason   string     `json:"reason,omitempty"`
}
