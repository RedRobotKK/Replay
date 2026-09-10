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

| Idea | §101 read | §102 known risk | §103 nearest art | Verdict |
|---|---|---|---|---|
| `CACHE-KEEPALIVE` — TTL-optimal client cache warming | Close. A timer is a technical act, but the inventive step is choosing an interval, which is arithmetic. | Published 2026-07 as a paper. **The paper itself is prior art against anyone, including us.** | CDN cache pre-warming, TTL refresh | PENDING |
| `CACHE-DEADPOINT` — LCP vs declared breakpoint offset | Weakest link is that the comparison is a string operation. Strongest is that the result changes what the provider bills, not what a person reads. | — | Prefix matching, cache key validation | PENDING |
| Three-way cache-miss attribution | Likely ineligible. Classify observations, display result — *Electric Power Group* squarely. | — | Root-cause analysis, log correlation | PENDING |
| Reasoning-token double-bill ledger | Likely ineligible. Accounting method. | — | Metering, chargeback | PENDING |
| Compaction invoice | Likely ineligible. Same shape. | — | FinOps waste detection | PENDING |
| Recoverable-token report | Likely ineligible. Same shape. | `WASTE-TRAJECTORY` (2509.23586) discloses the waste classes | Deduplication, redundant-request detection | PENDING |
| Guard-reachability with coverage classification | Genuine argument. Compile, execute, observe — technical throughout. | **`MUTATE-META` (2010.13464). Treat as anticipating until distinguished.** | Mutation testing + coverage | PENDING |
| Oracle-signal classification of covering tests | Same argument, and the newest element. | `ORACLE-SMOKE` (2606.18168) discloses the taxonomy and the 80.2% measurement | Static assertion analysis | PENDING |
| Mechanically gated test remediation | Strongest §101 position: the acceptance criterion is a compile-and-execute result, not a judgement. | `CRITIC-LOOP` (2607.23002) discloses the loop | LLM test generation (crowded, recent) | PENDING |

---

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
