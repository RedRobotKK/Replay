package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AnythingLLM is the second surface found on this machine rather than derived
// from a vendor's repository, and it is the counter-example to the rest of the
// 2026 survey.
//
// MEASURED, 2026-09-12, ~/Library/Application Support/anythingllm-desktop/
// storage/anythingllm.db (749 KB SQLite), against AnythingLLM.app 1.15.0 in
// /Applications:
//
//   - table `workspace_chats`, 46 rows
//   - every row's `response` column is JSON with exactly the keys text,
//     sources, type, attachments, metrics
//   - all 46 `metrics` objects carry exactly prompt_tokens, completion_tokens,
//     total_tokens, outputTps, duration, model, provider, timestamp
//   - no cache key of any kind appears in any metrics object
//
// WHY THIS SURFACE MATTERS TO THE WORDING OF THE OTHERS.
//
// Most of this family fails because the record does not say which backend
// served a message, so token counts cannot select a price table. AnythingLLM
// does not fail that way: it stamps `provider` and `model` on every single
// row, so the route is never in doubt. Its failure is the narrower one, and
// the only one: there is no cache field, so a cached read is indistinguishable
// from a full-price one. Keeping those two failure modes in separate sentences
// is what lets a reader tell whose problem it is.
//
// ONE CORRECTION TO THE SURVEY THAT REACHED THIS FILE.
//
// The claim handed over was "zero hits for cache_read, cache_creation,
// cache_write or cached_tokens" across the whole database. A sweep run here
// found one: the string `cached_tokens` appears once, inside the `prompt`
// column of a workspace_chats row, in a raw API response a user had pasted
// into the chat. That is message content, not instrumentation. The difference
// is small and it decides the wording: a `why:` string saying no such key
// exists anywhere in the file would be false, and falsifiable by grep. The
// string below claims only what is true, that the metrics object has no cache
// field.

// anyLLMStorage is the storage directory, relative to home.
func anyLLMStorage(home string) string {
	return filepath.Join(home, "Library", "Application Support", "anythingllm-desktop", "storage")
}

// OS12: the desktop store is named, with no command, and the reason names the
// field that is present and the field that is not.
func TestOS12_TheDesktopStoreIsNamedWithItsMissingField(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(anyLLMStorage(home), "anythingllm.db"), "SQLite format 3\x00")

	got := surfaceNamed(findOtherSurfaces(home), "AnythingLLM")
	if got == nil {
		t.Fatalf("a desktop store is on disk and the empty state does not name it: %+v",
			findOtherSurfaces(home))
	}
	if got.cmd != "" {
		t.Errorf("this surface was offered the command %q, and this build cannot "+
			"price it: its metrics object has no cache field", got.cmd)
	}
	low := strings.ToLower(got.why)
	// What is recorded, so the reader knows the file is not empty.
	for _, want := range []string{"prompt_tokens", "completion_tokens"} {
		if !strings.Contains(low, want) {
			t.Errorf("the reason does not name %q, so the reader cannot tell that "+
				"tokens were recorded at all: %q", want, got.why)
		}
	}
	// What is absent, which is the actual blocker.
	if !strings.Contains(low, "cache") {
		t.Errorf("the reason never names the cache, which is the only thing "+
			"missing: %q", got.why)
	}
	// The overclaim this row was corrected for. A string asserting the key
	// appears nowhere in the database is false: `cached_tokens` occurs once,
	// in a message body a user pasted. The claim must stay on the metrics
	// object, which is the thing that was actually enumerated.
	if strings.Contains(low, "anywhere") {
		t.Errorf("the reason claims something about the whole database rather "+
			"than about the metrics object that was enumerated, and a pasted API "+
			"response in a chat row already falsifies it: %q", got.why)
	}
}

// OS13: a storage directory with no store in it is not a corpus.
//
// AnythingLLM creates storage/ and its subdirectories (documents, lancedb,
// vector-cache, hotdir, tmp) at first boot, all of them empty. A probe that
// answered true on those would announce records to a reader who has none.
func TestOS13_AnEmptyDesktopStorageTreeIsNotACorpus(t *testing.T) {
	home := withHome(t)
	for _, d := range []string{"documents", "lancedb", "vector-cache", "hotdir", "tmp"} {
		if err := os.MkdirAll(filepath.Join(anyLLMStorage(home), d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if got := surfaceNamed(findOtherSurfaces(home), "AnythingLLM"); got != nil {
		t.Errorf("a first-boot storage tree with no store in it was reported as a "+
			"corpus, pointing the reader at %q", got.dir)
	}
}

// OS14: the Cursor reason cites both artefacts that were checked, not one.
//
// The shipped string named only state.vscdb. That understates the search and
// invites the reply "you only looked in the database". The agent transcripts
// are a separate tree in a separate format and were enumerated too: 118 JSONL
// files under ~/.cursor/projects/*/agent-transcripts, 4,838 rows of which
// 4,566 are message rows, and the complete top-level key set across all of
// them is role, message, type, status, error. Zero rows carry a usage or token
// key at any level.
//
// Both halves belong in the sentence because they fail differently. In the
// database the counter exists and is zero; in the transcripts there is no
// counter at all. A reader who knows only the first might reasonably go
// looking for usage in the other file.
func TestOS14_TheCursorReasonCitesBothArtefacts(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".cursor", "x.db"), "x")

	got := surfaceNamed(findOtherSurfaces(home), "Cursor")
	if got == nil {
		t.Fatalf("Cursor data is on disk and the surface was not found: %+v",
			findOtherSurfaces(home))
	}
	low := strings.ToLower(got.why)
	for _, want := range []string{"state.vscdb", "zero", "transcript"} {
		if !strings.Contains(low, want) {
			t.Errorf("the Cursor reason does not name %q, so it reports less than "+
				"was actually checked: %q", want, got.why)
		}
	}
}
