package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// SourceCodex marks a session read from a Codex rollout log.
const SourceCodex Source = "codex-rollout"

// CodexQuota is the rate-limit record Codex writes into its own log.
//
// It is the only live quota signal found in any client this project has
// examined. The Anthropic surface reports a rising utilization fraction only
// on the wire; the Grok CLI advertises x-ratelimit headers that never moved
// across 8 calls and 940KB of responses. This one moves: measured across 6,871
// events in a local corpus, used_percent ranged 0 to 52 over two windows.
//
// It needs no proxy, because Codex writes it to disk itself.
type CodexQuota struct {
	LimitID  string
	PlanType string
	// Primary is the short rolling window, secondary the long one. Measured
	// locally at 300 and 10080 minutes — five hours and seven days — but both
	// are read from the record rather than assumed.
	PrimaryUsedPercent     float64
	PrimaryWindowMinutes   int
	PrimaryResetsAt        int64
	SecondaryUsedPercent   float64
	SecondaryWindowMinutes int
	SecondaryResetsAt      int64
}

// CodexSession is one rollout file.
//
// Billed and Reported are kept apart on purpose. They are the same quantity
// only while the session has never compacted, and collapsing them is the
// defect this reader exists to avoid.
type CodexSession struct {
	ID            string
	Path          string
	ClientVersion string
	Source        Source
	// Billed sums the per-turn deltas. This is what was paid for.
	Billed Usage
	// Reported is the client's own final running total. Codex rebases it on
	// compaction, so on a compacted session it is smaller than Billed and is
	// not a bill.
	Reported Usage
	// Rebased records that a compaction happened, so Reported and Billed are
	// known to describe different things rather than to disagree.
	Rebased       bool
	Compactions   []Compaction
	Quota         *CodexQuota
	ContextWindow int
	// Skipped counts records this reader refused. Non-zero is not an error,
	// but it is reported, because a format change must not pass silently.
	Skipped int
}

// Total is every token in the snapshot that the provider counted.
func (u Usage) Total() int { return u.Input + u.Output }

type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type       string           `json:"type"`
	ID         string           `json:"id"`
	CLIVersion string           `json:"cli_version"`
	Info       *codexTokenInfo  `json:"info"`
	RateLimits *codexRateLimits `json:"rate_limits"`
}

type codexTokenInfo struct {
	Total         *codexUsage `json:"total_token_usage"`
	Last          *codexUsage `json:"last_token_usage"`
	ContextWindow int         `json:"model_context_window"`
}

type codexUsage struct {
	Input     int `json:"input_tokens"`
	Cached    int `json:"cached_input_tokens"`
	Output    int `json:"output_tokens"`
	Reasoning int `json:"reasoning_output_tokens"`
	Total     int `json:"total_tokens"`
}

type codexWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int     `json:"window_minutes"`
	ResetsAt      int64   `json:"resets_at"`
}

type codexRateLimits struct {
	LimitID   string       `json:"limit_id"`
	PlanType  string       `json:"plan_type"`
	Primary   *codexWindow `json:"primary"`
	Secondary *codexWindow `json:"secondary"`
}

// usage converts a Codex snapshot, or reports that it cannot be believed.
//
// cached_input_tokens is a share of input_tokens and reasoning_output_tokens a
// share of output_tokens. Either exceeding its parent describes a state that
// cannot occur, so the record is refused rather than added to a total someone
// is billed against. Negative counters are refused for the same reason.
func (c *codexUsage) usage() (Usage, bool) {
	if c == nil {
		return Usage{}, false
	}
	if c.Input < 0 || c.Output < 0 || c.Cached < 0 || c.Reasoning < 0 {
		return Usage{}, false
	}
	if c.Cached > c.Input || c.Reasoning > c.Output {
		return Usage{}, false
	}
	return Usage{
		Input:          c.Input,
		CacheRead:      c.Cached,
		Output:         c.Output,
		ThinkingTokens: c.Reasoning,
	}, true
}

func (u *Usage) add(o Usage) {
	u.Input += o.Input
	u.CacheRead += o.CacheRead
	u.Output += o.Output
	u.ThinkingTokens += o.ThinkingTokens
}

// ParseCodexFile reads one rollout-*.jsonl.
func ParseCodexFile(path string) (*CodexSession, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	s, err := ParseCodex(f)
	if err != nil {
		return nil, err
	}
	s.Path = path
	return s, nil
}

// ParseCodex reads a rollout log.
//
// Streamed a line at a time rather than read whole: the largest rollout in the
// corpus this was built against is 9,599 lines, and a reader that must hold a
// session in memory to answer a question about it will meet a longer one.
func ParseCodex(r io.Reader) (*CodexSession, error) {
	s := &CodexSession{Source: SourceCodex}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var l codexLine
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			s.Skipped++
			continue
		}
		var p codexPayload
		if len(l.Payload) > 0 {
			if err := json.Unmarshal(l.Payload, &p); err != nil {
				s.Skipped++
				continue
			}
		}
		switch l.Type {
		case "session_meta":
			s.ID, s.ClientVersion = p.ID, p.CLIVersion
		case "event_msg":
			s.event(p)
		}
	}
	return s, sc.Err()
}

func (s *CodexSession) event(p codexPayload) {
	switch p.Type {
	case "context_compacted":
		s.Compactions = append(s.Compactions, Compaction{Trigger: "context_compacted"})
		s.Rebased = true
	case "token_count":
		// The rate limits ride on the same event as the usage, and on the
		// session's opening event they arrive alone: info is null because
		// nothing has been spent yet. That record is a quota reading, not a
		// broken usage reading, and refusing it discards the signal entirely.
		if p.RateLimits != nil {
			s.Quota = p.RateLimits.quota()
		}
		if p.Info == nil {
			return
		}
		if p.Info.ContextWindow > 0 {
			s.ContextWindow = p.Info.ContextWindow
		}
		if u, ok := p.Info.Last.usage(); ok {
			s.Billed.add(u)
		} else if p.Info.Last != nil {
			s.Skipped++
		}
		if u, ok := p.Info.Total.usage(); ok {
			s.Reported = u
		}
	}
}

func (r *codexRateLimits) quota() *CodexQuota {
	q := &CodexQuota{LimitID: r.LimitID, PlanType: r.PlanType}
	if r.Primary != nil {
		q.PrimaryUsedPercent = r.Primary.UsedPercent
		q.PrimaryWindowMinutes = r.Primary.WindowMinutes
		q.PrimaryResetsAt = r.Primary.ResetsAt
	}
	if r.Secondary != nil {
		q.SecondaryUsedPercent = r.Secondary.UsedPercent
		q.SecondaryWindowMinutes = r.Secondary.WindowMinutes
		q.SecondaryResetsAt = r.Secondary.ResetsAt
	}
	return q
}
