package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
)

// FreezePrefix is the smallest kernel-shaped behaviour that can live behind
// existing serve: pin same-length volatile system bytes, and label a tool-set
// epoch from the exact tools JSON on the wire.
//
// Off by default (ADR-0011). It rewrites the body only to replace a
// cc_version hash with a same-length constant so the prefix key does not
// fork on Claude Code's billing header. Tool-set changes are labelled, not
// rewritten. The epoch is our id, not the provider's cache key. PX8: a new
// epoch is a set change, not a claim the next request will miss.
//
// The tools hash is sha256 of the RawMessage as forwarded — Fisher: if we
// hash a parse, we have two readings of tools/list.

var ccVersion = regexp.MustCompile(`cc_version=[0-9A-Za-z._-]+`)

const frozenVersion = "cc_version=frozen"

func toolsWireHash(body []byte) string {
	var obj map[string]json.RawMessage
	if json.Unmarshal(body, &obj) != nil {
		return ""
	}
	raw, ok := obj["tools"]
	if !ok || len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

func freezeBillingHeader(body []byte) ([]byte, bool) {
	loc := ccVersion.FindIndex(body)
	if loc == nil {
		return body, false
	}
	old := body[loc[0]:loc[1]]
	repl := []byte(frozenVersion)
	if len(repl) > len(old) {
		return body, false
	}
	if len(repl) < len(old) {
		padded := make([]byte, len(old))
		copy(padded, repl)
		for i := len(repl); i < len(old); i++ {
			padded[i] = ' '
		}
		repl = padded
	}
	out := make([]byte, len(body))
	copy(out, body)
	copy(out[loc[0]:loc[1]], repl)
	return out, true
}
