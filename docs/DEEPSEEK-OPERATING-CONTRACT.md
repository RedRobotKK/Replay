# DeepSeek: closeout and operating contract

**DeepSeek is now an experimental instrument, not a product claim generator.**

The DeepSeek experimental campaign is closed.

| | |
| --- | --- |
| Final campaign commit | `3c2ef9a` |
| Working tree | clean |
| CI | passing |
| Valid trials, prompt campaign | 396 |
| Prompt-optimization result | **NULL** |
| Patent prior-art search | **NOT_VERIFIED** |

## What we learned

DeepSeek has served its purpose as a controlled experimental instrument. We
tested cache behaviour, billing reconstruction, usage and evidence
reconciliation, reasoning configuration, prompt optimization, task-level
accuracy, instrumentation validity, and reproducibility under replication.

The most important methodological result from the final campaign is that
apparently attractive optimization effects did **not** survive replication.

**Do not use DeepSeek experiments to manufacture optimization claims.** A result
is useful even when the result is null.

## 1. DeepSeek is an experimental instrument

Use it when it can answer a bounded empirical question cheaply: falsifying a
hypothesis, testing a mechanism, generating independent measurements, probing
runtime behaviour, testing whether an observed Replay phenomenon generalises,
stress-testing an evidence or reconstruction procedure, producing adversarial or
heterogeneous cases.

Do not use it merely because it is inexpensive. The experiment must have a
question whose answer changes what we do next.

## 2. It is NOT our optimization oracle

The prompt-optimization campaign produced a clean null. The lesson is not that
DeepSeek cannot be optimized. It is that **simple prompt interventions did not
produce a reproducible effect in the tested design**.

Nobody should report prompt cost savings, reasoning savings, accuracy
improvements, structure benefits or decomposition benefits unless they have been
independently measured **and replicated**.

## 3. It is especially valuable for falsification

```text
Observation -> Hypothesis -> Discriminating experiment
            -> Independent replication -> Claim classification
```

A cheap experiment that kills a bad hypothesis is a successful experiment.
Optimize for information, not for positive findings.

## 4. It can help test Replay mechanisms

The current Replay research hypothesis is not prompt optimization. It is:

```text
durable work-state -> addressable claims -> targeted evidence acquisition
                   -> evidence enters context -> claim reconciliation
```

DeepSeek is one independent model and runtime in which to test whether that
mechanism survives. That is worth more than another collection of prompt
rewrites.

## 5. Use it as an independent environment

Test whether a Replay mechanism is Claude-specific, Codex-specific,
DeepSeek-specific, or model and runtime independent.

Do not assume cross-model generality. **A result observed on DeepSeek is a
DeepSeek result until replicated elsewhere.** A result discovered with Claude or
Codex is not a universal agent property without evidence.

## 6. It is useful for evidence reconstruction

The billing experiment demonstrated the pattern that matters:

```text
provider response evidence -> independent reconstruction
                           -> external account state -> reconciliation
```

Reach for DeepSeek when an experiment needs independently reported usage,
independently observable runtime behaviour, machine-checkable outputs, or
provider state a derived claim can be reconciled against.

**Agreement is consistency, not proof of the provider's internal
implementation.**

## 7. Preserve the evidence

Every campaign must leave enough behind for another researcher to reproduce the
conclusion: exact request configuration, raw or sufficient request and response
evidence, hashes and provenance, model and version, runtime configuration, usage
data, derived calculations, the verifier, a claim ledger, and explicit evidence
gaps.

**If the primary evidence is ephemeral, the campaign is not finished.**

## 8. Evidence classes are mandatory

OBSERVED, DERIVED, SUPPORTED_HYPOTHESIS, NOT_OBSERVED, REFUTED, ASSUMED,
UNRESOLVED.

Never silently promote a prediction to a result, a derived value to an observed
provider fact, one model to universal behaviour, or consistency to a causal
mechanism. The recent campaigns demonstrated why this matters.

## 9. Replication is the gate

If a result looks surprisingly good, do not celebrate it yet. Ask: was the
analysis preregistered; was the comparison paired where appropriate; was task
heterogeneity controlled; was cache state controlled; was the exact request
recorded; can the effect survive independent replication.

If not, it is a lead, not a finding.

## 10. What we are not doing

No arbitrary prompt tweaking. No chasing a desired percentage. No rerunning
failed experiments. No proving a mechanism after the result. No expanding a null
result indefinitely. No collecting data without a discriminating question.

The prompt-optimization campaign is closed. **The v4-pro cache experiment is not
automatically justified merely because it was listed as a possible next step.**
Every future experiment must earn its budget with clear information value.

## 11. Current research priority

> Does addressable durable work-state cause a fresh agent, after context
> discontinuity, to acquire and reconcile relevant evidence differently from an
> agent without that state?

That question has already produced behavioural evidence. The next work isolates
the mechanism rather than broadening the prompt search. In particular:

```text
claim only
vs. claim + correct evidence anchor
vs. claim + incorrect evidence anchor
```

preserving the existing contamination controls and trajectory-level evidence
inspection. The objective is whether the **evidence anchor itself** changes
retrieval behaviour. If it does, that is a far more concrete mechanism than
generic memory or better prompts.

See `replay-work-state-relay-2026-09-27.md` for the trajectory discipline this
must inherit, including its list of conclusions that are not to be written.

## 12. DeepSeek's role

A cheap independent laboratory for testing Replay hypotheses. Not a source of
marketing statistics, not a prompt-optimization engine, not evidence that a
behaviour is universal.

Its value is that it lets us cheaply ask: **is this thing we think Replay
discovered actually real?** If yes, replicate it elsewhere. If no, kill the
hypothesis and move on.

## Final status

| | |
| --- | --- |
| DeepSeek campaign | **CLOSED** |
| Prompt optimization | **NULL under the tested design** |
| Billing reconstruction | **demonstrated as a DERIVED reconciliation procedure** |
| Cache findings | **scoped to the tested regimes** |
| Universal claims | **not established** |
| Patent prior art | **NOT_VERIFIED** |
| Next DeepSeek use | only where a bounded experiment can discriminate a Replay hypothesis |

Evidence: `experiment/deepseek-suite/FINAL-REPORT.md` and
`experiment/prompt-opt/FINAL-REPORT.md`.
