package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/RedRobotKK/Replay/internal/consent"
	"github.com/RedRobotKK/Replay/internal/observation"
	"github.com/RedRobotKK/Replay/internal/probe"
)

// Contribution: build a submission, print where it is, send nothing.
//
// A floor measured on one account, in one region, at one hour is one account's
// floor. Whether there is a single global floor is the question worth asking
// and it cannot be answered from here — several vantage points can answer it,
// one cannot. So there is a way to hand a reading to somebody aggregating
// them.
//
// The binary does not send it. It writes a file and prints the path, and a
// person moves it. That is not squeamishness: it means the contributor has
// seen the payload before it leaves, and it means no future change to a
// consent gate can turn contribution into transmission, because there is no
// transmitting code to reach. internal/observation has no transport and a test
// that keeps it that way.
//
// Scope, stated because the README got this wrong for a while: that is a claim
// about the CONTRIBUTION PATH, not about the binary. The proxy reaches the
// network by definition, and `probe --execute` originates billable requests on
// the operator's own key.

// contributorSecretName holds the machine-local secret the contributor tag is
// derived from. Owner-only: anyone who can read it can compute this machine's
// tag for any campaign.
const contributorSecretName = "contributor-secret"

// contribute builds a submission file from the most recent recorded reading.
func contribute(campaign, dir, model, seriesPath string, stdout io.Writer) error {
	decision, err := readCorpusConsent()
	// An unreadable consent file is not a decision. It must stop the build
	// rather than quietly becoming a yes or a no, which is the whole reason
	// the consent package refuses symlinks and shared-writable files instead
	// of best-guessing them.
	if err != nil {
		return fmt.Errorf("the corpus consent file cannot be read, so it is not a decision: %w", err)
	}
	if decision.ShouldAsk() {
		return fmt.Errorf("contributing is off until you turn it on. Write `corpus_opt_in = true` in %s, "+
			"or run the installer with --corpus-opt-in. Nothing was built: %w", decision.Path, errUsage)
	}
	if !decision.Allowed() {
		return fmt.Errorf("corpus contribution is declined in %s. Nothing was built: %w", decision.Path, errUsage)
	}

	reading, ok := latestReading(seriesPath, model)
	if !ok {
		return fmt.Errorf("no recorded reading for %s to contribute. Measure one first: "+
			"replay probe --model %s --execute: %w", model, model, errUsage)
	}

	secret, err := contributorSecret()
	if err != nil {
		return err
	}
	// Local, not account: this secret was minted here, so it deduplicates and
	// nothing more. Anyone can mint unlimited ones, so a count of distinct
	// local tags is not a count of people, and the payload says so rather than
	// leaving an aggregator to assume otherwise.
	tag, err := observation.LocalTag(campaign, secret)
	if err != nil {
		return err
	}
	obs, err := observation.Build(campaign, decision, tag, reading)
	if err != nil {
		return err
	}
	if dir == "" {
		dir = "."
	}
	path, err := observation.WriteObservation(dir, obs)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "\nwrote %s\n", path)
	_, _ = fmt.Fprintf(stdout, "Nothing was sent. Read it, and if you are happy with it, attach it to a\n"+
		"pull request against the campaign's observations file. It carries the bracket,\n"+
		"the method, the provenance and a per-campaign tag — no prompts, no paths, no\n"+
		"spend, and nothing that links this submission to another campaign's.\n")
	return nil
}

// readCorpusConsent resolves the config directory the installer writes to and
// asks the consent package what the user said.
func readCorpusConsent() (consent.Decision, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return consent.Decision{}, err
		}
		dir = filepath.Join(home, ".config")
	}
	return consent.ReadCorpusConsent(dir)
}

// latestReading returns the most recent reading recorded for a model.
func latestReading(path, model string) (probe.Reading, bool) {
	readings, err := probe.LoadSeries(path)
	if err != nil {
		return probe.Reading{}, false
	}
	for i := len(readings) - 1; i >= 0; i-- {
		if readings[i].Model == model {
			return readings[i], true
		}
	}
	return probe.Reading{}, false
}

// contributorSecret reads the machine-local secret, generating it on first
// use. It must be stable: a tag that changed every run would make one
// contributor look like many, which destroys the only thing the tag is for.
func contributorSecret() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".replay")
	path := filepath.Join(dir, contributorSecretName)

	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s is a symlink; refusing to take this machine's identity from a redirected path", path)
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return "", rerr
		}
		if s := string(body); len(s) >= 32 {
			return s, nil
		}
		return "", fmt.Errorf("%s is too short to be the secret it should hold; delete it and it will be regenerated", path)
	case !errors.Is(err, os.ErrNotExist):
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		return "", err
	}
	return secret, nil
}
