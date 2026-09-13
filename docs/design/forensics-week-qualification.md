# Qualifying a forensics week, before anyone pays

**2026-09-12.** Companion to
[the deliverable](forensics-week-deliverable.md), which describes what a paid
week hands over. This describes what has to be true before the week is worth
selling — and, more often, how to find out quickly that it is not.

Nobody has bought a week, so this is a procedure proposed rather than observed.

## The problem

The deliverable doc says **Day 1 can end the engagement**: a corpus that cannot
answer the question is itself the finding, delivered, with the remaining days
unbilled.

That is honest and it is the wrong place for two of the three checks. A review
put it plainly: it converts a free qualifying question into an unpriced refund
liability, which is the seller absorbing risk nobody asked them to absorb. Two
of the three abort conditions are answerable in **one email and three commands
the prospect runs themselves**, before money is discussed.

The third — whether their corpus calibrates well enough to attribute causes —
genuinely needs the tool and genuinely belongs on Day 1.

## What the prospect runs

Three commands, on their machine, reading files they already have. **Nothing
leaves. No account, no upload, no key.** They can read the source of all three
first, and should.

```sh
replay doctor     # what is here, and can any of it be priced
replay cost       # what it cost, and whether that is money to anyone
replay corpus     # does it calibrate well enough to attribute causes
```

They send back **three lines** — not the transcripts, not the ledger, not the
per-session breakdown. The three lines below are enough to know whether a week
is worth either side's time.

## Reading the answers

### 1. Is there a corpus at all — `replay doctor`

It prints a `transcripts` line naming session and lane counts, and an `agents`
line saying how many surfaces can be priced:

```text
transcripts   126 sessions across 12 projects
              1893 transcript files in all
agents        2 of 4 can be priced; the rest are conversation only
```

**Disqualifies now:** no transcripts, or every surface conversation-only (for
example a Cursor-only shop — Cursor transcripts carry no usage fields, so there
is no spend to read).

### 2. Is any of it money — `replay cost`

This is the question that decides everything and it is answered in one line the
tool already prints. `replay cost` names the **route**, and claims a billing
mode only where the model id settles it — Bedrock and Vertex ids are metered by
construction; a bare first-party id is emitted whether the caller holds an API
key or a subscription, so it is reported and not claimed.

```text
total          $11696.09
re-billed      $329.68  (3% of the total)
               69.0M tokens re-billed
```

Beneath it, in every run, the tool says the thing a seller would rather it did
not:

> On a subscription seat — Claude Pro or Max, Copilot, Cursor — none of that is
> money: you are not billed per token, so the dollars above are list price for
> someone who is.

**Disqualifies now: a flat-seat shop.**
[`MONEY-PATH.md`](../MONEY-PATH.md) measured the recoverable figure for a
subscriber at **zero dollars**, not a small number, and the attempt to bill
against rate-limit budget instead moved 3.09M tokens and shifted the utilisation
counter by **zero** steps. There is no second currency. Say so in the first
email and do not sell them a week.

**Worth a conversation:** metered spend where `re-billed` is a figure their
finance function would notice. What counts as noticeable is theirs to say, not
ours to assert — and the honest framing is that `re-billed` is **what was
already spent twice**, never a forecast of what a change recovers.

### 3. Can causes be attributed — `replay corpus`

This is the one that needs Day 1 and the tool. ADR-0002's calibration gate
refuses to score a corpus that does not reproduce at or above the match bar, and
`replay apply` already declines with *"the calibration for this corpus is not
good enough to act on"*.

**Ends the engagement on Day 1, delivered as the finding:** a corpus that does
not calibrate. The dollars are still real; what cannot be done is attach causes
to them, and a week that cannot say *why* is not the week that was sold.

## What we need before Day 0

- **Their redaction, done by them.** `replay redact` runs on their machine. We
  receive nothing until they have run it and read the output.
- **The three lines above**, not the corpus.
- **Their billing basis in writing** — metered, flat-seat, or mixed — because
  the route line narrows it and does not settle it for first-party ids, and
  every dollar figure in the finding is qualified by it.
- **A named owner** who can act on the result. A finding delivered to nobody
  changes nothing, and the drift the week is about is configuration drift, which
  has an owner or does not get fixed.

## What this is not

It is not a sales call with a tool attached. Every command above is free,
already shipped, and runs without us — which is the point, and is the same
reason the week itself cannot be sold as a subscription. If the three lines say
there is nothing here, **the correct outcome is that we say so and nobody
pays**, and that is a cheaper answer for both sides than finding out on Day 1 of
a billed week.

## Open

1. Does a prospect actually run three commands before a first call, or does that
   ask too much before trust exists? The alternative is running them on a shared
   screen, which is slower and answers the same questions.
2. Is the calibration check moveable earlier? It needs the corpus, and the
   corpus needs redaction, and redaction is the step with wall-clock cost.
3. What `re-billed` figure is worth a week is unknown, because nobody has sold
   one. It should be recorded after the first, not guessed before it.
