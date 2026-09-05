# Architecture Decision Records

One file per decision. A record is written when a choice is made that would be expensive to reverse, and it is never edited after acceptance; a later decision supersedes it with a new record.

Files are named `NNNN-short-title.md`. Copy [`template.md`](template.md) to start one.

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-transparent-proxy-first.md) | Ship a byte-transparent proxy before any context transformation | Proposed |
| [0002](0002-replay-engine-and-truth-tiers.md) | Replay engine with calibration gate and two tiers of truth | Proposed |
| [0003](0003-policy-application-constraints.md) | Live policies use only provider-sanctioned mechanisms | Proposed |
| [0004](0004-masking-and-scoped-rehydration.md) | Secret masking with persistent vault and scoped rehydration | Proposed |
| [0005](0005-apache-2-license.md) | License the project under Apache 2.0 | Accepted |
| [0006](0006-learning-selection.md) | Learning selection | Accepted |
| [0007](0007-federated-calibration-corpus.md) | Improving the cache model from many machines | Proposed |
| [0008](0008-corpus-at-launch.md) | Collecting a corpus from a public launch without shipping telemetry | Proposed |
| [0009](0009-crowdsourced-waste-and-predictive-guards.md) | Crowdsource the waste taxonomy, not the cache model | Proposed |
| [0010](0010-storage-and-retention.md) | Where waste data lives, and what gets thrown away | Proposed |
| [0011](0011-opt-in-request-rewriting.md) | Opt-in request rewriting | Proposed |
| [0012](0012-dual-licensing-deferred.md) | Dual licensing considered and declined; the CLA stays | Accepted |
| [0013](0013-x402-rules-feed.md) | Sell a maintained rules feed over x402; the binary never holds a key | Accepted |

---

[Documentation index](../README.md) · [Repository README](../../README.md)
