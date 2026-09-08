// Package facts holds vendor behaviour that Replay's advice depends on, pinned
// to the version it was read against.
//
// This exists because of a specific failure on 2026-09-07. Advice for the
// Ollama surface was about to be written against three defaults held from
// memory, and all three were wrong: flash attention is auto rather than off,
// context length auto-tiers by VRAM rather than defaulting to one value, and
// batch size is now chosen automatically. None of them announced themselves as
// stale, because a prior does not.
//
// Prices already had this discipline. cachemodel carries a table version, a
// separate date for when it was last checked against an independent observer,
// and a note that fires when the gap gets long. Behaviour had nothing, so a
// price could go stale loudly while a default went stale silently.
//
// The rule here is narrower than the price table's and easier to enforce: a
// behavioural fact is true of a VERSION, not of a date. Sixty days does not
// make "flash attention defaults to auto" wrong. One release does.
package facts

import (
	"fmt"
	"strings"
)

// Fact is one piece of vendor behaviour, with where it came from.
//
// Source and Method are separate on purpose. Source is where the value was
// read; Method is what reading it consisted of. "The docs say so" and "the code
// that implements it says so" are different strengths of evidence and the
// distinction is the first thing lost when a table is copied forward.
type Fact struct {
	Key    string
	Value  string
	Source string
	Method string
}

// Set is the facts for one surface, and the version they were read against.
type Set struct {
	Surface         string
	VerifiedAgainst string // the vendor version these were read from
	VerifiedOn      string // YYYY-MM-DD, for the human, not for the comparison
	Facts           []Fact
}

// Note reports whether these facts can be presented as current, given the
// version actually installed.
//
// Three outcomes, and only the first is silence. A match says nothing. A
// mismatch names both versions, because "these may be stale" sends the reader
// looking for something this function already knows. An unreadable version is
// NOT treated as a match: "cannot tell" collapsing into "probably fine" is
// exactly how a stale default ships as current advice.
func (s Set) Note(installed string) string {
	if strings.TrimSpace(s.VerifiedAgainst) == "" {
		return fmt.Sprintf("The %s behaviour table records no version it was checked "+
			"against, so nothing here can be reported as current.", s.Surface)
	}
	installed = strings.TrimSpace(installed)
	if installed == "" {
		return fmt.Sprintf("The installed %s version could not be read, so the behaviour "+
			"below cannot be confirmed as current. It was read from %s %s on %s.",
			s.Surface, s.Surface, s.VerifiedAgainst, s.VerifiedOn)
	}
	if installed == s.VerifiedAgainst {
		return ""
	}
	return fmt.Sprintf("This %s is %s and the behaviour below was read from %s %s on %s. "+
		"Defaults move between releases, so treat any figure derived from it as "+
		"unconfirmed until it is re-read against %s.",
		s.Surface, installed, s.Surface, s.VerifiedAgainst, s.VerifiedOn, installed)
}

// Ollama is what was read from Ollama's own source on 2026-09-07.
//
// Every entry names the source that implements the behaviour rather than a page
// describing it, because on this surface the documentation lagged the code on
// all three of the defaults that were wrong.
func Ollama() Set {
	return Set{
		Surface:         "ollama",
		VerifiedAgainst: "0.33.3",
		VerifiedOn:      "2026-09-07",
		Facts: []Fact{
			{
				Key: "flash_attention_default", Value: "auto",
				Source: "ollama/ollama server source, v0.33.3",
				Method: "read from the code that selects it, not from documentation",
			},
			{
				Key: "context_length_default", Value: "auto-tiers 4k / 32k / 256k by available VRAM",
				Source: "ollama/ollama server source, v0.33.3",
				Method: "read from the code that selects it, not from documentation",
			},
			{
				Key: "num_batch_default", Value: "automatic, 512 then 1024 then 2048",
				Source: "ollama/ollama server source, v0.33.3",
				Method: "read from the code that selects it, not from documentation",
			},
			{
				Key: "cache_reuse_default", Value: "0, disabled, and Ollama never passes the flag",
				Source: "llama.cpp b10760 argument defaults, and Ollama's invocation",
				Method: "read from both sides: the default in llama.cpp and the arguments Ollama builds",
			},
			{
				Key: "num_keep_default", Value: "4, and truncation discards from the middle from position 4",
				Source: "llama.cpp b10760 context handling",
				Method: "read from the code that performs the discard",
			},
			{
				Key: "engines", Value: "two: GGUF on upstream llama-server as a subprocess, safetensors on Ollama's MLX engine",
				Source: "ollama/ollama, subprocess runner since v0.30.0 and MLX engine since v0.19.0",
				Method: "read from the source tree; the Go-native runner no longer exists",
			},
			{
				Key: "kv_cache_type_scope", Value: "sets K and V from one value, and is ignored entirely for MLX models",
				Source: "ollama/ollama environment handling, v0.33.3",
				Method: "read from the code that applies it",
			},
		},
	}
}
