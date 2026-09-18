package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// SourceGrok marks a session read from a Grok CLI session directory.
const SourceGrok Source = "grok-session"

// GrokUsage is one usage object exactly as the Grok CLI (x.ai) writes it.
//
// The field names are the provider's, and the counts are carried verbatim.
// That matters more here than on any surface read so far, because Grok counts
// INCLUSIVELY: cachedReadTokens is a share of inputTokens, not a figure beside
// it. Measured on 2026-09-17 over every usage object in a 106-session corpus,
// inputTokens + outputTokens == totalTokens held 2,683 times out of 2,683, and
// inputTokens + outputTokens + cachedReadTokens == totalTokens held only on the
// 25 records whose cache was zero, where the two statements are the same
// statement. So the cache sits inside the input.
//
// The subtraction that turns that into a fresh-token count is usage.FromInclusive's
// job, and this package cannot import internal/usage because internal/usage
// imports this one. Doing the subtraction here as well is how it gets done
// twice, so this type does not do it at all: it reports what the provider said.
//
// costUsdTicks is deliberately absent. Grok's own client documentation states
// its scale, but a vendor statement nobody has reconciled against a statement
// of account is not a measurement, and a field decoded into a typed struct is
// a field somebody will multiply by something. The raw payload is kept on
// GrokTurn, so a later calibration that confirms the scale loses nothing by
// its absence here.
type GrokUsage struct {
	InputTokens         int `json:"inputTokens"`
	OutputTokens        int `json:"outputTokens"`
	TotalTokens         int `json:"totalTokens"`
	CachedReadTokens    int `json:"cachedReadTokens"`
	CacheCreationTokens int `json:"cacheCreationTokens"`
	ReasoningTokens     int `json:"reasoningTokens"`
	ModelCalls          int `json:"modelCalls"`
}

// GrokTurn is one completed turn: the provider's own counts, what produced
// them, and when.
type GrokTurn struct {
	Usage GrokUsage
	// Model is the model id the turn ran under, and is empty when the record
	// named more than one. The model decides the price, and a turn billed at
	// two rates cannot be attributed to one of them.
	Model string
	At    time.Time
	// Raw is the provider's usage object, verbatim. Normalising is lossy by
	// construction and a field nobody knew mattered is exactly what a later
	// calibration needs — costUsdTicks being the live example.
	Raw json.RawMessage
}

// GrokRefusal is one record this reader would not add to a total, with the
// reason. A bare count says something was dropped without saying what.
type GrokRefusal struct {
	Line   int
	Reason string
}

// GrokSession is one session directory.
//
// Turns and Rollup are kept apart on purpose. Rollup is the session's own
// usage.json total, and on this corpus a parent session's rollup absorbs the
// usage of sub-agent sessions that are ALSO on disk as sessions in their own
// right: one session of 34 reported 222,856,819 input tokens against a
// per-turn sum of 193,337,607, and the 18 turns that differ are exactly the
// ones whose prompt_id reads "subagent-completed-<session-id>". Summing the
// rollups across a machine would count the sub-agent work twice. Summing the
// per-turn records, once per session directory, counts it once.
type GrokSession struct {
	ID     string
	Path   string
	Source Source
	// Turns is every completed turn this reader accepted: one provider call
	// group each, not one file each. A session directory is not a turn and a
	// turn is not a request; modelCalls inside the usage says how many
	// requests the turn made.
	Turns []GrokTurn
	// Refused is every record that could not be believed. Non-zero is not an
	// error, but it is reported, because a format change must not pass
	// silently and an absent count is not a zero one.
	Refused []GrokRefusal
	// Rollup is the session's own usage.json total, when one is on disk. 34 of
	// 106 session directories carry one.
	Rollup *GrokUsage
}

// grokUpdate is the envelope updates.jsonl uses: a JSON-RPC style notification
// whose params carry one session update.
type grokUpdate struct {
	Timestamp int64 `json:"timestamp"`
	Params    struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string          `json:"sessionUpdate"`
			Usage         json.RawMessage `json:"usage"`
		} `json:"update"`
	} `json:"params"`
}

// grokRawUsage is the usage object plus the per-model breakdown, which is the
// only place a turn record names its model.
type grokRawUsage struct {
	GrokUsage
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
}

// grokRollup is usage.json: a session-level total, and the per-turn history
// that produced it.
type grokRollup struct {
	SessionID string     `json:"sessionId"`
	Session   *GrokUsage `json:"session"`
}

// check reports why this usage object cannot be believed, or "" when it can.
//
// Every rule below states something that cannot be true of a usage object, so
// a record breaking one is refused rather than added to a total somebody is
// billed against. The alternative is adding a zero, which loses the tokens
// without saying so.
func (u grokRawUsage) check() string {
	if u.InputTokens < 0 || u.OutputTokens < 0 || u.TotalTokens < 0 ||
		u.CachedReadTokens < 0 || u.CacheCreationTokens < 0 || u.ReasoningTokens < 0 {
		return "a negative token counter"
	}
	// cachedReadTokens is a share of inputTokens, and so is cacheCreationTokens
	// where the provider reports one. Either exceeding the prompt it is part
	// of describes a state that cannot occur. Without this the one counter
	// with no upper bound is how a bad record reaches a total.
	if u.CachedReadTokens > u.InputTokens {
		return fmt.Sprintf("cachedReadTokens %d exceeds inputTokens %d, which it is part of",
			u.CachedReadTokens, u.InputTokens)
	}
	if u.CachedReadTokens+u.CacheCreationTokens > u.InputTokens {
		return fmt.Sprintf("cachedReadTokens %d plus cacheCreationTokens %d exceeds inputTokens %d",
			u.CachedReadTokens, u.CacheCreationTokens, u.InputTokens)
	}
	if u.ReasoningTokens > u.OutputTokens {
		return fmt.Sprintf("reasoningTokens %d exceeds outputTokens %d, which it is part of",
			u.ReasoningTokens, u.OutputTokens)
	}
	// A total with no breakdown is absent, not zero.
	if u.TotalTokens > 0 && u.InputTokens+u.OutputTokens == 0 {
		return fmt.Sprintf("totalTokens %d arrived with no breakdown at all", u.TotalTokens)
	}
	// The surface's own conservation law, and the statement that makes the
	// inclusive reading safe to rely on. It held on 2,683 of 2,683 usage
	// objects measured. A record that breaks it is either a format this build
	// does not understand or a counter that moved, and in both cases the
	// cached share cannot be placed inside or beside the input with any
	// confidence, so it is not placed at all.
	if u.TotalTokens > 0 && u.InputTokens+u.OutputTokens != u.TotalTokens {
		return fmt.Sprintf("inputTokens %d plus outputTokens %d is not totalTokens %d, "+
			"so this build cannot say whether the cache sits inside the input",
			u.InputTokens, u.OutputTokens, u.TotalTokens)
	}
	return ""
}

// model names the model when the record names exactly one.
func (u grokRawUsage) model() string {
	if len(u.ModelUsage) != 1 {
		return ""
	}
	for k := range u.ModelUsage {
		return k
	}
	return ""
}

// ParseGrokSession reads one session directory: ~/.grok/sessions/<cwd>/<id>/.
//
// The per-turn records in updates.jsonl are the reading. usage.json is read
// where it exists, and kept apart: see GrokSession.
func ParseGrokSession(dir string) (*GrokSession, error) {
	f, err := os.Open(filepath.Join(dir, grokUpdatesFile))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	s, err := ParseGrokUpdates(f)
	if err != nil {
		return nil, err
	}
	s.Path = dir
	if s.ID == "" {
		s.ID = filepath.Base(dir)
	}
	if b, err := os.ReadFile(filepath.Join(dir, grokUsageFile)); err == nil {
		var r grokRollup
		if json.Unmarshal(b, &r) == nil && r.Session != nil {
			s.Rollup = r.Session
		}
	}
	return s, nil
}

// grokUpdatesFile is the per-turn record, and grokUsageFile the session's own
// rollup beside it.
const (
	grokUpdatesFile = "updates.jsonl"
	grokUsageFile   = "usage.json"
)

// ParseGrokUpdates reads an updates.jsonl stream.
//
// Streamed a line at a time rather than read whole. The largest session in the
// corpus this was built against carries 5,401 updates of which 133 are turns,
// and a reader that must hold a session in memory to answer a question about
// it will meet a longer one.
func ParseGrokUpdates(r io.Reader) (*GrokSession, error) {
	s := &GrokSession{Source: SourceGrok}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	line := 0
	for sc.Scan() {
		b := sc.Bytes()
		if len(trimSpaceBytes(b)) == 0 {
			continue
		}
		line++
		var up grokUpdate
		if err := json.Unmarshal(b, &up); err != nil {
			s.Refused = append(s.Refused, GrokRefusal{Line: line, Reason: "the line is not JSON this reader can parse"})
			continue
		}
		if s.ID == "" {
			s.ID = up.Params.SessionID
		}
		// The usage rides on the turn record and on nothing else. Tool calls,
		// message chunks and thought chunks outnumber it better than 20 to 1,
		// so a reader that took the usage off any update carrying one would
		// report a different number of provider calls than were made.
		if len(up.Params.Update.Usage) == 0 {
			continue
		}
		var u grokRawUsage
		if err := json.Unmarshal(up.Params.Update.Usage, &u); err != nil {
			s.Refused = append(s.Refused, GrokRefusal{Line: line, Reason: "the usage object is not a shape this reader understands"})
			continue
		}
		if why := u.check(); why != "" {
			s.Refused = append(s.Refused, GrokRefusal{Line: line, Reason: why})
			continue
		}
		s.Turns = append(s.Turns, GrokTurn{
			Usage: u.GrokUsage,
			Model: u.model(),
			At:    time.Unix(up.Timestamp, 0).UTC(),
			Raw:   append(json.RawMessage(nil), up.Params.Update.Usage...),
		})
	}
	return s, sc.Err()
}

// trimSpaceBytes is the blank-line test, kept local so the scan does not
// allocate a string per line to ask one question of it.
func trimSpaceBytes(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r' || b[i] == '\n') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\r' || b[j-1] == '\n') {
		j--
	}
	return b[i:j]
}
