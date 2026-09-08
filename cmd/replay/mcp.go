package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/version"
)

// A local MCP server, spoken over stdio.
//
// The hosted server at redrobot.jp/mcp.json answers questions about the WORLD:
// what a model costs, what its cache floor is, what a tool set weighs. It needs
// no user data, which is why it can be hosted at all.
//
// This one answers questions about THIS MACHINE, and that data can never leave
// it, so it can never be the hosted one. Same protocol, same vocabulary, second
// transport. It exists because every other surface in this program reports
// after the fact: an agent that can ask "what have I spent, and what can Replay
// even see here" DURING the work is a different tool from one that says so
// afterwards.
//
// JSON-RPC 2.0 over line-delimited stdio, standard library only.
const (
	mcpProtocolVersion = "2025-06-18"
	mcpServerName      = "replay"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// mcpTools is the whole surface, and it is deliberately small.
//
// Every description says what the tool returns and, where it matters, what it
// refuses to return. None of them return message text: the entire surface runs
// over the user's own transcripts, and a tool that hands an agent back the
// content of an earlier conversation is an exfiltration path with a friendly
// name. Counts, costs and causes only.
func mcpTools() []mcpTool {
	obj := func(props map[string]any, required ...string) any {
		m := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			m["required"] = required
		}
		return m
	}
	return []mcpTool{
		{
			Name: "replay_surfaces",
			Description: "Which AI agents leave readable state on this machine, how much of it " +
				"Replay can read, and what each surface reports. Returns counts and paths, never " +
				"the contents of a session.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name: "replay_price_check",
			Description: "The compiled price and cache table for one model: input and output rate, " +
				"cache read multiple, and the minimum cacheable prefix. States the table's date and " +
				"whether it is stale, because a rate without its date is not checkable.",
			InputSchema: obj(map[string]any{
				"model": map[string]any{"type": "string", "description": "a model id, for example claude-opus-5"},
			}, "model"),
		},
		{
			Name: "replay_quota",
			Description: "The rate-limit window closest to binding, and how long until it resets, " +
				"from the last reading the status line stored. Reports the reading's age, and says " +
				"so plainly when there is no reading rather than reporting a full window.",
			InputSchema: obj(map[string]any{}),
		},
	}
}

// runMCP serves one client over a pair of streams.
func runMCP(in io.Reader, stdout, stderr io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	enc := json.NewEncoder(stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// A parse failure has no id to answer against, so it is reported
			// with a null id per the spec rather than dropped.
			_ = enc.Encode(mcpResponse{JSONRPC: "2.0", ID: json.RawMessage("null"),
				Error: &mcpError{Code: -32700, Message: "parse error"}})
			continue
		}
		// No id means a notification, and JSON-RPC requires silence. A server
		// that answers one corrupts every client that batches.
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		resp := mcpResponse{JSONRPC: "2.0", ID: req.ID}
		switch req.Method {
		case "initialize":
			resp.Result = map[string]any{
				"protocolVersion": mcpProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": mcpServerName, "version": version.Version},
			}
		case "tools/list":
			resp.Result = map[string]any{"tools": mcpTools()}
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			text, err := mcpCall(p.Name, p.Arguments)
			if err != nil {
				resp.Error = &mcpError{Code: -32602, Message: err.Error()}
				break
			}
			resp.Result = map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
			}
		default:
			resp.Error = &mcpError{Code: -32601, Message: "no method " + req.Method}
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

// mcpCall dispatches one tool.
func mcpCall(name string, args json.RawMessage) (string, error) {
	switch name {
	case "replay_surfaces":
		home, _ := os.UserHomeDir()
		var b strings.Builder
		roots := defaultTranscriptRoots(home)
		fmt.Fprintf(&b, "claude-code transcript roots read by default: %d\n", len(roots))
		if d := desktopSandboxRoots(home); len(d) > 0 {
			fmt.Fprintf(&b, "claude desktop sandbox roots found but NOT in the default corpus: %d\n", len(d))
			b.WriteString("  they are a second population; folding them into one total would change every figure without saying so\n")
		}
		return b.String(), nil
	case "replay_price_check":
		var a struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Model) == "" {
			return "", fmt.Errorf("replay_price_check needs a model")
		}
		return priceCheckText(a.Model), nil
	case "replay_quota":
		q, _ := loadQuota(defaultQuotaPath())
		if q.RateLimits == nil {
			return "No quota reading has been stored. The status line records one while it runs; " +
				"this is not a full window, it is no reading.", nil
		}
		line := quotaLine(statusInput{RateLimits: q.RateLimits}, timeNow())
		if line == "" {
			return "A reading exists but every window in it has already reset, so it describes a " +
				"period that is over.", nil
		}
		return fmt.Sprintf("%s (reading taken %s ago)", line, shortUntil(timeNow().Add(q.Age(timeNow())), timeNow())), nil
	}
	return "", fmt.Errorf("no tool %q", name)
}

// priceCheckText renders one model's row from the compiled table.
//
// A rate without its date is not checkable, so the date and the staleness note
// travel with every figure. An unknown model returns a refusal rather than a
// default: this table declines to price what it does not know, and inventing a
// rate here would be the figure the whole program refuses to state.
func priceCheckText(model string) string {
	p, ok := cachemodel.PriceFor(model)
	if !ok {
		return fmt.Sprintf("%s is not in the compiled table (%s), so Replay prints no rate for "+
			"it rather than guessing one.", model, cachemodel.PriceTableVersion)
	}
	read := p.ReadMult
	if read == 0 {
		read = cachemodel.ReadMultiplier
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s, per million tokens, list price dated %s\n", model, cachemodel.PriceTableVersion)
	fmt.Fprintf(&b, "  input  $%.2f    output $%.2f    cache read $%.3f (%.3fx input)\n",
		p.InputPerMTok, p.OutputPerMTok, p.InputPerMTok*read, read)
	b.WriteString("  cache write multiples are 1.25x at a 5 minute TTL and 2.00x at 1 hour\n")
	if n := cachemodel.PriceTableAgeNote(timeNow()); n != "" {
		fmt.Fprintf(&b, " %s\n", strings.TrimSpace(n))
	}
	return b.String()
}
