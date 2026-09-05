# Is there a source of truth for token prices?

**2026-09-04. Probed rather than assumed.**

## The short answer: no first-party machine-readable one exists

| Source | Result |
|---|---|
| `api.anthropic.com/v1/models` | **401.** Needs a key, and the models endpoint is a catalogue, not a price list |
| `docs.anthropic.com/…/pricing` | **200, but 656 KB of HTML.** Human-readable, not machine-readable |
| `anthropic.com/pricing` | **200, 1.1 MB of HTML.** Same |
| **The API response itself** | Returns **token counts, never cost.** There is no `cost` field to read |

**Prices are published for people to read, not for programs.** Every price table in every tool that
reports dollars is hand-maintained or scraped, and every one of them goes stale silently.

## Third-party aggregators exist, are machine-readable, and are accurate

| Source | Result |
|---|---|
| **OpenRouter `/api/v1/models`** | **200, JSON, 27 Anthropic models** with `prompt`, `completion` and `input_cache_read` per token. No key needed |
| **LiteLLM `model_prices_and_context_window.json`** | **200, 2.1 MB JSON**, community-maintained |

**Cross-checked against Replay's hand-maintained table, and every row agrees:**

| Model | Replay, in/out per M | Live source | Agree |
|---|---:|---:|:---:|
| claude-fable-5.1 | 10 / 50 | 10 / 50 | yes |
| claude-opus-5 | 5 / 25 | 5 / 25 | yes |
| claude-sonnet-5 | 2 / 10 | 2 / 10 | yes |
| claude-haiku-4.5 | 1 / 5 | 1 / 5 | yes |

So the table is currently correct. **It is dated `2026-06-24`, which is 72 days ago, and nothing in
the tool notices when it stops being correct.**

## The bug this exposes, and it defeats exactly the feature you described

`listCost` (`internal/proxy/server.go:846`) returns **zero for any model the price table does not
know**, and the flag help says so plainly: *"models not in the price table count as free"*.

So **"keep my budget under $20" silently stops working** for:

- Any model released after 2026-06-24.
- `opus-4-5`, and bare `sonnet`, `opus-4` and `haiku`, all carried in the table with `priced: false`.

**The failure direction is the worst available.** An unknown model is usually a *new* model, new
models are usually more expensive, and the cap treats them as costing nothing. **The guard fails
open, silently, precisely when it matters most**, and the user finds out from the invoice, which is
the exact experience the product exists to prevent.

## What to do about it

**1. An unpriced model must never count as zero.** Two defensible options, and today's behaviour is
neither: refuse the request and say the model is unpriced, or price it at the most expensive known
row and label the figure an upper bound. **Fail conservative, and say which.**

**2. Recommend the token cap for anyone who actually needs a limit.** `--max-session-tokens` needs no
price table, cannot go stale and cannot silently fail. **A dollar cap is a token cap with a lossy
conversion bolted on**, and the conversion is the part that rots.

**3. Do not fetch prices at runtime.** It would put a third-party host in the request path of a tool
whose whole claim is that it talks only to your provider. If prices are ever refreshed from
OpenRouter or LiteLLM, it belongs in **CI, as a pull request that bumps the table and its date**, with
a human reading the diff. That also gives the staleness a visible owner.

**4. Say the table is stale when it is.** `doctor` knows today's date and the table's date. **A price
table 72 days old should say so** next to every dollar figure it produces.

## The honest framing for "keep my budget under $20"

It is a reasonable thing to want and **Replay can only approximate it**. List price is not your price:
it ignores discounts, batch rates, negotiated terms, and it is meaningless on a flat subscription
where the marginal cost of a token is zero.

**What Replay can say truthfully is: at published list prices, on a table dated X, this session would
have cost about Y.** That is useful for a team on API billing and close to meaningless for a
subscriber. **The token cap is the honest primitive; the dollar cap is a convenience built on a
number nobody publishes in a form a program can read.**

---

[Documentation index](README.md) · [Repository README](../README.md)
