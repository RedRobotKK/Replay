package transcript

import (
	"strings"
	"testing"
)

// An Ollama request whose n_past line never appeared has an UNKNOWN cached
// prefix, and the parser used to call it zero.
//
// That coercion is not neutral on this surface. Ollama's prompt_eval_count
// excludes whatever it reused, so the cached prefix is the only thing that
// says how much was reused. Zero there does not mean "no information", it
// positively asserts that the whole prompt was recomputed: a full cache miss,
// which is the worst possible reading of a request nobody measured.
//
// The parser already carried the right idea and dropped it at the last step.
// It tracks CachedPrefix as -1 for "not seen", keeps -1 through the whole
// block, and then, three lines before appending, converts it to 0.
//
// A note on how this defect was originally filed, because getting it wrong is
// instructive. It was recorded as atoiOr returning 0 on an unparseable number.
// It cannot: every integer the regexes capture is (\d+), so strconv.Atoi fails
// only on overflow, which needs a token count above 2^63. The mechanism was
// wrong and the class was right, which is the more dangerous way to be wrong,
// because the fix would have gone to the wrong line and the report would have
// said it was closed.

// a block with prompt eval, eval and total, and deliberately no n_past.
const noNPast = `
time=2026-09-08T04:00:00Z level=INFO msg="starting" model=registry.ollama.ai/library/llama3:8b
prompt eval time =     412.11 ms /   838 tokens (    0.49 ms per token)
eval time =    1201.55 ms /   120 tokens (   10.01 ms per token)
total time =    1613.66 ms /   958 tokens
`

// a block with n_past, for the contrast.
const withNPast = `
time=2026-09-08T04:00:00Z level=INFO msg="starting" model=registry.ollama.ai/library/llama3:8b
slot context shift: n_past = 116
prompt eval time =     412.11 ms /   838 tokens (    0.49 ms per token)
eval time =    1201.55 ms /   120 tokens (   10.01 ms per token)
total time =    1613.66 ms /   958 tokens
`

// OU-1: an absent n_past is not a measured zero.
func TestOU1_AnAbsentPrefixIsNotAZeroPrefix(t *testing.T) {
	rs, err := ParseOllamaLog(strings.NewReader(noNPast))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("want one request, got %d", len(rs))
	}
	if rs[0].PrefixMeasured() {
		t.Error("a block with no n_past line reports its cached prefix as measured")
	}
	if _, ok := rs[0].CacheHitRate(); ok {
		t.Error("a cache hit rate was reported for a request whose reuse was never observed; " +
			"zero there reads as a full cache miss, which is a claim about the request rather than about the log")
	}
}

// OU-2: a measured prefix still works, and still says it is measured.
//
// Without this, OU-1 is satisfied by a parser that reports nothing as measured
// ever, which is the shape of check this project has found repeatedly.
func TestOU2_AMeasuredPrefixIsStillMeasured(t *testing.T) {
	rs, err := ParseOllamaLog(strings.NewReader(withNPast))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("want one request, got %d", len(rs))
	}
	r := rs[0]
	if !r.PrefixMeasured() {
		t.Fatal("an n_past line was present and the prefix is not reported as measured")
	}
	if r.CachedPrefix != 116 {
		t.Errorf("cached prefix = %d, want 116", r.CachedPrefix)
	}
	rate, ok := r.CacheHitRate()
	if !ok {
		t.Fatal("no rate for a measured request")
	}
	if want := 116.0 / float64(116+838); rate != want {
		t.Errorf("rate = %v, want %v", rate, want)
	}
}

// OU-3: a genuine zero prefix is distinguishable from an absent one.
//
// "n_past = 0" is a real measurement: the server looked and reused nothing.
// It must report a 0% hit rate AND report that it was measured, which is the
// whole distinction this file exists to keep.
func TestOU3_AMeasuredZeroIsAResult(t *testing.T) {
	zero := strings.Replace(withNPast, "n_past = 116", "n_past = 0", 1)
	rs, err := ParseOllamaLog(strings.NewReader(zero))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("want one request, got %d", len(rs))
	}
	r := rs[0]
	if !r.PrefixMeasured() {
		t.Error("n_past = 0 is a measurement and must report as one")
	}
	rate, ok := r.CacheHitRate()
	if !ok {
		t.Fatal("a measured zero must still produce a rate")
	}
	if rate != 0 {
		t.Errorf("rate = %v, want 0 for a measured full miss", rate)
	}
}
