package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// The status line answers one question Claude Code cannot: not what this
// session has cost, which it already tells you, but how much of that cost was
// avoidable, and why.
//
// Claude Code reports cache misses in tokens. Replay owns the price table, so
// it reports them in money, live, while there is still a session left to change.
// That is the whole idea: the invoice arriving a month after the mistake is the
// problem this tool exists for, and this is the shortest possible version of the
// feedback loop.
//
// It runs on a 300ms debounce during active work, so it does arithmetic on the
// JSON it is handed and never opens a transcript.

type statusInput struct {
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Cost struct {
		TotalUSD float64 `json:"total_cost_usd"`
	} `json:"cost"`
	ContextWindow struct {
		UsedPercentage float64 `json:"used_percentage"`
	} `json:"context_window"`
	PromptCache *struct {
		Warm              bool    `json:"warm"`
		HitRatio          float64 `json:"hit_ratio"`
		Misses            int     `json:"misses"`
		TTL               string  `json:"ttl"`
		MissRecacheTokens int     `json:"miss_recache_tokens"`
		LastMissCause     *struct {
			Causes []string `json:"causes"`
		} `json:"last_miss_cause"`
	} `json:"prompt_cache"`
}

func parseStatusInput(r io.Reader) (statusInput, error) {
	var s statusInput
	body, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return s, err
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return s, nil
	}
	// A malformed payload is not worth an error: the status line's job is to
	// render something harmless whatever it is handed.
	_ = json.Unmarshal(body, &s)
	return s, nil
}

const (
	ansiDim   = "\x1b[2m"
	ansiRed   = "\x1b[31m"
	ansiGreen = "\x1b[32m"
	ansiReset = "\x1b[0m"
)

// statusLine renders one line. Colour is a parameter so the same string is
// asserted in tests and painted in a terminal.
func statusLine(s statusInput, colour bool) string {
	paint := func(code, text string) string {
		if !colour {
			return text
		}
		return code + text + ansiReset
	}

	var parts []string
	if s.Cost.TotalUSD > 0 {
		parts = append(parts, fmt.Sprintf("$%.2f", s.Cost.TotalUSD))
	}

	pc := s.PromptCache
	if pc != nil && (pc.HitRatio > 0 || pc.Misses > 0 || pc.Warm) {
		hit := fmt.Sprintf("cache %.0f%%", pc.HitRatio*100)
		switch {
		case pc.HitRatio >= 0.9:
			hit = paint(ansiGreen, hit)
		case pc.HitRatio < 0.7:
			hit = paint(ansiRed, hit)
		}
		parts = append(parts, hit)
	}

	waste, priced := avoidableUSD(s)
	// Our figure is list price applied to token counts; theirs is the charge.
	// When ours exceeds theirs the two disagree, and the one to doubt is ours.
	// Print the cause without the number rather than a figure a reader can see
	// is impossible.
	credible := priced && (s.Cost.TotalUSD <= 0 || waste <= s.Cost.TotalUSD)
	if credible {
		parts = append(parts, paint(ansiRed, fmt.Sprintf("$%.2f avoidable", waste)))
	}
	if priced && pc.LastMissCause != nil && len(pc.LastMissCause.Causes) > 0 {
		parts = append(parts, paint(ansiDim, humanCause(pc.LastMissCause.Causes[0])))
	}

	if len(parts) == 0 {
		return paint(ansiDim, "replay: waiting for the first response")
	}
	return strings.Join(parts, "  ")
}

// avoidableUSD prices the tokens a cache miss forced to be written again.
//
// Returns false when the model is not in the price table. A fabricated rate in
// front of somebody every 300ms is worse than no figure at all, and the whole
// argument of this tool is that its numbers can be checked.
func avoidableUSD(s statusInput) (float64, bool) {
	pc := s.PromptCache
	if pc == nil || pc.MissRecacheTokens <= 0 {
		return 0, false
	}
	price, ok := cachemodel.PriceFor(s.Model.ID)
	if !ok || price.InputPerMTok <= 0 {
		return 0, false
	}
	ttl := cachemodel.TTLShort
	if strings.HasPrefix(pc.TTL, "1h") {
		ttl = cachemodel.TTLLong
	}
	usd := float64(pc.MissRecacheTokens) / 1_000_000 * price.InputPerMTok * cachemodel.WriteMultiplier(ttl)
	if usd < 0.01 {
		return 0, false
	}
	return usd, true
}

// humanCause turns the API's cause token into something readable at a glance.
func humanCause(c string) string {
	switch c {
	case "tools_changed":
		return "tools changed"
	case "system_prompt_changed":
		return "system prompt changed"
	case "history_rewritten":
		return "history rewritten"
	case "ttl_expired":
		return "cache expired"
	}
	return strings.ReplaceAll(c, "_", " ")
}

func runStatusline(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("statusline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	noColour := fs.Bool("no-color", false, "never emit ANSI escape codes")
	install := fs.Bool("install", false, "print the settings.json snippet that wires this in")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if *install {
		_, err := fmt.Fprintf(stdout, `Add this to %s:

  {
    "statusLine": {
      "type": "command",
      "command": "replay statusline",
      "padding": 1
    }
  }

It reads the JSON Claude Code sends on stdin and opens no files, so it costs
nothing to run on every render. It shows spend, cache health, and what the
cache misses cost you, while the session is still running.
`, statusSettingsHint())
		return err
	}

	s, err := parseStatusInput(os.Stdin)
	if err != nil {
		// Never fail loudly: a broken status line should disappear, not shout.
		return nil
	}
	colour := !*noColour && os.Getenv("NO_COLOR") == ""
	_, err = fmt.Fprintln(stdout, statusLine(s, colour))
	return err
}

func statusSettingsHint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.claude/settings.json"
	}
	return claudeConfigDir(home) + "/settings.json"
}

var _ = time.Now
