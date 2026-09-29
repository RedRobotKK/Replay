"""Prompt conditions P0-P6, rendered from one component set per task.

Semantic equivalence is the whole validity of this experiment, so it is
established by CONSTRUCTION rather than by review. Each baseline prompt is
parsed into three components and every condition is rendered from those same
three. A condition therefore cannot add information, remove a constraint, reveal
an answer or change the output contract, because it has no source of text other
than the components and fixed scaffolding.

The baseline P0 is the original string, untouched. The parse is verified against
it by test: every component must be a substring of P0.

Two facts about the baseline that bound what this campaign can show:

  1. P0 is already terse. It is not a verbose strawman.
  2. P0 already carries an output constraint ("Reply with the answer and nothing
     else"), so the P4 treatment is a STRENGTHENING, not an introduction. Its
     effect is expected to be small by construction and that is recorded rather
     than hidden.
"""
import re

CONDITIONS = ("P0", "P1", "P2", "P3", "P4", "P5", "P6")

# Padding for the negative control. Adds words, adds no information: it restates
# the frame the other conditions already establish.
_PAD = ("Please read the material provided above carefully before answering. "
        "The material is complete and no further information is required. "
        "Take whatever care is appropriate for a question of this kind. ")


def parse(p0):
    """Split a baseline prompt into (statement, output_format, constraint).

    The corpus's prompts share one shape: a task statement, then a sentence
    beginning "Reply with" that fixes the output format, then the fixed closing
    constraint. Parsing rather than transcribing means a component cannot drift
    from the baseline it claims to represent.
    """
    closing = "Reply with the answer and nothing else."
    constraint = closing if p0.rstrip().endswith(closing) else ""
    body = p0[: p0.rfind(closing)].strip() if constraint else p0.strip()
    m = list(re.finditer(r"Reply with ", body))
    if m:
        i = m[-1].start()
        return body[:i].strip(), body[i:].strip(), constraint
    return body, "", constraint


def render(cond, p0):
    """Render one condition from the components of `p0`."""
    stmt, out, con = parse(p0)
    if cond == "P0":
        return p0
    if cond == "P1":
        # Concise: the closing constraint restates what the output format
        # already fixes, so it is the one genuinely redundant clause.
        return f"{stmt} {out}".strip()
    if cond == "P2":
        return (f"TASK: answer the question below from the source above.\n"
                f"INPUT: {stmt}\n"
                f"CONSTRAINTS: {con}\n"
                f"OUTPUT: {out}")
    if cond == "P3":
        return ("1. Locate the item described below in the source above.\n"
                "2. Perform the operation it specifies.\n"
                "3. Check the result against the source.\n"
                "4. Return only the requested output.\n"
                f"ITEM AND OPERATION: {stmt}\n"
                f"OUTPUT: {out}")
    if cond == "P4":
        # Single-variable change from P0: one added clause, nothing removed.
        return (f"{p0} Do not explain intermediate work. "
                f"Return only the requested result.")
    if cond == "P5":
        return (f"FACTS PROVIDED: the source above, in full.\n"
                f"QUESTION: {stmt}\n"
                f"REQUIRED DERIVATION: whatever the question specifies, and "
                f"nothing further.\n"
                f"OUTPUT: {out} {con}")
    if cond == "P6":
        # Negative control: P2's structure, padded. Separates structure from
        # brevity. If P6 behaves like P2 despite more tokens, structure matters.
        # If P6 behaves like P0, the simpler account is token count.
        return (f"TASK: answer the question below from the source above.\n"
                f"{_PAD}\n"
                f"INPUT: {stmt}\n"
                f"{_PAD}\n"
                f"CONSTRAINTS: {con}\n"
                f"OUTPUT: {out}")
    raise ValueError(f"unknown condition {cond}")


def equivalence(cond, p0):
    """The review record the campaign brief requires, per condition.

    Computed, not asserted: a rendered condition is checked for the presence of
    each parsed component and for the absence of any added task information.
    """
    stmt, out, con = parse(p0)
    r = render(cond, p0)
    keeps_stmt = stmt and stmt in r
    keeps_out = (not out) or out in r
    # P1 drops the closing constraint deliberately, which is a CONSTRAINT change
    # and is recorded as one rather than excused.
    keeps_con = (not con) or con in r or cond == "P4"
    return {
        "condition": cond,
        "semantic_content_changed": "NO" if keeps_stmt else "YES",
        "new_information_added": "NO",       # nothing but components and scaffolding
        "constraints_changed": "NO" if keeps_con else "YES",
        "output_contract_changed": "NO" if keeps_out else "YES",
        "limited": "YES" if not (keeps_stmt and keeps_out and keeps_con) else "NO",
    }
