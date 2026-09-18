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

| Model | Replay, in/out per M | Replay, cache read | Live source, in/out | Agree |
|---|---:|---:|---:|:---:|
| claude-fable-5.1 | 10 / 50 | 0.025x | 10 / 50 | yes |
| claude-opus-5 | 5 / 25 | 0.10x | 5 / 25 | yes |
| claude-sonnet-5 | 2 / 10 | 0.10x | 2 / 10 | yes |
| claude-haiku-4.5 | 1 / 5 | 0.10x | 1 / 5 | yes |

**Cache read is 0.10x of input for every model except two.** Anthropic's Fable 5.1 and Mythos 5.1
read cache at 0.025x. A tool that applies one multiplier to every row overstates the cache read on
those two by a factor of four, which is why the column is in the table rather than in a constant.

So the table is currently correct. **It is dated `2026-06-24`, and nothing in the tool notices when
it stops being correct.** Age is what rots, not the date: that table was 85 days old on 2026-09-17.
Read the age off `replay doctor`, which computes it against today's date, rather than off a number
written into this file.

## The bug this exposes, and it defeats exactly the feature you described

`listCost` (`internal/proxy/server.go:846`) returns **zero for any model the price table does not
know**, and the flag help says so plainly: *"models not in the price table count as free"*.

So **"keep my budget under $20" silently stops working** for:

- Any model released after 2026-06-24.
- **Every OpenAI model.** The compiled table is Anthropic only, so on an OpenAI Codex corpus the cap
  fails open across the board: `gpt-6-astra`, `gpt-5.6-terra`, `gpt-5.4` and `gpt-5.4-mini` all
  count as free until you install `docs/rules/openai-2026-09-15.json` with `replay rules --update`.
  Installing a rules document replaces the table in effect rather than merging into it, so while
  that one is in effect the Anthropic rows are the unpriced ones.
- **`gpt-5.1-codex-mini`, which cannot be priced at all.** It is the most common model on a real
  Codex machine and OpenAI publishes no rate for it, so no rules document fixes this one.
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
table this far past its date should say so** next to every dollar figure it produces, with the age
computed at run time rather than written down.

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
