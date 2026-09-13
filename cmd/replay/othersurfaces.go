package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Before declaring that there is nothing to read, look at the other agents.
//
// `defaultTranscriptRoots` knows only where Claude Code writes. `codexRoots`
// and the Ollama log glob have existed for as long as `replay codex` and
// `replay burn` have, and `discover` has known `~/.grok` and `~/.cursor`
// longer still. The empty-state path consulted none of them. So a reader with
// hundreds of Codex rollout logs on disk installed Replay, ran it, and was
// told "Claude Code is not installed here, or has never run" — true, and the
// wrong answer to the question they asked.
//
// It is the worst case to get wrong, because Codex is the richest surface
// available: it reports a cache-write field Anthropic's own wire does not,
// separates reasoning tokens, and carries a quota counter that actually moves.
// Sending that reader away is sending away the one user whose data can answer
// the most.

// codexHome is where Codex keeps its rollout logs, relative to the user's home.
//
// It lives here rather than in codex.go so that the set of agent homes this
// build knows about has exactly one home. FD-11 reads this file against every
// other `filepath.Join(home, ".<agent>")` in cmd/replay; a root defined
// somewhere else is a root the empty state can go blind to.
const codexHome = ".codex"

// otherSurface is an agent whose records are on this machine but which the
// default transcript roots do not cover.
//
// `cmd` is empty for a surface this build cannot yet read. That distinction is
// the whole point of the type: naming a surface tells the reader their data
// was seen, and offering a command that returns nothing would be the same
// defect as pointing them at an empty directory — a second empty report, this
// time with the tool's word behind it.
type otherSurface struct {
	name string // what the reader calls it
	dir  string // where its records were found
	cmd  string // the command that reads it, or empty if none does yet
	why  string // why there is no command, when there is none
}

// knownSurfaces is the full set of agent homes this build knows about.
//
// It is deliberately the single place that set is written down. FD-11 checks
// this file against every `filepath.Join(home, ".<agent>")` in cmd/replay, so
// a surface taught to `burn` or `discover` and forgotten here fails the suite
// rather than silently reintroducing the blind spot.
func knownSurfaces(home string) []otherSurface {
	codex := otherSurface{name: "Codex", cmd: "replay codex"}
	for _, dir := range codexRoots(home) {
		if hasEntries(dir) {
			codex.dir = dir
			break
		}
	}
	return []otherSurface{
		codex,
		{
			name: "Ollama",
			dir:  firstWithEntries(filepath.Join(home, ".ollama", "logs")),
			cmd:  "replay burn",
		},
		{
			name: "Grok",
			dir:  firstWithEntries(filepath.Join(home, ".grok")),
			why:  "Replay cannot read Grok's wire yet: it posts to /responses, which this build does not parse",
		},
		{
			name: "Cursor",
			dir:  firstWithEntries(filepath.Join(home, ".cursor")),
			// Measured on Cursor 3.17.19, 2026-09-12: state.vscdb holds 29,665
			// bubbleId rows, every one carries a tokenCount object, and every
			// inputTokens and outputTokens in them is zero. There is no
			// cacheRead or cacheWrite key anywhere in bubbleId, agentKv or
			// composerData, and usageData is {} on all 234 composers.
			//
			// The wording matters more than it looks. "carries structure but no
			// token usage" reads as "there is no such field", which points a
			// reader at writing a parser. The field is there and it is empty, so
			// no parser recovers anything: Cursor did not record it. Saying
			// which of the two it is decides who has to act.
			// The transcript half was added after a second sweep. The shipped
			// string cited state.vscdb alone, which understated the search and
			// invited the reply "you only looked in the database". The agent
			// transcripts are a separate tree in a separate format and were
			// enumerated too: 118 JSONL files under
			// ~/.cursor/projects/*/agent-transcripts across 31 session
			// directories, 4,838 rows of which 4,566 are message rows, and the
			// complete top-level key set across every one of them is role,
			// message, type, status, error. No row carries a usage or token key
			// at any level.
			//
			// Both halves belong in the sentence because they fail differently.
			// In the database the counter exists and is zero; in the transcripts
			// there is no counter at all. A reader told only the first might
			// reasonably go looking for usage in the other file, which is the
			// wasted afternoon this sentence exists to prevent.
			why: "Replay cannot read Cursor yet: every message row in its state.vscdb carries a tokenCount field and every one of them is zero, and its 4,566 agent-transcript rows carry no usage field at all",
		},
		{
			name: "AnythingLLM",
			// macOS only, and deliberately the single candidate that was
			// actually opened. The Linux spelling
			// ~/.config/anythingllm-desktop/storage is plausible and UNVERIFIED,
			// and a candidate nobody has seen is the kind of path that makes a
			// probe look thorough while never firing.
			dir: firstWithEntries(filepath.Join(home, "Library", "Application Support",
				"anythingllm-desktop", "storage")),
			// Measured on this machine, 2026-09-12, against AnythingLLM.app
			// 1.15.0: storage/anythingllm.db, table `workspace_chats`, 46 rows.
			// Every row's `response` column is JSON with the keys text, sources,
			// type, attachments, metrics, and all 46 metrics objects carry
			// exactly prompt_tokens, completion_tokens, total_tokens, outputTps,
			// duration, model, provider and timestamp. No cache key appears in
			// any of them.
			//
			// This surface is the counter-example to the rest of its family, and
			// the reason the wording is worth care. Most desktop chat frontends
			// are unpriceable because the stored message does not say which
			// backend served it, so a token count cannot select a price table.
			// AnythingLLM stamps `provider` and `model` on every row, so that
			// failure does not apply here. Its failure is the single narrower
			// one: no cache field, so a cached read cannot be told from a
			// full-price one. Naming the wrong failure would send a reader to
			// fix a thing that is not broken.
			//
			// The claim stays on the metrics object rather than on the file. A
			// sweep of the whole database does turn up the string
			// `cached_tokens` once, inside the `prompt` column of one row, in a
			// raw API response somebody pasted into a chat. That is content, not
			// instrumentation, and a `why:` string asserting the key appears
			// nowhere in the database would be false and falsifiable by grep.
			//
			// Detection is install level, not chat level, and cannot be narrowed
			// here. The record is rows inside a SQLite file this build does not
			// open, and anythingllm.db is written at first boot, so a machine
			// that installed AnythingLLM and never chatted looks the same from
			// outside. That is the same bargain the Cursor and Grok rows above
			// already make, and it is why this row offers no command: being sent
			// to a reader that finds nothing is the defect this file exists to
			// avoid, and saying "records are here, and here is what cannot be
			// got out of them" is the honest version.
			why: "Replay cannot price AnythingLLM yet: every workspace_chats row records prompt_tokens and completion_tokens in its metrics object, and that object has no cache field, so a cached read there cannot be told from a full-price one",
		},
		{
			name: "OpenClaw",
			dir:  openclawSessions(home),
			// Measured on this machine, 2026-09-12: two session files under
			// ~/.openclaw/agents/main/sessions, 707 entries, 447 of them
			// assistant messages. Every assistant message carries `api`,
			// `provider`, `model` and a `usage` object holding `input`,
			// `output`, `cacheRead`, `cacheWrite`, `totalTokens` and a nested
			// `cost`. 229 rows have a non-zero input; 113 have a non-zero
			// cacheRead, summing to 5,208,111 tokens; and zero rows have a
			// non-zero cacheWrite, across both providers present
			// (openrouter/anthropic/claude-3.5-haiku at 254 rows, and
			// openclaw/delivery-mirror at 193).
			//
			// So this is not the Codex case and not quite the Cursor case. The
			// file is plain jsonl, it names the model, and it counts cache
			// reads in the millions: on format alone Replay could read it. What
			// it cannot do is state a cache bill, because the write side of the
			// cache was never counted. A cached read at 0.1x is only a saving
			// against the 1.25x write that put it there, and with the write
			// unrecorded the arithmetic has one side of the trade missing.
			//
			// The wording says which half. Cursor's string reports a counter
			// that is zero everywhere, which tells a reader the data was never
			// written at all. Here one counter moves and its partner does not,
			// and a reader who wants OpenClaw priced has to get cacheWrite
			// populated upstream rather than wait for a parser here. Naming the
			// surviving cacheRead counter in the same sentence is what makes
			// that difference legible: without it the string reads as "no cache
			// data", which is false and points at the wrong repository.
			why: "Replay cannot price OpenClaw yet: every assistant row in its session log carries a cacheWrite counter and every one of them is zero, beside cacheRead counters on the same rows that are not",
		},
	}
}

// openclawSessions returns the first OpenClaw session directory holding a file.
//
// The agent id is a path segment the reader chooses, so this globs rather than
// naming one. `~/.openclaw/agents/main/sessions` is what a default install
// looks like, and `main` is the default agent rather than the only one: the
// same tree carries a `subagents/` directory and a `cron/jobs.json` that
// schedules work, so a second agent is an ordinary thing to have. A probe
// hardcoded to `main` would find the default install and report nothing for
// everybody else, which is the blind spot FD-11 is about.
//
// It probes the sessions directory and not `~/.openclaw` itself, which is the
// difference between detecting data and detecting an install. The root holds
// openclaw.json, exec-approvals.json, update-check.json and several other loose
// files from the moment the tool is installed, so `hasEntries` on the root
// answers true on a machine that has never run a session. Announcing that as
// "records are on this machine" is OS4's defect one directory over: a claim
// about the reader's data that their disk does not support.
//
// Glob's error is deliberately not branched on. The only error it can return is
// ErrBadPattern, and this pattern is a compile-time constant with one `*` in
// it, so a branch here could never be taken and would be a guard that cannot
// fail (ADR-0014). A missing or unreadable directory yields no matches, which
// firstWithEntries already answers "" for.
func openclawSessions(home string) string {
	matches, _ := filepath.Glob(filepath.Join(home, ".openclaw", "agents", "*", "sessions"))
	return firstWithEntries(matches...)
}

// findOtherSurfaces reports the non-Claude-Code agents with records on disk.
//
// A directory that exists but is empty does not count. That is what a machine
// which once installed an agent looks like, and pointing its owner at a second
// command only to show them a second empty report is worse than saying nothing.
func findOtherSurfaces(home string) []otherSurface {
	// Without a home there is nothing to look in, and looking anyway is worse
	// than not looking: filepath.Join("", ".codex") is the relative path
	// ".codex", so every probe below would resolve against the working
	// directory and report whatever the reader happened to `cd` into.
	if home == "" {
		return nil
	}
	var found []otherSurface
	for _, s := range knownSurfaces(home) {
		if s.dir != "" {
			found = append(found, s)
		}
	}
	// Readable surfaces first: a reader with both wants the one they can act
	// on at the top, not sorted under an apology for one they cannot.
	sort.SliceStable(found, func(i, j int) bool {
		return found[i].cmd != "" && found[j].cmd == ""
	})
	return found
}

// firstWithEntries returns the first candidate directory that holds a file, or
// "".
//
// It took exactly one directory for as long as every surface this build knew
// about kept its records in one place. That stopped being true. A survey of the
// 2026 surfaces found the same agent writing under ~/Library/Application
// Support on macOS, ~/.config or ~/.local/share on Linux, and a path the user
// can move with an environment variable: Roo Code has customStoragePath, Kilo
// Code has KILO_DB and XDG_DATA_HOME, the Copilot CLI has COPILOT_HOME. A
// detector that checks one of those and reports nothing tells a reader their
// bill has no blind spot when it has one, which is worse than not knowing the
// surface exists at all.
//
// An empty string candidate is safe to pass: os.ReadDir("") returns no entries,
// so hasEntries already answers false and a caller building a path from an unset
// environment variable does not have to branch first. An explicit `dir != ""`
// guard sat here doing that job and mutating it away failed no test, because it
// could not: it guarded nothing hasEntries did not already handle. A guard that
// cannot fail is not a guard (ADR-0014), so it is gone rather than left as a
// claim nothing enforces.
func firstWithEntries(dirs ...string) string {
	for _, dir := range dirs {
		if hasEntries(dir) {
			return dir
		}
	}
	return ""
}

// hasEntries reports whether a directory exists and holds at least one file.
//
// The read error is deliberately not branched on. ReadDir returns no entries
// for a directory that is missing or unreadable, so the loop below already
// answers false; and where it returns partial entries alongside an error,
// those entries are a true answer to the question being asked.
func hasEntries(dir string) bool { return hasEntriesWithin(dir, entryProbeDepth) }

// entryProbeDepth is how far below a probe root a data file may sit.
//
// Three, because the deepest real layout seen is three: OpenClaw keeps
// agents/<agentId>/sessions/<uuid>.jsonl, and Grok keeps
// sessions/<urlencoded-cwd>/<uuid>/updates.jsonl. Four would buy nothing and
// cost a wider walk on every run.
const entryProbeDepth = 3

// hasEntriesWithin answers whether any FILE exists at or below dir, to depth.
//
// THE BUG THIS FIXES WAS SILENT, WHICH IS WHY IT MATTERS. The previous version
// looked only at direct children and counted only non-directories, so a root
// holding project/chats/session.jsonl and nothing else at its top level
// answered false. Every surface shipped before 2026-09-13 happens to keep a
// file directly at its probe root, which is why nothing caught it: ~/.grok,
// ~/.cursor, ~/.ollama/logs and ~/.codex all do.
//
// The surfaces that do not are the ones being added. ~/.openclaw/agents holds
// only agent directories, ~/.oracle/sessions only session directories, and the
// Gemini CLI, OpenCode and OpenHands layouts all put a directory level between
// the root and the data. A detector for any of them could not have fired.
//
// A detector that answers "nothing here" for a surface that is present is
// worse than no detector, and worse than a crash. It does not merely miss the
// surface: the report then goes on to imply the reader's bill has no blind
// spot, while the tool is standing in front of one. That is the failure the
// rule at the top of AGENTS_STATE.md names.
//
// The depth limit is not a performance nicety. A home directory handed to this
// by mistake would otherwise become a full filesystem crawl, on a tool whose
// first promise is that it is cheap to run and touches nothing it was not
// pointed at.
//
// An empty tree is still empty. A store created but never written to is absent
// for this purpose, and announcing it would be the opposite defect: naming a
// surface with nothing in it sends a reader looking for data that is not there.
//
// The read error is deliberately not branched on, for the reason above: a
// missing directory and an unreadable one both yield no entries, and false is
// the honest answer to "is there data here for me to read".
func hasEntriesWithin(dir string, depth int) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	if depth <= 0 {
		return false
	}
	for _, e := range entries {
		if e.IsDir() && hasEntriesWithin(filepath.Join(dir, e.Name()), depth-1) {
			return true
		}
	}
	return false
}

// writeOtherSurfaces names what was found, or writes nothing.
//
// Nothing is the common case and must stay silent: a machine with no other
// agent data would otherwise carry this paragraph on every empty run.
func writeOtherSurfaces(found []otherSurface, w io.Writer) {
	for _, s := range found {
		if s.cmd != "" {
			_, _ = fmt.Fprintf(w, "%s records are on this machine, and Replay reads them:\n", s.name)
			_, _ = fmt.Fprintf(w, "  %s\n", s.cmd)
		} else {
			_, _ = fmt.Fprintf(w, "%s records are on this machine. %s.\n", s.name, s.why)
		}
		_, _ = fmt.Fprintf(w, "  found in %s\n\n", s.dir)
	}
}
