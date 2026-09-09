# Design

Open design questions, written up before a decision is made rather than after.

These are not the architecture (which describes what exists) and not the ADRs
(which record what was decided and are never edited). A document lives here
while the question is still open. When it is settled, the decision goes to an
ADR and the built thing goes to the architecture.

| Document | The question it opens | State |
|---|---|---|
| [Live context visibility: the options](P3-VISIBILITY-OPTIONS.md) | Where live context cost should surface, and who consumes it: the `serve` dashboard, the status line, a tool the agent calls, or a published shape with no consumer | Open. Four options, no recommendation, three checkable questions that decide between them |
| [**Built-but-unwired: running log**](UNWIRED-LOG.md) | The live register of capabilities that exist, pass tests, and cannot be reached by a user, and what was done about each | **Nine confirmed before the audit, plus five adjacent defects.** Guarded from 2026-09-09 |
| [Surface taxonomy 1: the request path](surface-taxonomy-1-request.md) | Every surface from the listen socket to the provider | 49 surfaces, 21 alterable, 11 untested pass conditions |
| [Surface taxonomy 2: response and ledger](surface-taxonomy-2-response.md) | The provider's first byte to what lands on disk | 41 surfaces, 18 alterable, 16 untested, 9 failing silently |
| [Surface taxonomy 3: per provider](surface-taxonomy-3-providers.md) | 48 surfaces x 7 providers, and where a shared path assumes something only one provider does | 93 of 116 cells are facts about Replay; only 17 measured |
| [Surface taxonomy 4: rewriting](surface-taxonomy-4-rewriting.md) | What content could be reshaped, and where the safe line falls | 13 transforms, 4 provably safe. "Better accuracy" is not a claim this tool can make |
| [Unwired 1: exported symbols](unwired-1-symbols.md) | Exported symbols nothing in production calls | **111 unwired of 798, and 82 have tests** |
| [Unwired 2: configuration](unwired-2-config.md) | Config, flags and env vars nothing sets, and docs promising settings that do not exist | 3 DEAD, 10 PHANTOM, 4 UNDOCUMENTED |
| [Unwired 3: branches and docs](unwired-3-branches-and-docs.md) | Unreachable branches, unproducible values, and documentation promising what the code cannot do | **49 unreachable identifiers, 41 with passing tests.** 8 overpromised claims |
| [Quota estimator: blue](quota-estimator-blue.md) | What the captured rate-limit headers can honestly answer for a subscription seat | Forecast yes, avoidable-share refused |
| [Quota estimator: red](quota-estimator-red.md) | A hostile review of the quota claim | Five findings rated thread-defining; its lead retracted an evidence document of this repository's own |
| [Quota data census](quota-data-census.md) | What rate-limit data actually exists on this machine | **Zero of 17 ledger records carry any quota header** |
| [CLI tool integration survey](cli-tool-integration-survey.md) | Which AI CLIs are actually installed on this machine, and which of four integration routes each offers | **Five installed CLIs were missing from the original list**, one "not installed" verdict was wrong, and Grok turns out to be a spend surface the census had written off |

---

[Documentation index](../README.md) · [Repository README](../../README.md)
