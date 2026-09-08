package transcript

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// SourceOllama marks a request read from an Ollama server log.
const SourceOllama Source = "ollama-server-log"

// OllamaRequest is one completed generation.
//
// The fields are kept apart because Ollama's own total does not mean what the
// other surfaces' totals mean. Everywhere else the headline prompt figure is
// the size of the context: Anthropic partitions the cached tokens out of it,
// OpenAI nests them inside it, DeepSeek splits it into hit and miss. Ollama
// reports the work it performed. A prefix already resident in the KV cache is
// not re-evaluated and does not appear in Total at all.
//
// A reader asking "what did this cost" wants Total, because locally the cost is
// compute. A reader asking "how big was the prompt" wants ContextTokens.
type OllamaRequest struct {
	Model string
	Slot  int
	Task  int
	// CachedPrefix is n_past: tokens already in the KV cache, carried from the
	// previous turn and not recomputed. Free, and invisible to Total.
	CachedPrefix int
	// PromptEval is the part of the prompt that actually had to be computed.
	PromptEval int
	// Generated is the completion.
	Generated int
	// Total is Ollama's own figure, and equals PromptEval + Generated.
	Total                     int
	PromptMS, EvalMS, TotalMS float64
}

// PrefixMeasured reports whether the log said how much prefix was reused.
//
// Ollama's prompt_eval_count EXCLUDES whatever it reused, so the n_past line
// is the only place the reuse appears. A block without one has an unknown
// prefix, and unknown is not zero: zero asserts that the whole prompt was
// recomputed, which is the worst reading available and a claim about the
// request rather than about the log.
func (r OllamaRequest) PrefixMeasured() bool { return r.CachedPrefix >= 0 }

// ContextTokens is how large the prompt was, which is not what Total measures.
//
// The second return says whether it is known. It is not derivable without the
// prefix, and returning PromptEval alone would silently answer a smaller
// question than the one asked.
func (r OllamaRequest) ContextTokens() (int, bool) {
	if !r.PrefixMeasured() {
		return 0, false
	}
	return r.CachedPrefix + r.PromptEval, true
}

// CacheHitRate is the share of the prompt that did not have to be recomputed,
// and whether that share is known at all.
//
// Three outcomes, deliberately, where there used to be two. A measured reuse
// gives a rate. A measured zero gives 0 and true, because the server looked
// and reused nothing and that is a result. An unmeasured request gives false,
// because the alternative is reporting a full cache miss for a request nobody
// observed.
func (r OllamaRequest) CacheHitRate() (float64, bool) {
	ctx, ok := r.ContextTokens()
	if !ok || ctx == 0 {
		return 0, false
	}
	return float64(r.CachedPrefix) / float64(ctx), true
}

var (
	reModel      = regexp.MustCompile(`model=(?:registry\.ollama\.ai/library/)?([^\s/]+:[^\s]+)`)
	reNPast      = regexp.MustCompile(`n_past (?:was set to|=) (\d+)`)
	rePromptEval = regexp.MustCompile(`prompt eval time =\s*([\d.]+) ms /\s*(\d+) tokens`)
	reEval       = regexp.MustCompile(`\beval time =\s*([\d.]+) ms /\s*(\d+) tokens`)
	reTotal      = regexp.MustCompile(`total time =\s*([\d.]+) ms /\s*(\d+) tokens`)
	reSlotTask   = regexp.MustCompile(`id\s+(\d+) \| task (-?\d+)`)
)

// ParseOllamaLogFile reads one server log.
func ParseOllamaLogFile(path string) ([]OllamaRequest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseOllamaLog(f)
}

// ParseOllamaLog reads an Ollama server log.
//
// The log is line-oriented and a request is assembled across several of them,
// so state is carried forward and cleared when a total closes the block. A
// partial block at the end of a truncated or rotated log is dropped rather than
// reported with zeroes, because a request missing its eval line is not a
// request that generated nothing.
func ParseOllamaLog(r io.Reader) ([]OllamaRequest, error) {
	var out []OllamaRequest
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	model := ""
	cur := OllamaRequest{CachedPrefix: -1, PromptEval: -1, Generated: -1}
	reset := func() { cur = OllamaRequest{CachedPrefix: -1, PromptEval: -1, Generated: -1} }

	for sc.Scan() {
		line := sc.Text()
		if m := reModel.FindStringSubmatch(line); m != nil {
			model = m[1]
		}
		if m := reNPast.FindStringSubmatch(line); m != nil {
			cur.CachedPrefix = atoiOr(m[1], 0)
		}
		if m := rePromptEval.FindStringSubmatch(line); m != nil {
			cur.PromptMS = atofOr(m[1])
			cur.PromptEval = atoiOr(m[2], 0)
		} else if m := reEval.FindStringSubmatch(line); m != nil && !strings.Contains(line, "prompt eval") {
			cur.EvalMS = atofOr(m[1])
			cur.Generated = atoiOr(m[2], 0)
		}
		if m := reTotal.FindStringSubmatch(line); m != nil {
			cur.TotalMS = atofOr(m[1])
			cur.Total = atoiOr(m[2], 0)
			// A total closes the block. Keep it only when both halves of the
			// conservation law are present: a total with no eval line is a
			// truncated record, not a request that produced nothing.
			if cur.PromptEval >= 0 && cur.Generated >= 0 {
				cur.Model = model
				if s := reSlotTask.FindStringSubmatch(line); s != nil {
					cur.Slot = atoiOr(s[1], 0)
					cur.Task = atoiOr(s[2], 0)
				}
				// CachedPrefix stays -1 when no n_past line appeared.
				//
				// It used to be set to 0 here, three lines before the append,
				// which threw away the distinction the sentinel existed to
				// carry. Everything above this line was already careful: the
				// block is dropped when PromptEval or Generated is missing,
				// on the stated grounds that a request missing its eval line
				// is not a request that generated nothing. The prefix got the
				// opposite treatment for no reason anybody wrote down.
				out = append(out, cur)
			}
			reset()
		}
	}
	return out, sc.Err()
}

func atoiOr(s string, d int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}

func atofOr(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
