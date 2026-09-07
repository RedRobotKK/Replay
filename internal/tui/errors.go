package tui

import "strings"

// Error screens, built from refusals Replay actually produces.
//
// PostHog's CLI documentation has a section answering the confusions its
// AGENT hits rather than the ones a person hits: "my agent doesn't know about
// the CLI", "the api command is unavailable", "the tool list looks outdated".
// That is the move. Whoever reads the message is stuck, and the message is the
// only thing standing between them and giving up.
//
// So every screen here has the same four parts, in this order:
//
//	what happened      one sentence, in the reader's terms rather than the
//	                   code's. Not the exception, the consequence.
//	why                the mechanism, because a fix somebody does not
//	                   understand is a fix they cannot adapt.
//	what to do         a command they can paste, or a decision to make.
//	what is unaffected named explicitly, because the worst thing an error
//	                   screen does is imply everything is broken.
//
// That last part is the one usually missing. A proxy that refuses one path is
// still proxying every other path, and a reader who cannot tell the difference
// stops the whole thing.

// Problem is one thing that went wrong and what to do about it.
type Problem struct {
	// Code is stable and greppable, so a user can search for it and an agent
	// can branch on it. Messages get rewritten; codes should not.
	Code string
	// Happened is the consequence in the reader's terms.
	Happened string
	// Why is the mechanism, in one or two lines.
	Why []string
	// Do is what to run or decide. Commands are copyable as written.
	Do []string
	// Unaffected is what still works, named rather than implied.
	Unaffected string
}

// Problems is every refusal a user can meet, with its screen.
func Problems() []Problem {
	return []Problem{
		{
			Code:     "not-parsed",
			Happened: "Replay is forwarding this traffic and reading none of it.",
			Why: []string{
				"This path is not one of the two request shapes Replay parses. Your",
				"bytes reach the provider unchanged, and nothing is recorded: no",
				"ledger entry, no spend cap, no loop detection, no masking.",
			},
			Do: []string{
				"replay serve --upstream api.anthropic.com   # a surface it can read",
				"",
				"Or keep going: forwarding is safe, it just measures nothing. The",
				"surface registry in docs/SURFACES.md says which paths are which.",
			},
			Unaffected: "every other path through this proxy is still recorded normally",
		},
		{
			Code:     "browser-request",
			Happened: "A browser tried to use the proxy, and was refused.",
			Why: []string{
				"Requests carrying browser origin headers are rejected outright. A page",
				"you visit could otherwise reach a proxy listening on your loopback and",
				"spend your tokens through it.",
			},
			Do: []string{
				"curl -s http://127.0.0.1:4000/replay/status    # from a terminal",
				"",
				"Nothing to fix if you did not mean to do this. If a tool of yours is",
				"sending browser headers, point it at the metrics listener instead.",
			},
			Unaffected: "the proxy is running and serving every non-browser client",
		},
		{
			Code:     "port-taken",
			Happened: "Something is already listening there, so Replay did not start.",
			Why: []string{
				"Taking over a socket another process owns would silently reroute its",
				"traffic through this one. Replay refuses rather than guess whether that",
				"was intended.",
			},
			Do: []string{
				"replay serve --listen 127.0.0.1:4001         # somewhere else",
				"lsof -i :4000                                # or find out what has it",
			},
			Unaffected: "nothing was changed, and the process already there is untouched",
		},
		{
			Code:     "consent-unreadable",
			Happened: "Your answer about the corpus was found and could not be trusted.",
			Why: []string{
				"The file is writable by anyone on this machine, so it is not evidence",
				"of your decision. Replay treats that as undecided rather than as a yes,",
				"and sends nothing.",
			},
			Do: []string{
				"chmod 600 ~/.config/replay/corpus-consent.toml",
				"",
				"Or delete it and answer again. Either is fine; being asked twice is",
				"cheaper than a grant nobody made.",
			},
			Unaffected: "everything local. This only governs what would leave the machine",
		},
		{
			Code:     "cap-unenforceable",
			Happened: "You set a dollar cap and you do not have one.",
			Why: []string{
				"Some requests could not be priced. They add nothing to the running",
				"total, so the limit can never be reached. The cap is configured and",
				"inert, which is worse than no cap because it reads as protection.",
			},
			Do: []string{
				"replay serve --max-day-tokens 5000000       # a cap that always counts",
				"",
				"Or keep the dollar cap and know it covers only priced traffic. The",
				"guards screen shows which is which.",
			},
			Unaffected: "token caps, the error budget and the loop guard all still fire",
		},
	}
}

// Render draws one problem inside the budget.
func (p Problem) Render() []string {
	out := []string{"  " + p.Happened, ""}
	for _, l := range p.Why {
		out = append(out, "  "+l)
	}
	out = append(out, "", "  what to do")
	for _, l := range p.Do {
		if l == "" {
			out = append(out, "")
			continue
		}
		out = append(out, "  "+l)
	}
	out = append(out, "", "  still working")
	out = append(out, "  "+p.Unaffected)
	out = append(out, "", "  "+cell("code", 10)+p.Code)
	return out
}

// Blocking reports whether a problem stops the run.
//
// Two of the five do not, and saying so is most of the value: a reader who
// cannot tell a refusal from a failure stops the whole thing over a warning.
func (p Problem) Blocking() bool {
	return strings.HasPrefix(p.Code, "port-")
}
