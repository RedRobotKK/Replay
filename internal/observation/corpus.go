package observation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EarlierSubmissions lists other corpus files in dir carrying the same source
// tag as c, newest name aside.
//
// It exists because a contributor who runs the tool weekly ends up with a
// directory of files that all look equally submittable, and only one of them
// is. A corpus is cumulative — the tool reads the whole transcript root on
// every run — so this week's submission contains last week's tasks. Sending
// both would have a pooler either double-count this machine or, if the pooler
// is this repository's own, silently discard one. Either way the contributor
// should be told before they attach anything to a pull request, not after.
//
// The scan is by filename prefix, which is enough because the tag is in the
// name and the name is derived, never typed. An unreadable directory returns
// no earlier submissions and no error: this is advice printed alongside a file
// that was already written successfully, and failing the command over it would
// turn a courtesy into an outage.
func EarlierSubmissions(dir, tag, exclude string) []string {
	// The error is discarded and the branch that returned on it is gone: it
	// could not be observed failing, because ranging over the nil slice a
	// failed ReadDir returns already yields no submissions. Two spellings of
	// the same behaviour, one of them untestable, is the shape ADR-0014 rules
	// out.
	entries, _ := os.ReadDir(dir)
	prefix := fmt.Sprintf("replay-corpus-%s-", safe(tag))
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == filepath.Base(exclude) {
			continue
		}
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".json") {
			found = append(found, name)
		}
	}
	return found
}

// CorpusSchema versions a corpus contribution independently of a probe
// observation, because they are different submissions with different bargains.
const CorpusSchema = "replay.corpus.v1"

// Corpus is one machine's aggregate spend, offered for pooling.
//
// It exists because a question the money path rests on cannot be answered from
// one vantage point. "Is 4.8% of spend re-billed by broken caches" is a claim
// about a population, and this repository has one machine. Publishing a
// population figure derived from a single corpus is precisely the shape of the
// 98.8%-to-4.2% correction and the 1,363-to-78 sample restatement it already
// published.
//
// It is a SEPARATE type from Observation on purpose, and the reason is worth
// stating because the alternative was one field edit away. Observation's own
// comment promises that no spend or request volume leaves in it, and OB1 now
// enforces that. A corpus contribution breaks no promise because it makes a
// different one: the contributor is knowingly sending aggregate money, having
// read it first. Folding these five figures into the struct that says it
// carries none would have made that comment false for everyone who had already
// believed it.
//
// Like every contribution here, nothing sends it. `replay contribute` writes a
// file and prints the path; a person moves it. internal/observation has no
// transport and a test that keeps it that way, so no future change to a consent
// gate can turn this into transmission — there is no transmitting code to
// reach.
type Corpus struct {
	Schema string `json:"schema"`
	// TakenAt is truncated to the hour, for the same reason Observation's is.
	TakenAt string `json:"takenAt"`

	// The five figures the money-path argument is built on. Tasks is the
	// denominator and travels with them, because a total without an n is a
	// number nobody can weight.
	Tasks          int     `json:"tasks"`
	TotalUSD       float64 `json:"totalUsd"`
	AvoidableUSD   float64 `json:"avoidableUsd"`
	AvoidableShare float64 `json:"avoidableShare"`
	MedianTaskUSD  float64 `json:"medianTaskUsd"`

	// What priced them. An aggregate of totals computed against different
	// price tables or caching rules is not an aggregate of anything, and the
	// reader of a pooled figure has no way to notice unless each submission
	// says which table it used.
	PricedAt     string `json:"pricedAt"`
	RulesVersion string `json:"rulesVersion"`
	// Unpriced is how many transcripts were read and left out because their
	// model is not in the table. Excluded rather than counted as free, and
	// reported so a pooled figure can say what it does not cover.
	Unpriced int `json:"unpriced"`

	// The waste distribution (ADR-0009). Content-free by construction: a ratio
	// and two counts, with no path from any of them back to code, prompts or a
	// project.
	//
	// These are what make a contribution worth making to the contributor rather
	// than only to the maintainer. The money figures above answer "how much did
	// this cost", which is a fact about the tool once it is pooled; a
	// distribution answers "is my error share normal", which is a fact about
	// the contributor's afternoon and is the reason to run the command twice.
	//
	// Pointers, because ADR-0018: a build that did not measure an error share
	// and a corpus whose error share is genuinely zero are different states,
	// and a pooled figure that cannot tell them apart reads an absence as a
	// finding. Absent when nil, which is also what keeps every submission
	// written before these existed poolable — Add recomputes the digest, so a
	// field that serialised when absent would invalidate all of them.
	CacheBreaks *int     `json:"cacheBreaks,omitempty"`
	ReReads     *int     `json:"reReads,omitempty"`
	ErrorShare  *float64 `json:"errorShare,omitempty"`

	SourceTag string `json:"sourceTag"`
	TagBasis  string `json:"tagBasis"`

	// Which binary did the arithmetic (#284).
	//
	// RulesVersion above names the provider's published rule document. It is
	// not enough, and 2026-09-12 is how we found out: two builds read the same
	// directory on the same machine and reported $4,088.49 and $11,969.37,
	// both stamped "anthropic-2026-09-01". The label was honest. The provider
	// had changed nothing; we had. Six models one build declined to price
	// became priced, and the unknown-model read multiple became a named rule
	// and moved.
	//
	// A pool that adds two such submissions is summing different arithmetic
	// under one name. PricingDigest is computed from the price table, the
	// caching floors and the unknown-model fallback actually compiled into
	// this binary, so it moves when any of them does, whatever the provider's
	// label says. BinaryVersion and Commit say which build, so a reader can go
	// and look at it.
	//
	// ALL THREE ARE OPTIONAL AND THAT IS DELIBERATE. Digested marshals this
	// struct, so a field that serialised when absent would change the digest of
	// every submission written before today and orphan every roster entry that
	// names one. Empty means a build from before these existed, which is a fact
	// a pool can act on rather than a gap it has to guess at (ADR-0018). It is
	// also why the schema string does not move: this is additive, exactly as
	// CacheBreaks, ReReads and ErrorShare were.
	//
	// These are strings, and they are the first strings in this payload that
	// are not a date, a schema or a tag the contributor chose. They carry no
	// path, no project and no content: a semantic version, a short hex SHA of
	// a public commit, and a hex digest of numbers that ship in every copy of
	// the binary. Anyone can compute the third from a release they downloaded.
	BinaryVersion string `json:"binaryVersion,omitempty"`
	Commit        string `json:"commit,omitempty"`
	PricingDigest string `json:"pricingDigest,omitempty"`

	// Digest names this submission by its content, so a pooled figure can list
	// what it is made of and a reader can check that the file they downloaded
	// is the one that was counted.
	//
	// "$847,000 of agent spend audited" is unfalsifiable. "$847,000 across 41
	// contributed corpora, each one downloadable" is a claim someone can walk
	// back to its parts, and this field is what makes the walk possible. It is
	// computed over the payload with this field empty, so it can be recomputed
	// from the file as published.
	Digest string `json:"digest"`
}

// Digested returns the corpus with its content digest filled in.
//
// The digest covers every other field, which is what lets a roster entry stand
// for a submission: change a figure and the name changes with it, so a pool
// that lists digests cannot be quietly restated after publication.
func (c Corpus) Digested() Corpus {
	c.Digest = ""
	// The error is discarded, and the branch that handled it is deliberately
	// gone. Corpus is flat scalars — no channels, no funcs, no cycles — so
	// encoding/json cannot fail on it, which made that branch unreachable and
	// therefore untestable. ADR-0014's rule is that a check must be able to
	// fail; a check that cannot is not a safeguard but an unexercised path
	// that will be wrong whenever it is finally taken. Validate is the real
	// guard: a Corpus whose digest did not compute has an empty Digest, and
	// Validate refuses that.
	body, _ := json.Marshal(c)
	sum := sha256.Sum256(body)
	c.Digest = hex.EncodeToString(sum[:])
	return c
}

// Validate refuses a contribution that would add a measured zero to a pool.
//
// "Nothing measured is not a pass" is the rule this repository applies to every
// other figure, and it applies hardest here: an empty corpus aggregated into a
// population total moves the denominator without moving the numerator, which
// biases the pooled share downward for everyone else in it.
func (c Corpus) Validate() error {
	switch {
	case c.Schema != CorpusSchema:
		return fmt.Errorf("schema is %q, want %q", c.Schema, CorpusSchema)
	case c.Digest == "":
		return fmt.Errorf("the submission has no content digest, so a pooled figure could " +
			"not name it or let a reader check it")
	case c.Tasks <= 0:
		return fmt.Errorf("NOT MEASURED: the corpus priced no tasks, so there is nothing here " +
			"to pool; aggregating it would move a population denominator without moving its " +
			"numerator")
	case c.TotalUSD <= 0:
		return fmt.Errorf("NOT MEASURED: %d task(s) priced to $0.00 in total", c.Tasks)
	case c.PricedAt == "" || c.RulesVersion == "":
		return fmt.Errorf("a figure without the price table and caching rules that produced it " +
			"cannot be pooled with figures from another table")
	case c.SourceTag == "" || c.TagBasis == "":
		return fmt.Errorf("a contribution needs its source tag and the basis of that tag")
	}
	return nil
}

// corpusFileName names a corpus submission distinctly from a probe one.
//
// The two carry different things and accept different bargains, so they must
// not be mistaken for each other in a directory, a pull request, or an
// aggregator's inbox.
func corpusFileName(c Corpus) string {
	// The digest is in the name so two readings from the same machine are two
	// submissions rather than one overwriting the other, and so a roster entry
	// names a file a reader can find.
	d := c.Digest
	if len(d) > 12 {
		d = d[:12]
	}
	return fmt.Sprintf("replay-corpus-%s-%s.json", safe(c.SourceTag), safe(d))
}

// WriteCorpus writes the payload where the contributor can read it, and returns
// the path so the caller can print it.
//
// The same two refusals as WriteObservation, for the same reason: this is the
// artifact a human is asked to inspect before sending, so silently replacing
// one, or writing through a link to somewhere else, defeats the only review
// step in the design.
//
// Validate runs first, so an empty corpus never reaches disk. A file that
// exists is a file somebody might send, and "nothing measured is not a pass"
// has to be enforced before the artifact exists rather than after.
func WriteCorpus(dir string, c Corpus) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, corpusFileName(c))
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s is a symlink; refusing to write a submission through a redirected path", path)
		}
		return "", fmt.Errorf("%s already exists; move or delete it rather than replacing a submission that has not been sent", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		// "I could not look" is not "there is nothing here". Collapsing them
		// would write into a path nobody has established is free, so this
		// names itself rather than deferring to whatever the write says next
		// — the two are distinguishable only by the message.
		return "", fmt.Errorf("cannot inspect %s, so whether a submission is already there "+
			"is unknown and it will not be overwritten: %w", path, err)
	}
	// Discarded for the reason Digested gives: Corpus cannot fail to marshal,
	// so the branch was unreachable and could never be observed failing.
	body, _ := json.MarshalIndent(c, "", "  ")
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
