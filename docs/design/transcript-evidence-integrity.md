# Transcript evidence-integrity contract

**Date:** 2026-11-02 · **Status:** Bundle B — shared contract for adapters, not a
change to ordinary decoding.

This document states the smallest contract an adapter must satisfy before
Replay can treat its output as source for occurrence-pinned verification
(`internal/evidenceverify`, Candidate B, frozen). It does not require any
adapter to support forensic verification; it requires that what an adapter
*supports* is stated, and that what it *cannot* support is reported as reduced
coverage rather than guessed.

## The one rule everything else follows

> **A decoded value is not occurrence-level evidence.** Ordinary decoding
> resolves duplicate keys, collapses absent booleans to `false`, and repairs
> invalid UTF-8 to U+FFFD. All three are convenient and all three are lossy.
> Any claim that requires the *exact recorded occurrence* must be answered from
> raw bytes before those bytes are decoded — never from the decoded struct.

Empirically pinned in `TestOrdinaryDecodeBoundaryIsLossy`:

- `encoding/json` resolves `{"is_error":true,"is_error":false}` last-wins → `false`.
- an absent `is_error` decodes to `false` (indistinguishable from recorded false).
- invalid UTF-8 in a string decodes successfully to U+FFFD (no error).

## Contract

### REQUIRED (for any adapter that claims FULL forensic coverage)

1. **Raw source bytes** — the smallest self-contained JSON value that contains
   the field under audit is addressable, byte-for-byte, before any decode.
2. **Physical byte offset** — the offset into those raw bytes is a raw byte
   index, never a rune/character/normalized-text position.
3. **Stable source identity** — the artifact these bytes belong to is named
   (path, session, or an existing digest), so evidence cannot cross artifacts.
4. **Deterministic boundaries** — the same immutable bytes yield the same
   record boundary every time.

### OPTIONAL

- **Re-readability** — raw bytes can be recovered later from the original
  artifact (path + re-parse). Needed only for historical re-verification.
- **Content digest** — a whole-artifact digest (reuse `evidencepin`/ledger
  hashes where they already exist; do not invent a second identity).

### UNSUPPORTED (must be reported, never guessed)

- Occurrence verification on a surface whose source cannot provide raw bytes,
  stable offsets, or source identity.
- Treating an adapter's decoded `Block.IsError bool` as forensic evidence.

## Adapter matrix

| Adapter | Source | Framing | Raw bytes | Duplicate keys | `is_error` forensic |
|---|---|---|---|---|---|
| Claude Code (`claudecode.go`) | JSONL transcript | `bufio.Scanner` + `TrimSpace` | line bytes transient; message `Content` retained as `json.RawMessage` | last-wins | PARTIAL (content bytes available before `DecodeContent`, not surfaced on `Session`) |
| Codex (`codex.go`) | JSONL rollout | `bufio.Scanner` + `TrimSpace` | line transient; `Payload` retained as `json.RawMessage` | last-wins | PARTIAL (offsets to file not stable after `TrimSpace`) |
| OpenAI (`openai.go`) | single request body | whole body | `body []byte` at call boundary, not retained | last-wins | PARTIAL (body available to caller; no `is_error` parsed) |
| Responses (`responses.go`) | single request body | whole body | `body []byte` at call boundary, not retained | last-wins | PARTIAL (body available to caller; no `is_error` parsed) |
| Ollama (`ollama.go`) | plain-text server log | line + regex | line text only | n/a (no JSON) | NOT APPLICABLE |
| JEV (`jev.go`) | JSONL capture | `bufio.Scanner` + `TrimSpace` | line transient | **rejects** (`jevRejectDuplicateKeys`) | NOT APPLICABLE (no `is_error`; demonstrates duplicate rejection) |
| Redact (`redact.go`) | JSONL transform | line + `map[string]any` | line transient, re-marshaled | last-wins | NONE (destructive rewrite; output bytes differ from source) |

## First irreversible information-loss boundary

`internal/transcript/wire.go:83` — `IsError bool json:"is_error"` on
`RawBlock`. When `DecodeContent` (`wire.go:161`) unmarshals message content
into `[]RawBlock`, duplicate `is_error` members collapse last-wins and an
absent member becomes `false`; `wire.go:216` then copies the collapsed bool
into `Block.IsError`. Before this boundary, `rawLine.Message.Content` still
holds the exact content bytes including duplicate keys. After it, the
distinction is gone and cannot be reconstructed from the decoded value.

## Historical and realtime paths

- **Historical** (offline reconstruction): adapters re-read the original file
  from `Session.Path` (`ParseClaudeCodeFile`, `ParseCodexFile`). Raw bytes are
  re-obtainable while the file is unchanged; source mutation changes the
  digest and invalidates any stale locator (proven in
  `TestTemporalAndCrossSessionIdentity`).
- **Realtime** (proxy/ledger): the ledger persists a content-free `Block`
  (including `Block.IsError bool`, already collapsed) and `BodyHashBefore` /
  `BodyHashAfter`, but **not** the raw content bytes. Occurrence verification
  on the realtime path therefore requires capturing raw bytes at ingestion;
  the current proxy path does not, so realtime forensic verification is
  **NOT APPLICABLE** until a capture exists.

## Identity rules (cross-session and temporal)

Evidence identity must be bound to the **whole artifact**, never to a local
fragment alone:

- a key/value pair is not evidence identity;
- a byte offset is not evidence identity;
- a local fragment hash is not evidence identity.

Two different transcripts that each contain `{"is_error":true}` at the same
offset are different evidence; the whole-transcript digest is what separates
them. `TestTemporalAndCrossSessionIdentity` pins this.

## What this bundle does and does not do

- Adds a conformance test suite (`evidenceconformance_test.go`) that pins the
  boundaries above so an adapter change cannot silently alter its duplicate
  handling or its forensic coverage class without a test noticing.
- Does **not** rewrite any adapter, add a second ledger, add a Work Graph, or
  auto-verify fields on the parse path. Ordinary `Block.IsError` behavior is
  unchanged.
