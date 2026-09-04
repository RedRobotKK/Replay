# Documentation

Four kinds of document live here, and which one you want depends on why you came.

## If you are using Replay

| | |
|---|---|
| [Getting started](guide/getting-started.md) | The first ten minutes. Install, see what Replay can find, read a session you have already paid for |
| [Commands](guide/commands.md) | Every subcommand, and the flags on `serve` |
| [Troubleshooting](guide/troubleshooting.md) | What goes wrong, what it means, and what to do |

## If you are deciding whether to trust it

| | |
|---|---|
| [Evidence](evidence/) | The measurements behind the claims in the README. Calibration across real sessions, and the latency the proxy adds |
| [Architecture](architecture/) | How the replay engine and the proxy work, including the wire protocol |
| [Decisions](adr/) | Why the design is the way it is. One record per decision, never edited after acceptance |

## If you are contributing

| | |
|---|---|
| [Contributing](../CONTRIBUTING.md) | The whole process, start to finish |
| [Requirements](requirements.md) | What Replay is meant to do, with the build status of each requirement |
| [Roadmap](ROADMAP.md) | What ships in each release, and what is deliberately deferred |
| [Maintainers](maintainers.md) | How the repository is run: branches, reviews, releases, labels |

## If you are curious how it got here

[Design review, 2026-09-02](design-review-2026-09-02.md) is an adversarial pass over the whole design,
kept because its findings shaped what shipped. It is a snapshot of one day, not a description of the
system today. For that, read the architecture.

Earlier drafts are not in the working tree. They are in the git history, where a superseded document
cannot be mistaken for a current one.
