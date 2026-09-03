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
- `trial`, present when the ledger holds sessions the proxy treated or held out as controls, judges the live trial: each arm's mean cost per token of new content with its band, the realized saving of treated over control, the predicted saving, and whether the policy graduated. Graduation needs five sessions per arm, arms separated above noise, and a realized saving of at least half the prediction.
- `types` holds one selection per session type under the same rules. A type is `<model family>/<small|large>-prefix`, both known at a session's first request: the model id's family and whether the first prompt was at least 20k tokens. A type with no selection falls back to the overall one; the overall selection stays null when the types disagree.

## Use

`buffy serve --policy-file ~/.buffy/policy.json` reads the file at each session's first request and applies the selected candidate when it is one the proxy can apply (the context-edit family; a TTL selection is a client setting and is logged as advice). The decision and its parameters are pinned for the session and written to `.pins` under the ledger directory, one JSON line per session, so a rewritten file or a restarted proxy never changes a running session. An explicit `--context-edit-trigger` wins over the file. A file from another schema or rules version applies nothing and says so in the log. With `--trial-share` below one, only a stable share of new sessions gets the selection and the rest are pinned as controls; with `--guardrail-reread` set, enough treated sessions breaching the re-read rate write a `.revert` record next to `.pins`, and new sessions run without the policy until a file with a newer `generated` time replaces it.
