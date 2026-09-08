package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

// Ollama reports the work it did, not the context it held.
//
// Three of the four surfaces this engine reads put the cached tokens somewhere
// inside a headline prompt figure: Anthropic partitions them out of
// input_tokens, OpenAI nests them inside it, DeepSeek splits the prompt into
// hit and miss. Ollama does none of that. total is prompt_eval + eval, and the
// prefix already resident in the KV cache is not in it at all.
//
// So a reader who wants context size has to add n_past back, and a reader who
// wants compute has to leave it out. Conflating them is the whole risk here.
func TestOllamaTotalIsWorkDoneNotContextHeld(t *testing.T) {
	rs, err := ParseOllamaLogFile("ollamadata/server.log")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("requests = %d, want 2", len(rs))
	}
	r := rs[0]
	if r.PromptEval != 838 || r.Generated != 93 || r.Total != 931 {
		t.Errorf("got prompt=%d gen=%d total=%d, want 838/93/931",
			r.PromptEval, r.Generated, r.Total)
	}
	// The conservation law, verified on 3,161 real requests with zero
	// disagreements: what was computed plus what was generated is the total.
	if r.PromptEval+r.Generated != r.Total {
		t.Errorf("conservation broken: %d + %d != %d", r.PromptEval, r.Generated, r.Total)
	}
	// And the cached prefix sits outside it.
	if r.CachedPrefix != 116 {
		t.Errorf("cached prefix = %d, want 116", r.CachedPrefix)
	}
	ctx, ok := r.ContextTokens()
	if !ok {
		t.Fatal("this fixture carries an n_past line, so the context is measured")
	}
	if ctx != 116+838 {
		t.Errorf("context = %d, want %d: the prompt the model saw is the resident "+
			"prefix plus what had to be computed", ctx, 116+838)
	}
}

// A request is attributed to the model that was loaded for it.
func TestOllamaAttributesEachRequestToItsModel(t *testing.T) {
	rs, err := ParseOllamaLogFile("ollamadata/server.log")
	if err != nil {
		t.Fatal(err)
	}
	if rs[0].Model != "qwen2.5-coder:7b" {
		t.Errorf("model = %q, want qwen2.5-coder:7b", rs[0].Model)
	}
	if rs[1].Model != "llama3:latest" {
		t.Errorf("model = %q, want llama3:latest", rs[1].Model)
	}
}

// The parser agrees with a measurement taken outside it.
//
// Skipped where there is no local Ollama. Where there is one, this is the check
// that the reader describes the world rather than the fixture. The conservation
// law is that the work computed plus the work generated is the total Ollama
// reports, and it held on 3,161 of 3,161 real requests when measured by hand
// before any of this was written.
func TestOllamaConservationHoldsOnTheLocalLogs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	logs, _ := filepath.Glob(filepath.Join(home, ".ollama", "logs", "server*.log"))
	if len(logs) == 0 {
		t.Skip("no local Ollama server logs on this machine")
	}
	var n, warm int
	byModel := map[string]int{}
	for _, p := range logs {
		rs, err := ParseOllamaLogFile(p)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(p), err)
			continue
		}
		for _, r := range rs {
			n++
			byModel[r.Model]++
			if r.CachedPrefix > 0 {
				warm++
			}
			if r.PromptEval+r.Generated != r.Total {
				t.Errorf("%s: %d + %d != %d", filepath.Base(p), r.PromptEval, r.Generated, r.Total)
			}
		}
	}
	if n == 0 {
		t.Skip("logs present but no complete request blocks")
	}
	t.Logf("local Ollama: %d requests, %d with a warm prefix, models %v", n, warm, byModel)
}

// A block whose eval line never arrived is dropped, not counted as zero output.
//
// Logs rotate and processes are killed mid-request, so a total can close a
// block that has no completion in it. Recording that as a request which
// generated nothing would put a real prompt cost against imaginary silence and
// drag every average down with it.
func TestOllamaDropsATruncatedBlockRatherThanCountItAsZero(t *testing.T) {
	rs, err := ParseOllamaLogFile("ollamadata/server.log")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if r.Generated == 0 && r.PromptEval > 0 {
			t.Errorf("kept a block with %d prompt tokens and no completion: a request "+
				"whose eval line is missing did not generate nothing, it was cut off",
				r.PromptEval)
		}
	}
	if len(rs) != 2 {
		t.Errorf("requests = %d, want 2: the third block is truncated", len(rs))
	}
}
