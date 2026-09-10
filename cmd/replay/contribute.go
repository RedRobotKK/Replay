package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/consent"
	"github.com/RedRobotKK/Replay/internal/money"
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

// corpusFigures is what a contribution is made of: the five figures the money
// argument rests on, and what priced them.
//
// It is a struct rather than eight parameters because the failure this whole
// repository keeps finding is a value standing in for a different value it is
// not, and four consecutive float64s in a call signature is that defect waiting
// for a refactor.
type corpusFigures struct {
	Tasks          int
	Unpriced       int
	TotalUSD       float64
	AvoidableUSD   float64
	AvoidableShare float64
	MedianTaskUSD  float64
}

// contributeCorpus builds a corpus submission from the figures the cost report
// just computed, writes it, and returns the path.
//
// It takes the summary rather than recomputing it. A contribution path with its
// own arithmetic would be a second implementation of the money argument, free
// to disagree with the one on the contributor's screen, and the disagreement
// would surface as a pooled figure nobody could reproduce from their own
// terminal.
//
// The probe path's statement ends "no prompts, no paths, no spend". This
// payload breaks that sentence deliberately — aggregate money is the whole
// point — so it gets its own flag, its own writer, its own file name and its
// own statement. Reusing the probe's would have made one true sentence false
// for everyone who had already read it.
// It returns the path written and any earlier submissions from this machine
// that it supersedes — see EarlierSubmissions for why the contributor has to be
// told about those before they attach anything anywhere.
func contributeCorpus(campaign, dir string, f corpusFigures, now time.Time) (string, []string, error) {
	// Opt-in only, and every one of these three branches is a refusal.
	//
	// consent.readDecision returns Granted for exactly one thing: a file
	// containing the line `corpus_opt_in = true`. A missing file is Unset, an
	// explicit false is Declined, and a file that is a symlink, writable by
	// anyone else, self-contradictory, empty of decisions, or holding a line it
	// cannot parse is an ERROR — never a yes. Nothing below runs until one of
	// those has passed, so there is no arrangement of the filesystem that
	// builds a submission the user did not ask for.
	decision, err := readCorpusConsent()
	if err != nil {
		return "", nil, fmt.Errorf("the corpus consent file cannot be read, so it is not a decision: %w", err)
	}
	if decision.ShouldAsk() {
		return "", nil, fmt.Errorf("contributing is off until you turn it on. Write `corpus_opt_in = true` in %s, "+
			"or run the installer with --corpus-opt-in. Nothing was built: %w", decision.Path, errUsage)
	}
	if !decision.Allowed() {
		return "", nil, fmt.Errorf("corpus contribution is declined in %s. Nothing was built: %w", decision.Path, errUsage)
	}

	secret, err := contributorSecret()
	if err != nil {
		return "", nil, err
	}
	tag, err := observation.LocalTag(campaign, secret)
	if err != nil {
		return "", nil, err
	}
	c := observation.Corpus{
		Schema: observation.CorpusSchema,
		// Truncated to the hour, as Observation's is: the hour is enough to
		// order submissions and to say which price table was current, and a
		// minute is closer to a keystroke timestamp than to a measurement.
		TakenAt:        now.UTC().Truncate(time.Hour).Format(time.RFC3339),
		Tasks:          f.Tasks,
		TotalUSD:       f.TotalUSD,
		AvoidableUSD:   f.AvoidableUSD,
		AvoidableShare: f.AvoidableShare,
		MedianTaskUSD:  f.MedianTaskUSD,
		PricedAt:       money.RatesDate,
		RulesVersion:   cachemodel.RulesVersion,
		Unpriced:       f.Unpriced,
		SourceTag:      tag.Value,
		TagBasis:       tag.Basis,
	}.Digested()
	if dir == "" {
		dir = "."
	}
	path, err := observation.WriteCorpus(dir, c)
	if err != nil {
		return "", nil, err
	}
	// Earlier submissions from this machine are reported, never deleted. The
	// contributor may have kept one deliberately, and a tool that tidied up
	// somebody's evidence directory on their behalf would be doing the one
	// thing this whole path exists to avoid.
	return path, observation.EarlierSubmissions(dir, tag.Value, path), nil
}

// corpusContributionNote is what the contributor is told, and the first
// sentence is the whole reason this path is separate from the probe's.
func corpusContributionNote(path string, supersedes []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nwrote %s\n", path)
	b.WriteString("This one carries SPEND: the total, the avoidable total and share, the median\n" +
		"task, the task count, and which price table produced them. No prompts, no\n" +
		"paths, no project or session names, and no per-task rows — the five figures\n" +
		"and their basis, nothing else.\n" +
		"Nothing was sent. Read it, and if you are happy with it, attach it to a pull\n" +
		"request against the campaign's corpus file. Its digest is in the filename, so\n" +
		"a pooled figure can name this submission and a reader can check it.\n")
	if len(supersedes) > 0 {
		// Said plainly, because the failure mode is a contributor helpfully
		// attaching all of them. A corpus is cumulative, so the older files
		// are contained in this one; a pooler that summed them would count
		// this machine's money more than once, and the avoidable share would
		// barely move while it happened.
		fmt.Fprintf(&b, "\nThis SUPERSEDES %d earlier submission(s) from this machine:\n", len(supersedes))
		for _, name := range supersedes {
			fmt.Fprintf(&b, "  %s\n", name)
		}
		b.WriteString("Send only the newest. Each run reads your whole transcript root, so this\n" +
			"file already contains everything the earlier ones did — sending both would\n" +
			"have your spend counted twice in the pooled total. They have been left on\n" +
			"disk; nothing here deletes your files.\n")
	}
	return b.String()
}
