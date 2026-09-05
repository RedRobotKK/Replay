package main

import (
	"fmt"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// claimLines renders what the provider says beside what traffic showed.
//
// This is the line that makes Replay a measuring instrument rather than a
// calculator. Every other tool in this space reads the provider's published
// numbers and multiplies. This one can say the published number is wrong, and
// point at the sessions that show it.
func claimLines(models []cachemodel.ModelRule) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		if m.MinPrefixClaim == nil {
			out = append(out, fmt.Sprintf("  %-22s min prefix: %d documented, untested", m.Match, m.MinPrefix))
			continue
		}
		line := "  " + fmt.Sprintf("%-22s ", m.Match) + m.MinPrefixClaim.Describe("min prefix")
		if m.MinPrefixClaim.Status() == cachemodel.StatusContradicted {
			// Shouted rather than listed. A refuted provider figure is the one
			// thing here that no dashboard will ever tell the reader, and it
			// must not read like another row.
			line += "\n" + "  " + fmt.Sprintf("%-22s ", "") + "CONTRADICTED: real traffic disagrees with the published figure."
		}
		out = append(out, line)
	}
	return out
}
