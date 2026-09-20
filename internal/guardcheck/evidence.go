package guardcheck

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Policy B: a surviving introduced guard is acceptable only when its survival
// carries evidence, per guard, that a reviewer read and a machine can check.
//
// The reviewer's default is unchanged and stays unchanged: a guard this change
// introduced, which survives having its condition neutralised, fails the run.
// That is the whole point of it. What this file adds is the one way out, and
// the way out is deliberately narrow.
//
// Two situations are permitted, and they are kept apart because they age
// differently. A DOMINATED guard is one whose removal leaves every
// contract-level outcome unchanged on a stated corpus: the input is still
// refused, and only which diagnostic names the failure differs, or nothing
// differs at all. That is a claim about the code as it stands today and a
// refactor can invalidate it. A STRUCTURALLY UNREACHABLE guard is one no input
// can enter, by a property of a type or a dependency rather than by absence of
// effort. That is a claim about a contract.
//
// Everything about the mechanism fails closed. An entry with no justification,
// no evidence, an unknown category, a duplicate address, or an address that
// does not resolve to a surviving introduced guard fails the run. A manifest
// that cannot be read fails the run. A survivor with no entry fails the run,
// exactly as before. There is no wildcard, no file-level entry, no
// package-level entry and no "waive this" flag: the address names one guard or
// it names nothing.
//
// An evidenced guard does not disappear. It is reported in its own category so
// a reader can see what was permitted and why, and it stops contributing to the
// failure count only after its address matched exactly.

// EvidenceCategory is the kind of claim an entry makes.
type EvidenceCategory string

const (
	// Dominated means neutralising the guard left every contract-level
	// outcome unchanged on the corpus the entry names. Diagnostic identity
	// may differ.
	Dominated EvidenceCategory = "dominated"
	// StructurallyUnreachable means no input can make the condition true, by
	// a stated property of a type or dependency.
	StructurallyUnreachable EvidenceCategory = "structurally-unreachable"
)

// EvidenceAddress names one guard occurrence.
//
// It is Identity plus the producing statement, and it is a separate concept on
// purpose. Identity is the grouping key base-tree pairing spends by count;
// this is the addressing key the manifest spends exactly once. Nothing here
// reaches back into Identity(), PairSurvivors or baseSurvivorCounts.
//
// There is no ordinal. An ordinal was the first design and it was dropped on
// evidence: inserting an identical conditional ahead of an evidenced one
// renumbers it, and the entry then matches a guard nobody reviewed, with every
// field of the address unchanged. The producing statement does not move when
// something else is inserted, and it changes when the guard itself changes,
// which is when an entry should stop matching.
type EvidenceAddress struct {
	Pkg      string `json:"pkg"`
	Func     string `json:"func"`
	Cond     string `json:"cond"`
	Producer string `json:"producer"`
}

// EvidenceAddress is the guard's Policy B address.
func (g Guard) EvidenceAddress() EvidenceAddress {
	return EvidenceAddress{Pkg: g.Pkg, Func: g.Func, Cond: g.Cond, Producer: g.Producer}
}

// Evidence is one manifest entry.
type Evidence struct {
	EvidenceAddress
	Category      EvidenceCategory `json:"category"`
	Justification string           `json:"justification"`
	Evidence      string           `json:"evidence"`
}

// Manifest is the evidence a repository has recorded, keyed by address.
type Manifest struct {
	byAddr map[EvidenceAddress]Evidence
}

// ParseManifest reads a manifest and refuses one it cannot trust.
//
// Every refusal here is a failure of the run rather than an empty manifest,
// because an unreadable or self-contradicting manifest that read as "nothing
// is evidenced" would be indistinguishable from one that is simply absent, and
// the two mean different things to a reviewer.
func ParseManifest(raw []byte) (*Manifest, error) {
	var entries []Evidence
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("guard evidence manifest: %w", err)
	}
	m := &Manifest{byAddr: make(map[EvidenceAddress]Evidence, len(entries))}
	for i, e := range entries {
		switch {
		case e.Pkg == "" || e.Func == "" || e.Cond == "":
			return nil, fmt.Errorf("guard evidence entry %d: pkg, func and cond are all required", i)
		case e.Category != Dominated && e.Category != StructurallyUnreachable:
			return nil, fmt.Errorf("guard evidence entry %d (%s %s): category %q is not one of %q or %q",
				i, e.Func, e.Cond, e.Category, Dominated, StructurallyUnreachable)
		case e.Justification == "":
			return nil, fmt.Errorf("guard evidence entry %d (%s %s): justification is required; a guard is "+
				"permitted to survive because someone said why, not because a line exists", i, e.Func, e.Cond)
		case e.Evidence == "":
			return nil, fmt.Errorf("guard evidence entry %d (%s %s): evidence is required; a justification "+
				"without evidence is an assertion", i, e.Func, e.Cond)
		}
		if _, dup := m.byAddr[e.EvidenceAddress]; dup {
			return nil, fmt.Errorf("guard evidence entry %d (%s %s): duplicate address; one guard, one entry",
				i, e.Func, e.Cond)
		}
		m.byAddr[e.EvidenceAddress] = e
	}
	return m, nil
}

// LoadManifest reads a manifest from disk. A missing file is an empty manifest
// and not an error: a repository with nothing evidenced is the ordinary case,
// and it fails on its survivors rather than on the absence of a file.
func LoadManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Manifest{byAddr: map[EvidenceAddress]Evidence{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("guard evidence manifest: %w", err)
	}
	return ParseManifest(raw)
}

// ClassifyEvidenced splits introduced survivors into the ones a manifest
// accounts for and the ones nobody has explained.
//
// stale is the entries that matched no introduced survivor. They fail the run
// too, and for the same reason the frozen-mutant catalogue fails on an anchor
// that no longer resolves: an entry nobody can tie to a guard has stopped
// describing the tree, and a mechanism that let those accumulate would become
// the blanket waiver this one exists not to be.
func ClassifyEvidenced(introduced []Guard, m *Manifest) (evidenced []Evidenced, unexplained []Guard, stale []Evidence) {
	if m == nil {
		return nil, introduced, nil
	}
	used := make(map[EvidenceAddress]bool, len(m.byAddr))
	for _, g := range introduced {
		addr := g.EvidenceAddress()
		e, ok := m.byAddr[addr]
		if !ok {
			unexplained = append(unexplained, g)
			continue
		}
		used[addr] = true
		evidenced = append(evidenced, Evidenced{Guard: g, Evidence: e})
	}
	for addr, e := range m.byAddr {
		if !used[addr] {
			stale = append(stale, e)
		}
	}
	sort.Slice(stale, func(i, j int) bool {
		if stale[i].Func != stale[j].Func {
			return stale[i].Func < stale[j].Func
		}
		return stale[i].Producer < stale[j].Producer
	})
	return evidenced, unexplained, stale
}

// Evidenced is a survivor and the entry that accounts for it.
type Evidenced struct {
	Guard    Guard
	Evidence Evidence
}

// ExitCodeWithEvidence is the process status of a run that consulted a
// manifest.
//
// ExitCode is left exactly as it was and is called here for the three
// categories it already decides, so the existing contract and the tests on it
// are untouched. What this adds is the fourth absence: an entry that matched no
// surviving introduced guard. It fails, for the reason the frozen-mutant
// catalogue fails on an anchor that no longer resolves. An entry nobody can tie
// to a guard has stopped describing the tree, and a mechanism that let those
// accumulate would drift into the blanket waiver this one exists not to be.
//
// unexplained is what introduced was before the manifest was consulted, less
// the guards it accounted for. Evidenced guards are not passed here at all:
// they are reported by the caller and they do not fail the run. That is the
// whole of the policy change, and it is the only thing here that can turn a
// red run green.
func ExitCodeWithEvidence(unexplained, unchecked, unbuilt []Guard, stale []Evidence) int {
	if code := ExitCode(unexplained, unchecked, unbuilt); code != 0 {
		return code
	}
	if len(stale) > 0 {
		return 1
	}
	return 0
}
