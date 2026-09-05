package cachemodel

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Reading an x402 payment demand, and nothing else.
//
// A rules feed may answer 402 Payment Required with machine-readable terms. This
// parses those terms so the caller can report them. It cannot pay: there is no
// wallet here, no key, and no code path that moves money, by the decision in
// ADR-0013.
//
// The reason is not squeamishness. Replay is a single binary people pipe from
// curl onto machines holding provider credentials, and a tool that signs
// transactions is a tool whose supply chain is a wallet compromise. Paying is
// also a decision rather than a step: an agent pays from a wallet its operator
// funded and budgeted, and that operator authorised the agent, not this.

// maxOptionsShown caps how many payment options are rendered. A seller with
// more than a handful of ways to be paid is not being helpful.
const maxOptionsShown = 5

// PaymentTerms is one accepted way to pay, as the server described it.
type PaymentTerms struct {
	Scheme     string `json:"scheme"`
	Network    string `json:"network"`
	Amount     string `json:"maxAmountRequired"`
	Asset      string `json:"asset"`
	PayTo      string `json:"payTo"`
	Resource   string `json:"resource"`
	MimeType   string `json:"mimeType"`
	MaxTimeout int    `json:"maxTimeoutSeconds"`
	// Description is the seller's own words. Displayed, never trusted: it is
	// attacker-controlled text arriving over the network, so it is shown
	// quoted and is not parsed for meaning.
	Description string `json:"description"`
}

// PaymentRequired is the body of a 402.
type PaymentRequired struct {
	X402Version int            `json:"x402Version"`
	Accepts     []PaymentTerms `json:"accepts"`
	Error       string         `json:"error"`
}

// ParsePaymentRequired reads a 402 body. It returns an error rather than a
// partial structure when the body is not x402: a server that answers 402 with
// something else is not one to guess at.
func ParsePaymentRequired(body []byte) (PaymentRequired, error) {
	var p PaymentRequired
	if err := json.Unmarshal(body, &p); err != nil {
		return PaymentRequired{}, fmt.Errorf("the 402 body is not x402 JSON: %w", err)
	}
	if p.X402Version == 0 && len(p.Accepts) == 0 {
		return PaymentRequired{}, fmt.Errorf("the 402 body carries no x402 version and no payment options")
	}
	if len(p.Accepts) == 0 {
		return PaymentRequired{}, fmt.Errorf("x402 version %d, but no payment option was offered", p.X402Version)
	}
	return p, nil
}

// Explain renders payment terms for a person, saying plainly that Replay will
// not pay and what to do instead.
//
// Every value that came off the network is printed as data. A seller's
// description is quoted so it cannot be mistaken for Replay's own words, and
// nothing here is executed, followed or auto-retried.
func (p PaymentRequired) Explain(resource string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This rules feed asks to be paid before it will answer.\n\n")
	fmt.Fprintf(&b, "  resource  %s\n", resource)
	shown := p.Accepts
	if len(shown) > maxOptionsShown {
		shown = shown[:maxOptionsShown]
	}
	for i, t := range shown {
		if len(p.Accepts) > 1 {
			fmt.Fprintf(&b, "\n  option %d\n", i+1)
		}
		// Every one of these came off the network and every one is quoted.
		// An earlier version quoted only the description, on the reasoning
		// that it was the seller's prose and the rest were structured values.
		// They are not: they are strings in the same attacker-controlled JSON,
		// and %s put their raw bytes on a terminal. A crafted `network` could
		// clear the screen and paint "RULES INSTALLED OK" in green; a crafted
		// amount could inject newlines and forge lines in Replay's own voice,
		// so a user believed a paid feed had installed when nothing had.
		fmt.Fprintf(&b, "  amount    %q %q\n", t.Amount, shortAsset(t.Asset))
		fmt.Fprintf(&b, "  network   %q\n", t.Network)
		fmt.Fprintf(&b, "  pay to    %q\n", t.PayTo)
		if t.Description != "" {
			fmt.Fprintf(&b, "  seller says %q\n", t.Description)
		}
	}
	if len(p.Accepts) > len(shown) {
		// Rendering is unbounded work over a bounded input: a 780 KB body
		// inside the 1 MB read cap held 15,000 options and produced 75,000
		// lines, enough to push the refusal below out of scrollback. Combined
		// with an escape sequence that is a spoofing amplifier, so it is capped.
		fmt.Fprintf(&b, "\n  ... and %d more payment options, not shown\n",
			len(p.Accepts)-len(shown))
	}
	fmt.Fprintf(&b, `
Replay will not pay this. It holds no wallet and no key, and has no code that
can move money — see docs/adr/0013-x402-rules-feed.md for why that is deliberate
rather than missing.

Nothing is blocked by this. The compiled rules are complete and every command
works on them; run `+"`replay rules`"+` to see which are in effect and how old they are.

To use a paid feed, have whatever holds your wallet fetch the document and
install it from a file:

  replay rules --update ./rules.json

Or, for an agent with a funded wallet, `+"`--x402-json`"+` emits these terms as JSON
so its own spending policy can decide.
`)
	return b.String()
}

// shortAsset trims a contract address to something readable without hiding it.
func shortAsset(a string) string {
	if len(a) > 12 && strings.HasPrefix(a, "0x") {
		return a[:6] + "…" + a[len(a)-4:]
	}
	return a
}
