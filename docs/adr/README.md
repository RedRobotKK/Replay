# Architecture Decision Records

One file per decision. A record is written when a choice is made that would be expensive to reverse, and it is never edited after acceptance; a later decision supersedes it with a new record.

Files are named `NNNN-short-title.md`. Copy [`template.md`](template.md) to start one.

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-transparent-proxy-first.md) | Ship a byte-transparent proxy before any context transformation | Proposed |
| [0002](0002-replay-engine-and-truth-tiers.md) | Replay engine with calibration gate and two tiers of truth | Proposed |
| [0003](0003-policy-application-constraints.md) | Live policies use only provider-sanctioned mechanisms | Proposed |
| [0004](0004-masking-and-scoped-rehydration.md) | Secret masking with persistent vault and scoped rehydration | Proposed |
| [0005](0005-apache-2-license.md) | License the project under Apache 2.0 | Superseded by 0016 |
| [0006](0006-learning-selection.md) | Learning selection | Accepted |
| [0007](0007-federated-calibration-corpus.md) | Improving the cache model from many machines | Proposed |
| [0008](0008-corpus-at-launch.md) | Collecting a corpus from a public launch without shipping telemetry | Proposed |
| [0009](0009-crowdsourced-waste-and-predictive-guards.md) | Crowdsource the waste taxonomy, not the cache model | Proposed |
| [0010](0010-storage-and-retention.md) | Where waste data lives, and what gets thrown away | Proposed |
| [0011](0011-opt-in-request-rewriting.md) | Opt-in request rewriting | Proposed |
| [0012](0012-dual-licensing-deferred.md) | Dual licensing considered and declined; the CLA stays | Reversed by 0016 |
| [0013](0013-x402-rules-feed.md) | Sell a maintained rules feed over x402; the binary never holds a key | Accepted |
| [0014](0014-checks-must-be-able-to-fail.md) | A check must be able to fail, and reachability is asserted mechanically | Accepted |
| [0015](0015-single-tenant-state-is-a-boundary.md) | Single-tenant state is a boundary, not an implementation detail | Accepted |
| [0016](0016-business-source-license.md) | Relicense under the Business Source License 1.1 | Accepted |
| [0017](0017-the-unintelligent-router.md) | Routing decided by cache structure, not by reading the prompt | Rejected |
| [0018](0018-this-is-an-instrument-not-an-app.md) | Provenance is a field, not a comment: absence, zero and unknown are three values | Accepted |
| [0019](0019-surfaces-declare-what-they-can-be-asked.md) | Surfaces declare what they can be asked, and the declaration is probed | Proposed |
| [0020](0020-compile-the-merge-not-the-branch.md) | Compile the merge, not the branch: two green PRs have put main in the red twice | Accepted |
| [0021](0021-unknown-model-cache-read-multiple.md) | Unknown-model cache-read multiple is two questions: 0.025 for the instrument, unpriced for money | Superseded by 0022 |
| [0022](0022-unknown-model-read-multiple-the-bias-runs-the-other-way.md) | Unknown-model read multiple: the bias runs the other way | Accepted |
| [0023](0023-entitlement-is-a-signed-document-not-an-account.md) | Entitlement is a signed document, not an account | Proposed |
| [0024](0024-deprecation-is-a-promise-made-before-1-0.md) | Deprecation is a promise that has to be made before 1.0 | Proposed |

---

[Documentation index](../README.md) · [Repository README](../../README.md)
