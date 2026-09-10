package main

import (
	"os"
	"path/filepath"
	"strings"
)

// store is one thing this tool writes to the reader's machine.
//
// The list exists because a retention command that covers one store out of
// eleven is worse than no retention command: it answers "yes, that can be
// deleted" while ten others keep it. `replay purge` was written against the
// ledger, which is where the gap was found, and looking properly turned up ten
// more — including the masking vault, the one store here that holds secrets
// rather than counts about them.
//
// Every consumer walks this list rather than naming paths of its own, so a
// store added in future is covered by all of them at once or by none, and ST1
// fails when the tool learns to write somewhere the list does not name.
type store struct {
	// Name is the file or directory under ~/.replay.
	Name string
	// Holds says what is inside, in the reader's terms. `replay privacy`
	// prints it verbatim, so it is written for them rather than for us.
	Holds string
	// Dir is true for a directory of records.
	Dir bool
	// Sensitive marks a store holding the reader's own secrets or content
	// rather than counts and timings about them.
	Sensitive bool
	// Purgeable is true when a retention WINDOW may remove it. False does not
	// mean undeletable — it means removal is a decision rather than a
	// schedule, because something else depends on the file existing.
	Purgeable bool
	// Prefix matches sibling directories, so ledger-grok is covered by the
	// ledger entry rather than needing one of its own.
	Prefix bool
}

// Path resolves the store under the reader's home directory.
func (s store) Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".replay", s.Name)
}

// homeStores is every path this tool writes under ~/.replay.
//
// Keep it in the order a reader would want to be told about them: what holds
// their secrets first, then their findings, then caches and markers.
func homeStores() []store {
	return []store{
		{
			Name: "vault", Dir: true, Sensitive: true, Purgeable: false,
			Holds: "the masking vault: the real values behind placeholders sent to a provider. " +
				"The only store here that holds your secrets rather than counts about them. " +
				"Deleting it breaks rehydration for every transcript that referenced it, so it " +
				"is never removed by a retention window.",
		},
		{
			Name: "ledger", Dir: true, Prefix: true, Purgeable: true,
			Holds: "per-request records: timings, token counts, cache outcomes, the request path " +
				"and a session id. Never message content. One directory per named upstream.",
		},
		{
			Name: "archive", Dir: true, Purgeable: true,
			Holds: "ledger records rotated out of the active directory. Same shape, same absence " +
				"of content.",
		},
		{
			Name: "advice.json", Purgeable: true,
			Holds: "the findings `replay advise` produced, and which of them you marked applied. " +
				"Derived from your transcripts; carries tool and file names, no message text.",
		},
		{
			Name: "policy.json", Purgeable: false,
			Holds: "the request policy `replay learn` derived. Configuration you chose, so a " +
				"retention window does not remove it.",
		},
		{
			Name: "cost-index.json", Purgeable: true,
			Holds: "a cache of costs already computed, keyed by transcript path. Rebuilt on demand.",
		},
		{
			Name: "measurements.jsonl", Purgeable: true,
			Holds: "probe readings: what a model actually billed for a known request. Measurements " +
				"you paid for, so removing them loses evidence rather than privacy.",
		},
		{
			Name: "seen.json", Purgeable: true,
			Holds: "one timestamp: when `replay since` last reported. Nothing else.",
		},
		{
			Name: "tip.json", Purgeable: true,
			Holds: "when the funding line was last shown, so it is not shown again for a month. " +
				"A date and a figure, no identifier.",
		},
		{
			Name: "serve.log", Prefix: true, Purgeable: true,
			Holds: "the proxy's own log. Request paths, statuses and refusals; no request bodies.",
		},
		{
			// Registered 2026-09-10, after SC1 found it by reading the source
			// rather than by anyone remembering. It had been written since the
			// contribution path shipped and disclosed by nothing.
			//
			// Sensitive, and beside the vault rather than beside a cache:
			// anyone who can read this can compute this machine's contributor
			// tag for ANY campaign, which is precisely the linkage the
			// per-campaign tag exists to prevent.
			//
			// Not purgeable by a window, for the vault's reason: the tag has to
			// be stable or one contributor looks like many, which destroys the
			// only thing the tag is for. Deleting it is a decision, not a
			// schedule.
			Name: contributorSecretName, Sensitive: true, Purgeable: false,
			Holds: "the machine-local secret your contributor tag is derived from. Not sent " +
				"anywhere and not derived from your account: it exists so two submissions " +
				"from this machine can be recognised as one contributor. Anyone who can read " +
				"it can compute this machine's tag for any campaign, so it is owner-only. " +
				"Deleting it is safe and makes your next contribution look like a new " +
				"contributor.",
		},
		{
			// Also found by SC1. A cache of the price rules, fetched only when
			// the reader asks for it.
			Name: rulesFileName, Purgeable: true,
			Holds: "the price and caching rules last fetched by `replay rules --update`. " +
				"Provider figures and a version, nothing about you. Deleting it means the " +
				"next report uses the table compiled into the binary.",
		},
	}
}

// resolveStores expands the registry against a real directory, so a prefixed
// entry like ledger also covers ledger-grok.
func resolveStores(root string) []resolved {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []resolved
	for _, s := range homeStores() {
		for _, e := range entries {
			name := e.Name()
			match := name == s.Name
			if s.Prefix && !match {
				match = strings.HasPrefix(name, strings.TrimSuffix(s.Name, ".log")) &&
					(s.Dir == e.IsDir())
			}
			if !match {
				continue
			}
			out = append(out, resolved{store: s, Actual: name, Full: filepath.Join(root, name)})
		}
	}
	return out
}

// resolved is a registry entry matched to something that exists on disk.
type resolved struct {
	store
	// Actual is the name found, which differs from Name for a prefixed entry.
	Actual string
	Full   string
}
