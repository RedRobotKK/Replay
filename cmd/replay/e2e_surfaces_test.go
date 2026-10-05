package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/selfupdate"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// End-to-end tests, one per surface in dispatch().
//
// Each one goes in where main() goes in: dispatch(args, stdout, stderr), on a
// HOME the test owns, with real input on disk, and asserts what the user would
// see and the error main() would turn into an exit status. There is no helper
// between the test and the command that main() itself does not use.
//
// cmd/replay/wiring_gate_test.go reads the dispatch switch and fails when a
// case has no TestE2E_<Name> here, or when that test does not reach dispatch
// with the case's own name. These are the functional tests the gate counts.
//
// The process is not exec'd: a test in this package may not import os/exec
// (TestX402_ExecIsConfinedToTheMutationHarness), because a binary piped from
// curl onto a machine holding provider credentials must not be able to start
// one. Going through dispatch() is the shipped path minus os.Args and os.Exit,
// and TestWiringGate_MainReachesDispatch pins that main() still takes it.

// e2e runs one command the way main() does.
func e2e(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	err = dispatch(args, &out, &errb)
	return out.String(), errb.String(), err
}

// e2eLedger writes schema-2 ledger records: a priced session with a cache
// break, and an unpriced session on a model no price table carries.
func e2eLedger(t *testing.T, dir string) {
	t.Helper()
	e2eLedgerAt(t, dir, time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC))
}

func e2eLedgerAt(t *testing.T, dir string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
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
						Tools: []transcript.ToolDef{{Name: "read", Bytes: 1200}, {Name: "write", Bytes: 800}},
						Messages: []ledger.Message{
							{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 81_000}}}}}},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 900}}},
			}
			b, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		if err := os.WriteFile(filepath.Join(dir, name), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("priced.jsonl", "priced", "claude-opus-5", []int{5000, 5000})
	write("unpriced.jsonl", "unp", "totally-unknown-model-x9", []int{900_000, 900_000})
}

// e2eTranscript is the one real Claude Code transcript, under an isolated HOME.
func e2eTranscript(t *testing.T) (home, transcriptPath string) {
	t.Helper()
	home = homeWithTranscript(t)
	return home, filepath.Join(home, ".claude", "projects", "proj", "session.jsonl")
}

func mustContain(t *testing.T, label, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s: output lacks %q:\n%s", label, w, text)
		}
	}
}

func TestE2E_Version(t *testing.T) {
	out, _, err := e2e(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "version", out, "replay ")
}

func TestE2E_Help(t *testing.T) {
	out, _, err := e2e(t, "help")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "help", out, "replay cost   <dir...>", "replay diff   <transcript|dir>", "replay serve  [flags]")
}

func TestE2E_Replay(t *testing.T) {
	_, p := e2eTranscript(t)
	out, errb, err := e2e(t, "replay", p)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "replay", out, "Session ", "Calibration", "as-run", "ttl-5m0s", "vs as-run")
}

func TestE2E_Blame(t *testing.T) {
	_, p := e2eTranscript(t)
	out, errb, err := e2e(t, "blame", p)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "blame", out, "Session ", "top token sources", "1. tool call: Bash", "2. system prompt and tool definitions")
}

func TestE2E_Diff(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	e2eLedger(t, dir)
	out, errb, err := e2e(t, "diff", filepath.Join(dir, "priced.jsonl"))
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	// RPL-C001: the turn it broke on, and the cause.
	mustContain(t, "diff", out, "turn 1 at ", "re-billed", "cause:")
}

func TestE2E_Corpus(t *testing.T) {
	home, _ := e2eTranscript(t)
	out, errb, err := e2e(t, "corpus", filepath.Join(home, ".claude", "projects"))
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "corpus", out, "# Calibration Corpus", "| redacted | 2.1.258 | estimated | 80 |", "Overall match rate: 98.73% (exact reproduction rate: 89.87%)")
}

func TestE2E_Pool(t *testing.T) {
	dir := t.TempDir()
	a := writeSubmission(t, dir, "a.json", corpusFixture("aaaaaaaaaaaaaaaa", 10, 100, 5))
	b := writeSubmission(t, dir, "b.json", corpusFixture("bbbbbbbbbbbbbbbb", 20, 200, 10))
	out, errb, err := e2e(t, "pool", a, b)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "pool", out, "$300.00 across 2 contributed corpora, 30 tasks.", "$15.00 of it re-billed, 5.0% of the pooled total.")
}

func TestE2E_Codex(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "rollouts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "internal", "transcript", "codexdata", "compacted.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-09-01.jsonl"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	out, errb, err := e2e(t, "codex", dir)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	if strings.Contains(out, "No Codex sessions found") {
		t.Fatalf("codex did not read the rollout:\n%s", out)
	}
	mustContain(t, "codex", out, "3,850 tokens billed across 1 Codex session(s)", "3,300 of that (86%) is invisible", "ran   replay codex")
}

func TestE2E_Grok(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	root := filepath.Join(home, ".grok", "sessions")
	writeGrokSession(t, root, "01a0e417", grokRealTurnLines(), "")
	out, errb, err := e2e(t, "grok")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "grok", out, "1 Grok session(s)", "7,142,396", "no session carries a usage.json", "UNAVAILABLE    1 session(s)", "never added")
	// An unknown flag is refused, like every other command. Until 2026-10-03
	// grok read args[0] as a root only when it did not start with "-" and
	// otherwise ignored the argument, so `replay grok --no-such-flag` (and
	// `--help`) ran the default root and exited 0.
	if _, _, err := e2e(t, "grok", "--no-such-flag"); err == nil {
		t.Error("grok accepted --no-such-flag and ran anyway")
	}
}

func TestE2E_Jev(t *testing.T) {
	out, errb, err := e2e(t, "jev", jevValidFixture)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "jev", out, "attempt 0  HTTP 200", "tokens 366 in, 78 out")
}

func TestE2E_Mcp(t *testing.T) {
	out, _, err := e2e(t, "mcp", "--install")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "mcp --install", out, "mcpServers", "replay")
}

func TestE2E_Agents(t *testing.T) {
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, "docs", "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "docs", "evidence", "ledger-2026-10-02.md"), []byte("# ledger\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errb, err := e2e(t, "agents", proj)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "agents", out, "Where this project's records are", "docs/evidence")
}

func TestE2E_Burn(t *testing.T) {
	home, _ := e2eTranscript(t)
	out, errb, err := e2e(t, "burn")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	_ = home
	mustContain(t, "burn", out, "surface", "not addable")
	if !regexp.MustCompile(`claude-code\s+80\s+23,352,040`).MatchString(out) {
		t.Errorf("burn did not read the Claude Code surface (80 requests, 23,352,040 tokens):\n%s", out)
	}
}

func TestE2E_Doctor(t *testing.T) {
	home, _ := e2eTranscript(t)
	out, errb, err := e2e(t, "doctor")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	_ = home
	mustContain(t, "doctor", out, "replay doctor", "transcripts   1 sessions")
	// RPL-C026, wired: doctor classifies what the transcripts on this machine
	// can show about the cache, through surface.Classify, and names the class.
	mustContain(t, "doctor", out, "cache signal  ")
	if strings.Contains(out, "cache signal  undetermined") {
		t.Errorf("a real transcript with cache counters classified as undetermined:\n%s", out)
	}
}

// RPL-C026, the refusal half: with nothing to classify, doctor says so and
// gives the reason, rather than a class or a zero.
func TestE2E_DoctorRefusesToClassifyAnEmptyBoundary(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	out, errb, err := e2e(t, "doctor")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "doctor", out, "cache signal  undetermined", "no records at this boundary")
}

func TestE2E_Probe(t *testing.T) {
	out, errb, err := e2e(t, "probe", "--model", "claude-opus-5")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "probe plan", out, "Nothing was sent. Add --execute to run it.")
}

func TestE2E_Rules(t *testing.T) {
	out, errb, err := e2e(t, "rules")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "rules", out, "rules      anthropic-", "file       ")
}

func TestE2E_Tui(t *testing.T) {
	home, _ := e2eTranscript(t)
	_ = home
	out, errb, err := e2e(t, "tui", "-once", "-screen", "cost")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "tui --once", out, "What did this cost me?", "$")
}

func TestE2E_Statusline(t *testing.T) {
	out, _, err := e2e(t, "statusline", "--install")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "statusline --install", out, "statusLine", "replay statusline")
}

func TestE2E_Cost(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	e2eLedger(t, dir)
	out, errb, err := e2e(t, "cost", dir)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "cost", out,
		"at list prices dated ",
		"The total above covers 50% of the requests read",
		"excluded rather than counted as free")
	jsonOut, _, err := e2e(t, "cost", "--json", dir)
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Unpriced int `json:"unpriced"`
		Summary  struct {
			Tasks            int     `json:"tasks"`
			TotalUSD         float64 `json:"totalUsd"`
			PricedRequests   int     `json:"pricedRequests"`
			UnpricedRequests int     `json:"unpricedRequests"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &rep); err != nil {
		t.Fatalf("cost --json: %v\n%s", err, jsonOut)
	}
	if rep.Unpriced != 1 || rep.Summary.Tasks != 1 || rep.Summary.PricedRequests != 2 || rep.Summary.UnpricedRequests != 2 {
		t.Errorf("cost --json: unpriced=%d tasks=%d priced=%d unpricedRequests=%d; want 1 1 2 2",
			rep.Unpriced, rep.Summary.Tasks, rep.Summary.PricedRequests, rep.Summary.UnpricedRequests)
	}
	if rep.Summary.TotalUSD <= 0 {
		t.Errorf("cost --json: the priced session priced to %v", rep.Summary.TotalUSD)
	}
}

func TestE2E_Ceiling(t *testing.T) {
	home, _ := e2eTranscript(t)
	out, errb, err := e2e(t, "ceiling", "--metered", filepath.Join(home, ".claude", "projects"))
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "ceiling", out, "Cache-blind arithmetic runs 3.66x high over these 80 requests")
}

func TestE2E_Trim(t *testing.T) {
	_, p := e2eTranscript(t)
	out, errb, err := e2e(t, "trim", "--cap", "8192", p)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "trim", out, "Scoring a 8192-byte cap on tool results over 1 session(s)", "4 block(s) over the cap, 30k bytes removable")
}

func TestE2E_Route(t *testing.T) {
	_, p := e2eTranscript(t)
	out, errb, err := e2e(t, "route", "--to", "claude-sonnet-5", p)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "route", out, "Write penalty is 1.25x at 5m and 2.00x at 1h")
}

func TestE2E_Context(t *testing.T) {
	_, p := e2eTranscript(t)
	out, errb, err := e2e(t, "context", p)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "context", out, "484k tokens of content entered this context", "41.8%")
}

func TestE2E_Advise(t *testing.T) {
	home, _ := e2eTranscript(t)
	out, errb, err := e2e(t, "advise", filepath.Join(home, ".claude", "projects"))
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "advise", out, "Sessions: 1 found, 1 calibrated", "cache breaks re-billed 1% of prompt tokens")
}

func TestE2E_Learn(t *testing.T) {
	home, _ := e2eTranscript(t)
	out, errb, err := e2e(t, "learn", filepath.Join(home, ".claude", "projects"))
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "learn", out, "Sessions: 1 found, 1 calibrated, 1 held out", "Selected: none")
}

func TestE2E_Redact(t *testing.T) {
	// A transcript carrying a real working directory and real message text,
	// built from the redacted fixture's structure: redaction is only
	// observable on input that has something to redact.
	_, p := e2eTranscript(t)
	src, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(string(src), "\n", 3)
	raw := strings.ReplaceAll(lines[0]+"\n"+lines[1]+"\n", `"cwd":"/redacted"`, `"cwd":"/Users/someone/secret-project"`)
	const secret = "the quick brown fox jumps over the lazy dog"
	raw = strings.Replace(raw, strings.Repeat("0", 0)+raw[strings.Index(raw, `"content":"`)+len(`"content":"`):][:len(secret)], secret, 1)
	if !strings.Contains(raw, secret) || !strings.Contains(raw, "secret-project") {
		t.Fatal("the unredacted fixture was not built")
	}
	unredacted := filepath.Join(t.TempDir(), "raw.jsonl")
	if err := os.WriteFile(unredacted, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errb, err := e2e(t, "redact", unredacted)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	first := strings.SplitN(out, "\n", 2)[0]
	var rec map[string]any
	if err := json.Unmarshal([]byte(first), &rec); err != nil {
		t.Fatalf("redact: first line is not JSON: %v\n%s", err, first)
	}
	if strings.Contains(out, "secret-project") {
		t.Error("redact kept the working directory")
	}
	if strings.Contains(out, secret) {
		t.Error("redact kept message text")
	}
	mustContain(t, "redact", out, `"cwd":"/redacted"`)
}

func TestE2E_Prefix(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.json")
	after := filepath.Join(dir, "after.json")
	if err := os.WriteFile(before, []byte(`{"mcpServers":{"a":{"command":"a"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(after, []byte(`{"mcpServers":{"a":{"command":"a"},"b":{"command":"b"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errb, err := e2e(t, "prefix", "--before", before, "--after", after)
	if err == nil {
		t.Fatalf("a changed tool set must exit non-zero, printed:\n%s%s", out, errb)
	}
	mustContain(t, "prefix", out+errb, "prefix")
}

func TestE2E_Since(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	proj := filepath.Join(home, ".claude", "projects", "proj")
	// The first look sets the marker; the records must land after it.
	if _, errb, err := e2e(t, "since"); err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	e2eLedgerAt(t, proj, time.Now().UTC().Add(2*time.Second))
	out, errb, err := e2e(t, "since")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	// RPL-C034: the worst window is ranked by tokens, so the unpriced session
	// leads it, while the dollar figure excludes it and says so.
	mustContain(t, "since", out,
		"largest: session unp",
		"excluded: their model is not in the price table")
}

func TestE2E_Simulate(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	e2eLedger(t, dir)
	policy := filepath.Join(home, "policy.json")
	if err := os.WriteFile(policy, []byte(`{"maxSessionUsd": 0.05}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errb, err := e2e(t, "simulate", "--policy", policy, "--json", dir)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	var rep struct {
		Simulated bool `json:"simulated"`
		Policy    struct {
			Hash string `json:"hash"`
		} `json:"policy"`
		Summary struct {
			Admitted int `json:"admitted"`
			Refused  int `json:"refused"`
		} `json:"summary"`
		Decisions []struct {
			Session   string  `json:"session"`
			ListUSD   float64 `json:"listUsd"`
			Simulated string  `json:"simulated"`
		} `json:"decisions"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("simulate --json: %v\n%s", err, out)
	}
	// The proxy's rule, recomputed: a request is refused exactly when the
	// session's admitted list spend before it has reached the cap. On this
	// ledger that is the second unpriced request (16.9 dollars at the dearest
	// rate) and nothing else, so 3 admitted, 1 refused.
	spent := map[string]float64{}
	for _, d := range rep.Decisions {
		want := "admitted"
		if spent[d.Session] >= 0.05 {
			want = "refused"
		}
		if d.Simulated != want {
			t.Errorf("%s: simulated %q, want %q", d.Session, d.Simulated, want)
		}
		if d.Simulated == "admitted" {
			spent[d.Session] += d.ListUSD
		}
	}
	if !rep.Simulated || rep.Policy.Hash == "" || rep.Summary.Admitted != 3 || rep.Summary.Refused != 1 {
		t.Errorf("simulated=%v hash=%q admitted=%d refused=%d; want true, a hash, 3, 1\n%s", rep.Simulated, rep.Policy.Hash, rep.Summary.Admitted, rep.Summary.Refused, out)
	}
	human, _, err := e2e(t, "simulate", "--policy", policy, dir)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "simulate", human, "SIMULATED: policy "+rep.Policy.Hash, "refused    1", "says nothing about requests")
}

func TestE2E_Budget(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	e2eLedger(t, dir)
	out, errb, err := e2e(t, "budget", dir, "--json")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	var doc struct {
		Standing struct {
			ToolBytes int `json:"tool_bytes"`
			ToolCount int `json:"tool_count"`
		} `json:"standing"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("budget --json: %v\n%s", err, out)
	}
	// The standing prefix is read from the ledger's tool definitions: two
	// tools, 2,000 bytes, as written by e2eLedger.
	if doc.Standing.ToolBytes != 2000 || doc.Standing.ToolCount != 2 {
		t.Errorf("budget --json: standing tool_bytes=%d tool_count=%d; want 2000 and 2\n%s",
			doc.Standing.ToolBytes, doc.Standing.ToolCount, out)
	}
}

func TestE2E_Purge(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	e2eLedger(t, dir)
	// purge ages a file by its mtime, which is what a retention sweep sees.
	old := time.Now().Add(-48 * time.Hour)
	for _, f := range []string{"priced.jsonl", "unpriced.jsonl"} {
		if err := os.Chtimes(filepath.Join(dir, f), old, old); err != nil {
			t.Fatal(err)
		}
	}
	out, errb, err := e2e(t, "purge", dir, "--older-than", "1h")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	// Without --yes nothing is removed.
	if _, err := os.Stat(filepath.Join(dir, "priced.jsonl")); err != nil {
		t.Fatalf("purge without --yes removed a file: %v", err)
	}
	mustContain(t, "purge", out, "2 ")
}

func TestE2E_Privacy(t *testing.T) {
	home, _ := e2eTranscript(t)
	// Write one store the registry must disclose: advise leaves advice.json,
	// which carries tool and file names from the transcripts.
	if _, errb, err := e2e(t, "advise", filepath.Join(home, ".claude", "projects")); err != nil {
		t.Fatalf("advise: %v\n%s", err, errb)
	}
	out, errb, err := e2e(t, "privacy", "--json")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	var doc struct {
		Stores []struct {
			Name  string `json:"name"`
			Files int    `json:"files"`
		} `json:"stores"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("privacy --json: %v\n%s", err, out)
	}
	found := false
	for _, s := range doc.Stores {
		if s.Name == "advice.json" && s.Files == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("privacy --json does not disclose the advice.json that advise just wrote:\n%s", out)
	}
}

func TestE2E_Serve(t *testing.T) {
	// serve binds a listener and runs until stopped, so the end-to-end path
	// exercised here is the one a user hits first: flag validation. A bad
	// --preflight must refuse before anything listens.
	// --upstream "" is a second refusal behind the first, so a build in which
	// the preflight guard stopped refusing still cannot reach a listener.
	_, errb, err := e2e(t, "serve", "--preflight", "-5", "--upstream", "")
	if err == nil {
		t.Fatal("serve accepted --preflight -5")
	}
	if !strings.Contains(err.Error(), "-preflight must be a positive token ceiling") {
		t.Errorf("serve: refused for the wrong reason: %v\n%s", err, errb)
	}
}

func TestE2E_Upgrade(t *testing.T) {
	// The release index is served from a test server: upgrade's own network
	// client is the one shipped, pointed at a host the test owns.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	restore := newUpdateClient
	t.Cleanup(func() { newUpdateClient = restore })
	newUpdateClient = func() *selfupdate.Client {
		return &selfupdate.Client{ReleasesBase: srv.URL}
	}
	out, errb, err := e2e(t, "upgrade", "--version", "v9.9.9", "--dry-run")
	if err == nil {
		t.Fatalf("a release the index does not carry must fail, printed:\n%s%s", out, errb)
	}
}

// replay context accounts for each recorded compaction: what the client kept
// and what the first prompt after the boundary carried.
func TestE2E_ContextReportsWhatACompactionKept(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.jsonl")
	src := `{"type":"user","uuid":"u0","sessionId":"s","timestamp":"2026-09-06T00:00:00Z","cwd":"/tmp/x","message":{"role":"user","content":"start"}}
{"type":"assistant","uuid":"a0","parentUuid":"u0","sessionId":"s","timestamp":"2026-09-06T00:00:01Z","requestId":"r0","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"working"}],"usage":{"input_tokens":1000,"cache_read_input_tokens":965000,"output_tokens":20}}}
{"type":"system","subtype":"compact_boundary","uuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:05Z","compactMetadata":{"trigger":"auto","preTokens":969218,"postTokens":26970}}
{"type":"user","uuid":"u1","parentUuid":"b1","sessionId":"s","timestamp":"2026-09-06T00:00:06Z","isCompactSummary":true,"message":{"role":"user","content":"summary of the work so far"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","sessionId":"s","timestamp":"2026-09-06T00:00:07Z","requestId":"r1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"continuing"}],"usage":{"input_tokens":79383,"output_tokens":20}}}
`
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errb, err := e2e(t, "context", p)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "context", out, "as the client recorded it", "26k kept", "the first prompt after it carried 79k tokens", "52k", "calculated")
}
