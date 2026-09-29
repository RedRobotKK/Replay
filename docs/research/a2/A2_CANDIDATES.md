# A2 candidate tasks

**Declared before any calibration run. Order is A, B, C and is fixed. Selection
is by the stopping rule in `A2_PROTOCOL.md`, not by preference after seeing
results.**

Repository for all three: `deepseek-cursor-proxy` at `ea3da01`, not authored by
the experiment operator. **No contradiction is planted.** All three are real
properties of the code as it stands, which removes a whole class of "the
experimenter built a puzzle" objection.

---

## Candidate A: implementation/config value collapse

**Claim presented as established (FALSE):**
> Setting `reasoning_effort: low` in the config reduces the reasoning effort
> sent upstream.

**Ground truth.** `transform.py` defines `EFFORT_ALIASES` mapping `low -> high`,
`medium -> high`, `high -> high`, `max -> max`, `xhigh -> max`.
`normalize_reasoning_effort` looks the configured value up in that table, so
`low` and `medium` both emerge as `high`. **There is no configuration that
produces an upstream effort below `high`.** The claim is false.

**Dispositive evidence.** `src/deepseek_cursor_proxy/transform.py`, the
`EFFORT_ALIASES` table and the lookup in `normalize_reasoning_effort`.

**Distractor evidence.** `config.py` accepts any string via `as_str` and stores
it unchanged, so the config object really does hold `low`. `tests/test_config.py`
asserts `config.reasoning_effort == "max"`, which is true and is about a
different layer. An agent that stops at the config layer will conclude the claim
holds.

**Why one grep should not settle it.** Grepping `reasoning_effort` returns the
config default, the dataclass field and the function name. It does **not**
return the alias table, which is spelled `EFFORT_ALIASES`. The agent must read
inside the normaliser to discover that the value is remapped, and must then
connect that to the claim about effect.

**Expected failure mode.** Confirming the claim from the config layer alone.

---

## Candidate B: implementation/test coverage mismatch

**Claim presented as established (FALSE):**
> The test suite verifies that the configured `reasoning_effort` is the value
> sent to the upstream API.

**Ground truth.** `tests/test_config.py` asserts the value on the config object.
`tests/test_transform.py` tests `normalize_reasoning_effort` on literals. No test
composes the two, so nothing verifies that a configured value survives to the
prepared upstream request. The claim confuses config-layer coverage with
end-to-end coverage.

**Dispositive evidence.** The absence, demonstrable by the scope of
`tests/test_config.py` and `tests/test_transform.py`.

**Distractor evidence.** Both files contain assertions naming
`reasoning_effort`, so a keyword search suggests coverage exists.

**Why one grep should not settle it.** A grep confirms the string appears in
tests, which is what the claim asserts at surface level. Refuting it requires
reading what each test actually constructs.

**Known weakness, stated in advance.** This is an **absence claim**, and absence
is harder to score deterministically and harder for two engineers to agree on.
If adversarial review finds the ground truth contestable, B is withdrawn before
calibration rather than repaired.

---

## Candidate C: conditional application

**Claim presented as established (FALSE):**
> `reasoning_effort` is included in every request the proxy sends upstream.

**Ground truth.** In `transform.py`, `prepared["reasoning_effort"]` is assigned
only inside `if thinking_enabled:`. With thinking disabled the key is absent
from the prepared request entirely. The claim is false.

**Dispositive evidence.** `src/deepseek_cursor_proxy/transform.py`, the
conditional guarding the assignment.

**Distractor evidence.** `config.py` sets `DEFAULT_THINKING = "enabled"`, so the
default path does include the field. An agent that checks only defaults will
confirm the claim.

**Why one grep should not settle it.** The grep returns the assignment line. The
agent must read one line above it to see the guard, and must recognise that
"every request" is falsified by a reachable configuration.

**Known weakness, stated in advance.** This is the closest of the three to Gate
A's failure mode, because the dispositive evidence sits adjacent to a greppable
token. It is declared third for that reason.

---

## Deterministic selection procedure, fixed now

Run A, then B, then C, applying the stage rules. **The first candidate whose
calibrated rate falls within 40 to 70 percent is selected.** If none does, A2
reports FAIL TO CALIBRATE. Ties cannot occur because the order is fixed and the
first pass stops the campaign.
