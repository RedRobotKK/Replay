//go:build mutation

package blackbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/observation"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// check is one invocation of the binary and what it must produce.
type check struct {
	Label  string
	Args   []string
	Stdin  string
	Env    []string
	Exit   int
	Stdout []string // substrings that must appear
	Stderr []string // substrings that must appear
	Forbid []string // substrings that must not appear in stdout+stderr
	JSON   func(map[string]any) error
	After  func() error // a post-condition on the filesystem
}

// spec is one surface's black-box contract: how to lay out its world, the
// checks a correct binary passes, the input it must refuse, and the machine-
// readable invocations whose output must parse.
type spec struct {
	Setup   func(t *testing.T, bin string) (home string, checks []check)
	Invalid func(home string) check
	JSONOut func(home string) [][]string
}

// evaluate runs one check and returns the ways it failed. It is deliberately
// not a *testing.T assertion, because the negative matrix needs the same
// judgement against a mutated binary without failing the test.
func evaluate(bin, home string, c check) []string {
	r := run(bin, home, c.Stdin, append([]string{}, c.Args...)...)
	if len(c.Env) > 0 {
		r = runEnv(bin, home, c.Stdin, c.Env, c.Args...)
	}
	var f []string
	if r.Exit != c.Exit {
		f = append(f, fmt.Sprintf("exit %d, want %d (stderr: %.200s)", r.Exit, c.Exit, r.Stderr))
	}
	for _, s := range c.Stdout {
		if !strings.Contains(r.Stdout, s) {
			f = append(f, fmt.Sprintf("stdout lacks %q", s))
		}
	}
	for _, s := range c.Stderr {
		if !strings.Contains(r.Stderr, s) {
			f = append(f, fmt.Sprintf("stderr lacks %q", s))
		}
	}
	for _, s := range c.Forbid {
		if strings.Contains(r.Stdout+r.Stderr, s) {
			f = append(f, fmt.Sprintf("output carries forbidden %q", s))
		}
	}
	if c.JSON != nil {
		var doc map[string]any
		if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil {
			f = append(f, "stdout is not a JSON object: "+err.Error())
		} else if err := c.JSON(doc); err != nil {
			f = append(f, "JSON: "+err.Error())
		}
	}
	if c.After != nil {
		if err := c.After(); err != nil {
			f = append(f, "after: "+err.Error())
		}
	}
	return f
}

// ---------------------------------------------------------------- fixtures

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p string, body []byte) {
	t.Helper()
	mustMkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// homeWithTranscript lays the one real Claude Code transcript under a fresh HOME.
func homeWithTranscript(t *testing.T) (home, transcriptPath string) {
	t.Helper()
	home = t.TempDir()
	transcriptPath = filepath.Join(home, ".claude", "projects", "proj", "session.jsonl")
	mustWrite(t, transcriptPath, fixture(t, "internal/transcript/testdata/session-redacted.jsonl"))
	return home, transcriptPath
}

func projects(home string) string { return filepath.Join(home, ".claude", "projects") }

// ledgerAt writes two schema-2 ledger sessions: a priced one with a cache break
// and tool definitions, and an unpriced one on a model no price table carries.
func ledgerAt(t *testing.T, dir string, at time.Time) {
	t.Helper()
	mustMkdir(t, dir)
	write := func(name, sid, model string, create []int) {
		var lines []byte
		for i, c := range create {
			u := transcript.Usage{Input: 500, CacheCreation: c, Output: 300}
			rec := ledger.Record{
				Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(i) * time.Second),
				SessionID: sid, RequestID: fmt.Sprintf("%s-%02d", sid, i),
				Path: "/v1/messages", Status: 200, LatencyMS: 900,
				RequestSummary: ledger.RequestSummary{Model: model,
					Prompt: ledger.Prompt{SystemBytes: 400, ToolBytes: 2000, ToolCount: 2,
						Tools:    []transcript.ToolDef{{Name: "read", Bytes: 1200}, {Name: "write", Bytes: 800}},
						Messages: []ledger.Message{{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 81_000}}}}}},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 900}}},
			}
			b, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		mustWrite(t, filepath.Join(dir, name), lines)
	}
	write("priced.jsonl", "priced", "claude-opus-5", []int{5000, 5000})
	write("unpriced.jsonl", "unp", "totally-unknown-model-x9", []int{900_000, 900_000})
}

func ledgerHome(t *testing.T) (home, dir string) {
	t.Helper()
	home = t.TempDir()
	dir = filepath.Join(home, "ledger")
	ledgerAt(t, dir, time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC))
	return home, dir
}

func grokTurnLine(in int) string {
	return fmt.Sprintf(`{"timestamp":"2026-09-27T18:41:22Z","method":"session/update","params":{"update":{"usage":{`+
		`"inputTokens":%d,"outputTokens":0,"totalTokens":%d,"cachedReadTokens":0,`+
		`"cacheCreationTokens":0,"reasoningTokens":0,"modelCalls":1,"costUsdTicks":0,`+
		`"modelUsage":{"grok-4.7-build":{"inputTokens":%d}}}}}}`, in, in, in)
}

func corpusSubmission(tag string, tasks int, total, rebilled float64) []byte {
	c := observation.Corpus{
		Schema: observation.CorpusSchema, TakenAt: "2026-09-10T04:00:00Z", Tasks: tasks,
		TotalUSD: total, RebilledUSD: rebilled, RebilledShare: rebilled / total, MedianTaskUSD: total / float64(tasks),
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01", SourceTag: tag, TagBasis: "local",
	}.Digested()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}

func num(doc map[string]any, path ...string) (float64, bool) {
	var cur any = doc
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return 0, false
		}
		cur, ok = m[p]
		if !ok {
			return 0, false
		}
	}
	f, ok := cur.(float64)
	return f, ok
}

func unknownFlag(name string, fixedArgs ...string) func(string) check {
	return func(string) check {
		return check{Label: "unknown flag refused", Args: append(append([]string{name}, fixedArgs...), "--no-such-flag"), Exit: 1}
	}
}

// ---------------------------------------------------------------- the specs

var specs = map[string]spec{
	"version": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			return t.TempDir(), []check{{Label: "prints the tool name and version", Args: []string{"version"}, Exit: 0, Stdout: []string{"replay "}}}
		},
		Invalid: func(string) check {
			return check{Label: "unknown command refused", Args: []string{"nosuchcommand"}, Exit: 1, Stderr: []string{"unknown command"}}
		},
	},
	"help": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			return t.TempDir(), []check{{Label: "usage names the commands", Args: []string{"help"}, Exit: 0, Stdout: []string{"replay cost   <dir...>", "replay diff   <transcript|dir>", "replay serve  [flags]"}}}
		},
		Invalid: func(string) check {
			return check{Label: "unknown command refused", Args: []string{"nosuchcommand"}, Exit: 1, Stderr: []string{"unknown command"}}
		},
	},
	"replay": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, p := homeWithTranscript(t)
			return home, []check{{Label: "replays and scores policies", Args: []string{"replay", p}, Exit: 0, Stdout: []string{"Session ", "Calibration", "as-run", "ttl-5m0s", "vs as-run"}}}
		},
		Invalid: unknownFlag("replay"),
	},
	"blame": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, p := homeWithTranscript(t)
			return home, []check{{Label: "ranks token sources", Args: []string{"blame", p}, Exit: 0, Stdout: []string{"Session ", "top token sources", "1. tool call: Bash", "2. system prompt and tool definitions"}}}
		},
		Invalid: unknownFlag("blame"),
	},
	"diff": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, dir := ledgerHome(t)
			return home, []check{{Label: "names the turn and the cause", Args: []string{"diff", filepath.Join(dir, "priced.jsonl")}, Exit: 0, Stdout: []string{"turn 1 at ", "re-billed", "cause:"}}}
		},
		Invalid: unknownFlag("diff"),
	},
	"corpus": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			return home, []check{{Label: "calibration corpus", Args: []string{"corpus", projects(home)}, Exit: 0, Stdout: []string{"# Calibration Corpus", "| redacted | 2.1.258 | estimated | 80 |", "Overall match rate: 98.73% (exact reproduction rate: 89.87%)"}}}
		},
		Invalid: unknownFlag("corpus"),
	},
	"pool": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home := t.TempDir()
			a, b := filepath.Join(home, "a.json"), filepath.Join(home, "b.json")
			mustWrite(t, a, corpusSubmission("aaaaaaaaaaaaaaaa", 10, 100, 5))
			mustWrite(t, b, corpusSubmission("bbbbbbbbbbbbbbbb", 20, 200, 10))
			return home, []check{{Label: "pools two submissions", Args: []string{"pool", a, b}, Exit: 0, Stdout: []string{"$300.00 across 2 contributed corpora, 30 tasks.", "$15.00 of it re-billed, 5.0% of the pooled total."}}}
		},
		Invalid: func(string) check { return check{Label: "no submissions refused", Args: []string{"pool"}, Exit: 1} },
		JSONOut: func(home string) [][]string {
			return [][]string{{"pool", "--json", filepath.Join(home, "a.json"), filepath.Join(home, "b.json")}}
		},
	},
	"codex": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home := t.TempDir()
			dir := filepath.Join(home, "rollouts")
			mustWrite(t, filepath.Join(dir, "rollout-2026-09-01.jsonl"), fixture(t, "internal/transcript/codexdata/compacted.jsonl"))
			return home, []check{{Label: "sums per-turn deltas", Args: []string{"codex", dir}, Exit: 0, Stdout: []string{"3,850 tokens billed across 1 Codex session(s)", "3,300 of that (86%) is invisible", "ran   replay codex"}}}
		},
		Invalid: unknownFlag("codex"),
	},
	"grok": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home := t.TempDir()
			var lines []string
			for _, in := range []int{3716413, 348136, 175281, 358340, 180838, 2363388} {
				lines = append(lines, grokTurnLine(in))
			}
			mustWrite(t, filepath.Join(home, ".grok", "sessions", "proj", "01a0e417", "updates.jsonl"), []byte(strings.Join(lines, "\n")+"\n"))
			return home, []check{{Label: "reconstructs and reconciles", Args: []string{"grok"}, Exit: 0, Stdout: []string{"1 Grok session(s)", "7,142,396", "no session carries a usage.json", "UNAVAILABLE    1 session(s)", "never added"}}}
		},
		Invalid: unknownFlag("grok"),
	},
	"jev": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			return t.TempDir(), []check{{Label: "reads a capture", Args: []string{"jev", filepath.Join(repoRoot(t), "internal", "transcript", "jevdata", "valid.jsonl")}, Exit: 0, Stdout: []string{"attempt 0  HTTP 200", "tokens 366 in, 78 out"}}}
		},
		Invalid: unknownFlag("jev"),
	},
	"mcp": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			rpc := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
			return t.TempDir(), []check{
				{Label: "install snippet", Args: []string{"mcp", "--install"}, Exit: 0, Stdout: []string{"mcpServers", "replay"}},
				{Label: "JSON-RPC over stdio", Args: []string{"mcp"}, Stdin: rpc, Exit: 0, Stdout: []string{`"jsonrpc":"2.0"`, "replay_surfaces"}},
			}
		},
		Invalid: unknownFlag("mcp"),
	},
	"agents": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home := t.TempDir()
			proj := filepath.Join(home, "project")
			mustWrite(t, filepath.Join(proj, "docs", "evidence", "ledger-2026-10-02.md"), []byte("# ledger\n"))
			return home, []check{{Label: "boot block names record directories", Args: []string{"agents", proj}, Exit: 0, Stdout: []string{"Where this project's records are", "docs/evidence"}}}
		},
		Invalid: unknownFlag("agents"),
	},
	"burn": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			return home, []check{{Label: "per-surface consumption", Args: []string{"burn"}, Exit: 0, Stdout: []string{"surface", "not addable", "claude-code", "23,352,040"}}}
		},
		Invalid: func(string) check {
			return check{Label: "a path argument is refused", Args: []string{"burn", "/tmp"}, Exit: 1, Stderr: []string{"takes no path argument"}}
		},
	},
	"doctor": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			return home, []check{{Label: "classifies the cache signal", Args: []string{"doctor"}, Exit: 0, Stdout: []string{"replay doctor", "transcripts   1 sessions", "cache signal  "}, Forbid: []string{"cache signal  undetermined"}}}
		},
		Invalid: unknownFlag("doctor"),
	},
	"probe": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			return t.TempDir(), []check{{Label: "plans without sending", Args: []string{"probe", "--model", "claude-opus-5"}, Exit: 0, Stdout: []string{"Nothing was sent. Add --execute to run it."}}}
		},
		Invalid: unknownFlag("probe"),
	},
	"rules": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			return t.TempDir(), []check{{Label: "names the rules in effect", Args: []string{"rules"}, Exit: 0, Stdout: []string{"rules      anthropic-", "file       "}}}
		},
		Invalid: func(home string) check {
			return check{Label: "a missing rules document is refused", Args: []string{"rules", "--update", filepath.Join(home, "nonexistent-rules.json")}, Exit: 1}
		},
	},
	"tui": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			return home, []check{{Label: "one frame of the cost screen", Args: []string{"tui", "-once", "-screen", "cost"}, Exit: 0, Stdout: []string{"What did this cost me?", "$"}}}
		},
		Invalid: func(string) check {
			return check{Label: "unknown screen refused", Args: []string{"tui", "-once", "-screen", "nosuchscreen"}, Exit: 1}
		},
	},
	"statusline": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			return t.TempDir(), []check{
				{Label: "install snippet", Args: []string{"statusline", "--install"}, Exit: 0, Stdout: []string{"statusLine", "replay statusline"}},
				{Label: "renders the JSON Claude Code sends", Args: []string{"statusline", "--no-color"}, Stdin: `{"model":{"id":"claude-opus-5"},"cost":{"total_cost_usd":0.5}}`, Exit: 0, Stdout: []string{"$"}},
			}
		},
		Invalid: unknownFlag("statusline"),
	},
	"cost": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, dir := ledgerHome(t)
			return home, []check{
				{Label: "JSON report", Args: []string{"cost", "--json", dir}, Exit: 0, JSON: func(d map[string]any) error {
					for _, want := range []struct {
						path []string
						v    float64
					}{{[]string{"unpriced"}, 1}, {[]string{"summary", "tasks"}, 1}, {[]string{"summary", "pricedRequests"}, 2}, {[]string{"summary", "unpricedRequests"}, 2}} {
						got, ok := num(d, want.path...)
						if !ok || got != want.v {
							return fmt.Errorf("%s = %v, want %v", strings.Join(want.path, "."), got, want.v)
						}
					}
					if total, _ := num(d, "summary", "totalUsd"); total <= 0 {
						return fmt.Errorf("totalUsd %v", total)
					}
					return nil
				}},
				{Label: "human report", Args: []string{"cost", dir}, Exit: 0, Stdout: []string{"at list prices dated ", "The total above covers 50% of the requests read", "excluded rather than counted as free"}},
				{Label: "share card names the priced population", Args: []string{"cost", "--share", dir}, Exit: 0, Stdout: []string{"30% of my priced agent spend was paid twice."}},
			}
		},
		Invalid: unknownFlag("cost"),
		JSONOut: func(home string) [][]string { return [][]string{{"cost", "--json", filepath.Join(home, "ledger")}} },
	},
	"ceiling": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			return home, []check{{Label: "cache-blind ratio", Args: []string{"ceiling", "--metered", projects(home)}, Exit: 0, Stdout: []string{"Cache-blind arithmetic runs 3.66x high over these 80 requests"}}}
		},
		Invalid: func(home string) check {
			return check{Label: "billing mode required", Args: []string{"ceiling", projects(home)}, Exit: 1}
		},
		JSONOut: func(home string) [][]string { return [][]string{{"ceiling", "--metered", "--json", projects(home)}} },
	},
	"trim": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, p := homeWithTranscript(t)
			return home, []check{{Label: "scores a cap", Args: []string{"trim", "--cap", "8192", p}, Exit: 0, Stdout: []string{"Scoring a 8192-byte cap on tool results over 1 session(s)", "4 block(s) over the cap, 30k bytes removable"}}}
		},
		Invalid: unknownFlag("trim"),
		JSONOut: func(home string) [][]string {
			return [][]string{{"trim", "--cap", "8192", "--json", filepath.Join(projects(home), "proj", "session.jsonl")}}
		},
	},
	"route": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, p := homeWithTranscript(t)
			return home, []check{{Label: "TTL write penalties", Args: []string{"route", "--to", "claude-sonnet-5", p}, Exit: 0, Stdout: []string{"Write penalty is 1.25x at 5m and 2.00x at 1h"}}}
		},
		Invalid: unknownFlag("route"),
		JSONOut: func(home string) [][]string {
			return [][]string{{"route", "--to", "claude-sonnet-5", "--json", filepath.Join(projects(home), "proj", "session.jsonl")}}
		},
	},
	"context": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, p := homeWithTranscript(t)
			return home, []check{{Label: "what filled the context", Args: []string{"context", p}, Exit: 0, Stdout: []string{"484k tokens of content entered this context", "41.8%"}}}
		},
		Invalid: unknownFlag("context"),
		JSONOut: func(home string) [][]string {
			return [][]string{{"context", "--json", filepath.Join(projects(home), "proj", "session.jsonl")}}
		},
	},
	"advise": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			return home, []check{{Label: "ranked findings", Args: []string{"advise", projects(home)}, Exit: 0, Stdout: []string{"Sessions: 1 found, 1 calibrated", "cache breaks re-billed 1% of prompt tokens"}}}
		},
		Invalid: unknownFlag("advise"),
	},
	"learn": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			return home, []check{{Label: "policy selection", Args: []string{"learn", projects(home)}, Exit: 0, Stdout: []string{"Sessions: 1 found, 1 calibrated, 1 held out", "Selected: none"}}}
		},
		Invalid: func(string) check { return check{Label: "no directory refused", Args: []string{"learn"}, Exit: 1} },
	},
	"redact": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, p := homeWithTranscript(t)
			src := string(fixture(t, "internal/transcript/testdata/session-redacted.jsonl"))
			lines := strings.SplitN(src, "\n", 3)
			raw := strings.ReplaceAll(lines[0]+"\n"+lines[1]+"\n", `"cwd":"/redacted"`, `"cwd":"/Users/someone/secret-project"`)
			const secret = "the quick brown fox jumps over the lazy dog"
			i := strings.Index(raw, `"content":"`) + len(`"content":"`)
			raw = raw[:i] + secret + raw[i+len(secret):]
			unredacted := filepath.Join(home, "raw.jsonl")
			mustWrite(t, unredacted, []byte(raw))
			_ = p
			return home, []check{{Label: "strips paths and text", Args: []string{"redact", unredacted}, Exit: 0, Stdout: []string{`"cwd":"/redacted"`}, Forbid: []string{"secret-project", secret}}}
		},
		Invalid: func(string) check { return check{Label: "no transcript refused", Args: []string{"redact"}, Exit: 1} },
	},
	"prefix": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home := t.TempDir()
			before, after, same := filepath.Join(home, "before.json"), filepath.Join(home, "after.json"), filepath.Join(home, "same.json")
			mustWrite(t, before, []byte(`{"mcpServers":{"a":{"command":"a"}}}`))
			mustWrite(t, after, []byte(`{"mcpServers":{"a":{"command":"a"},"b":{"command":"b"}}}`))
			mustWrite(t, same, []byte(`{"mcpServers":{"a":{"command":"a"}}}`))
			return home, []check{
				{Label: "a changed tool set exits 1", Args: []string{"prefix", "--before", before, "--after", after}, Exit: 1, Stdout: []string{"prefix"}},
				{Label: "an unchanged tool set exits 0", Args: []string{"prefix", "--before", before, "--after", same}, Exit: 0, Stdout: []string{"tool set unchanged"}},
			}
		},
		Invalid: func(string) check { return check{Label: "both files required", Args: []string{"prefix"}, Exit: 1} },
		JSONOut: func(home string) [][]string {
			return [][]string{{"prefix", "--json", "--before", filepath.Join(home, "before.json"), "--after", filepath.Join(home, "same.json")}}
		},
	},
	"since": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home := t.TempDir()
			proj := filepath.Join(projects(home), "proj")
			mustMkdir(t, proj)
			if r := run(bin, home, "", "since"); r.Exit != 0 {
				t.Fatalf("first since: %d %s", r.Exit, r.Stderr)
			}
			ledgerAt(t, proj, time.Now().UTC().Add(2*time.Second))
			time.Sleep(2500 * time.Millisecond)
			return home, []check{{Label: "the window since the marker", Args: []string{"since", "--peek"}, Exit: 0, Stdout: []string{"largest: session unp", "excluded: their model is not in the price table"}}}
		},
		Invalid: unknownFlag("since"),
	},
	"simulate": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, dir := ledgerHome(t)
			policy := filepath.Join(home, "policy.json")
			mustWrite(t, policy, []byte(`{"maxSessionUsd": 0.05}`))
			return home, []check{
				{Label: "replays the ledger under the cap", Args: []string{"simulate", "--policy", policy, "--json", dir}, Exit: 0, JSON: func(d map[string]any) error {
					a, _ := num(d, "summary", "admitted")
					r, _ := num(d, "summary", "refused")
					if sim, _ := d["simulated"].(bool); !sim || a != 3 || r != 1 {
						return fmt.Errorf("simulated=%v admitted=%v refused=%v, want true 3 1", d["simulated"], a, r)
					}
					return nil
				}},
				{Label: "human report is labelled SIMULATED", Args: []string{"simulate", "--policy", policy, dir}, Exit: 0, Stdout: []string{"SIMULATED: policy ", "refused    1"}, Forbid: []string{"saving", "forecast"}},
			}
		},
		Invalid: func(home string) check {
			p := filepath.Join(home, "bad-policy.json")
			_ = os.WriteFile(p, []byte(`{"maxSesionUsd": 1}`), 0o600)
			return check{Label: "a misspelled cap key is refused", Args: []string{"simulate", "--policy", p, filepath.Join(home, "ledger")}, Exit: 1}
		},
		JSONOut: func(home string) [][]string {
			return [][]string{{"simulate", "--policy", filepath.Join(home, "policy.json"), "--json", filepath.Join(home, "ledger")}}
		},
	},
	"budget": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, dir := ledgerHome(t)
			return home, []check{{Label: "standing prefix from the ledger", Args: []string{"budget", dir, "--json"}, Exit: 0, JSON: func(d map[string]any) error {
				tb, _ := num(d, "standing", "tool_bytes")
				tc, _ := num(d, "standing", "tool_count")
				if tb != 2000 || tc != 2 {
					return fmt.Errorf("standing tool_bytes=%v tool_count=%v, want 2000 and 2", tb, tc)
				}
				return nil
			}}}
		},
		Invalid: func(string) check {
			return check{Label: "no ledger directory refused", Args: []string{"budget"}, Exit: 1}
		},
		JSONOut: func(home string) [][]string { return [][]string{{"budget", filepath.Join(home, "ledger"), "--json"}} },
	},
	"purge": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, dir := ledgerHome(t)
			old := time.Now().Add(-48 * time.Hour)
			for _, f := range []string{"priced.jsonl", "unpriced.jsonl"} {
				if err := os.Chtimes(filepath.Join(dir, f), old, old); err != nil {
					t.Fatal(err)
				}
			}
			return home, []check{{Label: "reports without --yes and removes nothing", Args: []string{"purge", dir, "--older-than", "1h"}, Exit: 0, Stdout: []string{"2 session file(s)", "Nothing was changed. Add --yes to remove them."}, After: func() error {
				_, err := os.Stat(filepath.Join(dir, "priced.jsonl"))
				return err
			}}}
		},
		Invalid: func(home string) check {
			return check{Label: "a window is required", Args: []string{"purge", filepath.Join(home, "ledger")}, Exit: 1}
		},
	},
	"privacy": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			home, _ := homeWithTranscript(t)
			if r := run(bin, home, "", "advise", projects(home)); r.Exit != 0 {
				t.Fatalf("advise: %d %s", r.Exit, r.Stderr)
			}
			return home, []check{{Label: "discloses the store advise wrote", Args: []string{"privacy", "--json"}, Exit: 0, JSON: func(d map[string]any) error {
				stores, _ := d["stores"].([]any)
				for _, s := range stores {
					m, _ := s.(map[string]any)
					if m["name"] == "advice.json" {
						if f, _ := m["files"].(float64); f == 1 {
							return nil
						}
					}
				}
				return fmt.Errorf("advice.json with files=1 not disclosed")
			}}}
		},
		Invalid: unknownFlag("privacy"),
		JSONOut: func(string) [][]string { return [][]string{{"privacy", "--json"}} },
	},
	"serve": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			return t.TempDir(), []check{{Label: "refuses a negative preflight ceiling before listening", Args: []string{"serve", "--preflight", "-5", "--upstream", ""}, Exit: 1, Stderr: []string{"-preflight must be a positive token ceiling"}}}
		},
		Invalid: func(string) check {
			return check{Label: "a non-numeric ceiling refused", Args: []string{"serve", "--preflight", "not-a-number"}, Exit: 1}
		},
	},
	"upgrade": {
		Setup: func(t *testing.T, bin string) (string, []check) {
			// The release host is unreachable on purpose: every outbound
			// connection is sent to a closed local port through HTTPS_PROXY,
			// so the binary's own client fails before any network. The
			// contract exercised is the failure branch: a fetch that fails
			// must not be reported as verified.
			return t.TempDir(), []check{{Label: "a failed fetch is not reported as verified", Args: []string{"upgrade", "--version", "v9.9.9", "--dry-run"}, Env: []string{"HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9"}, Exit: 1, Forbid: []string{"Checksum verified", "Verified v9.9.9"}}}
		},
		Invalid: unknownFlag("upgrade"),
	},
}
