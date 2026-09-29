# What the documentation already said

2026-09-29. Read after the measurement campaign, which is the wrong order and is
recorded as such. Sources: `api-docs.deepseek.com` guides for KV cache,
reasoning model, pricing, and rate limits.

Three categories: what the docs already state (and we spent money confirming),
what the docs leave open (where measurement was the only route), and what we are
relying on that the docs do **not** cover (the fragile part).

## Confirmed, and we did not need to buy it

**Pricing.** The published table matches `experiment/harness/pricing.py` exactly,
including the peak windows 01:00-04:00 and 06:00-10:00 UTC Mon-Fri excluding
Chinese public holidays, and off-peak at half. No correction needed.

**Cache TTL.** "Usually within a few hours to a few days." Already recorded as
NOT MEASURED; the docs say the same thing we would have had to spend days to
observe.

**Full-prefix matching.** "Fully match" is required; partial matches do not hit.
Our leading-space result is a concrete demonstration of a documented rule, not a
discovery. It is still worth having as a worked example, because "fully match"
does not obviously tell a reader that one space at the front is fatal.

## Corrected by the docs

**DS-CONC said "no ceiling found at 64".** The docs state hard concurrency
limits: **2,500 concurrent connections for `deepseek-flash`** and **500 for
`deepseek-v4-pro`**, with HTTP 429 when exceeded. "No ceiling found" was true and
badly framed: we tested 64 against a documented limit of 2,500 and reported the
absence of a wall we were nowhere near. The ledger row is corrected.

This also means the fan-out is sized about 39x below what the account allows.
`workers=32` was chosen out of caution against an unknown limit that is in fact
published.

**A `user_id` request parameter exists** for partitioning concurrency within an
account. The harness does not send it. Not a defect yet, but it is the documented
mechanism for isolating a runaway experiment from everything else on the key.

## Measured because the docs leave it open

**The 128-token block.** The KV cache guide says the system "will carve out cache
prefix units at fixed token intervals" and **gives no interval**. The mechanism
is documented; the number is not. Our 17-rung result quantifies exactly the
parameter the documentation declines to state, including the "final block is
never served" behaviour, which is not described at all.

This is the one finding that would have been impossible to obtain by reading.

**Cache-hit pricing against money.** The rate is published; whether the biller
applies it is not something a document can establish. DS-F5 remains worth its
$0.04.

## Undocumented, and therefore fragile

**`reasoning_effort: "none"` and `thinking: {"type": "disabled"}` are not in the
documentation.** The docs show only the enabling direction: `reasoning_effort:
"high"` and `thinking: {"type": "enabled"}`.

Both disabling shapes work, measured. Neither is promised. An undocumented
parameter value can be changed or dropped without notice, and the failure would
be silent: reasoning switches back on, output tokens rise about 20x on lookups,
and nothing errors.

Any recommendation built on this needs a runtime check rather than a one-time
measurement. The cheap one already exists in the harness: `reasoning_tokens` is
reported per call, so an assertion that it is zero when the parameter was sent
turns a silent regression into a loud one. **Not yet implemented.**

**The cache is "best-effort".** The guide says so explicitly. The 128-token block
model is therefore a description of current behaviour, not a contract, and
`fanout.MIN_WARM_PREFIX_TOKENS` is tuned to an observation that the provider has
not promised to preserve.

## The process failure

This reading should have preceded the campaign. It would not have saved most of
the spend -- the block size, the disable shapes, and the billing check are all
things the documentation does not settle -- but it would have saved the
concurrency framing, and it would have flagged the reasoning parameters as
undocumented *before* they were written into a recommendation rather than after.

Total campaign spend to this point: $0.18 observed. The reading cost nothing.
