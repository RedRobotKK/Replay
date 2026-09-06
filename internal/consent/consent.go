// Package consent records decisions the user has to make out loud.
//
// Replay's headline promise is that it makes no network call except to the
// provider you configured. Anything that would break that — checking whether a
// newer build or a newer rules document exists — is not a default, a
// heuristic, or something an agent may decide on a person's behalf. It is a
// decision the user makes once, in a file they can read, edit and delete.
//
// The antivirus comparison is apt and its limits are the point. AV vendors
// push definitions silently because the threat model justifies it: a stale
// signature file misses a real attack. Replay's does not. A wrong price table
// silently changes every dollar figure the tool prints, which is worse than a
// stale one that announces its age — and it already announces it, locally,
// with no network at all. So the update path is pull-on-request, never push,
// and it reports rather than applies.
package consent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the consent file, kept separate from any config file so that
// writing it can never truncate settings the user already had. The installer
// uses the same convention for corpus consent.
const FileName = "update-consent.toml"

// CorpusFileName is the corpus opt-in, written by `install.sh --corpus-opt-in`.
//
// The installer has written this since before any Go code read it, and the
// file says so itself: "nothing is sent: no command in this release transmits
// it." Reading it here does not change that. It grants permission to BUILD a
// submission and to show it to the person who asked; sending remains something
// only a human does, deliberately, having seen the payload.
const CorpusFileName = "corpus-consent.toml"

// State is what the user has said. Three states, not two.
//
// Unset and Declined both mean "do not check now", so a boolean would merge
// them — and a tool that merges them either nags someone who already said no,
// or never asks someone who was never asked. The distinction is the whole
// reason this is not a bool.
type State int

const (
	// Unset means nobody has decided. Ask; do not check.
	Unset State = iota
	// Granted means checking is permitted.
	Granted
	// Declined means checking is forbidden, and asking again is rude.
	Declined
)

func (s State) String() string {
	switch s {
	case Granted:
		return "granted"
	case Declined:
		return "declined"
	default:
		return "unset"
	}
}

// Decision is the answer plus where it came from, so a caller can tell a user
// which file to edit to change their mind.
type Decision struct {
	State State
	Path  string
	// OwnershipChecked reports whether this build could verify that nobody
	// but the owner can write the file. True on platforms with meaningful
	// permission bits, false on Windows, where the mode Go reports is
	// synthetic and an ACL check is not available. A caller that treats a
	// decision as verified must read this first; a false here means the
	// state was parsed but its provenance was not established.
	OwnershipChecked bool
}

// Allowed reports whether a network call may be made. It is the only gate:
// exactly one state opens it, and a test asserts that stays true.
func (d Decision) Allowed() bool { return d.State == Granted }

// ShouldAsk reports whether it is appropriate to put the question to the user.
// Only when they have never answered it.
func (d Decision) ShouldAsk() bool { return d.State == Unset }

// ReadUpdateConsent reads the decision from a config directory.
//
// A missing file is the normal case and returns Unset with no error. Anything
// present but not exactly an affirmative or a refusal is an error AND Unset:
// a file we cannot read confidently must never become permission, and the
// caller should be told rather than left to assume.
func ReadUpdateConsent(configDir string) (Decision, error) {
	return readDecision(filepath.Join(configDir, "replay", FileName), "update_checks")
}

// ReadCorpusConsent reads whether the user has opted in to contributing
// structural observations.
//
// Same rules as every other consent here: absence is undecided, only the exact
// affirmative grants, a refusal is remembered, and a file that is a symlink or
// writable by anyone else is refused rather than believed.
func ReadCorpusConsent(configDir string) (Decision, error) {
	return readDecision(filepath.Join(configDir, "replay", CorpusFileName), "corpus_opt_in")
}

func readDecision(path, key string) (Decision, error) {
	d := Decision{State: Unset, Path: path}

	// Lstat, not Stat: a symlink here means someone else chose where this
	// user's answer comes from, which is not the same as the user answering.
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return d, fmt.Errorf("%s is a symlink; refusing to take a consent decision from a redirected path", path)
	}
	// A file any process on the box can write is not this user's decision.
	// What "writable by anyone else" means is platform-specific, so the check
	// lives behind a build tag and reports whether it was able to run.
	checked, err := ownershipIsExclusive(info, path)
	if err != nil {
		return d, err
	}
	d.OwnershipChecked = checked

	body, err := os.ReadFile(path)
	if err != nil {
		return d, err
	}

	// Deliberately strict and tiny. This is not a TOML parser: it recognises
	// exactly two sentences and treats everything else as unreadable. A
	// permissive parser here is a parser that eventually says yes to
	// something the user did not write.
	var found *State
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var want State
		switch line {
		case key + " = true":
			want = Granted
		case key + " = false":
			want = Declined
		default:
			return d, fmt.Errorf("%s: cannot read %q; the file must contain exactly `%s = true` or `%s = false`", path, line, key, key)
		}
		if found != nil && *found != want {
			return d, fmt.Errorf("%s contradicts itself; delete it and decide once", path)
		}
		found = &want
	}
	if found == nil {
		return d, fmt.Errorf("%s exists but records no decision", path)
	}
	d.State = *found
	return d, nil
}
