package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
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
	if !ok {
		return ""
	}
	// An empty or null tool set is NO tool set, not a tool set that happens to
	// be empty.
	//
	// The first version tested `len(raw) == 0`, which is the length of the raw
	// JSON bytes — `[]` is two of them and `null` is four, so neither was
	// caught and each got its own epoch hash. A session whose client spells the
	// absence one way on one request and the other way on the next would have
	// reported two epochs, both of them meaning no tools.
	//
	// Absence, zero and unknown are three values (ADR-0018); here all three
	// spellings are the same one, and it is absence.
	if t := strings.TrimSpace(string(raw)); t == "" || t == "null" || t == "[]" {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// freezeBillingHeader pins the FIRST cc_version in the body to a same-length
// constant. Two limits decide how much this is worth, and neither was written
// down until a review measured them.
//
// IT DOES NOT SURVIVE A VERSION STRING CHANGING LENGTH. The replacement is
// padded to the length of what it replaced, so:
//
//	cc_version=1.0.99   ->  cc_version=frozen␣
//	cc_version=1.0.100  ->  cc_version=frozen␣␣
//
// Different lengths pad to different bytes, so the prefix still forks across
// exactly the client upgrade this exists to survive — it only holds while the
// version string keeps its length. Same-length padding is not a choice: a
// replacement that shortened the body would move every byte after it, which is
// the one thing a prefix pin may not do. So the benefit is real and bounded,
// and the flag's help says so rather than promising the general case.
//
// IT TAKES THE FIRST MATCH, over the whole body, not the system block. A body
// where cc_version appears in user content before the system prompt gets the
// user's copy pinned and the system one left forking — the stated purpose
// defeated and a user's message altered. That is why the caller is gated to
// the Messages family, where the system block is the first place this string
// appears in a Claude Code request; it is a property of the traffic, not a
// guarantee of this function, and a body that breaks the assumption gets a
// pointless rewrite rather than a wrong measurement.
//
// A version shorter than the constant is refused outright and silently — no
// log line, no ledger field. The operator turns the flag on and nothing
// happens. Worth fixing; recorded here so the next reader does not have to
// rediscover it from the padding arithmetic.
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
