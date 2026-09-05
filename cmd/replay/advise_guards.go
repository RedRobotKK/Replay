package main

import (
	"fmt"

	"github.com/RedRobotKK/Replay/internal/advisor"
)

// guardAdviceLines suggests spend caps from the user's own session spread.
//
// A cap is a refusal waiting to happen, so the number behind it matters more
// than the fact of having one. This derives it from the ledger with Tukey's
// upper fence, prints the quartiles it came from and the N it was computed
// over, and refuses below a floor rather than dressing up a guess.
//
// It is print-only and deliberately not reachable from --apply. Replay writes
// exactly one setting for you, the prompt cache TTL, which costs nothing if it
// is wrong. A spend cap that the tool set is a refusal the user did not choose.
func guardAdviceLines(sessionUSD, sessionTokens []float64) []string {
	usd, okUSD := advisor.UpperFence(sessionUSD, advisor.MinGuardSessions)
	tok, okTok := advisor.UpperFence(sessionTokens, advisor.MinGuardSessions)
	if !okUSD && !okTok {
		return []string{
			fmt.Sprintf("not enough evidence for a spend cap: %d sessions is the floor, and a", advisor.MinGuardSessions),
			"threshold from fewer is a threshold from noise. It would refuse live",
			"requests with the same confidence as one from three hundred sessions.",
		}
	}
	out := []string{"caps from your own spread, Tukey's upper fence (Q3 + 1.5*IQR):"}
	if okUSD {
		out = append(out,
			fmt.Sprintf("  --spend-session-usd %.2f", usd.Upper),
			fmt.Sprintf("    Q1 $%.2f, median $%.2f, Q3 $%.2f, IQR $%.2f, over %d sessions", usd.Q1, usd.Median, usd.Q3, usd.IQR, usd.N))
	}
	if okTok {
		out = append(out,
			fmt.Sprintf("  --spend-session-tokens %.0f", tok.Upper),
			fmt.Sprintf("    Q1 %.0f, median %.0f, Q3 %.0f, IQR %.0f, over %d sessions", tok.Q1, tok.Median, tok.Q3, tok.IQR, tok.N))
	}
	out = append(out,
		"Above the fence is an outlier against your own history, not a rule.",
		"Nothing is written: pass these yourself if you want them.")
	return out
}
