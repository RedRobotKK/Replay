# Alerting on Replay's metrics

Expressions for `/replay/metrics`, written against the metric names that exist. The
[commands guide](commands.md#the-metrics) lists all twenty-one.

Replay ships no `alert.rules.yml`. It does not run Prometheus, has no opinion about your
alerting stack, and a config file for a system it does not operate would be the first thing
in the repository that could not be tested. These are expressions to copy, not a file to
install.

## Two things about the counters before you write a threshold

**They reset when the proxy restarts.** `stats` is in-memory derived data; the ledger is the
durable record. So `some_counter > 0` means "since this process started", not "ever". Use
`increase(...[window])` when you mean recent activity, and treat a bare `> 0` as a statement
about the current process only.

**Only eight carry a label**, none of them a session or a model, so there is no per-session
alerting and no cardinality to manage. `replay_refused_total{guard=...}` is the one you will
reach for most.

## The alerts worth having

### A secret crossed a path the masker does not cover

```promql
increase(replay_unmasked_requests_total[5m]) > 0
```

**Severity: page.** This is the sharpest unguarded edge in v0.2.0. `--mask` understands the
Messages body shape and not `/v1/chat/completions`, so any agent pointed at an
OpenAI-compatible provider is sending secrets in clear while the operator believes masking is
on. The proxy already prints `NOT MASKED` once per path; this catches it when nobody is
reading the log.

Threshold is zero because there is no acceptable rate. If you are deliberately running that
path without masking, silence this alert explicitly rather than raising the number, so the
decision is written down somewhere.

### A dollar cap is not being applied to some traffic

```promql
increase(replay_cost_unpriced_requests_total[15m]) > 0
  and on() (replay_cost_usd_day > 0)
```

The second clause is what makes this actionable rather than noisy: unpriced traffic only
matters for a *dollar* cap. An agent looping on an unpriced model runs up a real bill without
ever reaching the limit. `replay doctor` says the same thing at the terminal.

The fix is not to raise a threshold. Add `--max-day-tokens` or `--max-session-tokens`, which
count whether or not a model can be priced, then `replay rules --update <file|URL>` so the
model has a price at all.

### Requests are being forwarded with every guard inert

```promql
increase(replay_unparsed_requests_total[15m]) > 0
```

A path this build cannot read is proxied byte for byte with no ledger record, no spend cap,
no loop detection and no masking. Worth knowing about even though nothing is broken, because
the guards you configured are not running on that traffic.

### A guard is refusing work

```promql
increase(replay_refused_total{guard="spend_cap"}[1h]) > 0
increase(replay_refused_total{guard="loop"}[1h]) > 0
increase(replay_refused_total{guard="error_budget"}[1h]) > 0
increase(replay_refused_total{guard="circuit_open"}[5m]) > 0
```

Split by guard, because they mean different things. A spend cap firing means the agent stopped
where you told it to, which may be correct. A loop block means the agent was repeating itself.
An error budget trip means a session was mostly failing. An open circuit means the provider is
down and Replay is holding requests so the agent stops burning retries.

### Caching has degraded

```promql
replay_cached_share < 0.90
```

**Pick this number from your own data, not from this page.** Run `replay cost` over your own
sessions and look at what your cached share actually is; 0.90 is an example, and a threshold
copied from someone else's workload will either never fire or never stop. The same reasoning
is why `replay advise --guards` derives spend caps from your spread with a Tukey fence rather
than shipping a default.

### The provider is struggling

```promql
increase(replay_upstream_errors_total[5m]) > 0
increase(replay_retries_total[5m]) / increase(replay_requests_total{class="2xx"}[5m]) > 0.1
```

The second is a ratio rather than a count, so it survives a change in your traffic volume.

## What not to alert on

`replay_request_latency_seconds` measures the **whole round trip including the provider**, so
it is dominated by network and model time and tells you almost nothing about Replay. The
proxy's own overhead is 48µs at the median and 98µs at p99, measured against a local stub; it
is not in this metric. See [`requirements.md`](../requirements.md) section 12, which records
that the requirement asked for proxy overhead and the implementation measures something else.

## Scraping it

The endpoint honours `--token` when one is set and refuses browser origins.

```yaml
scrape_configs:
  - job_name: replay
    static_configs:
      - targets: ["127.0.0.1:4000"]
    metrics_path: /replay/metrics
    # If serve was started with --token, add:
    # authorization: { credentials: "<the token>" }
```

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
