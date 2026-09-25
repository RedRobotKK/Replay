package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RedRobotKK/Replay/internal/advisor"
)

// markAdvice records a triage decision against one finding.
//
// The screen let a reader rank findings and do nothing about them, which is a
// report with a cursor. This is the other half: what the reader decided
// outlives the keystroke, in the file `replay advise` already writes and
// already reads back.
//
// It refuses rather than no-ops on an id it cannot find or a file that is not
// there. A silent success is how somebody comes to trust a screen that is
// recording nothing — the failure this repository has spent the day cataloguing
// in other forms.
func markAdvice(id string, status advisor.Status) error {
	// A computed status cannot be recorded as a decision. track produces
	// Verified, NotVerified, AdviceOnly and Pending; a reader produces Applied
	// and Dismissed. Writing one of the first four here would put a machine's
	// inference into the only field on disk that claims a human author.
	decision, ok := readerDecision(status)
	if !ok {
		return fmt.Errorf("%q is not a decision a reader can make: only %q and %q are",
			status, advisor.Applied, advisor.Dismissed)
	}
	path := filepath.Join(tipStateDir(), adviceFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no advice to mark: run `replay advise` first (%w)", err)
	}
	var f adviceFile
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("advice file is not readable by this build: %w", err)
	}
	// A file this build does not understand is not advice, the same rule
	// adviceFromCache applies before rendering one. Upgrading it in place would
	// carry statuses of unknown provenance forward under a schema number that
	// asserts they are trustworthy. `replay advise` rewrites it honestly.
	if f.Schema != advisor.AdviceFileSchema {
		return fmt.Errorf("advice file is schema %d, this build writes %d: run `replay advise` to rebuild it",
			f.Schema, advisor.AdviceFileSchema)
	}
	found := false
	for i := range f.Suggestions {
		if f.Suggestions[i].ID == id {
			f.Suggestions[i].Status = status
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no finding with id %q in %s", id, path)
	}
	// The status above is for the next render. This is the record: it is what
	// the next run reads, and it is the only thing that survives the status
	// being recomputed from the corpus.
	if f.Decisions == nil {
		f.Decisions = map[string]advisor.Decision{}
	}
	f.Decisions[id] = decision

	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	// Written through a sibling and renamed, like every other file this tool
	// owns: a half-written advice file read on the next keystroke would lose
	// every decision the reader had made, not just the one in flight.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readerDecision maps the status a triage keystroke asks for onto the decision
// that gets recorded, and reports whether the status is one a reader can set at
// all.
func readerDecision(status advisor.Status) (advisor.Decision, bool) {
	switch status {
	case advisor.Applied:
		return advisor.DecisionApplied, true
	case advisor.Dismissed:
		return advisor.DecisionDismissed, true
	default:
		return "", false
	}
}

// cacheCoversCorpus reports whether cached advice describes the corpus on disk.
//
// Coverage, not age. Reading advice.json made the advise screen instant and, on
// the machine this was written on, wrong: it showed three findings over one
// transcript while the corpus held 140 over 1,729, because an earlier run
// against a fixture had left the file behind. A timestamp would not have caught
// that — the advice was minutes old and about a different corpus.
//
// Equality is not the test either. The corpus grows while the tool runs; the
// transcript count moved 1,691 to 1,719 in an hour on 2026-09-09. So the
// question is whether the cache saw substantially what is there now.
//
// A cache LARGER than the corpus is also rejected. That means files were
// removed or the reader has pointed at a different directory, and advice about
// transcripts that are gone is advice about somebody else's machine.
func cacheCoversCorpus(cached, onDisk int) bool {
	if onDisk <= 0 || cached <= 0 {
		return false
	}
	if cached > onDisk {
		return false
	}
	// Within a tenth. Wide enough to survive a session's growth, narrow enough
	// that a fixture run against one file cannot pass.
	return float64(cached) >= float64(onDisk)*0.9
}

// readerDecisions reads what the reader has actually said about each finding.
//
// The decisions object is the only part of the advice file a person authored.
// Statuses are recomputed on every run and are never read back: they are output.
// Until 2026-09-23 this function's predecessor read them as input, so a status
// the pre-8f32a31 verifier had inferred from corpus drift came back as a human
// decision forever after and let a suggestion promote itself. See ADR-0027.
//
// A missing or unreadable file means nobody has marked anything, which is
// Pending everywhere rather than an error: not having triaged is the normal
// state, not a fault.
//
// A file at an older schema contributes nothing. Its statuses have unknown
// provenance and it has no decisions object at all, so there is nothing in it
// that anybody can be shown to have decided. Reading its statuses instead would
// be the same laundering wearing a new schema number.
func readerDecisions() map[string]advisor.Decision {
	b, err := os.ReadFile(filepath.Join(tipStateDir(), adviceFileName))
	if err != nil {
		return nil
	}
	var f adviceFile
	if json.Unmarshal(b, &f) != nil || f.Schema != advisor.AdviceFileSchema {
		return nil
	}
	out := map[string]advisor.Decision{}
	for id, d := range f.Decisions {
		// Only the two a reader can make. The file is editable, and a
		// hand-written "verified" here would walk straight back into the
		// defect this record exists to close.
		if d == advisor.DecisionApplied || d == advisor.DecisionDismissed {
			out[id] = d
		}
	}
	return out
}

// appliedIDs reads which findings the reader has marked applied.
//
// Applied only. A dismissal is a decision and it is not an application: the
// reader said they were not doing it, so nothing about the corpus moved and
// there is nothing for the verifier to judge. Folding it in here would be this
// same defect built out of a different constant.
func appliedIDs() map[string]bool {
	out := map[string]bool{}
	for id, d := range readerDecisions() {
		if d == advisor.DecisionApplied {
			out[id] = true
		}
	}
	return out
}

// suggestForReader computes suggestions and overlays the reader's own
// decisions, and returns the decisions so a caller rewriting the file can carry
// them forward.
//
// One function because `replay advise` and the `replay tui` advise screen both
// need it, and this screen has already been the site of three separate
// drifts between the two paths.
func suggestForReader(obs []advisor.Observation) ([]advisor.Suggestion, map[string]advisor.Decision) {
	decisions := readerDecisions()
	return advisor.ApplyDecisions(advisor.Suggest(obs, appliedIDs()), decisions), decisions
}
