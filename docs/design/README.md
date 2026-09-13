# Design

Open design questions, written up before a decision is made rather than after.

These are not the architecture (which describes what exists) and not the ADRs
(which record what was decided and are never edited). A document lives here
while the question is still open. When it is settled, the decision goes to an
ADR and the built thing goes to the architecture.

| Document | The question it opens | State |
|---|---|---|
| [Qualifying a forensics week, before anyone pays](forensics-week-qualification.md) | What has to be true before a week is worth selling, and how to find out quickly that it is not | Open. Three commands the prospect runs themselves; three questions unresolved until a first sale |
| [Operating 500 to 1,000 repositories, as one person](operating-at-scale.md) | What it takes to run the revenue the money path sizes, and what breaks first | Open. Seven gaps, ranked by what fails soonest. The binding one is that the paid capability does not exist; the unresolved one is that entitlement issuance either leaves a ten month revocation lag or puts the signing key on a robot |
| [The forensics week: what the customer actually receives](forensics-week-deliverable.md) | What a paid week hands over, and what the finding may not claim | Open. Shape proposed before the first sale; five questions a first customer settles |
| [Live context visibility: the options](P3-VISIBILITY-OPTIONS.md) | Where live context cost should surface, and who consumes it: the `serve` dashboard, the status line, a tool the agent calls, or a published shape with no consumer | Open. Four options, no recommendation, three checkable questions that decide between them |
| [A reference distribution](reference-distribution.md) | Whether Replay should carry published population figures the way it carries published provider figures, so one machine can be read against many while the contributed pool still has one member | Open. Two 2026 publications supply the first populations; the corpus tested here differs from them by 2x on system-prompt share and 8x on conversation history. The comparison must be derived, never declared, and every difference must carry the population's definition beside it |
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
| [Doctor discovery: beginner lens](doctor-discovery-lens-beginner.md) | A first-time reader on the new `agents` block: what confused them, and whether they would have typed anything next | **They would have closed the terminal.** Found the Codex undercount that two other reviews missed |
| [Doctor discovery: junior lens](doctor-discovery-lens-junior.md) | Could a new maintainer add a fifth agent unaided, and what would they break first | `Priceable` had no consumer; `knownStores` was the fifth parallel store-location list |
| [Doctor discovery: senior lens](doctor-discovery-lens-senior.md) | A staff review: correctness, failure modes, and which tests would still pass if the feature were quietly broken | **Three blockers, and eleven of fourteen mutations escaped the original tests** |
| [Doctor discovery, read by a new maintainer](doctor-discovery-lens-junior.md) | Whether `doctor`'s agent-discovery block can be safely changed by someone who has never seen it: what a fifth `knownStores` entry forces you to guess, and what no test would catch | **`Priceable` has no consumer**; Ollama's file count disagrees with the command it recommends; a fifth parallel store-location list |
| [**Vacuous tests: what would still pass if it broke**](vacuous-test-audit.md) | The adjacent question to the unwired audits: not what is unreachable, but what is checked and unfalsifiable | **20 vacuous tests; 144 of 239 refusal sites never execute.** Five escaping mutations verified by running them |
| [**Replay Watch**](replay-watch.md) | One record per session per repository, carried to whoever owns the invoice, so that "spend on this repository went up and nobody can say which week" has an answer | **Step 1 of 3 built 2026-09-13: the record and its opt-in.** Counts only, enforced on the wire form; consent is a third grant rather than a flag on the corpus one, and unset is refused as declined is. Nothing in a release emits one |
| [Benefit gap analysis](benefit-gap-analysis.md) | Everything here tests whether the code does what it says; this maps what is untested: whether the tool helps | **The verified status was circular** — the drop was both the evidence and the measurement. `EstimateTokens` has no tests and 49% median error |

---

[Documentation index](../README.md) · [Repository README](../../README.md)
