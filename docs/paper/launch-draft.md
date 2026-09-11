# Launch drafts

The preprint that shares these figures is [`replay-preprint.tex`](replay-preprint.tex)
in this directory. It has not been compiled — no LaTeX toolchain was available
where it was written — and is checked structurally only.

Every figure below is traceable to a dated file under `docs/evidence/`. The
number beside each claim is the corpus it came from, and corpora are not
merged. A draft of this text carried "1.5M+ simulated sessions" and
"13.5 million production sessions" for a measurement taken over **60
transcripts**; both are struck, and the note at the end explains where they
probably came from, because the same slip is easy to make twice.

## What is true, with its source

| Claim | Corpus | Tier | File |
|---|---|---|---|
| 735 breaks, 31,264,349 re-billed tokens | 1,506 transcripts | measured | `break-causes-2026-09-06.md` |
| 50.8 / 33.9 / 7.0 / 5.8 / 2.6 % by cause | same 1,506 | measured | same |
| 87% of creation spend on gaps under 5 min | 60 transcripts, 26,675 pairs | **estimated** | `keepalive-vs-prefix-stability-2026-09-10.md` |
| $77.24 recoverable vs $508.83 not | same 60 | estimated | same |
| 4.2% of billed prompt tokens re-billed | 60 requests, 5 sessions, 16 lanes | measured | `lane-isolation-2026-09-06.md` |
| Replay reproduces provider cache reads | 1,751 transcripts, 116 sessions | measured | `calibration-corpus-2026-09-10.md` |

Three separate corpora carry the three headline numbers. Saying "we analysed
N sessions and found all of this" would be false for every value of N.

---

## Show HN

**Title:** Show HN: Replay – find the exact turn your agent's prompt cache
broke and re-billed you

When an agent's prompt cache is invalidated, the provider does not fail. It
recomputes the prefix, bills it at write rates, returns a normal response, and
raises nothing. The event is visible only in the usage counters on that
response, and only to something comparing them against the previous request.
In practice the first signal is the invoice.

Replay is a local Go binary that reads the transcripts your agent already
writes and tells you which turn re-billed, against which predecessor, and why.
No account, no telemetry, no network request you did not invoke.

**What the corpus says.** Across 1,506 transcripts on one machine it isolates
735 cache breaks totalling 31.26M re-billed tokens:

| cause | breaks | share of re-billed tokens | mean per break |
|---|---:|---:|---:|
| client re-rendered history after the system prefix | 583 | 50.8% | 27,232 |
| cache expired (gap longer than TTL) | 39 | 33.9% | 271,436 |
| prefix diverged inside message history | 102 | 7.0% | 21,503 |
| system prompt or tool definitions changed | 5 | 5.8% | 361,400 |
| model changed between requests | 6 | 2.6% | 133,667 |

The shape matters more than the ranking. Re-renders are frequent and small;
TTL expiries are rare and about ten times the size. One developer going to
lunch costs more than a hundred re-renders.

**Keepalive is the wrong lever here.** On a separate corpus — 60 main-lane
transcripts, 26,675 consecutive turn-pairs, at the *estimated* tier — 87% of
cache-creation spend follows gaps shorter than five minutes. The cache had not
expired; something rewrote the prefix while it was still warm. Pricing the
counterfactual: $77.24 recoverable by keepalive, $508.83 not. That is one
workload on one machine, and it does not generalise to workloads with real
idle structure.

**Two of our own measurement failures, because they are the interesting part.**

The cause table above was first run on the 40 largest sessions and gave nearly
the opposite answer — TTL expiry 75.2% instead of 33.9%, client re-render 2.5%
instead of 50.8% — and the conclusion drawn from it ("layout isn't worth
building") reverses on the full corpus. Largest sessions are longest-running,
longest-running contain the longest gaps, and a long gap is what TTL expiry
*is*. Sorting by size selected for the cause.

Separately, an early build compared each request's prefix against one
session-wide field. Under fan-out that compares a request against whichever
sibling wrote last, so every lane looked rewritten by every other. It reported
98.8% of re-billed tokens as "tool definitions changed" and that reached a
commit message as fact before it was identified as an artifact. Per-lane
isolation fixed it. The corrected run reports 4.2% — **a different quantity,
not a corrected estimate of the same one**: the 98.8% was a share of the broken
run's own deficit total, the 4.2% is re-billed tokens over all prompt tokens
billed.

**Provenance is enforced, not asserted.** Every figure is `[measured]`,
`[estimated]` or `[structural]`. If a provider did not return a counter, the
tool reports *unknown* and refuses to print a number. Absence, zero and unknown
are three values; collapsing them is how a figure comes to look measured
without being measured.

Business Source License 1.1, converting to Apache 2.0 on 2029-09-06.

```bash
curl -fsSL https://redrobot.jp/replay.sh | less               # read it first
curl -fsSL https://redrobot.jp/replay.sh | sh -s -- --dry-run # see what it does
curl -fsSL https://redrobot.jp/replay.sh | sh                 # install
```

<https://github.com/RedRobotKK/Replay>

---

## LinkedIn / X

Do not open with a number the corpus does not support. The version below is
the same argument at the size it actually is; the honest number is more
defensible in a thread where someone will ask.

> **The keepalive ping is the wrong fix for agent cache decay, and our own data
> says so.**
>
> The usual advice for keeping a prompt cache warm is a periodic background
> ping. There's a 2026 paper deriving the break-even window for it, and the
> derivation is fine.
>
> We checked it against our own transcripts — 60 agent sessions, 26,675
> consecutive turn-pairs — and the window is nearly empty:
>
> 87% of cache-creation spend followed gaps of **under five minutes**. The
> cache had not expired. Something rewrote the prefix while it was still warm.
>
> Priced out: $77.24 recoverable by keepalive, $508.83 not. Wrong lever by
> about 7×.
>
> Caveats, because they matter: one workload, one machine, estimated tier. It
> does not say keepalive fails for workloads with real idle gaps. It says this
> workload's cache dies of prefix churn, not idleness — and a ping cannot reach
> that.
>
> We built Replay to find which turn did the rewriting. Local Go binary, reads
> transcripts you already have, no account, no telemetry.
>
> github.com/RedRobotKK/Replay

---

## Where the bad numbers came from

Worth recording, because the failure is systematic rather than careless — and
because the first diagnosis written here was itself wrong.

- **"13.5 million production sessions."** The figure is real and it is
  published: arXiv:2608.00101, *Agentic Coding in the Wild*, 13.5M sessions,
  760.5M LLM calls, 95T tokens. It is recorded in
  `docs/design/reference-distribution.md`. **It is Copilot's corpus, not
  ours.** The draft's sentence was "we parsed 13.5 million production
  sessions", and the error is attribution, not arithmetic: a cited paper's
  population migrating into the first person.

  `reference-distribution.md` already anticipated exactly this: "Copilot's
  13.5M sessions are Copilot users on Copilot's harness, with Copilot's system
  prompt and Copilot's tool set... Neither is a sample of 'people who run
  coding agents'; each is a census of one product's traffic." The design
  document makes `Population` a mandatory field for this reason, and
  `validate()` refuses a reference without one. The launch copy did by hand
  what the code refuses to do.

  A first pass at this note claimed the title and the figure were invented,
  having grepped only `RESEARCH-INDEX.md` — which records the same paper under
  a different facet, 3.2M users. Both are true: 13.5M sessions from 3.2M users.
  Checking one index and concluding a number does not exist is the same class
  of error as the one being documented, committed while documenting it.

- **"1.5M+ simulated sessions."** No published figure matches. Almost certainly
  `1,506` transcripts with the comma read as a magnitude separator.
  "Simulated" is wrong twice over: these are real transcripts from unrelated
  work, and nothing here is simulated except the counterfactual replays, which
  are labelled as such.

The rule this project actually holds is that a figure carries its population.
Neither struck number broke it by being false; both broke it by arriving
without the population attached, at which point the nearest available
population was assumed to be ours.
