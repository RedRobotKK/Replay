// Package stateledger records how an agent's working beliefs changed, and what
// stayed open.
//
// A trace answers what calls were made. This answers a different question:
// what did the agent believe, what evidence moved it, and what is still
// unresolved. The unit is therefore not a span but a CLAIM with a standing
// that changes, each change naming the check that caused it.
//
// # The invariant this exists to hold
//
// Contradicting a claim does not establish its replacement. Finding that a
// belief is false teaches one thing, not two. `Replacement` is a pointer and
// nil means genuinely unknown; the package will not fill it, and a
// contradiction with no replacement leaves an open question behind that
// Render prints.
//
// # Checks are not equal
//
// A check that runs is not a check that settles. `Locates` names where related
// evidence lives, `Executable` is something the agent can run and finish, and
// only `Dispositive` can settle a claim. Settle refuses the other two. That
// distinction is the difference between an agent finishing a check and an
// agent answering the question.
//
// Nothing here reads a repository, a transcript or a network. A caller
// constructs the ledger from observations it has already made.
package stateledger

import (
	"fmt"
	"sort"
	"strings"
)

// Standing is what is currently believed about a claim.
type Standing string

// The standings a claim can hold.
const (
	Asserted     Standing = "ASSERTED"
	Supported    Standing = "SUPPORTED"
	Contradicted Standing = "CONTRADICTED"
	Superseded   Standing = "SUPERSEDED"
	Unresolved   Standing = "UNRESOLVED"
)

// Kind is what a check is capable of, which is not the same as whether it ran.
type Kind string

// What a check is capable of, in increasing strength.
const (
	Locates     Kind = "locates"     // names where related evidence lives
	Executable  Kind = "executable"  // can be run and finished
	Dispositive Kind = "dispositive" // can settle the claim
)

// Outcome is what a check returned.
type Outcome string

// What a check returned.
const (
	Supports     Outcome = "supports"
	Contradicts  Outcome = "contradicts"
	Inconclusive Outcome = "inconclusive"
	NotRun       Outcome = "not-run"
)

// OriginUnknown marks a claim whose origin was not observable, so that an
// unrecorded origin is visible rather than blank.
const OriginUnknown = "origin not recorded"

// Entry is one standing of a claim at a point in time.
type Entry struct {
	Standing Standing
	Because  string // check id, empty for the original standing
	At       string
}

// Claim is a proposition the agent worked from.
type Claim struct {
	ID          string
	Text        string
	Origin      string
	Standing    Standing
	Replacement *string // nil means the replacement is genuinely unknown
	History     []Entry
}

// Check is an investigation attached to a claim.
type Check struct {
	ID      string
	ClaimID string
	Kind    Kind
	Action  string
	Result  string
	Outcome Outcome
	At      string
}

// Ledger is one session's claims, checks and open questions.
type Ledger struct {
	Session string
	claims  []*Claim
	checks  []*Check
	open    map[string][]string
}

// New starts a ledger for a session.
func New(session string) *Ledger {
	return &Ledger{Session: session, open: map[string][]string{}}
}

// Claim records a proposition and returns its id.
func (l *Ledger) Claim(text, origin string) string {
	if strings.TrimSpace(origin) == "" {
		origin = OriginUnknown
	}
	id := fmt.Sprintf("c%d", len(l.claims)+1)
	l.claims = append(l.claims, &Claim{
		ID: id, Text: text, Origin: origin, Standing: Asserted,
		History: []Entry{{Standing: Asserted}},
	})
	return id
}

// Check records an investigation and returns its id.
func (l *Ledger) Check(claimID string, k Kind, action, result string, o Outcome) string {
	id := fmt.Sprintf("k%d", len(l.checks)+1)
	l.checks = append(l.checks, &Check{
		ID: id, ClaimID: claimID, Kind: k, Action: action, Result: result, Outcome: o,
	})
	return id
}

// Settle reports whether a check is capable of settling its claim.
//
// It refuses anything but a dispositive check. An executable check that ran
// cleanly still cannot close a question it was never able to answer.
func (l *Ledger) Settle(claimID, checkID string) error {
	c := l.checkByID(checkID)
	if c == nil {
		return fmt.Errorf("no such check %q", checkID)
	}
	if c.Kind != Dispositive {
		return fmt.Errorf("check %q is %s, which cannot settle claim %q: running a check is not answering the question",
			checkID, c.Kind, claimID)
	}
	return nil
}

// Revise moves a claim to a new standing because of a check.
//
// replacement is nil when the replacement is unknown, which is the normal case
// after a contradiction. A nil replacement records an open question rather
// than a conclusion.
func (l *Ledger) Revise(claimID, checkID string, to Standing, replacement *string) {
	c := l.ClaimByID(claimID)
	if c == nil {
		// A revision of a claim that does not exist is still an observation
		// somebody made. Dropping it would lose evidence silently, which is
		// the one thing this package must not do, so it is kept as an open
		// question under the id it named and Render prints it as orphaned.
		l.open[claimID] = append(l.open[claimID],
			fmt.Sprintf("revision to %s by check %s names no claim", to, checkID))
		return
	}
	c.Standing = to
	c.Replacement = replacement
	c.History = append(c.History, Entry{Standing: to, Because: checkID})
	if to == Contradicted && replacement == nil {
		l.open[claimID] = append(l.open[claimID], "replacement unresolved")
	}
}

// Open records a question left behind.
func (l *Ledger) Open(claimID, question string) {
	l.open[claimID] = append(l.open[claimID], question)
}

// HasOpenQuestion reports whether anything remains unresolved for a claim.
func (l *Ledger) HasOpenQuestion(claimID string) bool { return len(l.open[claimID]) > 0 }

// ClaimByID returns a claim, or nil.
func (l *Ledger) ClaimByID(id string) *Claim {
	for _, c := range l.claims {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// History returns every standing a claim has held, oldest first.
func (l *Ledger) History(id string) []Entry {
	if c := l.ClaimByID(id); c != nil {
		return c.History
	}
	return nil
}

func (l *Ledger) checkByID(id string) *Check {
	for _, c := range l.checks {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// Render prints the lifecycle of every claim.
//
// Open questions are printed, not omitted. A rendering that showed only what
// was settled would read as though nothing were outstanding, which is the
// failure this package exists to prevent.
func (l *Ledger) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session %s\n", l.Session)
	b.WriteString(strings.Repeat("-", 64) + "\n")
	for _, c := range l.claims {
		fmt.Fprintf(&b, "CLAIM        %s\n", c.Text)
		fmt.Fprintf(&b, "  origin     %s\n", c.Origin)
		for _, k := range l.checks {
			if k.ClaimID != c.ID {
				continue
			}
			label := strings.ToUpper(string(k.Kind)) + " CHECK"
			fmt.Fprintf(&b, "      |\n  %-10s %s\n", label, k.Action)
			fmt.Fprintf(&b, "  RESULT     %s  (%s)\n", k.Result, k.Outcome)
		}
		fmt.Fprintf(&b, "      |\n  STANDING   %s\n", c.Standing)
		if c.Replacement != nil {
			fmt.Fprintf(&b, "  REPLACED BY %s\n", *c.Replacement)
		} else if c.Standing == Contradicted {
			b.WriteString("  REPLACED BY UNRESOLVED\n")
		}
		for _, q := range l.open[c.ID] {
			fmt.Fprintf(&b, "  UNRESOLVED %s\n", q)
		}
		b.WriteString("\n")
	}
	l.renderOrphans(&b)
	return b.String()
}

// renderOrphans prints evidence that was recorded against no claim: a check
// whose ClaimID matches nothing, or an open question under an id no claim
// holds. Until 2026-10-03 both were accepted and then never printed, because
// the rendering walks claims and these hang off none. The section is omitted
// when there is nothing in it, so a ledger with no orphans renders as before.
func (l *Ledger) renderOrphans(b *strings.Builder) {
	known := map[string]bool{}
	for _, c := range l.claims {
		known[c.ID] = true
	}
	var lines []string
	for _, k := range l.checks {
		if !known[k.ClaimID] {
			lines = append(lines, fmt.Sprintf("  CHECK      %s on claim %s: %s\n  RESULT     %s  (%s)",
				k.ID, k.ClaimID, k.Action, k.Result, k.Outcome))
		}
	}
	var ids []string
	for id := range l.open {
		if !known[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		for _, q := range l.open[id] {
			lines = append(lines, fmt.Sprintf("  UNRESOLVED %s: %s", id, q))
		}
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("ORPHANED     evidence recorded against no claim\n")
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
}

func contains(h, n string) bool { return strings.Contains(h, n) }
