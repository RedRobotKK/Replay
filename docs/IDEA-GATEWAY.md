# The idea gateway

A USPTO-examiner-style screen every idea passes before anyone calls it
defensible. Written 2026-09-10.

**This is not legal advice and no part of it is written by a lawyer.** It is a
screening rubric, in the same spirit as the rest of this repository: a claim of
defensibility is a claim, and a claim nothing can refute is not evidence
(ADR-0014). The gateway exists to make "this is our moat" a falsifiable
statement rather than a hopeful one.

---

## 0. Why a patent gate, and why it usually says no

The honest headline first, because the rubric below is long and its answer is
short for almost everything:

**For a single-maintainer open-source CLI with no external users, a patent is
almost always the wrong instrument.** Filing costs real money, takes years,
publishes the method to every competitor, and buys a right that is worth only
what you can afford to enforce. Meanwhile the actual defensibility of this
project has been execution, measurement discipline, and the parsers — none of
which a patent protects and all of which a competitor must redo.

So the gateway is used three ways, and only one of them ends in a filing:

| Outcome | Meaning |
|---|---|
| **FILE** | Novel, non-obvious, eligible, and worth the disclosure. Rare. |
| **PUBLISH** | Publish it as prior art so nobody else can patent it and block us. Often the right answer for a project that ships in the open. |
| **KEEP** | Neither patentable nor worth publishing; the advantage is in doing it well. |

A `PUBLISH` verdict is a real result, not a consolation. A competitor patenting
`CACHE-DEADPOINT` and asserting it against this project is a live risk that
costs nothing to foreclose.

---

## 1. Gate 1 — §101 subject-matter eligibility

The hardest gate for software and the one most likely to be soft-pedalled.
Two steps, from *Alice* / *Mayo*.

**Step one: is the claim directed to an abstract idea?** Three buckets — a
mathematical concept, a mental process, or a method of organising human
activity.

**Step two: is there an inventive concept beyond the abstract idea?**
Specifically a **technical improvement to the functioning of a computer**
(*Enfish*), or a solution necessarily rooted in the technology and addressing a
problem specific to it (*DDR Holdings*).

Two precedents do most of the work against this project's ideas:

- ***Electric Power Group v. Alstom***: collecting information, analysing it,
  and displaying the result is abstract. **Most of this catalogue is exactly
  that shape.** Every measurement idea — the compaction invoice, the
  recoverable-token report, the thinking-token ledger — reads onto it directly.
- **Billing and accounting** are classic organising-human-activity territory,
  and post-*Alice* business-method claims fare badly.

The gate's own test, stated so it can be applied without a lawyer in the room:

> Does the claim make a *computer* work better, or does it make a *person*
> better informed? If the second, it fails step two, and no amount of
> "wherein a processor" saves it.

By that test the cost-measurement line fails and the test-adequacy line has a
genuine argument: forcing a condition false, recompiling, and re-executing a
suite is a technical process producing a technical result, and testing tools
have historically fared better than pure data-analysis claims.

## 2. Gate 2 — §102 anticipation

A single reference disclosing every element. One known landmine is already on
the board and must be handled first, not last:

> **arXiv:2010.13464 — "What It Would Take to Use Mutation Testing in Industry:
> A Study at Facebook" (26 Oct 2020)** describes mutation testing combined with
> instrumentation measuring which tests actually visited the mutated code.

That is a public disclosure of the core of the UNREACHED/INERT split, six years
early, from a company with a portfolio. Recorded in the research index as
`MUTATE-META`. **Until someone distinguishes it, the guard-reachability
classification must be described as validated-and-unshipped, never as new.**
Describing it as novel in a README would be the same defect this repository
freezes elsewhere: a claim with nothing behind it.

## 3. Gate 3 — §103 obviousness

A combination of references plus a motivation to combine. Two deep and directly
analogous arts sit under this project and will supply the easiest rejections:

- **Web and CDN caching.** Cache warming, TTL refresh, pre-warming and
  keep-alive are decades old. `CACHE-KEEPALIVE` has to survive "apply known
  cache-warming to a new kind of cache," which is the textbook motivation.
- **Cloud cost management / FinOps.** Showback, chargeback, waste detection,
  idle-resource detection and rightsizing are a mature patent art, and every
  cost-measurement idea here is a straightforward reading of it onto tokens.

Mutation testing itself is 1970s art (DeMillo, Lipton, Sayward) and coverage is
older, so the guard work faces "mutation testing plus coverage instrumentation,
combined for the obvious reason of reducing false positives."

## 4. Gate 4 — is it worth owning?

Passing three gates is not a reason to file. Four questions decide it:

1. **Does a narrowed claim still cover anything a competitor would want to
   do?** A claim narrowed until it is allowable is often narrowed until it is
   worthless.
2. **Would we detect infringement?** A method executed inside someone else's
   CI or CLI is close to undetectable.
3. **Could we afford to enforce it?** If not, the patent is a marketing badge.
4. **Does the disclosure hand a competitor the design?** Publishing a method
   that is currently non-obvious to build is a real cost.

---

## 5. The gate applied

Verdict column filled as examiner screens complete. `PENDING` means the
prior-art search is still running — recorded as pending rather than guessed,
because an unsearched idea has no verdict, and unknown is not the same as
allowable (ADR-0018).

| Idea | §101 | §102 nearest | §103 nearest | Verdict |
|---|---|---|---|---|
| Guard-reachability + coverage classification | **LIKELY ELIGIBLE** — mutate, compile, execute, instrument. Post-*Alice* grants exist in this art unit. | `MUTATE-META` discloses mutation plus instrumentation of which tests visited the mutated code | US12072790B1 (State Farm, diff-scoping); US12321257B2 (DevFactory, coverage analysis of survivors); US8997034B2 (Synopsys, activation vs detection) | **PUBLISH** |
| Oracle-strength interlock | Eligible **only** as a dependent refinement of the pipeline above; ineligible standalone | Schuler & Zeller checked coverage (2011); Zhang & Mesbah assertions (2015) | DevFactory + Zhang & Mesbah + Schuler & Zeller | **FILE (provisional, narrow)** |
| Mechanically gated test remediation | Eligible, and irrelevant | **arXiv:2501.12862 (ACH, Meta, Jan 2025) — anticipated.** LLM mutant, LLM test, acceptance = test kills mutant. arXiv:2402.09171 (TestGen-LLM) discloses the mechanical-filter architecture verbatim in substance | Fraser & Zeller ISSTA 2010 supplies the gate itself | **KEEP** |
| Reasoning-token double-bill ledger | **LIKELY INELIGIBLE** — mathematical concept *and* fundamental economic practice. *Electric Power Group* controls. | None found | US 12,699,595 (Mavvrik) meters LLM input/output tokens against a quota; US 8,380,736 (Microsoft) claims double-billing detection in a metered stream | **PUBLISH** |
| Compaction invoice | **CLOSE** — best pro-eligibility story of the cost line, via *Visual Memory v. NVIDIA*: it measures a caching subsystem. Fails as drafted because it reports and stops. | None. "Context compaction" is essentially absent from the patent record. | **US 9,619,397 (IBM)** — computes probability an item is needed again AND the cost of re-obtaining it, evicts on that weight. Nearly mechanical mapping. | **FILE (only if commercialising)** |
| Recoverable-token report | **LIKELY INELIGIBLE.** "Without modifying the agent" writes the practical application out of the claim on its face. | None found | US 10,133,557 (Mentor) analyses a trace for repeated activity + estimated gain; and **compiler liveness analysis** — dead-store elimination and dead-value analysis, decades of textbook art | **PUBLISH** |
| `CACHE-KEEPALIVE` — TTL-optimal client cache warming | **CLOSE.** Eligible only if drafted around avoided prefill computation and latency. Drafted around tokens billed, ineligible. | None. Notably **US12596764 (OpenAI, prompt caching) contains zero occurrences of "idle", "keep-alive", "periodic" or "refresh"** — grepped. | **US7693084 (Microsoft, 2007)** — determine an unknown expiry timeout, set keep-alive to the largest safe value below it, emit during idle. Structurally identical, 18 years early. Plus **US9292073 (Intel)**, break-even time from a cost ratio between two states. | **PUBLISH** |
| `CACHE-DEADPOINT` — LCP vs declared breakpoint | **LIKELY INELIGIBLE** — weakest of the three. Collect, analyse, display, then advise a human. *Electric Power Group* almost verbatim. | **US11792294 (Cloudflare, priority 2015)** computes the longest common prefix *to determine where the cacheable prefix boundary should be placed*. The heart of it, a decade early. | Cloudflare + OpenAI's declared-prefix cache | **PUBLISH** |
| Three-way cache-miss attribution | **LIKELY INELIGIBLE, strongest rejection in the set.** Also vulnerable as a mental process — an engineer does these three comparisons by eye. | **US12541443 (UMass)** classifies each miss into exactly one of three types. And the textbook 3C model (compulsory/capacity/conflict) is late-1980s art — flagged NOT VERIFIED, no NPL search was possible, but an examiner with NPL access would cite it. | **Akamai US11445225 / US11743513** — and the motivation to combine is *printed in Akamai's own specification*: root-cause attribution of cache underperformance in order to tune configuration. | **PUBLISH** |

### The structural finding

**Meta holds no mutation-testing patents.** Zero results under assignee Meta or
Facebook; inventor searches for the `MUTATE-META` authors return nothing in
software testing. They published three times instead — 2020, 2024, 2025.

That cuts both ways, and the direction decides the strategy:

- **Against filing:** those papers are §102(a)(1) printed publications. They are
  not ours, so no grace period applies. They block.
- **For building:** there is no Meta patent to be asserted against us. Freedom
  to operate in this space is excellent *because* they published.

### The one thing worth owning

Not the classification — that is obvious over Zhang & Mesbah plus Schuler &
Zeller. The **interlock**:

> classifying each covering test by oracle strength, and **conditioning
> emission of a code-removal recommendation on that classification, suppressing
> it when no covering test contains a value assertion**, emitting a
> test-remediation recommendation instead.

Every reference found stops at diagnosis. None claims suppressing a destructive
action on the basis of oracle strength. It is the interlock that stops an
autonomous agent deleting correct production code because a vacuous test failed
to notice its absence — a failure mode that is new, common, and expensive.

Narrow, and the only limitation in the set that is both non-obvious on the
record searched and commercially load-bearing.

### The substrate belongs to the counterparty

**US12596764 B2 — "Prompt caching in generative response engines", OpenAI OpCo LLC**, priority 2025-03-12, issued 2026-04-07. Verified by full-text fetch: it discloses prefix caching, hash-based routing to a warm instance, an explicit caching window, cache-read token counts returned in the response, and the cached-token discount. **OpenAI is already patenting the server side of exactly this, including the pricing mechanics.**

Anthropic does hold US patents (US12619815, computer-use agents, verified) but **none surfaced on prompt caching**.

Every cache idea here depends on provider-defined TTL, breakpoint and pricing semantics. Those can be changed in a changelog entry, unilaterally and without notice, and a claim built on them would then cover nothing anyone would build.

### Infringement would be undetectable

The keepalive interval and the miss-attribution logic execute wholly client-side, on a third party's laptop. An unenforceable claim is a decoration. This applies to the cost line generally and is the practical reason `PUBLISH` beats `FILE` even where a claim might issue.

### Two flags before anyone spends money

1. **The disclosure clock is running and may already have run.** A public
   repository starts a 12-month US grace period under §102(b)(1) and is an
   **immediate absolute bar in the EPO and JPO**, which have no grace period.
   This project is Tokyo-based. If non-US rights matter, the first public
   commit date may already govern.
2. **The §102 case against two inventions rests on abstracts.** `MUTATE-META`
   and arXiv:2501.12862 were not read in full. If `MUTATE-META` does not
   actually articulate the unreached/inert split as a *reported classification*,
   that posture improves — though the §103 case over DevFactory plus Synopsys
   stands regardless.

### What the screens could not do

Both examiners recorded the same gaps, and they are recorded here rather than
papered over. `ppubs.uspto.gov` requires an authenticated session and **was not
searched** by any of the three; Google Patents bot-blocked every host and was
reached only through a proxy or through FreePatentsOnline. Espacenet and
PATENTSCOPE returned 403. Coverage is therefore **US-only**, resting on one or
two indices.

**No systematic non-patent-literature search was possible in any of the three
screens**, and for the cost line the NPL is the more dangerous art: provider
documentation, OpenTelemetry GenAI conventions, and the observability vendors
have described token-class accounting publicly since 2023. An examiner will
miss that. An IPR petitioner will not.

One examiner validated its search tool with a nonsense control query after
catching it returning stale cached results — negative results are trustworthy
within US patent text and are not evidence of global novelty. Several promising
numbers came back **NOT VERIFIED** and are excluded rather than cited.

This is a screen, not a clearance.

---

## 5b. All nine screened: the answer

**Eight PUBLISH, one FILE, and the FILE is narrow.**

Nothing in the cost line is patentable. Two of three are likely ineligible
under §101 as pure reporting, the third is close and only if drafted around
avoided computation rather than tokens billed, and all three fall to §103 art
that predates the LLM field by a decade or more — Microsoft's 2007 NAT
keep-alive, Cloudflare's 2015 longest-common-prefix boundary placement,
Akamai's cache root-cause attribution with the motivation printed in its own
specification.

The test-adequacy line is eligible — mutate, compile, execute, instrument is
technical throughout, and post-*Alice* grants exist in that art unit — and it is
foreclosed by publication instead. Meta published three times and patented
none of it.

The one thing worth filing is the **oracle-strength interlock**, and only if
there is a product beyond the CLI. A second candidate exists — the narrow
keepalive refresh construction, a truncated strict-prefix request with
generation suppressed so the provider extends the TTL on a cache-read rather
than a cache-write charge — but it is defined entirely by one provider's
current billing semantics and would be mooted by a changelog entry.

**What to do instead, and it is not a consolation prize.** The genuine risk to
a small open-source tool is somebody else patenting this and asserting it. A
dated defensive publication forecloses that worldwide, for essentially nothing:
a timestamped post plus a filing to IP.com's Prior Art Database or Technical
Disclosure Commons. Given that the FinOps and observability portfolios are
actively expanding into token metering, and that OpenAI already holds
US12596764 on the server side of prompt caching, this is not theoretical.

## 6. Standing rules

Whatever the screens return:

1. **Never describe the UNREACHED/INERT split as novel.** `MUTATE-META`
   shipped it in 2020. "Validated at Meta scale and never packaged" is both
   true and a better story.
2. **Every idea grounded in a published paper is already disclosed.** The paper
   is prior art against us as much as anyone. An idea taken from arXiv cannot
   be patented by the person who read it there.
3. **A `PUBLISH` verdict is actionable.** Write the method down publicly, dated,
   in this repository. It costs nothing and forecloses a competitor's claim.
4. **The gateway runs before the pitch, not after.** A defensibility claim in a
   README or a deck that has not passed this screen is an unverified assertion,
   and this project does not ship those.
