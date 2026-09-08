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
			Name: "replay_rules_free",
			Description: "The complete free Replay rules table: prices and cache floors for every " +
				"model Replay prices, in the format `replay rules --update` installs. Generated from " +
				"the compiled table in this binary, so it needs no network and states its own date.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name: "replay_mcp_overhead",
			Description: "Price the tool definitions a client is carrying. Every connected MCP " +
				"server puts its tool definitions in the prompt on every request, so their cost is " +
				"recurring rather than one-off. Takes a byte size and a request count and returns " +
				"what carrying them costs, cached and cold.",
			InputSchema: obj(map[string]any{
				"model":    map[string]any{"type": "string", "description": "a model id, for example claude-opus-5"},
				"bytes":    map[string]any{"type": "number", "description": "total size of the tool definitions in bytes"},
				"requests": map[string]any{"type": "number", "description": "how many requests they are carried across"},
			}, "model", "bytes"),
		},
		{
			Name: "replay_rules_latest",
			Description: "The maintained rules feed, which lives at redrobot.jp and is sold over " +
				"x402. This server does not hold it and will not invent it: it says where it is and " +
				"what it costs. Replay never pays.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name: "replay_installer_release",
			Description: "The current installer release, its commit and digest. That is a fact " +
				"about a remote release rather than about this machine, so this server names the " +
				"source instead of answering from a compiled-in value that would go stale.",
			InputSchema: obj(map[string]any{}),
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

// mcpInstallSnippet is the whole setup, and it is one line of config.
//
// There is no daemon, no port and nothing to host: the agent spawns this binary
// as a child process, talks JSON-RPC over its pipes, and kills it when the
// session ends. Printing the snippet rather than describing it exists because
// the question "how do I wire this in" was asked three times before anyone
// noticed the guide answered it for the status line and for nothing else.
func mcpInstallSnippet(bin string) string {
	return "  Add this to your agent's MCP configuration:\n\n" +
		"    {\n" +
		"      \"mcpServers\": {\n" +
		"        \"replay\": { \"command\": \"" + bin + "\", \"args\": [\"mcp\"] }\n" +
		"      }\n" +
		"    }\n\n" +
		"  Claude Code:  ~/.claude/settings.json, or `claude mcp add replay -- " + bin + " mcp`\n" +
		"  Codex:        ~/.codex/config.toml\n\n" +
		"  No account, no port, nothing left running. The agent starts this binary\n" +
		"  when it needs an answer and stops it when the session ends.\n"
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
	case "replay_rules_free":
		b, err := json.MarshalIndent(cachemodel.ExportRules(), "", "  ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "replay_mcp_overhead":
		var a struct {
			Model    string  `json:"model"`
			Bytes    float64 `json:"bytes"`
			Requests float64 `json:"requests"`
		}
		_ = json.Unmarshal(args, &a)
		return mcpOverheadText(a.Model, a.Bytes, a.Requests)
	case "replay_rules_latest":
		return "This server does not hold the maintained feed and will not invent it. It is served " +
			"over the network at https://redrobot.jp/mcp.json, priced over x402, and Replay never " +
			"pays: `replay rules --update <url>` reports the terms and installs nothing when payment " +
			"is demanded. The free table is complete and is available here as replay_rules_free.", nil
	case "replay_installer_release":
		return "The current installer release is a fact about a remote release, not about this " +
			"machine, so this server does not answer it from a compiled-in value that would go stale. " +
			"It is served over the network at https://redrobot.jp/mcp.json, and the installed binary " +
			"reports its own build with `replay version`.", nil
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

// mcpOverheadText prices what a client's tool definitions cost to carry.
//
// The cost of a connected MCP server is not the call, it is the definitions:
// they sit in the prompt on every request whether or not the agent uses them.
// Measured on this project's own corpus, tool-definition changes are the single
// largest re-billing cause, so the recurring half of that arithmetic is the one
// nobody does.
//
// The token figure is derived from bytes and is therefore an ESTIMATE. It says
// so, because a count that came from a divisor and a count that came from a
// tokenizer are different kinds of number and only one of them is checkable.
func mcpOverheadText(model string, bytes, requests float64) (string, error) {
	if strings.TrimSpace(model) == "" || bytes <= 0 {
		return "", fmt.Errorf("replay_mcp_overhead needs a model and a byte size")
	}
	if requests <= 0 {
		requests = 1
	}
	p, ok := cachemodel.PriceFor(model)
	if !ok {
		return fmt.Sprintf("%s is not in the compiled table (%s), so no rate is printed for it.",
			model, cachemodel.PriceTableVersion), nil
	}
	read := p.ReadMult
	if read == 0 {
		read = cachemodel.ReadMultiplier
	}
	tokens := bytes * defaultTokensPerByteMCP
	cold := tokens / 1e6 * p.InputPerMTok
	cached := tokens / 1e6 * p.InputPerMTok * read
	var b strings.Builder
	fmt.Fprintf(&b, "%s bytes of tool definitions on %s, carried across %s requests\n",
		commaFloat(bytes), model, commaFloat(requests))
	fmt.Fprintf(&b, "  about %s tokens, ESTIMATED from bytes at %.2f tokens per byte, not counted by a tokenizer\n",
		commaFloat(tokens), defaultTokensPerByteMCP)
	fmt.Fprintf(&b, "  cold  $%.4f per request, $%.2f across %.0f\n", cold, cold*requests, requests)
	fmt.Fprintf(&b, "  cached $%.4f per request, $%.2f across %.0f\n", cached, cached*requests, requests)
	b.WriteString("  Definitions sit in the prompt on every request whether the agent calls the tool or not,\n")
	b.WriteString("  so this cost is recurring. A tool never called is the whole figure wasted.\n")
	return b.String(), nil
}

// defaultTokensPerByteMCP is the same coarse prose ratio analysis.Fit falls
// back to, and carries the same warning: it is an English-prose average and
// tool schemas are not English prose.
const defaultTokensPerByteMCP = 0.25

func commaFloat(f float64) string {
	s := fmt.Sprintf("%.0f", f)
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(c)
	}
	return out
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
