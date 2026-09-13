package observation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/RedRobotKK/Replay/internal/consent"
)

// WatchSchema versions a Watch record separately from the corpus and the probe.
//
// Three schemas rather than one because they are three different bargains, and
// a reader who accepts one has not accepted the others. Merging them would make
// a version bump on any of them a renegotiation of all three.
const WatchSchema = "replay.watch.v1"

// Watch is one session against one repository, counts only.
//
// Replay reads one machine once. Watch is the standing version: the record is
// produced where the session happened and carried to whoever owns the invoice,
// so that "spend on this repository went up and nobody can say which week or
// which change" has an answer.
//
// THE BOUNDARY IS THE PRODUCT. Counts, ratios, cause classes and hour-resolution
// timestamps travel. Content does not: no prompt, no path, no tool name, no file
// name, no session id. The bridge between the two is one local command named at
// the end of a report, and that command runs on the operator's machine against
// data that never left it.
//
// This package still cannot send. TestO7_ThisPackageCannotSend walks these
// imports and there is no transport among them. A record is a value and a file,
// and something outside this package posts it. That separation is what lets the
// binary keep saying it makes no network call while a plugin the user configured
// makes one.
type Watch struct {
	Schema string `json:"schema"`

	// Repo is the unit the invoice is argued about, and it is the one string
	// here a person typed. It is a key the operator chooses, not a path the
	// tool discovered: nothing walks a filesystem to fill this in, because a
	// discovered value is a directory name and a directory name is content.
	Repo string `json:"repo"`

	// SessionTag de-duplicates re-reads and identifies nobody. See
	// WatchSessionTag for why it is keyed rather than hashed.
	SessionTag string `json:"sessionTag"`

	// Hour resolution, for the reason the contribution-payload evidence gives:
	// a stable tag over repeated submissions is a spend time series, and a
	// minute is closer to a keystroke than to a measurement. An hour orders the
	// week, which is the job, and does not say when somebody was at their desk.
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`

	Provider string `json:"provider"`
	Model    string `json:"model"`
	Lanes    int    `json:"lanes"`
	Turns    int    `json:"turns"`

	TotalUSD        float64 `json:"totalUsd"`
	AvoidableUSD    float64 `json:"avoidableUsd"`
	AvoidableTokens int     `json:"avoidableTokens"`

	// Cause CLASSES, not causes. "toolChange" is a category this binary already
	// assigns; the tool that changed is not in the record and cannot be
	// recovered from it.
	BreaksByCause   map[string]int `json:"breaksByCause"`
	IdleGapsOverTTL int            `json:"idleGapsOverTtl"`
	Compactions     int            `json:"compactions"`

	// Which build did the arithmetic. Same reason as the corpus, and the reason
	// is measured: two builds priced one corpus at $4,088.49 and $11,969.37
	// under one rules label. A weekly timeline that silently mixes them reports
	// a step change in spend that was a step change in arithmetic, which is the
	// worst failure this product has available to it.
	//
	// These are REQUIRED here, unlike on Corpus, and the difference is the
	// point: Corpus has submissions in the wild from before those fields
	// existed and making them mandatory would orphan every one. No Watch record
	// has ever been written, so the stricter rule costs nothing and can be set
	// now. It cannot be set later, which is ADR-0024's whole argument.
	BinaryVersion string `json:"binaryVersion"`
	Commit        string `json:"commit"`
	PricingDigest string `json:"pricingDigest"`
	RulesVersion  string `json:"rulesVersion"`

	// Digest names this record by its content, computed with this field empty
	// so it can be recomputed from the record as published. A dashboard that
	// lists a week cannot restate one of its records afterwards.
	Digest string `json:"digest"`
}

// WatchSessionTag names a session to itself and to nobody else.
//
// It is keyed by the repository key rather than being a bare hash, and that is
// the whole design. A bare hash of a session id is stable everywhere it is
// computed, so two organisations that happened to receive records derived from
// the same session would hold a value that joins their data sets. Keying it
// means the tag for one session under two repository keys is two unrelated
// values, and neither side can discover that by looking.
//
// Truncated to 16 hex characters: enough that a collision inside one
// repository's week is not a practical concern, short enough that it reads as
// an opaque label rather than as something to try and reverse. It is not
// reversible in either length, and the length is not what makes it so.
func WatchSessionTag(repoKey, sessionID string) string {
	mac := hmac.New(sha256.New, []byte("replay.watch.sessiontag.v1\x00"+repoKey))
	mac.Write([]byte(sessionID))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

// Digested returns the record with its content digest filled in.
func (w Watch) Digested() Watch {
	w.Digest = ""
	// The error is discarded for the reason Corpus.Digested gives: this is flat
	// scalars and a map of ints, so encoding/json cannot fail on it, and a
	// branch that cannot be taken is not a safeguard. Validate is the real
	// guard, and it refuses an empty digest.
	body, _ := json.Marshal(w)
	sum := sha256.Sum256(body)
	w.Digest = hex.EncodeToString(sum[:])
	return w
}

// Validate refuses a record that would put a measured zero on a timeline.
//
// Nothing measured is not a pass (ADR-0018). A week of empty records draws a
// flat line, and a flat line reads as a quiet week rather than as an absent
// one. The difference matters most to exactly the customer this is for: the one
// asking why a number moved.
func (w Watch) Validate() error {
	if w.Schema != WatchSchema {
		return fmt.Errorf("watch record has schema %q, want %q", w.Schema, WatchSchema)
	}
	if w.Repo == "" {
		return errors.New("watch record names no repository, and the repository is the unit")
	}
	if w.SessionTag == "" {
		return errors.New("watch record has no session tag, so re-reading it would double count")
	}
	if w.Turns == 0 {
		return errors.New("watch record measured no turns. Aggregated into a week it draws a " +
			"flat line that reads as a quiet week rather than as an absent one")
	}
	for _, ts := range []struct{ name, val string }{{"startedAt", w.StartedAt}, {"endedAt", w.EndedAt}} {
		if ts.val == "" {
			return fmt.Errorf("watch record has no %s, so it cannot be placed on a timeline", ts.name)
		}
		if !strings.HasSuffix(ts.val, ":00:00Z") {
			return fmt.Errorf("watch record %s is %q, which is finer than the hour this record "+
				"is allowed to know", ts.name, ts.val)
		}
	}
	for _, f := range []struct{ name, val string }{
		{"binaryVersion", w.BinaryVersion},
		{"pricingDigest", w.PricingDigest},
		{"rulesVersion", w.RulesVersion},
	} {
		if f.val == "" {
			return fmt.Errorf("watch record has no %s. A timeline that mixes two builds shows "+
				"an arithmetic change as a spend change", f.name)
		}
	}
	if w.Digest == "" {
		return errors.New("watch record has no digest, so nothing names it and a published " +
			"figure could be restated after the fact")
	}
	return nil
}

// ErrWatchNotConsented is returned when a record is built on a machine that has
// not opted in.
//
// It is a distinct error rather than a silent empty record because the caller
// has to be able to tell "this machine said no" from "this session measured
// nothing", and those two produce the same absence at the far end.
var ErrWatchNotConsented = errors.New("watch is off on this machine")

// BuildWatch is the only supported way to produce a sendable Watch record, and
// it refuses unless this machine has opted in.
//
// The gate is here rather than at the sending edge on purpose. A check at the
// edge is a check somebody adds a second code path around; a record that cannot
// be constructed without a decision means the opt-in is the thing that makes
// the value exist. The emitter that eventually posts one has nothing to post
// until this returns.
//
// Unset is refused exactly as Declined is. Silence is not a grant anywhere in
// this repository, and this is the path where that rule earns its keep: Watch
// is the first thing here that a machine would send on a schedule with no
// person present for each send.
func BuildWatch(d consent.Decision, w Watch) (Watch, error) {
	if !d.Allowed() {
		return Watch{}, fmt.Errorf("%w (decision: %s)", ErrWatchNotConsented, d.State)
	}
	out := w.Digested()
	if err := out.Validate(); err != nil {
		return Watch{}, err
	}
	return out, nil
}
