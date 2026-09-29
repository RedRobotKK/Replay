# Contributing a measurement

**The bargain, in one sentence.** Replay measures how your agent sessions
actually consumed resources; you can optionally contribute the aggregate
measurement, without sending your prompts, responses, code or session contents,
to help build a public measurement corpus.

This page is the contract. It describes what the artifact contains, what it
cannot contain, and what is not yet true.

## How to contribute

```sh
# once, to turn it on. Absent means off.
echo 'corpus_opt_in = true' > ~/.config/replay/corpus-consent.toml

replay cost ~/.claude/projects --contribute <campaign>
```

The command prints the artifact, then the path it wrote it to, then the
sentence `Nothing was sent.` Read the artifact. If you are happy with it,
attach the file to a pull request against the campaign's corpus file.

**There is no upload endpoint, and that is the design.** The pull request is
the transport, GitHub is the identity, and CI is the server-side validator. A
binary that could post your measurements is a binary that could post something
else after the next refactor, and `internal/observation` is held to an import
allowlist so it cannot acquire one by accident.

## What the artifact contains

Nineteen aggregate fields at most: the task count, four money figures, the
price table date and rules version, the unpriced count, the waste distribution
(cache breaks, re-reads, error share), a source tag, and provenance (binary
version, commit, pricing digest, content digest).

That is the whole payload. It is printed to your terminal before you decide.

## What it cannot contain

Not "does not". **Cannot.** `Corpus` is an allowlist, and a submission carrying
any key the struct does not declare is refused on read, by name, before it can
reach a pool. That is enforced by `TestSB2` and `TestSB3` in
`internal/observation`, which check sixteen field names a real accident would
produce and one arbitrary key nobody would think to ban.

No prompts, responses, tool arguments, tool output, source code, repository
paths, filesystem paths, session names, project names, credentials, API keys or
environment values. No per-task rows.

## What is NOT claimed

This matters more than the list above, because the easy words here are the
dishonest ones.

- **Not anonymous.** `sourceTag` is a stable per-machine identifier by
  construction. It is what lets you prove a row is yours. Repeated submissions
  under one tag are a time series, and a spend trajectory is a correlate. The
  tag counts machines at most, never people: anyone can mint unlimited ones.
- **Not tamper-proof.** The digest lets a reader check that the file they
  downloaded is the one that was counted. It does not establish that the
  figures were honestly produced.
- **Not a leaderboard.** There is no ranking, no score, and no cross-provider
  comparison, because Replay cannot currently establish that two independent
  operators accomplished equivalent work. A ranking without that is won by
  doing less. See [the research closeout](RESEARCH-CLOSEOUT-2026-09-29.md).
- **Not yet a population.** At the time of writing the corpus has very few
  members. A distribution over a handful of submissions is not a norm, and
  nothing here will pretend otherwise.

## What you get today

A receipt: the artifact, its digest, and the path. If your submission is pooled,
a roster row naming it, which anyone can download and re-hash.

That is a modest benefit and it is stated modestly on purpose. The larger ones,
self-placement against a distribution, provider-specific cache norms, anomaly
detection, are **hypotheses that require a population that does not exist yet**.
They are not promises, and they will not be made into promises before the
population is real.

## What the corpus is for

Pooled figures that name their parts. "$847,000 of agent spend audited" is
unfalsifiable. "$847,000 across 41 contributed corpora, each one downloadable"
is a sentence a reader can walk back to its evidence. Every pooled total is a
method over the roster rather than a stored field, so a pool cannot state a
number its listed submissions do not sum to.
