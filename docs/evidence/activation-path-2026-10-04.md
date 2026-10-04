# The self-serve activation path: three repairs and one transport measurement

**2026-10-04. Items 1 to 4 of the activation gate. No telemetry, no
recruitment, no change to Experiment 9 or 10, no change to the frozen
usage-absence work (QT-7a, 7b, 7c), no Responses parsing. The Responses work
is called R-1 here and was not implemented.**

## What a stranger met before this change

The installer opened `replay tui` on the cost screen. A Claude Code user with
transcripts saw a cost report at once. Anyone else saw "No transcripts to add
up" and one note, an environment variable for transcripts kept elsewhere; the
hand-off to `replay serve` lived in the no-argument command and in `doctor`,
not on that screen. `replay serve` printed one client variable,
`ANTHROPIC_BASE_URL`, and nothing about OpenAI-compatible clients, Aider, or
the one path it forwards unread. The guide's serve section said the same. A
Codex CLI user who pointed the tool at the proxy saw traffic flow and got an
empty report with no error, because Codex speaks `/v1/responses`.

## Item 1, the banner

`replay serve`'s first screen now names the variable each client reads
(`ANTHROPIC_BASE_URL`, `OPENAI_BASE_URL`, `OPENAI_API_BASE` for Aider), the two
request shapes this build reads, guards and records, and says that
`/v1/responses` is forwarded unread with no ledger record and no cap, secrets
still masked.

- RED: a black-box test started the shipped binary and read its banner; it
  lacked every line but the Anthropic one.
- GREEN: the banner is one function, `serveBanner`, called where the old
  string was. Routing and admission untouched; the only change to the proxy
  package is an exported name for the Responses path so the banner cannot
  retype it.
- Mutation: M125 makes the banner claim the Responses path is read; killed by
  the unit test through the registered harness.
- Black box: `TestBB_ServeBannerNamesEveryClientAndWhatItCannotRead` reads
  the lines from the built binary as a child process.

## Item 2, the empty cost screen

The screen now carries, in its notes, `replay serve` as the live path for any
agent, and, when another agent's records are on the machine, the command that
reads them, for example `replay codex`. The command fills that field from the
same detection the no-argument command and `doctor` already run.

- RED: two tests on the renderer, the serve line and the Codex line, both
  failing on the shipped screen.
- GREEN: one field on the machine model, two note lines, no other change to
  the screen.
- Mutation: M126 drops the serve line; killed by the renderer test through the
  registered harness.
- Production wiring: `TestE2E_TuiEmptyCostScreenNamesTheNextSteps` runs the
  command through `dispatch` on a HOME holding Codex rollouts and no Claude
  Code transcripts and finds both commands on the rendered screen.

## Item 3, the documentation

The guide's serve section lists the three variables, the two readable shapes,
the Responses limitation in plain words, and the offline `replay codex` path.
Nothing future is described as current. The docs guards and the prose scans
pass.

## Item 4, the transport probe

**Question.** When Codex CLI is pointed at a custom base URL, does an ordinary
run reach an HTTP proxy over SSE, or does it need WebSocket handling? Codex's
source carries a Responses WebSocket client with prewarming, which would make
an SSE-only proxy blind to it.

**Method.** A local stub on 127.0.0.1 that records method, path and transport
headers only, never a body, refuses WebSocket upgrades with 426 and answers
POSTs with a provider-shaped error. Codex CLI 0.154.0, installed on this
machine, run as `codex exec` with an isolated `CODEX_HOME`, a custom
`model_providers` entry on the Responses wire pointing at the stub, and a
dummy key in the provider's environment variable. No real endpoint was
reachable; no API call was possible; nothing was spent.

**Observed, 2026-10-04T21:55:49Z to 21:56:14Z.**

| Fact | Value |
|---|---|
| Requests that reached the stub | 30 |
| Method and path | `POST /v1/responses`, all 30 |
| `Upgrade` or `Sec-WebSocket-*` headers | none, on any request |
| `Accept` | `text/event-stream`, all 30 |
| `Content-Type` | `application/json` |
| `Content-Length` | 39,915 bytes, identical on all 30: the full input resent each attempt |
| `x-codex-beta-features` | `remote_compaction_v2` |
| `User-Agent` | `codex_exec/0.154.0 (Mac OS 26.5.2; arm64)` |
| Codex's own output | "Reconnecting... 2/5" through "5/5", then an error |

**Answer.** With a custom base URL, Codex CLI reaches an ordinary HTTP proxy
over SSE. It did not attempt WebSocket at all, so no fallback was needed or
observed; the WebSocket client in its source did not engage on this
configuration. The retry loop resends the whole input, which is what a
request-side parser will see.

**Evidence.** Outside the repository, in `~/replay-experiment/probes/`,
listed in that folder's `MANIFEST.sha256`:

| File | SHA-256 |
|---|---|
| `codex-transport-2026-10-04.jsonl`, headers only, 30 lines | `3d35374b90ba07bc407c7d0576120acebb7eb474dca06b77a4c2cbc98dfd1f8c` |
| `codex-transport-stub.py`, the stub | `accc6b31110e8bde371d91c4173702ceb27551f031347821eae6b6148139fb8c` |

The temporary Codex home, working directory and output were removed.

**Not measured.** GPT-6 Astra's transport; Codex's behaviour when
authenticated through ChatGPT rather than an API key; Codex versions other
than 0.154.0; what a Responses stream looks like from a real endpoint, which
needs a recorded reply and is R-1's first fixture.

## Decision

**R-1 JUSTIFIED: HTTP/SSE REACHES PROXY.** Implementation is not started; it
waits for its own authorisation.

## What remains between a stranger and a first session

With these three repairs, a Claude Code user activates on install, an Aider or
chat-completions user is told the variable, and a Codex user is told the truth
and pointed at the offline reader. The remaining blocker for live measurement
of Codex traffic is R-1. Nothing here measures whether any stranger has done
any of this, by design.
