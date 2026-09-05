package probe

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Running probes against a provider.
//
// This is the first thing in Replay that originates a billable request. The
// proxy forwards what an agent already sent; this creates traffic on purpose,
// with the operator's credential, and spends their money to learn something
// they cannot learn any other way. Every constraint here follows from that.
//
// The credential is read from the environment by the caller and held only for
// the life of the run. It is never a flag — a key on a command line lands in
// shell history and in the process table, where every other user on the box
// can read it — never written to the ledger, and never printed, including on
// the error paths where a dumped request would otherwise carry it.
//
// Nothing is sent until execution is explicitly asked for. Plan prints what
// would happen and what it would cost, because finding out the price of an
// experiment should not cost the price of the experiment.

// Runner executes a search against a provider.
type Runner struct {
	BaseURL string
	// APIKey is held for the run and never printed or persisted.
	APIKey string
	Client *http.Client
	Out    io.Writer
}

// tokensPerProbeChar approximates how many characters make a token. Probe
// prefixes are filler, so this only has to be close: the figure that matters
// is what the provider reports it cached, not what we intended to send.
const tokensPerProbeChar = 4

// Plan describes what a run would do, and sends nothing.
func (r *Runner) Plan(cfg Config, model string) {
	s := New(cfg)
	fmt.Fprintf(r.Out, "probe plan for %s\n\n", model)
	fmt.Fprintf(r.Out, "  range        %d to %d tokens\n", cfg.Min, cfg.Max)
	if cfg.RelativeResolution > 0 {
		fmt.Fprintf(r.Out, "  resolution   within %.1f%% of the answer\n", cfg.RelativeResolution*100)
	} else {
		fmt.Fprintf(r.Out, "  resolution   %d tokens\n", cfg.Resolution)
	}
	fmt.Fprintf(r.Out, "  confirm      %d agreeing answers per decision\n", max2(cfg.Confirm, 1))
	fmt.Fprintf(r.Out, "  budget       %d probe requests\n", cfg.MaxProbes)
	if d := s.AffordableDecisions(); d > 0 {
		fmt.Fprintf(r.Out, "  which buys   %d bisection decisions\n", d)
	}
	if s.BudgetTooSmall() {
		fmt.Fprintf(r.Out, "\n  This budget cannot reach that resolution. The run will stop early\n"+
			"  with a wider bracket, which is fine — it is said here so it is not\n"+
			"  a surprise afterwards.\n")
	}
	fmt.Fprintf(r.Out, "\nEach probe is one billable request to your provider, with a cache\n"+
		"breakpoint at the size being tested and content that has never been sent\n"+
		"before. Nothing has been sent yet.\n")
}

// Run executes the search, returning it whether or not it completed.
//
// It returns the search even on error so a partial result is still usable: the
// probes already paid for established a real bracket, and discarding them
// would mean paying for them twice.
func (r *Runner) Run(cfg Config, model string) (*Search, error) {
	s := New(cfg)
	sent := 0
	for {
		n := s.Next()
		if n == 0 {
			return s, nil
		}
		if cfg.MaxProbes > 0 && sent >= cfg.MaxProbes {
			return s, nil
		}
		res, err := r.probe(model, n)
		sent++
		if err != nil {
			return s, err
		}
		if rerr := s.Record(res); rerr != nil {
			// Inconclusive: it read an existing entry. The loop will propose
			// the same size again with fresh content, and the budget already
			// counted the request.
			fmt.Fprintf(r.Out, "  %d tokens: inconclusive (read an existing entry), retrying\n", n)
			continue
		}
		fmt.Fprintf(r.Out, "  %d tokens: %s\n", n, wroteWord(res.Wrote))
	}
}

func wroteWord(w bool) string {
	if w {
		return "cached"
	}
	return "not cached"
}

// probe sends one request with a breakpoint at the given prefix size.
func (r *Runner) probe(model string, prefixTokens int) (Result, error) {
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1,
		"system": []map[string]any{{
			"type": "text",
			// Unique every time. Repeated content caches on the first probe
			// and is READ by every later one, and a read tests nothing — the
			// run would learn nothing while costing full price.
			"text":          probeFiller(prefixTokens),
			"cache_control": map[string]string{"type": "ephemeral"},
		}},
		"messages": []map[string]any{{"role": "user", "content": "."}},
	})
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(r.BaseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", r.APIKey)

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		// Deliberately not wrapping the request: a dumped request carries the
		// key, and this text reaches a terminal and often an issue tracker.
		return Result{}, fmt.Errorf("probe request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{}, err
	}
	if resp.StatusCode != http.StatusOK {
		// The status, never the body: a provider error can echo the request.
		// An error is not evidence either — treating its absent usage as
		// "cached nothing" would push the floor above a size never tested.
		return Result{}, fmt.Errorf("the provider answered %d and the run stopped; nothing was recorded from it", resp.StatusCode)
	}

	var parsed struct {
		Usage struct {
			CacheCreation int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Result{}, fmt.Errorf("the provider's answer could not be read as usage")
	}
	return Result{
		PrefixTokens: prefixTokens,
		Wrote:        parsed.Usage.CacheCreation > 0,
		Read:         parsed.Usage.CacheRead > 0,
		CachedTokens: parsed.Usage.CacheCreation,
	}, nil
}

// probeFiller builds prefix content of roughly the requested token size that
// has never been sent before.
//
// The random prefix is the load-bearing part. Identical content would cache on
// the first probe and be read by every later one, and a read establishes
// nothing about the floor — the run would cost full price and learn nothing.
func probeFiller(tokens int) string {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	var b strings.Builder
	b.WriteString("replay probe ")
	b.WriteString(hex.EncodeToString(nonce))
	b.WriteString(" ")
	for b.Len() < tokens*tokensPerProbeChar {
		b.WriteString("filler ")
	}
	return b.String()
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
