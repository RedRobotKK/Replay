package observation

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The contribution boundary, enforced on the RECEIVING side.
//
// A corpus file is the one artifact this project asks a person to publish. It
// reaches the pool through a pull request, so the receiving side is a reviewer
// and a CI check rather than an HTTP handler, and the boundary has to hold
// there or it does not hold at all.
//
// WHAT WAS MISSING. encoding/json ignores keys it does not recognise. Nothing
// in this repository asked it not to, so a document carrying `"prompt": "..."`
// parsed cleanly, passed Validate, pooled correctly, and kept the prompt.
// Every existing guard is about fields that ARE in the struct: CC2 checks that
// no field NAME describes the contributor's machine, the retired-spelling check
// refuses names that used to mean something else, and Validate checks ranges.
// None of them could see a key that simply is not in the type, which is exactly
// the shape an accidental or malicious submission takes.
//
// The struct is the allowlist. A key outside it is refused by name.

// corpusAllowedKeys is every JSON key a corpus submission may carry, derived
// from the struct rather than typed, so the two cannot drift.
func corpusAllowedKeys() map[string]bool {
	out := map[string]bool{}
	rt := reflect.TypeOf(Corpus{})
	for i := 0; i < rt.NumField(); i++ {
		name := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			out[name] = true
		}
	}
	return out
}

// valid returns a submission that passes every existing guard, so a test below
// fails for the reason it names and not because the fixture was thin.
func validCorpusJSON(t *testing.T, extra map[string]any) []byte {
	t.Helper()
	c := Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-29T18:00:00Z",
		Tasks: 3, TotalUSD: 1.5, RebilledUSD: 0.1, RebilledShare: 6.67,
		MedianTaskUSD: 0.5, PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		SourceTag: "abcdef0123456789", TagBasis: "local",
	}.Digested()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) == 0 {
		return b
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for k, v := range extra {
		m[k] = v
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// SB1: the fixture itself is accepted, so every refusal below is attributable.
//
// Without this, a test asserting "refused" passes even if the fixture was
// malformed for an unrelated reason, which is the vacuous-guard shape ADR-0014
// rules out.
func TestSB1_TheBaseFixtureIsAccepted(t *testing.T) {
	var c Corpus
	if err := json.Unmarshal(validCorpusJSON(t, nil), &c); err != nil {
		t.Fatalf("the base fixture was refused, so no refusal below is attributable: %v", err)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("the base fixture does not validate: %v", err)
	}
}

// SB2: a key that is not in the struct is refused, and the refusal names it.
//
// These are the field names a real accident produces. A contributor pasting a
// session summary, a script templating a file, a build carrying a field this
// version does not know: all of them arrive as an unrecognised key, and all of
// them used to be accepted and silently dropped into the repository.
func TestSB2_AnUnrecognisedKeyIsRefusedByName(t *testing.T) {
	for _, key := range []string{
		"prompt", "response", "toolOutput", "toolArguments", "sourceCode",
		"path", "repoPath", "sessionName", "projectName", "apiKey",
		"credential", "env", "userId", "email", "hostname", "cwd",
	} {
		t.Run(key, func(t *testing.T) {
			var c Corpus
			err := json.Unmarshal(validCorpusJSON(t, map[string]any{key: "x"}), &c)
			if err == nil {
				t.Fatalf("a submission carrying %q was accepted. encoding/json drops keys it "+
					"does not recognise, so the field would have travelled into the published "+
					"corpus and stayed there", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("the refusal does not name %q, so a contributor cannot tell which "+
					"key to remove: %v", key, err)
			}
		})
	}
}

// SB3: the refusal is by allowlist, not by a list of bad words.
//
// A banned-word list only catches the names somebody thought of. This asserts
// the complement: a key nobody would think to ban is refused too, because the
// rule is "not in the struct" rather than "on a list".
func TestSB3_TheRuleIsAnAllowlistAndNotABannedWordList(t *testing.T) {
	var c Corpus
	err := json.Unmarshal(validCorpusJSON(t, map[string]any{"zq7": 1}), &c)
	if err == nil {
		t.Fatal("an arbitrary key was accepted. The boundary must be the struct, " +
			"because a banned-word list only ever catches the words somebody listed")
	}
}

// SB4: every key the writer emits is one the reader accepts.
//
// The round trip is the point: a guard that refused a key Replay itself writes
// would reject every real submission, which is how a fail-closed check gets
// loosened later by somebody in a hurry.
func TestSB4_EverythingReplayWritesIsAccepted(t *testing.T) {
	allowed := corpusAllowedKeys()
	if len(allowed) == 0 {
		t.Fatal("no keys were derived from the struct; this guard proves nothing")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(validCorpusJSON(t, nil), &m); err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("the fixture serialised to no keys at all")
	}
	for k := range m {
		if !allowed[k] {
			t.Errorf("Replay writes %q and the allowlist does not carry it", k)
		}
	}
}

// SB5: the hand-written allowlist equals the struct's own JSON tags.
//
// corpusKeys is written out rather than reflected, because this package's
// import allowlist (TestO7) may not carry reflect: the guard's value is that
// nobody widens it for a convenience. The cost of that choice is a list that
// can drift from the type it describes, and this is the payment. reflect is
// free in a test.
//
// It fails in BOTH directions on purpose. A field added to Corpus and not to
// corpusKeys would be refused on read, which would reject every submission
// from a newer build. A key left in corpusKeys after its field was removed
// would silently re-admit something the type no longer carries.
func TestSB5_TheAllowlistEqualsTheStruct(t *testing.T) {
	fromStruct := corpusAllowedKeys()
	if len(fromStruct) == 0 {
		t.Fatal("no keys were derived from the struct; this guard proves nothing")
	}
	for k := range fromStruct {
		if !corpusKeys[k] {
			t.Errorf("Corpus declares %q and corpusKeys does not. A submission from a "+
				"build carrying this field would be refused on read", k)
		}
	}
	for k := range corpusKeys {
		if !fromStruct[k] {
			t.Errorf("corpusKeys carries %q and Corpus no longer declares it, so a key "+
				"the type dropped is still being admitted", k)
		}
	}
}
