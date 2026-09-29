"""Track C task classes: five classes, computed ground truth, machine checkers.

Registered before execution. `experiment/deepseek-suite/results/track-c-prereg.md`
carries the predictions and the falsifiers, and was written before any call.

Why this module exists in this shape:

  - The class label is an ASSUMPTION about what an answer requires, and the last
    time that assumption went unchecked it cost a whole cell. A "mechanical"
    class asked which function sat on a given LINE NUMBER of a 48,000-character
    document, which is positional counting wearing a lookup's label. Every class
    boundary below is stated as a rule about the ANSWER, not about how the
    question sounds.
  - No ground truth is authored by hand. Every expected answer on this page is
    computed from the corpus at import time by the functions in this file. A
    hand-typed expected answer is a second, unversioned copy of the corpus that
    goes stale without failing.
  - The corpus is real Replay source, not filler. Synthetic filler measures a
    model against a distribution nothing in production resembles.
  - Checkers are independent Python predicates. The model never grades itself,
    and no checker consults the model's own reasoning about whether it was right.

Evidence classes used throughout: OBSERVED / DERIVED / ASSUMED / NOT_OBSERVED.
Nothing here is OBSERVED about model behaviour. This file builds the instrument.
"""
import os
import re

import policy
from fanout import Outcome
from policy import TaskClass


# --- the corpus ----------------------------------------------------------
# The same two files, in the same order, with the same markers, as
# deepseek/reasoning-scope-2026-09-29.md. Keeping them identical means the two
# counting truths that experiment reported (39 `func ` lines, 14 `type ` lines)
# are a cross-check on this harness, not just on the model.
CORPUS_FILES = ("internal/advisor/advisor.go", "internal/transcript/wire.go")
MARKER = "===== FILE: {} ====="

# MEASURED need, not a guess. reasoning-scope-2026-09-29.md: the aggregative
# reasoning-ON cell spent about 931 output tokens per call, and an earlier run of
# the same experiment voided a whole cell by capping at 1024 so that reasoning-ON
# never reached an answer. 4096 is the value that run was repeated at.
MAX_TOKENS = 4096


def repo_root():
    """The Replay checkout this file lives in."""
    return os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def load_corpus(root=None):
    """The SHARED block. Passed to session.Run.fan as `shared`, so it is the
    cached prefix and every task's variable part is the suffix.

    policy.assemble refuses a prefix whose leading bytes vary, and the first
    bytes here are a constant marker line, so the whole corpus caches.
    """
    root = root or repo_root()
    parts = []
    for rel in CORPUS_FILES:
        with open(os.path.join(root, rel), "r", encoding="utf-8") as fh:
            parts.append(MARKER.format(rel) + "\n" + fh.read())
    return "\n".join(parts)


# --- corpus readers: every expected answer comes from one of these --------
#
# Each reader asserts the uniqueness its question claims. A question that says
# "exactly one" and is answered by a corpus where two match is not a hard
# question, it is an unanswerable one, and the model would be scored wrong for
# the harness's mistake. These raise at import rather than letting that happen.

class CorpusAmbiguity(Exception):
    """The corpus does not support a question this file asks of it."""


def _only(matches, what):
    if len(matches) != 1:
        raise CorpusAmbiguity(
            f"expected exactly one {what} in the corpus, found {len(matches)}: "
            f"{matches[:5]}. The question built on it would be ambiguous, and a "
            f"wrong answer to an ambiguous question is not a model failure.")
    return matches[0]


def const_int(corpus, name):
    """The integer a named constant is declared with. Go's `10_000` is 10000."""
    pat = re.compile(r"(?m)^(?:const\s+)?\s*" + re.escape(name) +
                     r"\s*=\s*([0-9][0-9_]*)\s*(?://.*)?$")
    return int(_only(pat.findall(corpus), f"declaration of const {name}").replace("_", ""))


def const_float(corpus, name):
    pat = re.compile(r"(?m)^(?:const\s+)?\s*" + re.escape(name) +
                     r"\s*=\s*([0-9]*\.[0-9]+)\s*(?://.*)?$")
    return float(_only(pat.findall(corpus), f"declaration of const {name}"))


def lines(corpus):
    return corpus.split("\n")


def count_lines_starting(corpus, prefix):
    return sum(1 for ln in lines(corpus) if ln.startswith(prefix))


def count_lines_starting_and_ending(corpus, prefix, suffix):
    return sum(1 for ln in lines(corpus)
               if ln.startswith(prefix) and ln.endswith(suffix))


def count_substring(corpus, needle):
    return corpus.count(needle)


def func_named_like(corpus, prefix):
    """The one top-level (column-1) function whose name starts with `prefix`."""
    names = re.findall(r"(?m)^func\s+(" + re.escape(prefix) + r"\w*)\s*\(", corpus)
    return _only(sorted(set(names)), f"top-level func whose name begins {prefix!r}")


def method_receiver_type(corpus, method):
    """The receiver type of a named method, pointer marker stripped."""
    pat = re.compile(r"(?m)^func\s+\(\s*\w+\s+\*?(\w+)\s*\)\s+" +
                     re.escape(method) + r"\s*\(")
    return _only(sorted(set(pat.findall(corpus))), f"method named {method!r}")


def _struct_body(corpus, name):
    pat = re.compile(r"(?m)^type\s+" + re.escape(name) + r"\s+struct\s*\{\n(.*?)\n\}",
                     re.S)
    return _only(pat.findall(corpus), f"struct type named {name!r}")


def struct_fields(corpus, name):
    """Field names of a struct, in source order. Comment and blank lines are not
    fields, so they are dropped; an embedded field would be a name on its own and
    the corpus has none in these two structs."""
    out = []
    for ln in _struct_body(corpus, name).split("\n"):
        s = ln.strip()
        if not s or s.startswith("//"):
            continue
        m = re.match(r"([A-Za-z_]\w*)\s+\S", s)
        if m:
            out.append(m.group(1))
    return out


def struct_declaring_field(corpus, field):
    """The one struct type that declares a field with this name."""
    hits = []
    for m in re.finditer(r"(?m)^type\s+(\w+)\s+struct\s*\{", corpus):
        name = m.group(1)
        if field in struct_fields(corpus, name):
            hits.append(name)
    return _only(sorted(set(hits)), f"struct declaring a field named {field!r}")


def const_block_identifiers(corpus, typename):
    """Identifiers of a `const (...)` block whose entries carry an explicit type.

    Source order. Comment lines inside the block are not entries.
    """
    blocks = []
    for m in re.finditer(r"(?m)^const\s*\(\n(.*?)\n\)", corpus, re.S):
        names = re.findall(r"(?m)^\s*(\w+)\s+" + re.escape(typename) + r"\s*=", m.group(1))
        if names:
            blocks.append(names)
    return _only(blocks, f"const block declaring values of type {typename!r}")


# --- checkers: independent Python predicates -----------------------------
#
# Scoring rule, fixed before execution and identical in both arms:
#
#   numeric answer  -> the LAST number appearing anywhere in the response must
#                      equal the truth exactly. Binary. No partial credit.
#   literal answer  -> the last non-empty line, stripped of code fences, quotes,
#                      backticks and one trailing period, must equal the truth;
#                      or that line's last whitespace-separated token must.
#
# "Last" rather than "only" is deliberate and arm-neutral. A reasoning-ON
# response is verbose by construction and will restate intermediate numbers; an
# "only" rule would score that arm wrong for being verbose and would read as an
# accuracy result. "Last" reads the model's answer and ignores its work. The
# residual risk is a response that enumerates and happens to end on the truth,
# which inflates the arm that enumerates. That is the reasoning-ON arm, so the
# bias runs AGAINST this campaign's cost-saving hypothesis, which is the safe
# direction. Recorded, not corrected.
#
# A checker must never raise. session.Run catches an exception from a checker and
# records passed=None, which is an undecided cell -- a paid call that measured
# nothing. Every predicate below returns False on garbage instead.

_FENCE = re.compile(r"```[a-zA-Z0-9_+-]*\n?")
_NUM = re.compile(r"-?\d[\d_,]*(?:\.\d+)?")


def _clean(text):
    if not isinstance(text, str):
        return ""
    return _FENCE.sub("\n", text)


def _last_line(text):
    for ln in reversed(_clean(text).split("\n")):
        s = ln.strip().strip("`\"'").rstrip(".").strip()
        if s:
            return s
    return ""


def _numbers(text):
    return [m.group(0).replace("_", "").replace(",", "") for m in _NUM.finditer(_clean(text))]


def expect_int(want):
    def check(text):
        nums = _numbers(text)
        if not nums:
            return False
        try:
            return int(float(nums[-1])) == want and "." not in nums[-1]
        except (ValueError, OverflowError):
            return False
    check.want = want
    return check


def expect_float(want, places=2):
    def check(text):
        nums = _numbers(text)
        if not nums:
            return False
        try:
            return round(float(nums[-1]), places) == round(want, places)
        except (ValueError, OverflowError):
            return False
    check.want = want
    return check


def expect_literal(want):
    """Case-sensitive. These answers are Go identifiers and Go is case-sensitive,
    so `suggestion` is not `Suggestion` and must not score as it."""
    def check(text):
        line = _last_line(text)
        if line == want:
            return True
        tok = line.split()[-1].strip("`\"',.;:") if line.split() else ""
        return tok == want
    check.want = want
    return check


# --- the five classes ----------------------------------------------------
#
# Each boundary is a rule about the ANSWER, never about the wording:
#
#   LOOKUP         the answer is written verbatim at ONE place in the corpus and
#                  is reached by matching a name. No arithmetic, no enumeration.
#   COUNTING       the answer is the cardinality of a set defined by a mechanical
#                  predicate over the corpus. It is written nowhere.
#   AGGREGATION    the answer combines TWO OR MORE separately located facts by a
#                  rule stated in the question. It is written nowhere.
#   TRANSFORMATION ONE located value, put through a deterministic rule stated in
#                  the question. Distinguished from AGGREGATION by arity: one
#                  input, not several.
#   MULTI_STEP     several DEPENDENT hops, where the target of hop n is named
#                  only by the result of hop n-1. The final answer may be
#                  verbatim in the corpus; what is not verbatim is the path to
#                  it. This is the class most likely to be argued with, so it is
#                  stated here rather than defended later.
#
# The label never appears in a prompt. A prompt that said "count" or "look up"
# would be handing the model the class, and the class is the variable.

LOOKUP = "LOOKUP"
COUNTING = "COUNTING"
AGGREGATION = "AGGREGATION"
TRANSFORMATION = "TRANSFORMATION"
MULTI_STEP = "MULTI_STEP"

CLASSES = (LOOKUP, COUNTING, AGGREGATION, TRANSFORMATION, MULTI_STEP)


# --- mapping onto policy.TaskClass ---------------------------------------
#
# policy.TaskClass has THREE members: LOOKUP, AGGREGATE, UNKNOWN. Five classes do
# not fit into three, and session.Run.ask requires one, so the mapping is lossy
# in exactly these places. Named here rather than discovered later:
#
#   COUNTING -> AGGREGATE        exact. This is the class AGGREGATE was measured
#                                on (7/24 with reasoning off).
#   AGGREGATION -> AGGREGATE     exact by definition.
#   MULTI_STEP -> AGGREGATE      LOSSY. Multi-hop is not aggregation; it is
#                                dereferencing. It lands on AGGREGATE because
#                                policy has nowhere else that keeps reasoning on,
#                                and because reasoning-off's measured failure
#                                mode -- confident wrong answers with no signal --
#                                is the one a dereference chain would produce too.
#                                Whether that is true is a Track C question.
#   TRANSFORMATION -> UNKNOWN    LOSSY, and deliberately so. It is neither
#                                single-location retrieval nor aggregation, and
#                                policy.config_for sends UNKNOWN to reasoning ON,
#                                which is the fail-safe. If Track C shows
#                                transformation survives reasoning-off, policy
#                                gains a fourth class; until then an
#                                unmeasured class must not be cheap by default.
#   LOOKUP -> LOOKUP             exact.
POLICY_CLASS = {
    LOOKUP: TaskClass.LOOKUP,
    COUNTING: TaskClass.AGGREGATE,
    AGGREGATION: TaskClass.AGGREGATE,
    TRANSFORMATION: TaskClass.UNKNOWN,
    MULTI_STEP: TaskClass.AGGREGATE,
}

# The experiment needs both arms on every class, and session.Run.fan takes a
# task_class but no kwargs override, so the ONLY way to force a reasoning setting
# through fan() is to hand it a task_class whose policy config has the setting
# wanted. That is an abuse of the enum and it is written down here so nobody
# reads a run log and concludes Track C believed COUNTING was a lookup.
#
# Forcing has a cost that the first version of this file got wrong, and the test
# caught: a forced cell is INVISIBLE to policy.review, because review is handed
# the same lie the transport is. Reviewing COUNTING-forced-off as a lookup
# against lookup kwargs is self-consistent and reports nothing. review_plan below
# therefore reviews the TRUE class against the kwargs actually sent, which is the
# only arrangement where the forcing shows up in an audit.
#
# The honest fix is one line in session.Run.fan: an `extra_kwargs` passthrough to
# ask(), which already accepts it. That change is NOT made here because Track C
# does not own session.py and a shared harness should not grow a parameter on a
# subagent's say-so. It is recorded in the prereg as the one harness change this
# track needs.
ARM_ON = "on"       # reasoning enabled
ARM_OFF = "off"     # reasoning disabled via the UNDOCUMENTED parameter
ARM_POLICY = "policy"  # whatever policy.config_for would choose in production


def policy_class_for(cls, arm):
    """The task_class to hand session.Run.fan to realise `arm` on `cls`."""
    if arm == ARM_POLICY:
        return POLICY_CLASS[cls]
    if arm == ARM_ON:
        # config_for returns {} for anything that is not LOOKUP.
        return TaskClass.AGGREGATE
    if arm == ARM_OFF:
        return TaskClass.LOOKUP
    raise ValueError(f"unknown arm {arm!r}; expected one of on/off/policy")


class Task(object):
    """One question, with everything needed to run and score it.

    `prompt_variable_part` is the SUFFIX. The corpus is the prefix and is passed
    separately to session.Run.fan, so it caches once for the whole fan-out.
    """

    __slots__ = ("cls", "label", "prompt_variable_part", "checker", "ground_truth")

    def __init__(self, cls, label, prompt_variable_part, checker, ground_truth):
        self.cls = cls
        self.label = label
        self.prompt_variable_part = prompt_variable_part
        self.checker = checker
        self.ground_truth = ground_truth

    @property
    def policy_class(self):
        return POLICY_CLASS[self.cls]

    def __repr__(self):
        return f"<Task {self.label} [{self.cls}] truth={self.ground_truth!r}>"


_ONLY = " Reply with the answer and nothing else."


def task_set(corpus=None):
    """Every Track C task, with ground truth computed from the corpus now.

    Returns a list of Task. The corpus is NOT embedded in any prompt here; pass
    the same corpus to session.Run.fan as `shared`.
    """
    c = corpus if corpus is not None else load_corpus()
    t = []

    # -- LOOKUP: named, single-location, verbatim -------------------------
    for name in ("HashedLabelBytes", "labelMaxLen", "minInjectedTokens"):
        v = const_int(c, name)
        t.append(Task(
            LOOKUP, f"lookup.const.{name}",
            f"The source above declares a constant named `{name}` with an "
            f"integer value. Reply with that value as a plain integer, with no "
            f"digit separators." + _ONLY,
            expect_int(v), v))

    field = "RealizedShare"
    owner = struct_declaring_field(c, field)
    t.append(Task(
        LOOKUP, "lookup.struct.owner",
        f"Exactly one struct type in the source above declares a field named "
        f"`{field}`. Reply with the name of that struct type." + _ONLY,
        expect_literal(owner), owner))

    recv = method_receiver_type(c, "noteReads")
    t.append(Task(
        LOOKUP, "lookup.method.receiver",
        "Exactly one method in the source above is named `noteReads`. Reply "
        "with the name of its receiver type, without any pointer marker." + _ONLY,
        expect_literal(recv), recv))

    # -- COUNTING: cardinality of a mechanically defined set --------------
    n = count_lines_starting(c, "func ")
    t.append(Task(
        COUNTING, "counting.func.lines",
        "In the source above, how many lines begin, at their first character, "
        "with the four letters f, u, n, c followed by a single space? Reply "
        "with the integer." + _ONLY,
        expect_int(n), n))

    n = count_lines_starting(c, "type ")
    t.append(Task(
        COUNTING, "counting.type.lines",
        "In the source above, how many lines begin, at their first character, "
        "with the four letters t, y, p, e followed by a single space? Reply "
        "with the integer." + _ONLY,
        expect_int(n), n))

    n = count_substring(c, "json.RawMessage")
    t.append(Task(
        COUNTING, "counting.rawmessage.occurrences",
        "In the source above, how many times does the exact character sequence "
        "`json.RawMessage` occur? Every occurrence is included, including "
        "repeats on the same line. Reply with the integer." + _ONLY,
        expect_int(n), n))

    n = count_lines_starting_and_ending(c, "type ", " struct {")
    t.append(Task(
        COUNTING, "counting.struct.decls",
        "In the source above, how many lines both begin with the five "
        "characters `type ` and end with the nine characters ` struct {`? "
        "Reply with the integer." + _ONLY,
        expect_int(n), n))

    # -- AGGREGATION: two or more located facts, combined by a stated rule -
    a, b, d = (const_int(c, "minInjectedTokens"), const_int(c, "labelMaxLen"),
               const_int(c, "HashedLabelBytes"))
    s = a + b + d
    t.append(Task(
        AGGREGATION, "aggregation.const.sum3",
        "The source above declares constants named `minInjectedTokens`, "
        "`labelMaxLen` and `HashedLabelBytes`, each with an integer value. "
        "Reply with the sum of those three values as a plain integer, with no "
        "digit separators." + _ONLY,
        expect_int(s), s))

    names = ("MinShare", "trimShare", "appliedDrop", "verifyShare")
    fs = sum(const_float(c, x) for x in names)
    t.append(Task(
        AGGREGATION, "aggregation.const.sumfloat",
        "The source above declares constants named " +
        ", ".join(f"`{x}`" for x in names) +
        ", each with a decimal value. Reply with the sum of those four values "
        "as a decimal number written to two decimal places." + _ONLY,
        expect_float(fs, 2), round(fs, 2)))

    e = const_int(c, "minReads")
    v = a - (b * e)
    t.append(Task(
        AGGREGATION, "aggregation.const.mixed",
        "The source above declares constants named `minInjectedTokens`, "
        "`labelMaxLen` and `minReads`, each with an integer value. Subtract "
        "from the value of `minInjectedTokens` the product of the values of "
        "`labelMaxLen` and `minReads`. Reply with the result as a plain "
        "integer, with no digit separators." + _ONLY,
        expect_int(v), v))

    # -- TRANSFORMATION: one located value, one stated deterministic rule --
    fn = func_named_like(c, "HashedPath")
    rev = fn[::-1]
    t.append(Task(
        TRANSFORMATION, "transform.reverse.funcname",
        "The source above declares exactly one top-level function whose name "
        "begins with `HashedPath`. Reply with that function's name written "
        "with its characters in reverse order, preserving the case of every "
        "character." + _ONLY,
        expect_literal(rev), rev))

    fn2 = func_named_like(c, "Sanitize")
    srt = "".join(sorted(fn2))
    t.append(Task(
        TRANSFORMATION, "transform.sort.funcname",
        "The source above declares exactly one top-level function whose name "
        "begins with `Sanitize`. Reply with that function's name written with "
        "its characters reordered into ascending Unicode code point order, as "
        "one string with no separators. Uppercase ASCII letters have lower "
        "code points than lowercase ASCII letters." + _ONLY,
        expect_literal(srt), srt))

    dig = str(const_int(c, "HashedLabelBytes"))[::-1]
    t.append(Task(
        TRANSFORMATION, "transform.reverse.digits",
        "The source above declares a constant named `HashedLabelBytes` with an "
        "integer value. Reply with that value's decimal digits written in "
        "reverse order, keeping every digit including any that becomes "
        "leading." + _ONLY,
        expect_literal(dig), dig))

    # -- MULTI_STEP: dependent hops ---------------------------------------
    fields = struct_fields(c, owner)
    nxt = fields[fields.index(field) + 1]
    t.append(Task(
        MULTI_STEP, "multistep.next.field",
        f"Find the struct type in the source above that declares a field named "
        f"`{field}`. Within that struct type, find the field declared "
        f"immediately after `{field}` in source order, ignoring comment lines. "
        f"Reply with the name of that field." + _ONLY,
        expect_literal(nxt), nxt))

    last = struct_fields(c, recv)[-1]
    t.append(Task(
        MULTI_STEP, "multistep.receiver.lastfield",
        "Find the method named `noteReads` in the source above and read its "
        "receiver type. Find the struct type declaration of that receiver "
        "type. Reply with the name of the last field declared in that struct "
        "type, ignoring comment lines." + _ONLY,
        expect_literal(last), last))

    k = const_int(c, "recentSessions")
    ident = const_block_identifiers(c, "Status")[k - 1]
    t.append(Task(
        MULTI_STEP, "multistep.ordinal.const",
        "Read the integer value V of the constant named `recentSessions` in "
        "the source above. Then find the `const` block whose entries are "
        "declared with the type `Status`. Reply with the identifier of the "
        "V-th entry of that block, where entries are numbered from 1 in source "
        "order and comment lines are not entries." + _ONLY,
        expect_literal(ident), ident))

    return t


# --- running and scoring -------------------------------------------------

def fan_args(run_tasks, corpus, arm):
    """Arguments for session.Run.fan for one (class, arm) cell.

    Returns a dict with task_class, shared, variables, checks and max_tokens,
    matching fan()'s signature exactly. Every task in one call must share a
    class, because fan() takes one task_class for the whole fan-out.
    """
    classes = {t.cls for t in run_tasks}
    if len(classes) != 1:
        raise ValueError(
            f"fan() takes one task_class for the whole fan-out, so one call "
            f"must be one class; got {sorted(classes)}")
    return {
        "task_class": policy_class_for(run_tasks[0].cls, arm),
        "shared": corpus,
        "variables": [t.prompt_variable_part for t in run_tasks],
        "checks": [t.checker for t in run_tasks],
        "max_tokens": MAX_TOKENS,
    }


def score_by_class(rows, labels):
    """Per-class tally from session rows. `labels` maps row id -> class.

    Undecided is a first-class outcome, never a wrong answer:
      TRUNCATED        the response ran out of room. Not an answer.
      REFUSED_BUDGET   never sent.
      ERROR            never answered, or the checker raised.
    Accuracy denominators exclude all of them. A cell whose undecided count is
    material is reported as undecided, not rounded into an accuracy.
    """
    out = {}
    for r in rows:
        cls = labels.get(r.get("id"))
        if cls is None:
            continue
        cell = out.setdefault(cls, {"passed": 0, "decided": 0, "undecided": 0})
        if r.get("outcome") == Outcome.ANSWERED and r.get("passed") is not None:
            cell["decided"] += 1
            cell["passed"] += 1 if r["passed"] else 0
        else:
            cell["undecided"] += 1
    for cell in out.values():
        cell["accuracy"] = (cell["passed"] / cell["decided"]) if cell["decided"] else None
    return out


def review_plan(cls, corpus, arm, tasks=None, workers=32, model="deepseek-flash"):
    """What policy.review says about a planned cell. Advisory, as policy is.

    The class reviewed is the TRUE class, and the kwargs reviewed are the ones
    the forced arm will actually send. Reviewing the forced class against its own
    kwargs is self-consistent by construction and reports nothing, which would
    make every deliberate off-arm look compliant.
    """
    tasks = tasks or [t for t in task_set(corpus) if t.cls == cls]
    return policy.review({
        "task_class": POLICY_CLASS[cls],
        "call_kwargs": policy.config_for(policy_class_for(cls, arm)),
        "prompts": [policy.assemble(corpus, t.prompt_variable_part) for t in tasks],
        "workers": workers,
        "model": model,
        "warm": True,
    })
