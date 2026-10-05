# Compaction panel, 2026-10-05

**Question.** Should Replay expose a "compaction approaching" signal for
Claude Code sessions, computed as the last prompt total over the compaction
ceiling observed for that model on this machine, and if so where and in what
words. Two adjacent rulings: (B) the same signal for Codex; (C) reopening the
durable scratch substrate, killed three times, as the "preserve state before
compaction" answer.

**Evidence put to the panel.** The measurements in
[capability-map-2026-10-05.md](capability-map-2026-10-05.md): 99 recorded
compactions with a prior reading, per-model pre-compaction totals, 0 of 967
non-compacting sessions reaching 90% of their model's median pre size, lead
times, the two ceilings seen under one model id, Claude Code's own statusline
percentage, Codex's recorded window and its 10 compactions at 0.54 to 0.90 of
it, the three negative continuity rooms, and the project's measurement rules.
The panel was simulated from that evidence alone, without repository access.

**Roles.** Agent architecture; context windows; inference economics;
developer-tools product; observability; security and enterprise architecture;
human-computer interaction; developer productivity research; product-market
fit; CFO and FinOps; skeptical buyer; red team.

## Vote

| Role | Main | (B) Codex | (C) durable scratch |
|---|---|---|---|
| Agent architecture | MODIFY | MODIFY | REJECT |
| Context windows | MODIFY | MODIFY | REJECT |
| Inference economics | MODIFY | MODIFY | REJECT |
| Developer tools | MODIFY | DEFER | REJECT |
| Observability | MODIFY | MODIFY | REJECT |
| Security and enterprise | MODIFY | DEFER | REJECT |
| HCI | MODIFY | DEFER | REJECT |
| Productivity research | MODIFY | MODIFY | REJECT |
| Product-market fit | MODIFY | DEFER | REJECT |
| CFO and FinOps | MODIFY | MODIFY | REJECT |
| Skeptical buyer | REJECT | DEFER | REJECT |
| Red team | MODIFY | DEFER | REJECT |

Main: 11 MODIFY, 1 REJECT. (B): 6 MODIFY, 6 DEFER, a split. (C): 12 REJECT.

## Consensus

Do not ship a "compaction approaching" signal, and change nothing in the
statusline: Claude Code already shows its user a percentage of the window,
and Replay passes it through. Ship a retrospective compaction section whose
figures come from the client's own `compactMetadata`, placed under the
existing overstatement note in `replay context`. An inferred-ceiling line was
allowed only gated on at least ten recorded compactions per model and tier
on the reader's own machine, with three explicit unavailable states (two
ceilings seen under one model id; fewer than ten records; no compaction
recorded), and never as a percentage of a window.

**Minority (skeptical buyer).** Even the inferred line should wait for a
second operator's corpus; one machine cannot source a shipped constant.

**Red-team points accepted by the majority.** The 0-of-255 specificity is
a tautology: every compaction crossed its own 90%, so the figure restates that
the window filled. The 90% threshold was chosen after seeing the answer, so
it is a pre-registration for the next corpus, not a validated predictor. Four
of five models are under n=10. A session showing no number must distinguish
"no compaction recorded" from "tier undetermined" from "too few records".

**Split on (B).** Codex records its window, so pressure is measured; but its
ten compactions sit between 0.54 and 0.90 of it, so compaction there is
configuration-driven and no threshold may be stated. Deferred.

**(C) closed, 12 to 0.** Three evidence rooms and the 2026-10-03
qualification found nothing claimable; this corpus adds no carrier, only a
better measurement of what a boundary costs. The "preserve state" ask is
answered by measuring what the next turn re-sends, not by a substrate.

## What must not be claimed

That compaction is approaching, likely, imminent, expected, or will occur.
Any percentage of the context window for Claude Code: the window is not in
the record. That an observed ceiling is a provider limit or applies to any
other machine. That lead time is a prediction. A threshold for Codex. A
retention percentage until the two definitions below are reconciled.

## Reconciliation the panel asked for, measured the same day

The 2026-09-06 document reported a median retention of 2.55%; the probe
behind the capability map reported post over pre at 8.2%. Measured over 98
boundaries with both figures on this machine: the client's `preTokens` is
the prompt total of the last request before the boundary (ratio 1.003 at the
median), and the client's `postTokens` is what the summary kept (median
26,970, 2.8% of pre), while the first prompt after the boundary is a
different quantity (median 79,383): the kept summary plus the system prompt,
tools and whatever the first turn re-sent. Retention is 2.8% in the client's
own terms; 8.2% was the first prompt after, over the prompt before. Both are
now printed by name, and the difference, median 52k tokens, is marked
calculated.

## Repeat cost at file level, measured the same day

The productivity researcher's item: is "did I have to repeat information
afterward" attributable at file level. Over the 22 boundaries on this
machine with at least five Read calls after them, the share of the first
thirty Reads that were files already read before the boundary is 0.00 at
the median and 0.10 at p90; 18 of the 22 boundaries re-read nothing. So on
this machine the item collapses to the token pair above, as its proposer
said it would: the 52k tokens beyond the kept summary are not re-reads of
files, and their composition is not established here.

## What shipped from this decision

- `replay context` prints one line per recorded compaction: before, kept,
  the kept share, the first prompt after, and the calculated remainder, with
  the three stated absences. Frozen as M139.
- Finding the first prompt after a boundary exposed that every post-compaction
  segment was reported as a sub-agent lane (46 lanes on a 46-compaction
  session with no sidechain record; 10 of 10 compacted sessions on this
  machine). Segments are now marked as the conversation continued and named
  as segments in the note; frozen as M138. Merging them into one lane was
  tried first and withdrawn: `replay advise` then reported a source at 155%
  of prompt tokens, because content from 46 contexts was set against one
  lane's prompts. Each segment is a different context and stays its own lane.
- Not shipped: the inferred-ceiling line. It needs a learner over the home
  corpus and the three unavailable states, and the minority objection stands
  until a second operator's corpus exists.
