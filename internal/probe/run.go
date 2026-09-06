package probe

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
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

	// overhead is the tokens a request carries outside the system block: the
	// user message and the envelope. Measured once per run.
	//
	// It matters more than its size suggests. The cache breakpoint sits on the
	// system block, so the prefix that must clear the provider's minimum is
	// that block alone — but the counting endpoint reports the whole request.
	// On this API the gap is 7 tokens, and it is the difference between a
	// result that reads as a contradiction and one that confirms the
	// documentation exactly: a 519-token request carries a 512-token prefix,
	// which is precisely the documented minimum for opus-5 and does cache,
	// while a 512-token request carries 505 and does not.
	overhead    int
	overheadSet bool

	// seen records what actually answered, per field.
	seenModel, seenTier, seenGeo map[string]bool

	// tokensPerRune is learned from the first sizing and reused.
	//
	// Sizing used to be a search — build, count, adjust, repeat — which was
	// necessary while the filler was English words, where a character is a
	// blunt and irregular dial. Varied CJK measures at exactly 2.00 tokens per
	// rune on this API and is linear across neighbouring sizes, so the rune
	// count for a target is a division. It is learned rather than assumed,
	// because another model or a future tokenizer may differ, and every probe
	// still verifies the result.
	tokensPerRune float64
}

// Provenance is what answered a run, as opposed to what was asked for.
//
// A floor measured against `claude-opus-5` is a floor measured against whatever
// snapshot that alias resolved to at the time, on whatever tier and in
// whatever geography the request was routed to. Without those recorded, two
// readings cannot be compared and neither can be reproduced — which is the
// whole value of a dated series.
type Provenance struct {
	ResolvedModel string
	ServiceTier   string
	Geo           string
	// Mixed is true when more than one snapshot, tier or geography answered a
	// single run. The bracket then has more than one subject in it.
	Mixed bool
}

// Provenance reports what answered, after a run.
func (r *Runner) Provenance() Provenance {
	p := Provenance{
		ResolvedModel: soleValue(r.seenModel),
		ServiceTier:   soleValue(r.seenTier),
		Geo:           soleValue(r.seenGeo),
	}
	p.Mixed = len(r.seenModel) > 1 || len(r.seenTier) > 1 || len(r.seenGeo) > 1
	return p
}

// soleValue returns the single observed value, or a joined list when several
// were seen — never one of them silently standing for the rest.
func soleValue(seen map[string]bool) string {
	if len(seen) == 0 {
		return ""
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func (r *Runner) note(seen *map[string]bool, v string) {
	if v == "" {
		return
	}
	if *seen == nil {
		*seen = map[string]bool{}
	}
	(*seen)[v] = true
}

// Overhead is the measured non-prefix cost of a request, available after a run.
func (r *Runner) Overhead() int { return r.overhead }

// measureOverhead counts a request with no system block at all.
func (r *Runner) measureOverhead(model string) error {
	if r.overheadSet {
		return nil
	}
	n, err := r.countBody(map[string]any{
		"model":    model,
		"messages": []map[string]any{{"role": "user", "content": "."}},
	})
	if err != nil {
		return err
	}
	r.overhead, r.overheadSet = n, true
	return nil
}

// tokensPerProbeChar approximates how many characters make a token. Probe
// prefixes are filler, so this only has to be close: the figure that matters
// is what the provider reports it cached, not what we intended to send.
const tokensPerProbeChar = 4

// runeBytes is how many bytes a CJK ideograph takes in UTF-8, and
// fillerPrefixBytes the fixed "replay probe <nonce> " header. Both are needed
// to convert a rune count back into the byte length fillerOfChars builds to.
const (
	runeBytes         = 3
	fillerPrefixBytes = 46
)

// Plan describes what a run would do, and sends nothing.
func (r *Runner) Plan(cfg Config, model string) {
	s := New(cfg)
	_, _ = fmt.Fprintf(r.Out, "probe plan for %s\n\n", model)
	_, _ = fmt.Fprintf(r.Out, "  range        %d to %d tokens\n", cfg.Min, cfg.Max)
	if cfg.RelativeResolution > 0 {
		_, _ = fmt.Fprintf(r.Out, "  resolution   within %.1f%% of the answer\n", cfg.RelativeResolution*100)
	} else {
		fmt.Fprintf(r.Out, "  resolution   %d tokens\n", cfg.Resolution)
	}
	fmt.Fprintf(r.Out, "  confirm      %d agreeing answers per decision\n", max2(cfg.Confirm, 1))
	fmt.Fprintf(r.Out, "  budget       %d probe requests\n", cfg.MaxProbes)
	if cfg.Prior > 0 {
		fmt.Fprintf(r.Out, "  testing      the documented %d first, then the size below it\n", cfg.Prior)
	}
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
			// Say so. StoppedEarly is otherwise set from the DECISION count,
			// and an inconclusive probe spends a request without deciding
			// anything — so a provider answering every request with a cache
			// read burns the budget and the run reports the full range as a
			// measured bracket, unqualified.
			s.stoppedEarly = true
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

// countTokens asks the provider how many tokens a prefix actually is.
//
// The counting endpoint is separate from inference and is not billed for the
// tokens it counts, so this costs a round trip rather than money.
func (r *Runner) countTokens(model, text string) (int, error) {
	return r.countBody(map[string]any{
		"model":    model,
		"system":   []map[string]any{{"type": "text", "text": text}},
		"messages": []map[string]any{{"role": "user", "content": "."}},
	})
}

func (r *Runner) countBody(payload map[string]any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(r.BaseURL, "/")+"/v1/messages/count_tokens", bytes.NewReader(body))
	if err != nil {
		return 0, err
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
		return 0, fmt.Errorf("token count request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("the provider answered %d when asked to count tokens", resp.StatusCode)
	}
	var parsed struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, fmt.Errorf("the token count could not be read")
	}
	return parsed.InputTokens, nil
}

// sizedFiller builds prefix content the provider counts as the requested size.
//
// The first live runs stalled because the size was a guess: the search asked
// for n tokens built from a chars-per-token approximation, the prefix became
// some other size, and the upper bound could never fall below whatever it
// actually became. Four models stalled at brackets about 170 tokens wide, and
// raising the budget from 20 probes to 40 changed nothing — the budget was
// never the constraint.
//
// Measuring closes that gap. A few counting round trips per probe cost no
// tokens and turn both bounds into real ones.
func (r *Runner) sizedFiller(model string, target int) (string, int, error) {
	// Start from the approximation, then correct against the provider's own
	// count. Two or three iterations reach the target for any tokenizer,
	// because each correction scales by the observed ratio rather than
	// assuming one.
	if err := r.measureOverhead(model); err != nil {
		return "", 0, err
	}
	best, bestTokens, bestGap := "", 0, 1<<30
	chars := target * tokensPerProbeChar
	if r.tokensPerRune > 0 {
		// Arithmetic, not search: runes needed for the target, converted back
		// to the byte length fillerOfChars builds to.
		chars = int(float64(target)/r.tokensPerRune)*runeBytes + fillerPrefixBytes
	}
	var text string
	for i := 0; i < 40; i++ {
		text = fillerOfChars(chars)
		total, err := r.countTokens(model, text)
		if err != nil {
			return "", 0, err
		}
		// Net of the envelope: the target is the size of the cacheable prefix,
		// not of the request that carries it.
		got := total - r.overhead
		if got <= 0 {
			return "", 0, fmt.Errorf("the provider counted the probe prefix as no tokens")
		}
		if runes := len([]rune(text)); runes > 0 {
			// Learned from what was actually built and counted, so a model
			// whose tokenizer differs corrects itself on the first probe.
			r.tokensPerRune = float64(got) / float64(runes)
		}
		if gap := abs(got - target); gap < bestGap {
			best, bestTokens, bestGap = text, got, gap
		}
		if got == target {
			return text, got, nil
		}
		// Scale to get close, then step a word at a time to land exactly.
		//
		// Exactly, not approximately. A tolerance of one percent is five
		// tokens at this scale, which is wider than the resolution the search
		// is asking for — and because each attempt regenerates the nonce, two
		// confirmations of the same target were different sizes. That produced
		// a "non-deterministic boundary" at 507 tokens on opus-5 which was
		// this function's noise, not the provider's behaviour.
		// Correct arithmetically too. Stepping a few characters at a time was
		// how this worked before the ratio was known, and it cost four and a
		// half counting round trips per probe where the ratio makes it one or
		// two: the distance to the target in tokens divides straight into a
		// rune count.
		switch {
		case r.tokensPerRune > 0:
			step := int(float64(target-got)/r.tokensPerRune) * runeBytes
			if step == 0 {
				// Inside one rune of the target. Move by the smallest unit
				// that changes anything, in the right direction.
				step = runeBytes
				if got > target {
					step = -runeBytes
				}
			}
			chars += step
		case got < target-8 || got > target+8:
			chars = chars * target / got
		case got < target:
			chars += 3
		default:
			chars -= 3
		}
		if chars < 1 {
			chars = 1
		}
	}
	// Not every token count is reachable: tokens span several characters, so a
	// tokenizer may step 512 to 515 with nothing in between. Rather than fail,
	// probe at the closest size that exists and report THAT — the search does
	// not need the size it asked for, it needs to know the size it got.
	if best == "" {
		return "", 0, fmt.Errorf("could not build a probe prefix near %d tokens", target)
	}
	return best, bestTokens, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// probe sends one request with a breakpoint at the given prefix size.
func (r *Runner) probe(model string, prefixTokens int) (Result, error) {
	filler, actual, err := r.sizedFiller(model, prefixTokens)
	if err != nil {
		return Result{}, err
	}
	// The size that was actually sent, not the one asked for. The search
	// bisects on what exists rather than on what it requested, which is what
	// keeps both bounds denominated in measured tokens.
	prefixTokens = actual
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1,
		"system": []map[string]any{{
			"type": "text",
			// Unique every time. Repeated content caches on the first probe
			// and is READ by every later one, and a read tests nothing — the
			// run would learn nothing while costing full price.
			"text":          filler,
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
		Model string `json:"model"`
		Usage *struct {
			ServiceTier   string `json:"service_tier"`
			Geo           string `json:"inference_geo"`
			Input         int    `json:"input_tokens"`
			CacheCreation int    `json:"cache_creation_input_tokens"`
			CacheRead     int    `json:"cache_read_input_tokens"`
			// The per-TTL breakdown. This API reports a write here as well as,
			// or instead of, the flat field, and the rest of this repository
			// already parses it.
			CacheCreationSplit struct {
				Ephemeral5m int `json:"ephemeral_5m_input_tokens"`
				Ephemeral1h int `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Result{}, fmt.Errorf("the provider's answer could not be read as usage")
	}
	// Absence is not a measurement.
	//
	// Reading a missing or reshaped usage object as "this prefix did not
	// cache" pushes the lower bound UP, which is exactly the direction that
	// manufactures a confirmation of a documented figure. A stub returning 200
	// with no usage produced "floor above 61490" with no error and no caveat.
	if parsed.Usage == nil || parsed.Usage.Input <= 0 {
		return Result{}, fmt.Errorf("the provider's answer carried no usage, so it says nothing about caching")
	}
	r.note(&r.seenModel, parsed.Model)
	r.note(&r.seenTier, parsed.Usage.ServiceTier)
	r.note(&r.seenGeo, parsed.Usage.Geo)

	created := parsed.Usage.CacheCreation
	if split := parsed.Usage.CacheCreationSplit.Ephemeral5m + parsed.Usage.CacheCreationSplit.Ephemeral1h; split > created {
		created = split
	}
	return Result{
		PrefixTokens: prefixTokens,
		Wrote:        created > 0,
		Read:         parsed.Usage.CacheRead > 0,
		CachedTokens: created,
	}, nil
}

// probeFiller builds prefix content of roughly the requested token size that
// has never been sent before.
//
// The random prefix is the load-bearing part. Identical content would cache on
// the first probe and be read by every later one, and a read establishes
// nothing about the floor — the run would cost full price and learn nothing.
// fillerOfChars builds probe content of a given length in RUNES.
//
// The body is varied CJK ideographs rather than English words, which is a
// meaningful optimisation rather than a curiosity. Measured against this API:
// English "filler " repeated is about 3.4 characters per token, so a character
// is a blunt dial and the reachable token counts are sparse. Varied Han
// characters count at almost exactly 2 tokens each — 200 runes to 420 tokens,
// 201 to 422, 202 to 424 — perfectly linear, so one rune moves the size by a
// known and constant amount.
//
// Varied is the load-bearing word. A REPEATED character compresses: "あ" a
// hundred times counts 60 tokens, and the hundred-and-first adds nothing at
// all, because the tokenizer merges the run. A probe built from repeats cannot
// be sized at all near the boundary.
//
// The nonce keeps every probe unique, so no probe can read an earlier probe's
// cache entry.
func fillerOfChars(chars int) string {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	var b strings.Builder
	b.WriteString("replay probe ")
	b.WriteString(hex.EncodeToString(nonce))
	b.WriteString(" ")

	// A stride coprime with the block size walks the range without repeating
	// adjacent characters, so nothing merges.
	for i := 0; b.Len() < chars; i++ {
		b.WriteRune(rune(0x4E00 + (i*7919)%20000))
	}
	return b.String()
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
