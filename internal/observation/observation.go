// Package observation builds the file a contributor may choose to submit.
//
// It builds a file. It does not send one, and it cannot: the import allowlist
// in this package's tests refuses every transport. That is a property of THIS
// PACKAGE, and the distinction matters — the binary as a whole is a proxy and
// also originates billable requests under `probe --execute`, so a claim that
// the binary never reaches the network would be false. What is true and worth
// having is narrower: contribution cannot become a submission by accident,
// because there is no code here that could send one. That is a stronger promise than any consent flow,
// because it does not depend on reasoning about a gate. A human reads the file
// and moves it — a pull request, an issue, an email.
//
// The reason to collect these at all is that a floor measured on one account,
// in one region, at one hour is one account's floor. Whether there is a single
// global floor is the question worth asking, and it is unanswerable from here.
// Several vantage points can answer it; one cannot.
//
// What may leave this machine is the struct below and nothing else. It is
// built field by field from a stored reading rather than by embedding one, so
// a field added to Reading later does not silently become a disclosure — and
// the field list is asserted, so adding one here takes a deliberate edit to a
// test that says why the list is short.
package observation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/consent"
	"github.com/RedRobotKK/Replay/internal/probe"
)

// Schema identifies the payload shape so an aggregator can refuse one it does
// not understand rather than reading it as something it is not.
const Schema = "replay.observation.v1"

// How a contributor tag was derived. The two are not interchangeable and the
// aggregator must weight them differently, so the payload carries which.
const (
	// BasisAccount is a tag derived from the provider's own account
	// identifier. Two submissions carrying the same one are the same account.
	BasisAccount = "account"
	// BasisLocal is a tag derived from a secret this machine generated.
	// It deduplicates and nothing more: anyone can mint unlimited local
	// secrets, so a count of distinct local tags is not a count of people.
	BasisLocal = "local"
)

// Observation is the complete set of fields that may leave this machine.
//
// Everything here is a number, an enumerated string, or a keyed digest.
// Nothing derived from prompts, responses, file paths, project names, session
// ids, spend, request volume, or any credential appears, and none of it is
// reachable from the reading this is built out of.
type Observation struct {
	Schema   string `json:"schema"`
	Campaign string `json:"campaign"`
	Method   string `json:"method"`

	// TakenAt is truncated to the hour. Second resolution would be a
	// correlation handle against the provider's own request logs and buys
	// nothing: a campaign window is hours wide.
	TakenAt string `json:"takenAt"`

	Model       string `json:"model"`
	AnsweredBy  string `json:"answeredBy,omitempty"`
	ServiceTier string `json:"serviceTier,omitempty"`
	Geo         string `json:"geo,omitempty"`

	Above      int `json:"above,omitempty"`
	AtMost     int `json:"atMost,omitempty"`
	Documented int `json:"documented,omitempty"`

	Outcome string `json:"outcome,omitempty"`
	// Anomalies travel with the reading because a bare label is not something
	// anyone can learn from, and because a submission that reports only its
	// clean runs is publication bias with extra steps. Every field on an
	// Anomaly is a number or a compile-time kind; none is provider text.
	Anomalies    []probe.Anomaly `json:"anomalies,omitempty"`
	Inconclusive int             `json:"inconclusive,omitempty"`

	Probes  int `json:"probes,omitempty"`
	Confirm int `json:"confirm,omitempty"`

	SourceTag string `json:"sourceTag"`
	TagBasis  string `json:"tagBasis"`
}

// Tag identifies a contributor within one campaign and nowhere else.
type Tag struct {
	Value string
	Basis string
}

// AccountTag derives a tag from a provider account identifier.
func AccountTag(campaign, account string) (Tag, error) {
	v, err := derive(campaign, account)
	return Tag{Value: v, Basis: BasisAccount}, err
}

// LocalTag derives a tag from a secret this machine generated.
func LocalTag(campaign, secret string) (Tag, error) {
	v, err := derive(campaign, secret)
	return Tag{Value: v, Basis: BasisLocal}, err
}

// derive keys the digest on the campaign, so a tag is scoped to one campaign
// and two campaigns' tags for one identity cannot be linked. That is what
// keeps this from becoming a persistent installation id.
//
// The campaign salt is public, so someone holding a candidate list of account
// identifiers can test membership against a tag. Account identifiers are
// high-entropy and not public, which makes that an oracle for someone who
// already knows their target rather than a way to learn who contributed. It is
// a real residual risk and it belongs in the docs, not in one maintainer's
// head.
func derive(campaign, identity string) (string, error) {
	if campaign == "" {
		return "", errors.New("a campaign is required: it is the salt, and without one every campaign's tags would be linkable")
	}
	if identity == "" {
		return "", errors.New("an identity is required: an empty one would give every contributor the same tag")
	}
	m := hmac.New(sha256.New, []byte(campaign))
	m.Write([]byte(identity))
	// Half the digest. Enough that collisions are not a practical concern at
	// any plausible number of contributors, short enough to read aloud.
	return hex.EncodeToString(m.Sum(nil))[:16], nil
}

// Build turns a recorded reading into a submission payload.
//
// Consent is a parameter rather than something the caller is trusted to have
// checked, because a gate that lives only in a command is a gate the next
// command forgets. Unset and Declined are both refusals and the caller can
// still tell them apart: somebody who was never asked should be asked, and
// somebody who declined should be left alone.
func Build(campaign string, d consent.Decision, tag Tag, r probe.Reading) (Observation, error) {
	if !d.Allowed() {
		return Observation{}, fmt.Errorf("corpus contribution is %s; nothing was built. "+
			"To contribute, write `corpus_opt_in = true` in %s", d.State, d.Path)
	}
	if campaign == "" {
		return Observation{}, errors.New("a campaign is required")
	}
	if tag.Value == "" {
		return Observation{}, errors.New("a contributor tag is required; an empty one merges every contributor into a single row")
	}
	if tag.Basis != BasisAccount && tag.Basis != BasisLocal {
		return Observation{}, fmt.Errorf("unrecognised tag basis %q; the aggregator weights %q and %q differently and must not guess",
			tag.Basis, BasisAccount, BasisLocal)
	}
	taken, err := truncateToHour(r.TakenAt)
	if err != nil {
		return Observation{}, err
	}
	// Field by field, deliberately. Copying the reading wholesale would mean a
	// field added to Reading tomorrow leaves this machine without anyone
	// deciding that it should.
	return Observation{
		Schema:       Schema,
		Campaign:     campaign,
		Method:       r.Method,
		TakenAt:      taken,
		Model:        r.Model,
		AnsweredBy:   r.AnsweredBy,
		ServiceTier:  r.ServiceTier,
		Geo:          r.Geo,
		Above:        r.Above,
		AtMost:       r.AtMost,
		Documented:   r.Documented,
		Outcome:      r.Outcome,
		Anomalies:    r.Anomalies,
		Inconclusive: r.Inconclusive,
		Probes:       r.Probes,
		Confirm:      r.Confirm,
		SourceTag:    tag.Value,
		TagBasis:     tag.Basis,
	}, nil
}

// truncateToHour drops minutes and below, in UTC.
func truncateToHour(ts string) (string, error) {
	if ts == "" {
		return time.Now().UTC().Truncate(time.Hour).Format(time.RFC3339), nil
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "", fmt.Errorf("the reading's timestamp %q does not parse: %w", ts, err)
	}
	return t.UTC().Truncate(time.Hour).Format(time.RFC3339), nil
}

// observationFileName names the file after the campaign and the contributor,
// so several submissions can sit in one directory without colliding and a
// person can see at a glance which is which.
func observationFileName(o Observation) string {
	return fmt.Sprintf("replay-observation-%s-%s.json", safe(o.Campaign), safe(o.SourceTag))
}

// safe reduces a name to characters that cannot escape a directory or confuse
// a shell. Campaign ids come from a file the user was given, which is not the
// same as one they wrote.
func safe(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 64 {
		out = out[:64]
	}
	if out == "" {
		out = "unnamed"
	}
	return out
}

// WriteObservation writes the payload where the contributor can read it, and
// returns the path so the caller can print it.
//
// It refuses to overwrite and refuses to follow a symlink. Both matter more
// here than they would for a cache file: this is the artifact a human is asked
// to inspect before sending, so silently replacing one, or writing through a
// link to somewhere else, defeats the only review step in the design.
func WriteObservation(dir string, o Observation) (string, error) {
	path := filepath.Join(dir, observationFileName(o))
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s is a symlink; refusing to write a submission through a redirected path", path)
		}
		return "", fmt.Errorf("%s already exists; move or delete it rather than replacing a submission that has not been sent", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	body, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return "", err
	}
	return path, f.Sync()
}
