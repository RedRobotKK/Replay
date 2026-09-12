# Commercial terms

**2026-09-11.** What is sold, what is not, and what has to be true before anyone is invoiced.

Replay is licensed under [BSL 1.1](LICENSE). The binary is free to use, read, modify and run,
including inside a company. What the licence reserves is offering Replay itself as a service to
third parties. If you need that, it is a letter — see [Additional use](#additional-use) — and not a
feature flag.

The pricing below is derived from [`docs/MONEY-PATH.md`](docs/MONEY-PATH.md) (2026-09-06), which is
written against this repository's own measurements rather than against a template. Read that first
if you want the arithmetic. This file is the summary and the ban list.

## The unit is a repository, never a seat

A per-developer subscription does not survive our own numbers, and we are not going to sell one.

The heaviest machine in the corpus spent **$3,018.99** at list prices in a month with **$150.27**
re-billed by broken caches — 5% of the total. [`docs/WHAT-YOU-GET.md`](docs/WHAT-YOU-GET.md) puts
realistic recovery nearer 2–3%, because some breaks have legitimate causes. To clear $25/month at a
2.5% recovery rate a seat has to spend roughly **$1,000/month on metered tokens**. That population
exists. It is not most people.

For anyone on a flat seat the recoverable figure is **zero dollars**, not a small number. Claude
Max, Team, Copilot and Cursor produce no invoice line for a broken cache, and the attempt to bill
against rate-limit budget instead moved 3.09M tokens and shifted the utilisation counter by **zero**
steps ([titration](docs/evidence/quota-titration-2026-09-06.md)). That is published as a null
result, so there is no second currency.

And the diagnosis itself is one-time. The cleanest measurement here found 4.2% of prompt tokens
re-billed and traced every break to an MCP connector's tool block arriving mid-session
([lane isolation](docs/evidence/lane-isolation-2026-09-06.md)). The fix is client-side sequencing:
bind MCP tools before the first cached request. That is a free configuration change, it takes one
afternoon, and it does not come back. **A subscription whose value is finding that is one the
customer cancels in month two, correctly.**

What recurs is **configuration drift** — a team keeps adding servers and instructions, and nobody
watches the per-request cost of doing so — and **provider drift**, because the prices and cache
floors every dollar figure rests on change on the provider's schedule. Both are per-repository
problems with a named owner and a budget.

So: **repository**, or **workspace** where a team's configuration spans more than one repo. One
word, used consistently. Never "seat", never "per developer".

## What is sold

**The bands below are opening positions, not measurements and not quotes.** They were set in
discussion, they have never been tested against a customer, and as of this file's date **nobody has
paid any of them**. The measured material — what a machine actually spends, what is actually
recoverable — is in MONEY-PATH; the prices are a judgement laid on top of it. Read them as such.

Nothing here is invoiced from a Replay estimate — see
[How an invoice is computed](#how-an-invoice-is-computed).

### 1. Forensics week

| | |
|---|---|
| Buyer | A team with a real API invoice |
| Band | $8,000–18,000 for one week |
| Shape | They redact on their own machine with `replay redact`; we read the file |

One operator, one week per freeze cycle. The deliverable is a written finding against their corpus:
what was re-billed, what caused it, what is avoidable versus a deliberate trade versus not
measurable. Transcripts stay on their machine. We never take a `.jsonl` into our infrastructure.

### 2. Rules workspace

| | |
|---|---|
| Buyer | Platform or FinOps |
| Band | $25–99/month per workspace; $4,000–12,000/year at org scale |
| Shape | Signed freshness of the world vector — prices, cache floors, provider rules |

The compiled price table in the free binary stays **complete**. It is not a crippled tier. What is
paid for is *freshness with a signature*: a dated, verifiable feed, so a figure computed today can
be shown to have used today's rules. A rate without a date is not checkable, which is why every
price table in this project carries one.

The PR prefix gate bundles here. [`replay prefix`](docs/CLI.md#prefix) already exists and answers
whether a change to a tool-server document voids the cached prefix; it exits 1 on a set change, so
it drops into CI as-is.

**Its limits are published, and we do not sell past them.** It is strong for "this PR added a
server." It is blind to Tool Search, to schema-only growth, to a server that defers over HTTP, and
to meta-tool proxies. It is a prefix *gate*, not full prefix physics.

### 3. Rated

| | |
|---|---|
| Buyer | An MCP vendor |
| Band | $0 to be listed; $3,000–8,000/year; $12,000–25,000 for cadence |
| Shape | A grade on a **public** `tools/list` schema, plus a static poster of the record |

Grades the upstream catalogue — tool count, schema bytes, estimated tokens **labelled EST**, sha256,
date, and a letter from a published table. It grades the upstream even when a compression proxy
wraps it, because the wrapper's first-paint definition cost is what the agent actually pays.

Being listed is free and stays free. A vendor cannot buy a better letter.

Forbidden on the badge, permanently: **saves**, **secure**, **certified**.

### 4. Embed

| | |
|---|---|
| Buyer | A vendor embedding the engine in their own product |
| Band | Annual letter |
| Shape | BSL additional use |

See below.

## The ban list

These are not "not yet". They are decisions, and reversing one needs a dated document saying why.

- **No transcripts leave the customer's machine.** No hosted session Replay, no `.jsonl` upload, no
  transcript storage. The hosted side is world-facts only — prices, cache floors, a tool-definition
  weight calculator — and holds no user data. That is the only reason it may exist at all.
- **No percent-of-savings billing.** It makes us the auditor of our own invoice, and the figure it
  would bill against is an estimate this project openly labels as one.
- **No `$25/developer/month`.** See above; our own numbers kill it.
- **No invoicing from `ExpectedRead` or from list-price "avoidable".** Both are Replay's estimates.
- **No "we find your waste" subscription.** The finding is one-time and the fix is free.
- **No priced fan-out called "waste".** Avoidable, trade and unknown are three different things and
  the UI must keep saying so.
- **No softening of `n=1`, of unidentified advice, or of "list price ≠ invoice" anywhere in the UI**
  to make a sales page read better.

## How an invoice is computed

Never from a Replay figure.

A forensics week is billed as a week. A workspace is billed as a workspace. Rated is billed per
grade or per cadence. Every one of those is a fixed, agreed number that does not move with anything
this tool measures, which is what keeps the measurement honest: there is no figure Replay could
report that would increase what you owe.

## Additional use

BSL 1.1 permits any use that is not offering Replay as a service to third parties. If you want to
do that — embed the engine in a product you sell, or run it as a hosted service — it is a letter,
not a licence key and not a feature flag. There is no code path that checks entitlement, and there
will not be one.

Write to the address in [`CONTRIBUTING.md`](CONTRIBUTING.md).

## What is not decided

Recorded rather than smoothed over, because an open question presented as settled is the defect this
project spends most of its effort catching.

- Whether the Rules feed's free tier stays complete indefinitely, or only through 1.0.
- Whether Rated's cadence tier is a product or a service. It is priced as both above.
- The x402 path for `replay_rules_latest` exists as a pointer and a refusal, not as revenue.
