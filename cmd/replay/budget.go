package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The standing cost of a configuration, as a file a repository can commit.
//
// MONEY-PATH section 5 makes this step 4 of 8: "emit a signed-by-nothing
// artefact from a developer's own ledger recording the standing per-request
// cost of the current configuration. Free. This is the file the gate compares
// against."
//
// Standing cost is what every request pays before anybody types anything: the
// system prompt, plus every tool definition the session offers whether or not
// it calls them. It is the figure that moves when somebody adds an MCP server,
// and it moves for everyone who commits to the repository.
//
// # Why it records a measurement and never a configuration
//
// The gate in step 5 has to answer "did the standing cost grow" inside CI. It
// could do that by parsing .mcp.json and counting tool definitions, and that
// would be the obvious design and the wrong one: WHAT-YOU-GET.md records that
// this tool reads no configuration at all except the one key `advise --apply`
// writes behind a flag you type, and .mcp.json can hold credentials. A CI job
// that opens it would cross that boundary in the environment where secrets are
// most exposed.
//
// Comparing two artefacts crosses nothing. So the artefact has to be complete
// enough to compare, which is why it carries the per-server breakdown rather
// than one number: a gate that can only say "it grew" sends the reader back to
// the config it was designed not to read.
//
// # Why it needs a ledger
//
// A transcript records what was CALLED. Only a ledger records what was OFFERED,
// because only the proxy sees the request body. Deriving standing cost from
// called tools would report a budget that FALLS when somebody uses fewer tools,
// which is backwards — the definitions were sent either way. So this refuses on
// a transcript-only corpus rather than answering from the wrong quantity.

// BudgetSchema versions the artefact. A committed file outlives the binary that
// wrote it, and a gate that cannot tell which shape it is reading is a gate
// that will one day compare two different things and call it a pass.
const BudgetSchema = 1

type budgetFile struct {
	Schema    int       `json:"schema"`
	Generated time.Time `json:"generated"`
	Standing  standing  `json:"standing"`
	// Servers is tokens per request per MCP server, so a gate can name which
	// one grew. Built-in tools are grouped under the same "built-in" label
	// advise uses, because there is no server to name and inventing one would
	// point the reader at something that does not exist.
	Servers  map[string]int `json:"servers"`
	Measured measured       `json:"measured"`
}

type standing struct {
	TokensPerRequest int `json:"tokens_per_request"`
	SystemTokens     int `json:"system_tokens"`
	ToolTokens       int `json:"tool_tokens"`
	ToolCount        int `json:"tool_count"`
}

// measured is the provenance, and it is not optional.
//
// A budget with no corpus behind it is a number somebody typed. Every figure in
// this repository says how it was obtained, and a file that will be committed
// and read months later by a gate needs that more than most.
type measured struct {
	Sessions int    `json:"sessions"`
	Requests int    `json:"requests"`
	Source   string `json:"source"`
	Model    string `json:"model,omitempty"`
}

func runBudget(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("budget", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the artefact as JSON, for committing and for the gate to read")
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, "Usage: replay budget <ledger-dir...> [--json]\n\n"+
			"What this configuration costs on every request before any work starts:\n"+
			"the system prompt plus every tool definition the session offers, called\n"+
			"or not. Commit the JSON and a later run can tell you what grew.\n\n"+
			"Needs a ledger from `replay serve`. Transcripts record what was called,\n"+
			"never what was offered, so they cannot answer this.\n\n"+
			"Reads no agent configuration. The figure comes from the ledger alone.\n\n")
		fs.PrintDefaults()
	}
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("one or more ledger directories are required: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		// "no transcripts found" is the right message for a command that
		// reads transcripts. This one reads a ledger, and a reader who is
		// told the wrong noun goes looking in the wrong place.
		return fmt.Errorf("NOT MEASURED: no ledger found under %s (%v). `replay serve` writes "+
			"one to ~/.replay/ledger; a budget with no corpus behind it is a number somebody "+
			"typed: %w", strings.Join(fs.Args(), ", "), err, errUsage)
	}

	var (
		sysBytes, toolBytes  int
		perServer            = map[string]int{}
		toolCount            int
		sessions, requests   int
		sawLedger, sawAnyReq bool
		model                string
		fit                  analysis.TokenFit
	)
	var newest time.Time
	_ = forEachSession(files, func(_ string, session *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || session == nil {
			return nil
		}
		sessions++
		lane := analysis.MainLane(session)
		if lane == nil || len(lane.Requests) == 0 {
			return nil
		}
		sawAnyReq = true
		requests += len(lane.Requests)
		first := lane.Requests[0]
		if len(first.Tools) == 0 {
			// A transcript, or a ledger session that offered nothing. Either
			// way it cannot say what the configuration costs.
			return nil
		}
		// The newest session wins rather than an average across the corpus:
		// the question is what the configuration costs NOW, and a mean over a
		// fortnight of edits describes a setup nobody has.
		if !first.Timestamp.After(newest) && sawLedger {
			return nil
		}
		if rep == nil {
			rep = analysis.AnalyzeLane(session, lane)
		}
		if rep == nil || rep.Fit.TokensPerByte <= 0 {
			return nil
		}
		sawLedger, newest, fit = true, first.Timestamp, rep.Fit
		if model == "" {
			model = first.Model
		}
		sysBytes, toolBytes, toolCount = 0, 0, 0
		for _, m := range first.Context {
			if m.Role != transcript.RoleSystem {
				continue
			}
			for _, b := range m.Blocks {
				// The tool-definition block is counted from first.Tools
				// instead, which is the same bytes with the names attached.
				// Counting both would double the larger half of the figure.
				if b.Label == "tool definitions" {
					continue
				}
				sysBytes += b.Bytes
			}
		}
		clear(perServer)
		for _, t := range first.Tools {
			toolBytes += t.Bytes
			toolCount++
			perServer[serverLabel(t.Name)] += t.Bytes
		}
		return nil
	})

	if !sawAnyReq && sessions == 0 {
		return fmt.Errorf("NOT MEASURED: no ledger found under %s. `replay serve` writes one to "+
			"~/.replay/ledger; a budget with no corpus behind it is a number somebody typed: %w",
			strings.Join(fs.Args(), ", "), errUsage)
	}
	if !sawLedger {
		return fmt.Errorf("NOT MEASURED: %d session(s) read, none from a ledger carrying tool "+
			"definitions. A transcript records what was called, never what was offered, so it "+
			"cannot say what the configuration costs when nothing is called. Run the work "+
			"through `replay serve` and try again: %w", sessions, errUsage)
	}
	if fit.TokensPerByte <= 0 {
		return fmt.Errorf("NOT MEASURED: the corpus has tool definitions but no usable "+
			"byte-to-token fit, so their size cannot be converted to tokens: %w", errUsage)
	}

	out := budgetFile{
		Schema:    BudgetSchema,
		Generated: time.Now().UTC(),
		Standing: standing{
			SystemTokens: fit.EstimateTokens(sysBytes),
			ToolTokens:   fit.EstimateTokens(toolBytes),
			ToolCount:    toolCount,
		},
		Servers:  map[string]int{},
		Measured: measured{Sessions: sessions, Requests: requests, Source: "ledger", Model: model},
	}
	// The headline is the sum of its parts by construction, never computed
	// separately. Two paths to one number is how they come to disagree.
	out.Standing.TokensPerRequest = out.Standing.SystemTokens + out.Standing.ToolTokens
	for srv, b := range perServer {
		out.Servers[srv] = fit.EstimateTokens(b)
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	// Exact, not formatCount's rounded "12k".
	//
	// formatCount is right for a blame table, where the reader wants a sense of
	// scale. It is wrong here for a specific reason: this figure is committed
	// and compared, and rounding breaks the addition on screen. 6,400 + 6,400
	// renders as "6k + 6k = 13k", three numbers that do not add up in a tool
	// whose entire pitch is that its figures do.
	p := analysis.NewPrinter(stdout)
	p.Printf("Standing cost of this setup, on every request:\n")
	p.Printf("  system prompt + instructions   %9s tokens\n", comma(out.Standing.SystemTokens))
	p.Printf("  tool definitions (%d tools)      %9s tokens\n", toolCount, comma(out.Standing.ToolTokens))
	p.Printf("  %-30s %9s tokens\n", "total, before any work", comma(out.Standing.TokensPerRequest))
	if len(out.Servers) > 0 {
		p.Printf("\nBy server:\n")
		names := make([]string, 0, len(out.Servers))
		for n := range out.Servers {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool { return out.Servers[names[i]] > out.Servers[names[j]] })
		for _, n := range names {
			p.Printf("  %-24s %9s tokens/request\n", n, comma(out.Servers[n]))
		}
	}
	p.Printf("\nMeasured from %d request(s) across %d session(s) of ledger.\n", requests, sessions)
	p.Printf("Commit `replay budget <dir> --json` and a later run can tell you what grew.\n")
	return p.Err()
}

// serverLabel is budget's copy of the advisor's attribution rule.
//
// Deliberately duplicated rather than exported: the advisor's version is about
// what to advise, this one is about what to commit, and coupling a file format
// to an advice heuristic would mean a change to advice wording could silently
// change the shape of every committed artefact.
func serverLabel(tool string) string {
	const p = "mcp__"
	if !strings.HasPrefix(tool, p) {
		return "built-in"
	}
	rest := tool[len(p):]
	i := strings.Index(rest, "__")
	if i <= 0 {
		return "built-in"
	}
	return rest[:i]
}
