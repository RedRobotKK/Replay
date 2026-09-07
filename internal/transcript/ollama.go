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

// ContextTokens is how large the prompt was, which is not what Total measures.
func (r OllamaRequest) ContextTokens() int { return r.CachedPrefix + r.PromptEval }

// CacheHitRate is the share of the prompt that did not have to be recomputed.
func (r OllamaRequest) CacheHitRate() float64 {
	ctx := r.ContextTokens()
	if ctx == 0 {
		return 0
	}
	return float64(r.CachedPrefix) / float64(ctx)
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
				if cur.CachedPrefix < 0 {
					cur.CachedPrefix = 0
				}
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
