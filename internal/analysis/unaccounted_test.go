package analysis

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Tokens a turn was billed that no visible block can account for.
//
// B2 in the benefit-gap analysis: the ledger and transcript recordings of the
// same session agree on the total and contradict each other on the split. On
// session 7022b9f2 the ledger reports
//
//	tool definitions (189 tools)   167k once
//	tool definitions (224 tools)    21k once     <- the tool list grew mid-session
//
// and the transcript reports
//
//	system prompt and tool definitions (not in transcript)  182k once
//	tool result: Bash echo one                               21k once   <- the bug
//
// The eight tool results in that session are `one` through `eight`: measured
// from the file, 32 bytes in total. The transcript path charges 21k tokens to
// one of them.
//
// The mechanism is shareByBytes. A turn's newly billed tokens are shared across
// the blocks new to that turn, in proportion to bytes. When the shared prefix
// grows and the transcript cannot see it — a second tool-definition re-lay, an
// MCP server binding late — the only new block visible in that turn is whatever
// the agent happened to be doing, and it absorbs the entire write.
//
// prefixChanged does not catch it. That flag fires when a message an earlier
// request carried is REPLACED, which is what the ledger sees; in the transcript
// the prefix message is not present at all, so nothing looks replaced. The only
// signal available is arithmetic: the write is far larger than the visible new
// bytes could be worth.
//
// The band is the fit's own. accountable × (1 + RelativeError) is a tight
// allowance for a well-fitted session and exactly double for an unfitted one,
// because RelativeError is 1 when no spread was measured. No new constant is
// introduced, and a session whose ratio genuinely understates its content — a
// schema-heavy turn — keeps its attribution as long as it stays inside the band
// the fit already claims.
//
// What this does not do: recover WHERE the mass belongs. The transcript does
// not contain it. It stops a number being charged to a label that provably
// cannot carry it, and names the remainder for what it is.

// UA1: a write far larger than the visible bytes is not charged to them.
func TestUA1_AWriteTheVisibleBytesCannotExplainIsNotChargedToThem(t *testing.T) {
	// 30 bytes of visible new content, and a 21,000-token write.
	acc := map[string]*labelAcc{}
	get := func(l string) *labelAcc {
		if acc[l] == nil {
			acc[l] = &labelAcc{}
		}
		return acc[l]
	}
	fit := TokenFit{TokensPerByte: DefaultTokensPerByte, RelativeError: 1, Turns: 0}
	blocks := []transcript.Block{{Label: "tool result: Bash echo one", Bytes: 30}}

	shareWithinTheFit(blocks, 21_000, 1, fit, get)

	visible := 0
	for l, a := range acc {
		if l == UnaccountedLabel {
			continue
		}
		visible += a.once.Total()
	}
	if visible > 100 {
		t.Errorf("30 bytes of visible content were charged %d tokens; at %.3f tokens/byte "+
			"they are worth about %d", visible, fit.TokensPerByte, fit.EstimateTokens(30))
	}
	un := acc[UnaccountedLabel]
	if un == nil || un.once.Total() < 20_000 {
		t.Fatalf("the unexplained remainder was not reported under %q: %+v", UnaccountedLabel, acc)
	}
}

// UA2: an ordinary write is untouched.
//
// The guard against a fix that quietly reroutes normal attribution. If a turn's
// write is what its bytes are worth, nothing goes to the remainder.
func TestUA2_AnOrdinaryWriteIsAttributedAsBefore(t *testing.T) {
	acc := map[string]*labelAcc{}
	get := func(l string) *labelAcc {
		if acc[l] == nil {
			acc[l] = &labelAcc{}
		}
		return acc[l]
	}
	fit := TokenFit{TokensPerByte: 0.25, RelativeError: 0.10, Turns: 12}
	blocks := []transcript.Block{{Label: "user text", Bytes: 4000}, {Label: "tool result: Read", Bytes: 12_000}}

	// 16,000 bytes at 0.25 is 4,000 tokens: exactly what the fit expects.
	shareWithinTheFit(blocks, 4_000, 1, fit, get)

	if un := acc[UnaccountedLabel]; un != nil && un.once.Total() > 0 {
		t.Errorf("an ordinary write produced %d unaccounted tokens", un.once.Total())
	}
	total := 0
	for _, a := range acc {
		total += a.once.Total()
	}
	if total != 4_000 {
		t.Errorf("attribution lost or invented tokens: %d of 4000", total)
	}
}

// UA3: the fit's own band is respected, so a dense turn is not reclassified.
//
// A schema-heavy turn is worth more per byte than the prose ratio says — that
// is the whole reason ADR-0018's EstimateOutsideFit exists. A turn inside the
// band the fit already claims must keep its attribution, or this fix would
// start relabelling exactly the content #121 was about.
func TestUA3_ADenseTurnInsideTheBandKeepsItsAttribution(t *testing.T) {
	acc := map[string]*labelAcc{}
	get := func(l string) *labelAcc {
		if acc[l] == nil {
			acc[l] = &labelAcc{}
		}
		return acc[l]
	}
	fit := TokenFit{TokensPerByte: 0.25, RelativeError: 0.50, Turns: 9}
	blocks := []transcript.Block{{Label: "tool definitions", Bytes: 4000}}
	// 4000 bytes is nominally 1000 tokens; 1400 is inside +50%.
	shareWithinTheFit(blocks, 1_400, 1, fit, get)
	if un := acc[UnaccountedLabel]; un != nil && un.once.Total() > 0 {
		t.Errorf("a turn inside the fit's own error band was partly reclassified: %d tokens",
			un.once.Total())
	}
}
