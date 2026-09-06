package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// How often the ask may appear.
//
// Reciprocity is the strongest lever available and the most fragile. It works
// because the tool gave something concrete first; it stops working the moment
// the ask reads as a toll. Replay asked on every qualifying run, so somebody
// running `replay cost` daily met the same request thirty times a month. By the
// fifth it is not a thank-you, it is nagging - which converts worse AND spends
// the goodwill that made the first one work.
//
// The highest-leverage change for conversion here is restraint.
const (
	// One month between asks. Long enough that the ask is an event rather than
	// furniture, short enough that a monthly user is thanked about as often as
	// they are helped.
	tipCooldown = 30 * 24 * time.Hour

	// A finding this many times larger than the one that last prompted an ask
	// is new value rather than the same finding growing, and re-opens the ask
	// inside the cooldown. Set high on purpose: a loophole that any increase
	// could open is the nag wearing a different hat.
	tipReAskMultiple = 3.0

	tipStateFile = "tip.json"
)

type tipState struct {
	AskedAt   time.Time `json:"askedAt"`
	AskedAtUS float64   `json:"askedAtUsd"`
	// Seed assigns this machine to an experiment arm. Random and local: a
	// hostname or a hardware id would be an identifier, and this file is
	// only ever read by the machine that wrote it.
	Seed string `json:"seed,omitempty"`
}

// tipSeed returns this machine's stable experiment seed, creating one on first
// use. A random local value rather than any machine property, so the arm is
// stable without anything identifying being derived or stored.
func tipSeed(dir string) string {
	if b, err := os.ReadFile(tipStatePath(dir)); err == nil {
		var st tipState
		if json.Unmarshal(b, &st) == nil && st.Seed != "" {
			return st.Seed
		}
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "fallback"
	}
	seed := hex.EncodeToString(raw[:])
	var st tipState
	if b, err := os.ReadFile(tipStatePath(dir)); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	st.Seed = seed
	writeTipState(dir, st)
	return seed
}

func writeTipState(dir string, st tipState) {
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(dir, 0o700)
	tmp := tipStatePath(dir) + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, tipStatePath(dir))
	}
}

func tipStatePath(dir string) string { return filepath.Join(dir, tipStateFile) }

// shouldAsk reports whether the ask may be shown now.
//
// With no usable state it returns true: a reader the tool cannot remember is
// better served by one ask than by silence, and failing the other way would
// silently disable the only revenue path on any machine with a read-only home.
func shouldAsk(dir string, foundUSD float64, now time.Time) bool {
	b, err := os.ReadFile(tipStatePath(dir))
	if err != nil {
		return true
	}
	var st tipState
	if json.Unmarshal(b, &st) != nil || st.AskedAt.IsZero() {
		return true
	}
	if now.Sub(st.AskedAt) >= tipCooldown {
		return true
	}
	// Materially more found since the last ask is fresh reciprocity.
	return st.AskedAtUS > 0 && foundUSD >= st.AskedAtUS*tipReAskMultiple
}

// noteAsked records that the ask was shown. Failure to persist is ignored: the
// consequence is one extra ask, which is a far smaller harm than refusing to
// run because a state file could not be written.
func noteAsked(dir string, foundUSD float64, now time.Time) {
	var st tipState
	if b, err := os.ReadFile(tipStatePath(dir)); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	st.AskedAt, st.AskedAtUS = now, foundUSD
	writeTipState(dir, st)
}
