# Documentation

Five kinds of document live here, and which one you want depends on why you came. Every page links
back to this index.

## If you are using Replay

| Document | What is in it |
|---|---|
| [Getting started](guide/getting-started.md) | The first ten minutes. Install, see what Replay can find, read a session you have already paid for |
| [Commands](guide/commands.md) | Every subcommand, and every flag on `serve` |
| [Troubleshooting](guide/troubleshooting.md) | What goes wrong, what it means, and what to do |
| [Alerting](guide/alerting.md) | Prometheus expressions for `/replay/metrics`, and the two thresholds you should derive from your own data rather than copy |

## If you are deciding whether to trust it

| Document | What is in it |
|---|---|
| [Evidence](evidence/README.md) | The measurements behind the claims in the README: calibration across real sessions, the latency the proxy adds, and an adversarial security review |
| [Architecture](architecture/README.md) | How the replay engine and the proxy work, including the wire protocol |
| [Decisions](adr/README.md) | Why the design is the way it is. One record per decision, never edited after acceptance |
| [Surfaces](SURFACES.md) | Every file, socket and process Replay touches, each row marked verified, read, or unknown |

## If you are contributing

| Document | What is in it |
|---|---|
| [Contributing](../CONTRIBUTING.md) | The whole process, start to finish |
| [Requirements](requirements.md) | What Replay is meant to do, with the build status of each requirement |
| [Roadmap](ROADMAP.md) | What ships in each release, its gate, and what is deliberately deferred |
| [Agent surface](AGENT-SURFACE.md) | What an agent-first CLI does that Replay does not, and the four gaps that are real |
| [Maintainers](maintainers.md) | How the repository is run: branches, reviews, releases, labels |
| [Audit outreach](AUDIT-OUTREACH.md) | What to say to someone who has not asked for anything yet, and the rule that the offer is always a measurement rather than a claim about their systems |
| [Flag surface](TUI-FLAG-SURFACE.md) | All 72 flags, and the six kinds of screen element they become. Twenty of them are one component with four states |
| [Dashboard design](DASHBOARD-DESIGN.md) | The live `replay serve` surface: the 25 states it has to survive, and why every frame element is ASCII |

## If you want to know where this is going

Dated arguments, not commitments. Each one was written to settle a specific question, and the roadmap
is the only place that says what will actually ship.

| Document | The question it settles |
|---|---|
| [Product direction](PRODUCT-DIRECTION.md) | The eight diagnostics already implemented are one waste taxonomy reported as eight unrelated numbers |
| [What counts as waste](WASTE-DEFINITION.md) | A definition Replay can actually compute, and the two of five categories that survive it |
| [What you actually get](WHAT-YOU-GET.md) | What pointing Replay at your sessions gives you today, and what it cannot recommend without reading your config |
| [Token prices](TOKEN-PRICES.md) | Whether a first-party machine-readable price source exists. It does not, which is why the table is hand-maintained and dated |
| [The feed worth publishing](THE-FEED.md) | Why the useful feed is verification of documented cache behaviour, not prices |
| [The money path](MONEY-PATH.md) | What a subscription could honestly charge for, what stays free forever, and why the unit is a repository rather than a developer |
| [Open design questions](design/README.md) | Questions written up before a decision, not after. Currently: where live context cost should surface and who consumes it |

## If you are curious how it got here

[Design review, 2026-09-02](design-review-2026-09-02.md) is an adversarial pass over the whole design,
kept because its findings shaped what shipped. It is a snapshot of one day, not a description of the
system today. For that, read the architecture.

Earlier drafts are not in the working tree. They are in the git history, where a superseded document
cannot be mistaken for a current one.

---

[Repository README](../README.md)
