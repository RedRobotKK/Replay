package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/usage"
)

// replay grok - what the Grok CLI recorded, in tokens, checked against Grok's
// own ledger.
//
// `replay doctor` has been naming this surface as present and unread: the usage
// fields are there, and no reader was built. This is that reader.
//
// # Two artifacts, two different standings
//
// A Grok session directory holds up to two records of the same work.
//
//	updates.jsonl  the per-turn stream Replay reconstructs from
//	usage.json     the vendor's own ledger, which `grok usage` reads
//
// They are NOT interchangeable and they are never added together. Measured on
// 48 local sessions, 2026-09-27 (docs/evidence/grok-source-contract-2026-09-27.md
// on branch docs/grok-source-contract): 35 sessions carry both, 13 carry
// updates.jsonl and no usage.json, and `grok usage` succeeds on 35 of 35 and
// fails on all 13. So a session with no ledger is one Grok itself reports no
// usage for. Its reconstruction is still shown, because the records exist and
// hiding them would be a different lie, but it is never folded into the vendor
// column and its absence is never rendered as a zero.
//
// # The records are per turn
//
// An earlier draft of this reader kept the largest inputTokens in a session and
// called the records cumulative running totals. That is false, and the vendor's
// own output falsifies it: on session 01a0e417 the six turns sum to 7,142,396
// while the largest is 3,716,413 and the last is 2,363,388, and the file states
// the session total as the sum. On the 131-turn session the last turn is 0.
// Within-session inputTokens falls as often as it rises. The aggregation is the
// sum of turns, and the goldens under testdata/grok/vendor-usage are the
// vendor's own stdout, kept so that cannot silently regress.
//
// # Counting is inclusive
//
// cachedReadTokens is a SHARE of inputTokens, not a figure beside it:
// input + output == total held on every record of a 106-session corpus. A
// reader that copies inputTokens into a fresh-token field double-counts every
// cached token, worst on the sessions that cache best. Conversion goes through
// usage.FromInclusive, and a record whose parts do not add back to its prompt
// is counted as inconsistent rather than quietly included.
//
// # No dollars
//
// Grok logs costUsdTicks, and the Grok 1.0.41 user guide states the scale:
// 10^10 ticks per USD. Nobody has reconciled that scale against a statement of
// account. Documented and checked are different words, and this command prints
// no dollar figure until the second one is true.

// grokCounts is one set of Grok counters, with the inclusive prompt preserved
// alongside the fresh remainder rather than replaced by it.
type grokCounts struct {
	// Prompt is inputTokens as the provider states it, cache included.
	Prompt int
	// Fresh is Prompt less the cached read and the cache write. It is derived,
	// never read off the wire.
	Fresh       int
	CachedRead  int
	CachedWrite int
	Output      int
	Reasoning   int
	ModelCalls  int
	Turns       int
}

// add accumulates one turn, converting through the inclusive normaliser rather
// than copying the provider's input field into a fresh-token field.
//
// It reports whether the turn's own parts add up. A turn that does not is not
// added, because every downstream share divides by the prompt.
func (c *grokCounts) add(u grokUsage) bool {
	rec := usage.FromInclusive("grok", "grok-session", "", time.Time{}, usage.InclusiveCounts{
		Prompt:    u.InputTokens,
		Cached:    u.CachedReadTokens,
		Written:   u.cacheWrite(),
		Output:    u.OutputTokens,
		Reasoning: u.ReasoningTokens,
	}, nil)
	if rec.Validate() != nil {
		return false
	}
	c.Prompt += rec.Prompt
	c.Fresh += rec.Fresh
	c.CachedRead += rec.CachedRead
	c.CachedWrite += rec.CachedWrite
	c.Output += rec.Output
	c.Reasoning += rec.Reasoning
	c.ModelCalls += u.ModelCalls
	c.Turns++
	return true
}

// grokReconciliation is the standing of one session's two records against each
// other. UNAVAILABLE is a third value, not a zero: ADR-0018.
type grokReconciliation string

// The three standings a session can hold.
const (
	grokLedgerMatches     grokReconciliation = "MATCH"
	grokLedgerDiffers     grokReconciliation = "DIFFERS"
	grokLedgerUnavailable grokReconciliation = "UNAVAILABLE"
)

// grokUsage is one Grok usage object, in the provider's own field names.
//
// CacheCreationTokens is a pointer because a stated zero and an absent field
// are different facts (ADR-0018). Grok states zero; Codex omits the field.
type grokUsage struct {
	InputTokens         int  `json:"inputTokens"`
	OutputTokens        int  `json:"outputTokens"`
	TotalTokens         int  `json:"totalTokens"`
	CachedReadTokens    int  `json:"cachedReadTokens"`
	CacheCreationTokens *int `json:"cacheCreationTokens"`
	ReasoningTokens     int  `json:"reasoningTokens"`
	ModelCalls          int  `json:"modelCalls"`
	TurnCount           int  `json:"turnCount"`
	// costUsdTicks is deliberately NOT decoded. The Grok 1.0.41 user guide
	// states the scale, 10^10 ticks per USD, and nobody has reconciled it
	// against a statement of account. A field decoded into a typed struct is a
	// field somebody eventually multiplies by something, and the multiplication
	// is the thing that is not yet justified. TestGK14 holds this.
	//
	// ModelUsage is the per-model breakdown. Its presence is what marks an
	// object in updates.jsonl as the turn's usage record rather than one of the
	// nested per-model copies, which carry the same field names.
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
}

func (u grokUsage) cacheWrite() int {
	if u.CacheCreationTokens == nil {
		return 0
	}
	return *u.CacheCreationTokens
}

// grokLedger is usage.json, which is what `grok usage` reads.
type grokLedger struct {
	SessionID string      `json:"sessionId"`
	Session   grokUsage   `json:"session"`
	Turns     []grokUsage `json:"turns"`
}

// grokSession is one session directory: what Replay reconstructed, and what the
// vendor's ledger says, kept apart.
type grokSession struct {
	ID string
	// Reconstructed is the sum of the per-turn records in updates.jsonl.
	Reconstructed grokCounts
	// Vendor is the session object of usage.json, nil when there is no ledger.
	// Nil means the vendor reports no usage for this session, which is not the
	// same as a vendor total of zero.
	Vendor *grokCounts
	// VendorTurnSum is the sum of the ledger's own turns, kept because the
	// ledger states both and a disagreement between them is a defect in the
	// ledger rather than in the reconstruction.
	VendorTurnSum grokCounts
	// WroteCacheField records that the source stated a cache-creation figure at
	// all, zero included.
	WroteCacheField bool
	// Models is a SET, not a tally. A session that names one model in both its
	// stream and its ledger has still used it in one session, and counting the
	// artifacts instead of the sessions reported three for two.
	Models       map[string]bool
	Unreadable   int
	Inconsistent int
}

// reconcile compares the two records without merging them.
func (s grokSession) reconcile() grokReconciliation {
	if s.Vendor == nil {
		return grokLedgerUnavailable
	}
	if s.Vendor.Prompt == s.Reconstructed.Prompt {
		return grokLedgerMatches
	}
	return grokLedgerDiffers
}

// promptDelta is the vendor prompt less the reconstructed one, and is zero when
// there is no ledger to compare against.
func (s grokSession) promptDelta() int {
	if s.Vendor == nil {
		return 0
	}
	return s.Vendor.Prompt - s.Reconstructed.Prompt
}

// grokReading is one answer over a directory of Grok sessions.
type grokReading struct {
	Sessions int
	// Reconstructed covers every session, ledger or not.
	Reconstructed grokCounts
	// Vendor covers ONLY the sessions carrying a ledger. Sessions without one
	// are absent from it, never zero in it, and never added into it.
	Vendor grokCounts

	Matched  int
	Diverged int
	NoLedger int
	// DivergedTokens is the total absolute prompt disagreement over the
	// diverging sessions.
	DivergedTokens int
	// NoLedgerTokens is what was reconstructed for sessions Grok reports no
	// usage for. Shown, and not counted as confirmed.
	NoLedgerTokens int

	// Unreadable counts lines that did not parse, so a truncated tail is
	// reported rather than passed over.
	Unreadable int
	// Inconsistent counts turns whose parts did not add back to their prompt.
	Inconsistent int
	// WritesReported counts sessions that stated a cache-creation figure at
	// all, zero included.
	WritesReported int
	Models         map[string]int
}

// readGrok walks a Grok session root and returns one reading.
//
// A missing root is not an error: a machine that has never run Grok is an
// ordinary machine.
func readGrok(root string) (grokReading, error) {
	out := grokReading{Models: map[string]int{}}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}

	var dirs []string
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// A directory entry that cannot be walked is skipped rather than
			// failing the read: one unreadable session must not hide the 103
			// that are fine. It is counted, so the reader is told.
			out.Unreadable++
			return nil //nolint:nilerr // counted above, not swallowed
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base != "updates.jsonl" && base != "usage.json" {
			return nil
		}
		dir := filepath.Dir(path)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		s := readGrokSession(dir)
		if s == nil {
			continue
		}
		out.Sessions++
		out.Unreadable += s.Unreadable
		out.Inconsistent += s.Inconsistent
		if s.WroteCacheField {
			out.WritesReported++
		}
		addCounts(&out.Reconstructed, s.Reconstructed)
		switch s.reconcile() {
		case grokLedgerMatches:
			out.Matched++
			addCounts(&out.Vendor, *s.Vendor)
		case grokLedgerDiffers:
			out.Diverged++
			addCounts(&out.Vendor, *s.Vendor)
			out.DivergedTokens += abs(s.promptDelta())
		case grokLedgerUnavailable:
			out.NoLedger++
			out.NoLedgerTokens += s.Reconstructed.Prompt
		}
		for m := range s.Models {
			out.Models[m]++
		}
	}
	return out, nil
}

func addCounts(dst *grokCounts, src grokCounts) {
	dst.Prompt += src.Prompt
	dst.Fresh += src.Fresh
	dst.CachedRead += src.CachedRead
	dst.CachedWrite += src.CachedWrite
	dst.Output += src.Output
	dst.Reasoning += src.Reasoning
	dst.ModelCalls += src.ModelCalls
	dst.Turns += src.Turns
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// readGrokSession reads one session directory: the per-turn stream, and the
// vendor ledger beside it if there is one.
func readGrokSession(dir string) *grokSession {
	s := &grokSession{ID: filepath.Base(dir), Models: map[string]bool{}}

	turns, models, bad, inconsistent := readGrokUpdates(filepath.Join(dir, "updates.jsonl"))
	s.Unreadable += bad
	s.Inconsistent += inconsistent
	s.Reconstructed = turns.counts
	s.WroteCacheField = turns.wroteCacheField
	for m := range models {
		s.Models[m] = true
	}

	if led, ok := readGrokLedger(filepath.Join(dir, "usage.json")); ok {
		var session grokCounts
		if session.add(led.Session) {
			// The ledger states one turn count of its own; keep it rather than
			// the one add() derived from a single object.
			session.Turns = led.Session.TurnCount
			s.Vendor = &session
		}
		var turnSum grokCounts
		for _, t := range led.Turns {
			turnSum.add(t)
		}
		s.VendorTurnSum = turnSum
		if led.Session.CacheCreationTokens != nil {
			s.WroteCacheField = true
		}
		for m := range led.Session.ModelUsage {
			s.Models[m] = true
		}
	}

	if s.Reconstructed.Turns == 0 && s.Vendor == nil {
		return nil
	}
	return s
}

// grokUpdates is the reconstruction from one updates.jsonl.
type grokUpdates struct {
	counts          grokCounts
	wroteCacheField bool
}

// readGrokUpdates sums the per-turn usage records in one updates.jsonl.
//
// The records are one per turn, not a running total, and they are summed. The
// vendor's own output is the evidence: on 35 of 35 sessions carrying a ledger
// the turns sum to the session total, and on a 131-turn session the last turn
// is 0 while the session total is 222,856,819.
func readGrokUpdates(path string) (grokUpdates, map[string]bool, int, int) {
	var out grokUpdates
	models := map[string]bool{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, models, 0, 0
		}
		return out, models, 1, 0
	}
	defer func() { _ = f.Close() }()

	bad, inconsistent := 0, 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Unreadable means the JSON did not parse. A line that parses and
		// simply carries no usage is an ordinary message or tool record, and
		// counting those reported 33,399 "unreadable" lines against a healthy
		// log on the first run, which would bury a real truncation in noise.
		var probe json.RawMessage
		if json.Unmarshal([]byte(line), &probe) != nil {
			bad++
			continue
		}
		u, ok := grokUsageIn([]byte(line))
		if !ok {
			continue
		}
		if u.CacheCreationTokens != nil {
			out.wroteCacheField = true
		}
		for m := range u.ModelUsage {
			models[m] = true
		}
		if !out.counts.add(u) {
			inconsistent++
		}
	}
	if err := sc.Err(); err != nil {
		bad++
	}
	return out, models, bad, inconsistent
}

// readGrokLedger reads usage.json, the record `grok usage` reads.
//
// A missing file is reported as missing, never as a ledger of zero.
func readGrokLedger(path string) (grokLedger, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return grokLedger{}, false
	}
	var led grokLedger
	if json.Unmarshal(b, &led) != nil {
		return grokLedger{}, false
	}
	return led, true
}

// grokUsageIn finds the turn's usage object in one updates.jsonl line.
//
// The per-model copies carry identical field names, so the outer record is
// identified by holding modelUsage rather than by its own fields. The descent
// takes keys in sorted order so the answer does not depend on map iteration.
func grokUsageIn(line []byte) (grokUsage, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(line, &top) != nil {
		return grokUsage{}, false
	}
	var found grokUsage
	var ok bool
	var walk func(map[string]json.RawMessage)
	walk = func(m map[string]json.RawMessage) {
		if ok {
			return
		}
		if raw, has := m["modelUsage"]; has && len(raw) > 0 {
			var u grokUsage
			b, _ := json.Marshal(m)
			if json.Unmarshal(b, &u) == nil {
				found, ok = u, true
				return
			}
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var sub map[string]json.RawMessage
			if json.Unmarshal(m[k], &sub) == nil {
				walk(sub)
				if ok {
					return
				}
			}
		}
	}
	walk(top)
	return found, ok
}

func (r grokReading) render(w io.Writer) {
	_, _ = fmt.Fprintf(w, "\n  %d Grok session(s)\n\n", r.Sessions)

	_, _ = fmt.Fprintf(w, "  REPLAY RECONSTRUCTION  from updates.jsonl, summed per turn\n")
	_, _ = fmt.Fprintf(w, "    turns             %14s\n", comma(r.Reconstructed.Turns))
	_, _ = fmt.Fprintf(w, "    prompt            %14s\n", comma(r.Reconstructed.Prompt))
	_, _ = fmt.Fprintf(w, "      of which cached %14s\n", comma(r.Reconstructed.CachedRead))
	_, _ = fmt.Fprintf(w, "      fresh           %14s\n", comma(r.Reconstructed.Fresh))
	_, _ = fmt.Fprintf(w, "    cache writes      %14s  (stated by %d of %d session(s))\n",
		comma(r.Reconstructed.CachedWrite), r.WritesReported, r.Sessions)
	_, _ = fmt.Fprintf(w, "    output            %14s\n", comma(r.Reconstructed.Output))
	_, _ = fmt.Fprintf(w, "      of which reason %14s\n", comma(r.Reconstructed.Reasoning))
	_, _ = fmt.Fprintf(w, "    model calls       %14s\n", comma(r.Reconstructed.ModelCalls))

	_, _ = fmt.Fprintf(w, "\n  VENDOR LEDGER  from usage.json, what grok usage reads\n")
	if r.Matched+r.Diverged == 0 {
		_, _ = fmt.Fprintf(w, "    UNAVAILABLE       no session carries a usage.json. That is not a\n")
		_, _ = fmt.Fprintf(w, "                      vendor total of zero: Grok reports no usage for\n")
		_, _ = fmt.Fprintf(w, "                      these sessions at all.\n")
	} else {
		_, _ = fmt.Fprintf(w, "    over %d of %d session(s)\n", r.Matched+r.Diverged, r.Sessions)
		_, _ = fmt.Fprintf(w, "    prompt            %14s\n", comma(r.Vendor.Prompt))
		_, _ = fmt.Fprintf(w, "      of which cached %14s\n", comma(r.Vendor.CachedRead))
		_, _ = fmt.Fprintf(w, "    output            %14s\n", comma(r.Vendor.Output))
	}

	_, _ = fmt.Fprintf(w, "\n  RECONCILIATION  the two are compared, never added\n")
	_, _ = fmt.Fprintf(w, "    %-11s %4d session(s)\n", grokLedgerMatches, r.Matched)
	_, _ = fmt.Fprintf(w, "    %-11s %4d session(s)", grokLedgerDiffers, r.Diverged)
	if r.Diverged > 0 {
		_, _ = fmt.Fprintf(w, ", by %s prompt token(s)", comma(r.DivergedTokens))
	}
	_, _ = fmt.Fprintf(w, "\n")
	_, _ = fmt.Fprintf(w, "    %-11s %4d session(s)", grokLedgerUnavailable, r.NoLedger)
	if r.NoLedger > 0 {
		_, _ = fmt.Fprintf(w, ", holding %s reconstructed prompt token(s)", comma(r.NoLedgerTokens))
	}
	_, _ = fmt.Fprintf(w, "\n")
	if r.NoLedger > 0 {
		_, _ = fmt.Fprintf(w, "      Those token(s) are reconstructed, not confirmed. Grok reports no\n")
		_, _ = fmt.Fprintf(w, "      usage for a session with no usage.json, so they are absent from\n")
		_, _ = fmt.Fprintf(w, "      the vendor figure above rather than zero in it, and the two\n")
		_, _ = fmt.Fprintf(w, "      columns are never added together.\n")
	}

	if r.Inconsistent > 0 {
		_, _ = fmt.Fprintf(w, "\n  [NOTE] %s turn(s) whose parts did not add back to their prompt were\n", comma(r.Inconsistent))
		_, _ = fmt.Fprintf(w, "         excluded. Grok counts the cache inside the prompt; a record\n")
		_, _ = fmt.Fprintf(w, "         where it does not fit cannot be normalised.\n")
	}
	if r.Unreadable > 0 {
		_, _ = fmt.Fprintf(w, "\n  [NOTE] %d line(s) could not be parsed. Absent from every figure\n", r.Unreadable)
		_, _ = fmt.Fprintf(w, "         above, not zero in it.\n")
	}

	if len(r.Models) > 0 {
		names := make([]string, 0, len(r.Models))
		for m := range r.Models {
			names = append(names, m)
		}
		sort.Strings(names)
		_, _ = fmt.Fprintf(w, "\n  models\n")
		for _, m := range names {
			_, _ = fmt.Fprintf(w, "    %-24s %d session(s)\n", m, r.Models[m])
		}
	}

	_, _ = fmt.Fprintf(w, "\n  no money figure\n")
	_, _ = fmt.Fprintf(w, "    costUsdTicks is documented as 10^10 ticks per USD, but that scale is\n")
	_, _ = fmt.Fprintf(w, "    unchecked against an invoice. Documented and checked are different\n")
	_, _ = fmt.Fprintf(w, "    words, and the second one is what a figure on this screen would be\n")
	_, _ = fmt.Fprintf(w, "    claiming. The token counts above are what the records actually state.\n")
}

func runGrok(args []string, stdout, stderr io.Writer) error {
	root := filepath.Join(os.Getenv("HOME"), ".grok", "sessions")
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		root = args[0]
	}
	r, err := readGrok(root)
	if err != nil {
		return err
	}
	if r.Sessions == 0 {
		_, _ = fmt.Fprintf(stderr, "no Grok sessions under %s\n", root)
		return nil
	}
	r.render(stdout)
	return nil
}
