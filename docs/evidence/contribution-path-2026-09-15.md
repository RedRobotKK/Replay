# One of the two advertised install paths cannot contribute

**Exercised end to end 2026-09-15, not inferred from reading code. A binary
from `go install ...@latest` produces `commit: unknown`, and the contribution
endpoint refuses it with HTTP 400 on production. A binary from `make build`
produces a real commit and is accepted with HTTP 201 against a local database.
The endpoint is correct; the defect is upstream of it.**

**What this measures:** whether a user following the README can actually get a
corpus submission into the pool, and where the first invariant breaks if not.

## Why it was run

The pool has one member. Every percentage this project publishes is therefore
a case study on one machine, and the contribution path had never been
exercised from end to end by anyone. Reading the code would not have settled
it: the question is whether an artefact produced by an advertised command
satisfies a contract enforced three systems away.

## The matrix

The environment column is not a footnote. Local acceptance and production
acceptance are different claims, and only one of them was ever observed.

| input | result | environment | establishes |
|---|---|---|---|
| `make build` valid v2 submission | **201 accepted** | **LOCAL ONLY** | the mechanical flow can succeed |
| `go install ...@latest` submission | **400 rejected** | **PRODUCTION** | an advertised path cannot contribute |
| `go install ...@latest` submission | 400 rejected | local | same rejection, same reason |
| tampered body | 400 | local | integrity check rejects modification |
| `replay.corpus.v1` body | 400 | production | the v2 migration is enforced |
| empty body | 400 | production | request validation works |
| database after the accepted submission | **1 to 2** | **LOCAL ONLY** | an accepted submission reaches persistence |

**Production has never accepted a corpus submission.** The local 201 does not
increase the production population and must not be reported as doing so.

Production submission count before and after: **1, unchanged.** Every
production request in this table was refused, and a refusal writes nothing.

## The controlled comparison

Same repository, same corpus, same endpoint, same schema, same validator. One
variable changed:

    make build                  -> replay v0.6.0-5-gb3667e3 (b3667e3)  -> 201
    go install ...@latest       -> replay dev (unknown)                -> 400

`Makefile:22` passes `-ldflags` setting `version.Commit`. `go install` passes
none, so `version.go:9`'s default survives:

    Commit = "unknown"

## Where the first invariant breaks, and it is not the endpoint

The API requires `commit` to be 7 to 40 lowercase hex and refuses anything
else. That is correct and should not be weakened to accept `"unknown"`, which
would destroy the invariant to accommodate a broken producer.

The first break is at the producer. `Commit` is declared `omitempty`:

    Commit string `json:"commit,omitempty"`

but its default is the literal string `"unknown"`, which is NOT empty, so
`omitempty` never elides it. The binary emits a provenance claim it knows
nothing about, and never checks its own provenance before writing a document
whose purpose is provenance.

A contributor on the `go install` path therefore receives a 400 naming a
field, with nothing saying their install method is the cause or that another
method exists.

`docs/guide/getting-started.md:14` says `go install` "does the same". For
contribution it demonstrably does not.

## Where this defect came from

The `omitempty` tag is two days old and this workstream added it. `Commit`
entered `internal/observation/corpus.go` in `f72e3f8` (#293, 2026-09-13), a
change whose own subject was build identity: a corpus that could not say which
build priced it. The `"unknown"` default was already in the tree, from
`d3cc909` on 2026-09-02, so the interaction was present and checkable at the
moment the tag was written.

It was not checked. A field was added to carry provenance, declared optional,
and never tested against the value it would actually hold. The defect was found
on 2026-09-15 by running the path end to end, not by reading the code, which is
the only reason it was found at all.

This section exists because a document arguing that provenance claims must be
verified would be a poor one if it omitted its own.

## Second finding, kept separate: a provenance field can be false while valid

While generating the fixtures, the active rules document on this machine had
been replaced by `openai-2026-09-15` during unrelated work. The corpus stamped
that version. Regenerated under the restored `anthropic-2026-09-05`:

| rulesVersion stamped | totalUsd |
|---|---:|
| `openai-2026-09-15` | 13517.666942800026 |
| `anthropic-2026-09-05` | 13518.635515800026 |

**$0.97 apart on $13,518.** Nearly all the traffic is Anthropic models, which
the OpenAI document does not cover, so those requests fell through to the
compiled fallback and were priced almost identically. The figure barely moved
while the provenance claim changed completely.

So `rulesVersion` was syntactically valid, internally consistent, numerically
plausible, and semantically false: it named a document that priced almost none
of the corpus. A sanity check on output magnitude cannot catch this.

Field presence does not establish what the field claims.

## Limits

**The 201 was local.** Production has never accepted a submission. A valid
corpus reaching production, incrementing the real counter and appearing on the
roster is NOT MEASURED.

**A release binary was not tested.** `make build` was, and CI may build
releases differently. Whether the downloaded artefact carries a valid commit
is NOT MEASURED.

**One machine, one operator, one corpus.** The same limitation as every other
figure here.

**Neither defect is fixed.** Which fix is right is a design decision about
what the build system should guarantee: refuse to contribute without real
provenance, make the default empty so `omitempty` works, stop advertising the
two paths as equivalent, or some combination.

## Reproducing

    make build && ./bin/replay cost --contribute <campaign> --contribute-dir /tmp/a
    GOBIN=/tmp/gi go install ./cmd/replay && /tmp/gi/replay cost --contribute <campaign> --contribute-dir /tmp/b
    curl -sS -X POST --data-binary @/tmp/b/*.json https://replay.doctor/api/contribute

Requires `corpus_opt_in = true` in `~/.config/replay/corpus-consent.toml`,
which is the operator's act and was granted for this run.
