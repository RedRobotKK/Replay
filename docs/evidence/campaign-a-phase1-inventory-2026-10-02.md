# Campaign A Phase 1: claim-surface inventory

**2026-10-02. Inventory and disposition only. No production code changed.**

Seven read-only audits over disjoint scopes, ~140 findings. Roughly a third were
returned as NOT A REAL ISSUE with a stated reason, which is the half I checked
hardest. Everything below that is marked VERIFIED was re-derived by me from the
source; everything else is the auditing agent's citation and is marked as such.

## The one invariant, and its instances

Every repair-grade finding in this inventory is one rule, which ADR-0018 line 67
already states: **absence, zero and unknown are three values.** The instances
differ only in which aggregate the sentinel zero enters.

| | surface | the sentinel | status |
|---|---|---|---|
| **Q01** | `foldSessions` | a folded session inherits `Unpriced` from whichever lane sorted first | **VERIFIED, BLOCKED** |
| **F1/F3** | proxy spend cap | the cap total is an upper bound and the flag that said so no longer fires | **VERIFIED** |
| **A6** | advisor | an unpriceable model costs $0 and sorts last | **VERIFIED** |
| **A16** | `replay verify` | an unpriceable session enters the headline median as $0.00 | agent-cited |
| **V2/V3** | published corpus and roster | `tasks`, `totalUsd`, `medianTaskUsd` absent reads as zero | **VERIFIED** |
| **F26** | learn graduation gate | an absent prediction becomes a 0% bar, so anything graduates | agent-cited |
| **F25** | learn degradation guard | a clean control arm reads as "no basis" and disarms the guard | agent-cited |
| **F14** | Prometheus `cached_share` | an empty denominator publishes `0.0000` as a measurement | agent-cited |
| **Q06** | `cost --usage` | a session nothing could price loses its break COUNT and deficit TOKENS, which need no price | agent-cited |
| **A13** | `TrimPlan.SavedUSD` | present-but-zero also means "nothing to save" | agent-cited |
| **F28/F31** | e005 median, OTLP export | latent only; e005 verified unaffected on the frozen table, OTLP is unwired | agent-cited |

## F1/F3, VERIFIED, with the agent's framing corrected

The audit reported `CapNotEnforced()` as unreachable. That is too strong, and the
true finding is sharper.

- `listCost` (`passthrough.go:488-517`) prices an unknown model at
  `DearestPrice()` as a deliberate upper bound, replacing a fail-OPEN zero. That
  change is right and its reasoning is sound: counting zero meant an operator who
  asked to stop at $20 had no cap on exactly the traffic most likely to be
  expensive.
- `Record` arms the disclosure at `guards.go:109` only when
  `usd <= 0 && tokens > 0`. With a populated price table that is now unreachable
  for an unknown model, because the dearest substitution makes `usd > 0`.
- It is **not dead**: `listCost:517` returns 0 when no priced row exists at all,
  so the flag still fires for a rules document in which every row is unpriced.
  Narrowed to a degenerate configuration, not removed.

**The defect is a false claim in the code's own comment.** `passthrough.go:509-512`
asserts "CapNotEnforced still fires and still reaches doctor and the TUI, because
the figure is now an over-estimate rather than a measurement and the operator
needs to know which one they are reading." In the common case it does not fire,
so the operator is not told. Meanwhile `guards.go:155-160` refuses traffic with
"$X of $Y at list price" over a total that may be an upper bound, and
`guards.go:21-24` still documents the old zero behaviour.

The repair direction is unambiguous and changes no published figure: arm the flag
where the substitution happens rather than where `usd <= 0`. Disposition REPAIR,
not escalation.

## Dispositions

**BLOCKED on the Q01 panel** (anything gating on `Unpriced` or on the folded
row): Q01, Q02, Q03, A16, Q04, Q05.

**REPAIR, independent of Q01** (12): F1/F3 and F2's stale doc; A6 and A1/A2 in
the advisor; V2, V3 and S5 on the published documents; F26 and F25 in learn;
F14's gauge; Q10/Q11's silent parse-drop in burn, where `burnGrok` in the same
file already shows the correct pattern; A1(obs)'s false comment about the
allowlist being reflected at run time, which `submissionkeys.go:29-34` refutes in
capitals.

**GENERALIZE, justified by three or more real surfaces** (3): the gate-exit
disclosure family Q04/Q05/Q07/Q08, which is one defect at four exits and where
the counters C035/C037 added already exist and only `renderCost` reads them;
C037's own invariant at `since.go:198-199` and `EffectiveTokens`; F4/F16/F24's
two-populations-under-one-name in the proxy and learn.

**DEFER** (9): A9, A10, A17, A21, A23, A25, F20, F28, F29, F31, P8 — real, but
each needs a judgement this phase has no evidence to settle.

**NOT A REAL ISSUE, confirmed bounded** (~35): `internal/money` entire;
`internal/quota` entire, both files refusing with a reason rather than inventing a
threshold; `sessionspend.go`, the only aggregate in `internal/analysis` that names
every excluded class; `calibrate.go`'s absent-is-not-good gate with its
18-of-1450 regression recorded; the card's peak-versus-sum discipline; the pool's
supersede-not-add rule and its nil-stays-nil optional fields; `doctor.go`'s
sessions-and-lanes pair, which is the model `advise` should copy; the labelled
Prometheus counters; `e005`'s compute/render split.

## Prose contradicted by code

Five README/ADR claims do not hold, agent-cited with implementing sites:
the list-price caveat is not printed on every dollar (`internal/tui/live.go:91`,
`guards.go:214-216`); a proxy that never answered returns `From: Measured` with
no banner (`live.go:46`), contradicting ADR-0018's own cheapness rule;
`docs/TUI-FLAG-SURFACE.md:3-9` describes the guards screen as example data when
it reads the live proxy; the truth-tier claim fails on five transcript-derived
TUI screens, and `Screen.From` is discarded at the frame boundary so nothing
downstream can enforce it. Thirteen other hard numbers in the README were found
correctly hedged, dated and in two cases explicitly retracted.

## Not audited

`internal/proxy`'s masking, rehydration, retry and lifecycle mechanics, and
`uds`/`hostguard`: no quantitative or evidentiary claim surfaced there under
targeted search. `internal/ledger`, `internal/transcript`, `internal/masking`,
`internal/selfupdate`, `internal/consent`, `internal/feed` were not in any scope.
**Stated as a gap, not as a clean result.**
