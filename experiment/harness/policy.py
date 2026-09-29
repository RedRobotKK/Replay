"""The measured optimisation levers, as executable policy.

Every rule here cites the measurement that established it. A lever written only
in a findings document is a lever nobody applies; a lever written here is applied
by construction and fails loudly when its premise stops holding.

The DeepSeek research campaign is CLOSED. `docs/DEEPSEEK-OPERATING-CONTRACT.md`
is authoritative; `docs/DEEPSEEK-OPTIMIZATION.md` carries the findings with their
scope.

These are REQUEST-CONSTRUCTION CONTROLS, not a promise of savings. Every figure
cited below is one workload on `deepseek-flash` over `/v1/chat/completions`, at
the n stated in its source. A control being enforced here says what the code
does. Only an experiment says whether it pays, and the experiments that exist
are narrow.

Evidence, authoritative first:
  deepseek-suite/FINAL-REPORT.md                  the closed cache/billing package
  prompt-opt/FINAL-REPORT.md                      prompt optimization: NULL
  deepseek/cache-characterisation-2026-09-29.md   the block relation, SEQUENTIAL regime
  deepseek/optimization-arms-2026-09-29.md        the arms, one benchmark
  deepseek/reasoning-scope-2026-09-29.md          reasoning is class-conditional
  deepseek/docs-reconciliation-2026-09-29.md      what the vendor does NOT promise
"""

# --- cache geometry, MEASURED (17/17 rungs) ------------------------------
# The vendor documents "cache prefix units at fixed token intervals" and gives
# no interval in its current guide. This is that interval, MEASURED.
#
# SCOPE: 35 rungs, all sequential cold/warm pairs. A cached count of 13,563,
# which is not a multiple of 128, was later observed once under concurrency, so
# this is a policy assumption about a measured regime and not a provider rule.
# The vendor also calls caching "best-effort". A 2024 vendor page states a
# 64-token unit for an earlier model generation; it does not predict flash.
BLOCK_TOKENS = 128
MIN_WARM_PREFIX_TOKENS = 2 * BLOCK_TOKENS   # the last block is never served
CHARS_PER_TOKEN_DENSE = 3                   # biased toward warming; see fanout
MIN_WARM_PREFIX_CHARS = MIN_WARM_PREFIX_TOKENS * CHARS_PER_TOKEN_DENSE

# --- concurrency, PUBLISHED ----------------------------------------------
# Not a measurement. api-docs.deepseek.com/quick_start/rate_limit.
# DS-CONC tested 64 and reported "no ceiling", roughly 39x below this.
PUBLISHED_CONCURRENCY = {"deepseek-flash": 2500, "deepseek-v4-pro": 500}


class TaskClass:
    """What the answer requires, which is what decides the reasoning setting."""
    LOOKUP = "lookup"        # answer appears verbatim at one place in the input
    AGGREGATE = "aggregate"  # answer is nowhere in the input; built by a rule
    UNKNOWN = "unknown"      # not classified


# Disabling reasoning is UNDOCUMENTED. The vendor guides show only the enabling
# direction. Both shapes below were measured to work and neither is promised.
DISABLE_REASONING = {"reasoning_effort": "none"}


def config_for(task_class):
    """Call kwargs for a task class.

    LOOKUP    -> reasoning off. Measured 18/18 vs 17/18, far fewer output
                 tokens, on ONE corpus at n=18. Once a prefix is cached the
                 total cost effect was about 1.48x, not the output ratio.
    AGGREGATE -> reasoning on.  Measured 24/24 vs 7/24 at n=24. 29% is not a
                 discount.
    UNKNOWN   -> reasoning on.

    Three further task classes were preregistered and never executed. They are
    NOT_OBSERVED and this mapping does not cover them.

    UNKNOWN defaults to ON deliberately. Reasoning-off does not degrade
    gracefully: it answered 39 as 67 and 14 as 1, with nothing in the response
    marking it wrong. An unclassified task must fail toward the expensive,
    correct setting, never toward the cheap, silently wrong one.
    """
    if task_class == TaskClass.LOOKUP:
        return dict(DISABLE_REASONING)
    return {}


class ReasoningRegression(Exception):
    """Reasoning was disabled and the provider reasoned anyway.

    The disabling parameter is undocumented, so it can be dropped or renamed
    without notice, and the failure is silent: output tokens rise about 20x on
    lookups and nothing errors. This converts that into a stop.
    """


def disables_reasoning(call_kwargs):
    """Whether these kwargs ask the provider NOT to reason.

    One definition, used by both the runtime guard and the static review, so the
    two can never disagree about what "disabled" means.
    """
    kw = call_kwargs or {}
    if kw.get("reasoning_effort") == "none":
        return True
    return (kw.get("thinking") or {}).get("type") == "disabled"


def check_reasoning_honoured(measurement, sent_kwargs):
    """Assert the undocumented parameter did what it did when we measured it.

    Cheap: `reasoning_tokens` is already reported on every chat-dialect call.
    Call this on every response whose request disabled reasoning.
    """
    if not disables_reasoning(sent_kwargs):
        return
    rt = measurement.get("reasoning")
    if rt:
        raise ReasoningRegression(
            f"reasoning was disabled and the response reported {rt} reasoning "
            f"tokens. The disabling parameter is undocumented and may have "
            f"changed. Every lookup-class cost figure downstream of this is wrong.")


class PrefixInstability(Exception):
    """Something varying was placed before the shared prefix."""


def assemble(shared, variable):
    """Build a prompt with the variable part LAST.

    Measured on one benchmark: ordering alone was 4.6x cheaper at no latency
    cost, and a single leading space dropped the hit rate to zero because
    matching is byte-exact.
    A timestamp, request id or rotating system preamble prepended to a prompt is
    therefore expensive in a way that looks free.
    """
    if not shared:
        return variable
    if shared[:1].isspace():
        raise PrefixInstability(
            "the shared prefix begins with whitespace. Matching is byte-exact, "
            "so a prefix whose leading bytes vary -- even by one space -- caches "
            "nothing. Strip it, or make the varying part the suffix.")
    return shared + "\n\n" + variable


def should_warm(prompts):
    """Whether a warm-then-fan run is worth its one serial round trip."""
    from fanout import common_prefix_len  # imported here: policy must not
    # drag the fan-out's threading imports into a caller that only wants rules
    if len(prompts) < 2:
        return False, "single prompt"
    n = common_prefix_len(prompts)
    if n < MIN_WARM_PREFIX_CHARS:
        return False, (f"shared prefix {n} chars is below {MIN_WARM_PREFIX_CHARS}; "
                       f"under {MIN_WARM_PREFIX_TOKENS} tokens nothing caches")
    return True, f"shared prefix {n} chars"


def review(plan):
    """Check a planned run against the levers. Returns a list of findings.

    `plan` is a dict: task_class, prompts, workers, model, call_kwargs.
    Advisory by design -- it reports, it does not rewrite someone's run.
    """
    out = []
    tc = plan.get("task_class", TaskClass.UNKNOWN)
    kw = plan.get("call_kwargs") or {}
    prompts = plan.get("prompts") or []
    model = plan.get("model", "deepseek-flash")

    disabled = disables_reasoning(kw)
    if tc == TaskClass.AGGREGATE and disabled:
        out.append("reasoning is disabled on an aggregative task. Measured 7/24 "
                   "against 24/24 at n=24, and the wrong answers are confident "
                   "numbers with nothing marking them wrong.")
    if tc == TaskClass.LOOKUP and not disabled:
        out.append("reasoning is on for a lookup task. One benchmark measured "
                   "far more output tokens with no accuracy gain at n=18; the "
                   "total cost effect on a cached prefix was about 1.48x.")
    if tc == TaskClass.UNKNOWN and disabled:
        out.append("reasoning is disabled on an unclassified task. Classify it "
                   "first; reasoning-off fails silently, not loudly.")

    warm, why = should_warm(prompts)
    if warm and not plan.get("warm"):
        out.append(f"{why}: warm one call before fanning out. Fanning out cold "
                   f"pays full input price on the prefix for every question.")
    if plan.get("warm") and not warm:
        out.append(f"warming is planned but {why}.")

    cap = PUBLISHED_CONCURRENCY.get(model)
    w = plan.get("workers") or 0
    if cap and w and w > cap:
        out.append(f"workers={w} exceeds the published limit of {cap} for "
                   f"{model}; the excess returns HTTP 429.")
    return out
