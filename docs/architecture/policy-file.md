# Policy file

What `buffy learn` writes and what the proxy will read. One file, `~/.buffy/policy.json`, owner-only, human-readable, diffable. It holds numbers and names, never content.

## Shape

```json
{
  "schema": 1,
  "generated": "2026-09-03T02:40:00Z",
  "rules": "anthropic-2026-09",
  "sessions": { "found": 42, "calibrated": 39, "holdout": 12 },
  "candidates": [
    {
      "name": "context-edit(keep=6,trigger=200000)",
      "family": "context-edit",
      "live": "buffy serve --context-edit-trigger 200000 --context-edit-keep 6",
      "context_edit": { "KeepLast": 6, "TriggerTokens": 200000 },
      "sessions": 17,
      "holdout_sessions": 5,
      "mean_saving": 0.18,
      "interval": [0.12, 0.24],
      "holdout_mean_saving": 0.15,
      "cached_share_delta": -0.03,
      "estimated": true,
      "decision": "selected"
    }
  ],
  "selected": { "name": "context-edit(keep=6,trigger=200000)", "family": "context-edit", "live": "..." },
  "reason": ""
}
```

## Fields

- `schema` is bumped on any incompatible change; a reader refuses a schema it does not know.
- `rules` is the provider rules version the simulator used (`cachemodel.RulesVersion`). A file written under other rules is stale.
- `sessions.found` is every transcript or ledger file seen; `calibrated` those whose replay reproduced the provider's reads well enough to simulate; `holdout` those kept out of selection.
- Each candidate's `decision` is `selected` or a `rejected: ...` string naming the rule that fired (ADR-0006). `mean_saving` and `interval` are the training-set saving as a share of as-run effective tokens and its two-standard-error band; `holdout_mean_saving` is the saving on sessions selection never saw.
- `estimated` is true when the score depends on the byte-to-token fit (context editing); TTL candidates are measured.
- `selected` is null with a `reason` when nothing qualified. That is the expected answer on a small corpus.

## Use

The proxy does not read the file yet (roadmap: PX-8). Until then the selected candidate's `live` field is the command or client setting to apply by hand.
