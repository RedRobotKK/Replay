# ADR-0010: Where waste data lives, and what gets thrown away

- **Status:** Proposed
- **Date:** 2026-09-04
- **Related:** ADR-0009 (crowdsourced waste), [`PRODUCT-DIRECTION.md`](../PRODUCT-DIRECTION.md)

## The question

Cross-session analysis is the product direction, and JSONL scattered across per-session files is a
poor shape for "show me every session where the error share exceeded 20%". SQLite is the obvious
reach. **The obvious reach is wrong here, and measuring first is what shows it.**

## What the data actually weighs

A heavy daily user, 1,500 sessions a year at 60 turns each:

| | Per year | Ten years |
|---|---:|---:|
| **Ledger**, one record per request | **81 MB** | **810 MB** |
| **Waste summary**, one row per session | **0.6 MB** | **6 MB, 15,000 rows** |

**Fifteen thousand rows is not a database workload. It is a file you load into a map.** A full scan
of 6 MB of JSON is a few milliseconds, and it needs no index, no schema migration, no driver and no
query planner.

## What SQLite would actually cost

`database/sql` is standard library. **The driver is not**, and there are only two real choices:

- **`mattn/go-sqlite3`** needs cgo, which breaks `CGO_ENABLED=0` at `.goreleaser.yaml:19` and with it
  the six-target cross-compile matrix.
- **`modernc.org/sqlite`** is pure Go but is a transpiled C tree of roughly a million lines with a
  string of transitive modules.

Either way `go.mod` stops being **45 bytes with zero dependencies**, which the blue-team review
identified as the single most checkable credibility claim the project has: a reviewer confirms it in
three seconds and it removes the entire supply-chain conversation. **Trading that for query
convenience over fifteen thousand rows is a bad trade at any point in the next decade.**

## Decision

**No database. Two tiers, one of which expires.**

### Tier 1: the ledger stays as it is, and starts expiring

Append-only JSONL, one file per session, `0600` in `0700`. Crash-safe by construction, greppable,
inspectable with `head`, and it needs nothing to read it. **That is the right shape for a forensic
log and the wrong shape for an aggregate.**

**Today it grows forever, and so does the vault.** There is no retention, pruning, expiry or rotation
anywhere in `internal/ledger` or `internal/masking`. **81 MB a year of an agent's request metadata
accumulating silently on a developer's laptop is the actual storage problem here**, and it is what
"send it to /dev/null" should mean.

- `--ledger-retention 30d`, defaulting to something finite.
- `replay prune` to run it by hand, and `doctor` reports the size and the oldest record.
- **The vault gets the same treatment.** It only ever inserts, so masking turns transient secrets
  into a permanent at-rest collection. An unreferenced entry older than the retention window should
  go.

### Tier 2: a derived summary, rebuilt not maintained

One JSON object per session, written when a session's last record ages out, holding exactly what
ADR-0009 needs: the five waste categories, the binned turn series, model, client version, durations.

**Derived, so it can always be rebuilt from a ledger that still exists, and never the source of
truth.** A corrupted summary is a `replay reindex` away. A corrupted database is an incident.

**And it must survive the ledger being deleted**, which is the whole point: you keep the shape of a
thousand sessions in six megabytes while the eight hundred megabytes that produced them are gone.

## When this decision should be revisited

**When a single user's summary passes roughly a million rows**, or when concurrent writers from
several machines share one store. Neither is close: a million rows is six centuries of heavy use for
one person. **If the aggregate ever moves server-side, that is a different program with different
constraints, and this ADR does not bind it.**

## What this does not solve

Retention is a decision about someone else's disk, and a default that deletes data is a default that
will delete something someone wanted. **The window has to be stated in `doctor`, warned about before
the first prune, and overridable to "never" for anyone who wants the old behaviour.**

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
