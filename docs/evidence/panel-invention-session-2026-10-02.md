# Locked-room invention session: result

**2026-10-02. Seven rounds. The leading candidate was killed in Round 7 by prior
art and by its own premise. A narrower result survived. No invention identified.**

## 1. THE PROBLEM

The candidate the room walked in with was "evidence-bounded reconstruction of
autonomous work". It did not survive as a problem statement, because it names a
method rather than a failure.

What survived, and it is drawn from six defects found and repaired in this
repository's own code this week:

> **Autonomous work produces numbers nobody can audit, and the failure mode is
> not wrong arithmetic. It is correct arithmetic over an unstated population.**

Every one of C035, C036, C037, SP-01, V1 and Q01 is that shape. A dollar figure
that covered the priced subset. A deficit priced at the wrong record's rate. A
re-billed total whose token count covered a different population. A gate that
passed while silent about its exclusions. An invoice-facing median that admitted
rows nobody could price. A session cost that depended on what its lane files
were named. In each case the arithmetic was right and the population was unstated.

## 2. THE MECHANISM ROUND 7 KILLED

**Candidate:** the context window is a complete, observable account of what an
agent could condition on, so a boundary observer can establish a verifiable
upper bound on what an agent could have known, and answer the negative
machine-checkably.

**Verified present in the substrate** (this part is real): `Request.Context` is
"every message the request carried as input, oldest first", and `Block` carries
`Label` ("tool result: Read internal/foo.go"), `ToolName`, `ToolUseID`,
`IsError`, and `CallKey`, which identifies a call "by tool name and input
**without holding the input**". `Text` is `json:"-"`. So identity-level context
membership is observable without content, and departure is a set difference
across consecutive requests.

**Killed by three independent attacks:**

1. **The theory is forty years old.** Fagin, Halpern, Moses and Vardi's
   interpreted-systems semantics already formalises "what agent A could have
   known at T" as a function of A's local state, with epistemic model checkers
   that machine-check it. Context window as local state is a mapping, not a new
   theory.
2. **The applied distinction is published and shipped.** Google Research's
   "Sufficient Context" (arXiv 2411.06037) separates "the context was
   insufficient" from "the context sufficed and the model still erred". RAGAS
   context recall and MLflow's context-sufficiency scorer ship "was X in the
   context" as a production metric.
3. **The premise is false as stated.** OpenAI's own documentation: reasoning
   tokens "are not visible via the API" yet "occupy space in the model's context
   window". A boundary observer cannot reconstruct the conditioning set for
   reasoning models. And parametric knowledge breaks the negative outright: "X
   was not in context" does not entail "the agent could not have known X".

**What the claim collapses to:** "X was not in the context window." That is not
an inference. It is a log query, and any tool that stored the prompt can answer
it.

I record that my own framing caused the overreach. I began with "complete and
observable" and had to narrow it twice under attack.

## 3. WHAT SURVIVED

Not a mechanism. A **discipline with receipts**, and one precisely-located gap.

**The discipline:** every aggregate carries its population, its exclusions and
its unknowns; a derived cache key is reflected from the data's own shape rather
than declared; and each guard is mutation-proven. 2,575 test functions, 25
registered claims each with a stated boundary, five CI mutation suites.

**The gap, verified and currently unoccupied:** nobody in the observability stack
asserts that what they recorded is what was sent. OpenTelemetry's GenAI
conventions mark message capture Opt-In and state that instrumentations MAY
filter or truncate. There is no completeness attribute and no enforcement.

**Replay cannot occupy that gap today either.** Its ledger `Append` does not
`Sync()`, `store.go:176-181` admits a reader can land inside a write, and a
schema mismatch is dropped. Asserting completeness would require fixing all
three. That is a bounded, honest engineering target and it is **NOT MEASURED**
whether anyone would pay for it.

## 4. THE DEMO

**Before:** an agent run costs $340. A finance reviewer asks what it covers. The
answer is a number.

**After:** the same run reports $340, states that it covers 3 of 4 transcripts
read, names the fourth as excluded rather than free, states that 20,000 of 60,000
re-billed tokens sit outside the dollar figure, and passes its ceiling gate with
those exclusions printed rather than silent.

The thing a reviewer can now do: **challenge the number and get a defensible
answer**, instead of a smaller number.

## 5. THE EVIDENCE

| | status |
|---|---|
| The six population defects existed and are repaired, mutation-proven | **MEASURED** |
| Identity-level context membership is observable without content | **MEASURED** |
| OTel declines to guarantee capture completeness | **MEASURED** |
| Reasoning tokens are invisible at the boundary | **MEASURED** (vendor documentation) |
| Anyone will pay for coverage-carrying claims | **NOT MEASURED** |
| Work continuity, durable scratch, communication, titration benefit | **NOT MEASURED** |
| The patent landscape and the forensics "knowable at the time" literature | **UNSEARCHED**, the search budget was exhausted. Not a negative result |

## 6. THE FIVE CAPABILITIES

Honest, and the brief permits not forcing them:

- **Quota titration**: connected. Allocation is a claim about a population and
  inherits the same failure mode.
- **Realtime**: connected only as a delta over established claims, not as event
  streaming.
- **Durable scratch, work continuity, inter-agent communication**: **not
  connected by this result.** They remain NOT MEASURED, and the one quantitative
  reading on agent messaging in this repository is a +28% token tax.

Forcing all five through this mechanism would be exactly the feature bingo the
brief forbids.

## 7. THE MOAT

**Weak, and I will not inflate it.** Coverage reporting is copyable. What is not
instantly copyable is the accumulated test discipline that makes the refusals
checkable, and that is a quality asset, not a structural one. No network effect,
no data advantage, no switching cost was identified.

## 8. IP CANDIDATES

**NO CONCRETE INVENTION IDENTIFIED.**

The strongest candidate was killed by three independent attacks above. The
residual unoccupied space (a boundary-sited record with an explicit completeness
assertion) is an engineering commitment rather than a mechanism, and two search
arms, patents and digital forensics, were never cleared, so the space cannot be
called clear either.

Elevating anything here would be manufacturing novelty, which the brief rules out
and which the evidence does not support.

## 9. THE KILL LIST

| killed | why |
|---|---|
| Knowledge-boundary reconstruction | prior art in epistemic logic; premise false for reasoning models; parametric knowledge breaks the negative |
| "Context sufficiency" as a differentiator | Google, RAGAS and MLflow ship it |
| Work Graph | already falsified: `RPL-C021` Established guards against cross-surface composition, `RPL-C019` NoEndpoint on the required identity |
| Agent flight recorder | Vorlon and others record behaviour; no epistemic claim, crowded |
| Content-free context membership as IP | a privacy-preserving engineering trade, not a mechanism |
| Durable scratch as a wedge | two measured nulls and a frozen research programme |

## 10. THE NEXT EXPERIMENT

One experiment, and it is not a technical one.

**Take one corpus from one operator who is not Daniel, and measure whether its
re-billed waste is non-trivial.** Every commercial hypothesis in this document
turns on whether the $19,178 measured across 981 sessions is a property of agent
work or a property of one machine. n=1 operator cannot distinguish them.

It costs no API credits, requires no new code, and it discriminates. If waste is
universal, the coverage discipline has a buyer. If it is idiosyncratic, the
product question reopens regardless of any mechanism.
