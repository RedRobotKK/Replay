# Invalid experiment register

An experiment whose instrumentation was compromised is INVALID. It is not
deleted. A deleted failure cannot be cited, cannot be re-run against, and cannot
stop anyone repeating it, and the record of what went wrong is usually worth
more than the result would have been.

Nothing in this register may be promoted to a finding. An INVALID experiment is
not a weak result; it is an absent one, and re-running it is the only way to
change that.

Opened by Track D, 2026-09-28. Other tracks append.

## Register

| ID | what it measured | what failed | class | why it is preserved |
|---|---|---|---|---|
| **DS-SURF-PO** | first prefix-ordering intervention; "98.3% reduction in fresh input tokens from reordering alone" | uncontrolled variable: a **fixed filler seed** made every trial share a prefix, so the effect was an artifact of the fixture, not of ordering. n=4 per arm. The outcome was, in the artifact's own words, "asserted by construction, not measured" | INVALID | It is the negative control for DS-F3, which re-ran the same question with a per-trial seed and found the effect is real **only when the prefix recurs**, 90.4% against 0% for unique prefixes. The retracted unconditional headline is still quoted downstream in DS-GAP's "96.3% lower input cost", so the retraction has to stay findable. |
| **DS-F2** | whether `reasoning.effort` controls reasoning | wrong instrument: the probe sent a nested `reasoning: {effort: ...}` shape; the documented parameter is the flat `reasoning_effort`. An API ignoring an unrecognised key is not a finding about reasoning | INVALID, correctly self-classified NOT_MEASURED | It is the reason DS-F2b exists and the reason `policy.DISABLE_REASONING` uses the flat shape. It is also the cleanest example in the campaign of a null result that was a null instrument, which is the failure `policy.check_reasoning_honoured` now guards at runtime. |
| **DS-F4** | DERIVED cost against OBSERVED balance; reported a 2.9x gap | two instrument failures at once: the balance was read **before settlement completed**, and the derived side covered 440 of roughly 574 calls, so a lagged total was compared against a partial subtotal | INVALID, correctly withdrawn | It defines the two failure modes every later reconciliation has to rule out, and it is why `reconstruct.py` treats an unsaved call as NOT_OBSERVED and a lower bound rather than as zero. DS-F4a and DS-F4b are only interpretable as its replacements. |

## Also recorded, not experiments

| ID | what it was | what failed | class | why it is preserved |
|---|---|---|---|---|
| **HH-02 mutation sweep, first run** | mutation check of the rebuilt fan-out | instrument failure: `__pycache__` mtime collision. Three mutations read as SURVIVED against code no longer on disk and the restored unmutated file read as FAILING. Reproduced and confirmed by Track D, D1, including 20 of 20 trials with a plain `cp` | VOID | It is the origin of the permanent guard in `scripts/harness-test` and of `experiment/harness/cleanstate.py`. Any sweep in this repo that cannot show it ran with `PYTHONDONTWRITEBYTECODE=1` and an empty `__pycache__` is this failure until proven otherwise. |
| **settle_test BIG/SMALL** | separating settlement lag from cent truncation | nothing failed; it refuses to run without `--confirm` and was never confirmed | NOT RUN | It is a well-designed discriminating test and the only instrument the campaign has for the $0.046 residual reported in Track D, D3. Recorded so it is not re-designed from scratch. |

## Candidates, not yet classified

| ID | why it may belong here | what would decide it |
|---|---|---|
| **DS-F1 counting cell** | `tasks.py` labels a counting task "mechanical" and DS-F1 finds reasoning-off **better**, 18/20 against 9/20. DS-RS classifies counting as aggregative and finds reasoning-off catastrophic at 29%, and `policy.py` ships the DS-RS rule. The two are unreconciled, and the "mechanical" label on a counting task is the same defect class as the line-number case that voided the first DS-RS run. The reasoning-on cell is separately confounded with a 200-token cap | A re-run of the counting cell alone with the cap removed and the DS-RS checker. Until then DS-F1 stays LIMITED, because its extraction and ordering cells are sound and separable. |
