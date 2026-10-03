# Replay 1.0 qualification: durable scratch, quota titration, realtime

**2026-10-03. Pass 1: establish truth. No production change. Nothing committed.**

The question is not whether code exists. It is whether a paying customer using only
the supported surface gets each capability, and with what measured performance.
Labels: **OBSERVED** is what a command or test printed. **READING** is what that
permits. Status words are the brief's: GREEN / YELLOW / RED / NOT MEASURED, and
CLEARED / PARTIAL / NOT CLEARED / NOT MEASURED in the clearance table.

---

## Phase 0: release candidate, locked

| | |
|---|---|
| commit | `7f7fa20` (`state ledger: commit the prototype, its evidence and the cursor`), direct child of the certified Release 1.0 commit `9694f1e` |
| branch | `fix/compaction-observed-vs-inferred`, remote equal |
| Replay version | `replay dev (unknown, built unknown)` (source build; release values are injected at tag time) |
| build | `go build -o $CLAUDE_JOB_DIR/tmp/replay ./cmd/replay` |
| runtime | go1.27.1 darwin/arm64 |
| OS | macOS 26.5.2 (25F84), Apple Silicon |
| configuration | per scenario: `replay serve --listen 127.0.0.1:<port> --upstream http://127.0.0.1:<port> --ledger <tmp>/ledger` plus the flag under test; `HOME` is a fresh temp dir per scenario; no rules document installed (compiled `anthropic-2026-09-01`); price table `2026-09-07` |
| provider / model | **no real provider.** A stand-in `/v1/messages` endpoint (`tmp/qual/harness.py`) answers with an Anthropic-shaped message whose `usage` the request asks for: 1,000 input + 500 output tokens, which the compiled price table prices at $0.0175 on `claude-opus-5`. No API call, no spend |
| test corpus | hand-built request bodies: plain chat turns; tool-call histories with 1 to 4 `tool_use` blocks, identical or distinct; the real transcript fixture `internal/transcript/testdata/session-redacted.jsonl` for the black-box pass |
| harness | `tmp/qual/harness.py` + `run_all.py` + `run_rest.py` + `run_rt8.py` + `run_mut.py` (python3 stdlib only), results under `tmp/qual/results/*.json`; not part of the repository |

---

## Phase 1: claim contracts

The product makes **no customer-facing claim** for any of the three capabilities.
`README.md`, `docs/guide/commands.md`, `docs/WHAT-YOU-GET.md` and `docs/ROADMAP.md`
contain no sentence promising durable scratch, quota titration or realtime
intervention; `docs/PROOF-1.0.md` lists "durable scratch improves outcomes" as NOT
MEASURED and "quota titration benefit" as RESEARCH; `docs/research/README.md` records
"Production implementation: NONE AUTHORIZED." The only quota sentence in the README
(line 245) says the subscription-quota question is "measured, unresolved, and written
up as null."

So each contract below is the **strongest precise claim the shipped surface could
honestly carry**, written so it can be benchmarked, next to the claim the brief names.

### Durable scratch

```text
INPUT              a piece of work state an agent wants a later session to have
TRIGGER            the agent writes it through a Replay interface
EXPECTED BEHAVIOR  Replay stores it durably, keyed to an owner and a scope
CUSTOMER EFFECT    a fresh process or session retrieves exactly that state through Replay
PERSISTENCE        survives process exit; updated state replaces stale state; corruption is reported, not papered over
FAILURE BEHAVIOR   missing or corrupt state is reported as missing; Replay never fabricates continuity
LATENCY/BUDGET     write and read under 100 ms at 10,000 items (proposed)
EVIDENCE REQUIRED  DS-1 to DS-8
```

**Contract status: NOT STATEABLE for the shipped product.** There is no interface
through which an agent writes a user-authored piece of state into Replay. The
closest shipped things are Replay's own derived stores (`advice.json` decisions,
`policy.json`, `seen.json`, the cost index) and `replay agents --write`, which
splices a *generated* block naming where a project's records live into
`AGENTS.md`; none takes content from the agent. The representation that was
designed for this, `internal/stateledger`, has no persistence, no carrier across a
process boundary and no production caller, by a recorded decision (E4-01, P1).

### Quota titration

```text
INPUT              usage observed on the request path (tokens, list price) against an operator-set budget
TRIGGER            cumulative spend for a session or a day reaches the cap
EXPECTED BEHAVIOR  the NEXT request is refused with a provider-shaped error naming the cap and the spend; never a response in flight
CUSTOMER EFFECT    the agent sees the refusal; nothing is forwarded until the cap is raised, a new session starts, or x-replay-override is sent once
PERSISTENCE        session totals live for the proxy process; the day total also in ~/.replay/ledger/spend-day.json
FAILURE BEHAVIOR   usage absent or unpriceable: the cap must not be silently unenforced
LATENCY/BUDGET     refusal decided in the request path, under 5 ms
EVIDENCE REQUIRED  QT-1 to QT-8
```

**Contract status: this is a hard cap, not titration.** Nothing in the product
re-allocates tokens, requests, frequency, model or scheduling as a budget
approaches. The one control the proxy exerts is binary: forward or refuse. The
benchmark below measures that control loop, honestly labelled.

### Realtime

```text
INPUT              a request from a running agent passing through replay serve
TRIGGER            a condition visible in that request or the proxy's state: a loop of identical tool calls, an exhausted cap, a failing provider
EXPECTED BEHAVIOR  Replay decides inside the request path and answers locally (400 replay_loop / replay_spend_cap, 503 replay_circuit_open with Retry-After) before anything reaches the provider
CUSTOMER EFFECT    the agent receives the local answer on that same request; the provider never sees it
PERSISTENCE        every refusal writes its own ledger record naming the guard and the reason
FAILURE BEHAVIOR   proxy down: the client gets a connection error, nothing is queued or replayed; provider down: the breaker opens after N failures and probes after the cooldown
LATENCY/BUDGET     decision latency under 5 ms at p99 in isolation
EVIDENCE REQUIRED  RT-1 to RT-9
```

**Contract status: in-path synchronous interception, not an event subscription.**
The mechanism is a reverse proxy; there is no event bus, no subscriber, no priority
and no notion of a stale event. `replay statusline` and the TUI live screen *read*
state on each render (stdin JSON; `/replay/status` polled every 250 ms) and
intervene in nothing.

---

## Phase 2: production-wiring audit

Legend: a complete chain has a file and symbol on every arrow and a runtime
observation from Pass 1. "none" on an arrow ends the chain.

### Durable scratch: NOT SHIPPED

| arrow | what exists |
|---|---|
| customer entry point | **none.** No command, flag, API or UI accepts agent-authored state |
| nearest shipped surfaces | `replay agents --write F` (`cmd/replay/agents.go:runAgents`, writes a generated source block between `<!-- replay:sources:begin/end -->`); `replay advise --apply --yes` (`cmd/replay/apply.go`, writes `promptCacheTtl` into Claude Code's settings and records the decision in `~/.replay/advice.json`); `replay learn` (`~/.replay/policy.json`); `replay since` (`~/.replay/seen.json`) |
| implementation designed for it | `internal/stateledger` (claims, graded checks, open questions, `Render`): in-memory only, no `Marshal`, no file, no reader; outside the shipped closure (`internal/regression/unwired_packages_test.go`, UNWIRED-LOG §15) |
| state / evidence | n/a |
| decision | n/a |
| observable effect | n/a |

DS-1 fails at the first arrow, so DS-2 to DS-8 are **NOT MEASURED**: there is no
production path to restart, isolate, mutate, corrupt or load.

### Quota titration: NOT SHIPPED. Spend capping: SHIPPED and measured

| arrow | file / symbol | observed in Pass 1 |
|---|---|---|
| customer entry point | `replay serve --max-session-usd / --max-day-usd / --max-session-tokens / --max-day-tokens` (`cmd/replay/serve.go`) | flags parsed; proxy up |
| production handler | `internal/proxy/passthrough.go` `(*Server).handle` → `guard` | every request passes `guard` before `rp.ServeHTTP` |
| implementation | `internal/proxy/guards.go` `SpendGuard.Record` (on the response, from `listCost`) and `SpendGuard.Check` (on the next request) | `Check` refuses with the reason string |
| state | per-session and per-day totals in memory; day total persisted to `~/.replay/ledger/spend-day.json` | `/replay/status` shows `list_cost_usd` per session and `caps` |
| decision | forward or `refuseSession(..., refusalSpendCap, ...)` (`internal/proxy/refusal.go`) | HTTP 400, body `{"type":"error","error":{"type":"replay_spend_cap","message":...}}` |
| observable effect | the request is not forwarded; a ledger record with `refusal: spend_cap` | upstream hit count stops; refusal records present |
| **titration** (changing an allocation short of refusal) | **none.** `internal/quota` (forecast) is outside the closure; `quotaline.go`/`quotastore.go` render subscription windows in the statusline; `ceiling` and `budget` compute and print | nothing re-allocates anything |

### Realtime: in-path interception SHIPPED and measured. Event subscription: NOT SHIPPED

| arrow | file / symbol | observed |
|---|---|---|
| customer entry point | `replay serve --loop-warn/--loop-block`, `--breaker-failures/--breaker-cooldown`, `--error-budget`, `--preflight` | flags parsed |
| production handler | `passthrough.go` `handle` (breaker check at entry) → `guard` (spend, error budget, `DetectLoop`, `preFlight`) | order as documented: spend first |
| implementation | `guards.go` `DetectLoop` (adjacent identical `tool_use` run by HMAC `CallKey`), `Breaker.Allow/Observe`, `preflight.go` | loop refused at 3; breaker opened after 2 failures |
| state | request summary only (loop); breaker state in memory | n/a |
| decision | local answer: 400 `replay_loop`, 503 `replay_circuit_open` + `Retry-After` | observed |
| observable effect | provider never receives the request; ledger refusal record; `x-replay-warning` header at warn | upstream hits 0 for refused requests |
| live *observation* surfaces | `replay statusline` (`cmd/replay/statusline.go`, stdin JSON per render), TUI live screen (`cmd/replay/tui.go:liveState`, GET `/replay/status`, 250 ms tick) | read-only; no intervention |

---

## Durable scratch: results

| test | result | status |
|---|---|---|
| DS-1 write | no supported interface accepts agent-authored state | **RED: NOT SHIPPED** |
| DS-2 restart, DS-3 session transition, DS-4 correctness, DS-5 isolation, DS-6 mutation, DS-7 corruption, DS-8 load | no path to exercise | **NOT MEASURED** |

**Ship gate: RED.** Not a near miss: the representation exists only in memory and
the carrier is, by the recorded decision, gated on an outcome signal that no
examined corpus carries (`docs/evidence/e4-01-state-representability-2026-10-02.md`).

---

## Quota: results (measured as spend capping)

All figures OBSERVED from `tmp/qual/results/pass1.json` and `pass1b.json`; per-request
list cost $0.0175.

| test | result |
|---|---|
| QT-1 baseline, no cap | 50 requests, all 200, 50 forwarded; 50,000 prompt tokens; list cost $0.875; latency through the proxy p50 0.64 / p95 0.96 / p99 1.52 / max 1.52 ms (n=50) |
| QT-2 detection | cap $0.05: requests 0,1,2 forwarded ($0.0525 cumulative), **request 3 refused**: HTTP 400, `session spend cap reached: $0.05 of $0.05 at list price` |
| QT-3 allocation change | binary: forward → refuse. No other allocation exists to change |
| QT-4 behavioural effect | after the cap: 20/20 further requests refused, **0 reached the provider**; `x-replay-override` let exactly one through (status 200), the next was refused again (400); a fresh session proceeded (200). Refusal latency p50 0.33 / p95 0.35 / p99 0.38 / max 0.38 ms (n=20) |
| QT-5 boundary | step function, see curve. **A cap of 0 is "off", not "nothing allowed"** |
| QT-6 hysteresis | day cap $0.10 shared by two alternating sessions: first refusal at request 6 (cumulative $0.105), 0 passes afterwards over 24 more requests; the decision latches because spend never retreats. The refusal names the leading session: `most of it from session qt6-a ($0.05)` |
| QT-7a usage absent | 30 responses with no `usage`: 0 refused, status cost $0, `spend_cap_not_enforced` **absent/false**. **The cap is silently unenforceable and nothing says so** |
| QT-7b model not in the price table | refused at request 1: the guard priced it at the dearest known rate as an upper bound; `/replay/status` reports `spend_cap_not_enforced: true` while `list_cost_usd` 0 and a refusal happened. **`docs/guide/commands.md` says "a model not in the table counts as free"; the code does the opposite** |
| QT-7c malformed response | 10 malformed bodies passed through as 200; 10 ledger records, 0 with usage; cap never advances; no signal |
| QT-8 adversarial | provider 500 ×5: 0 cost counted (correct); 4 concurrent sessions on a $0.26 day cap (15 requests' worth): 16 allowed, overshoot 1 request (in-flight responses are not counted until they return, as documented); duplicate usage: a resent request is a second request and is billed twice, which is correct |

QT-5 response curve (14 requests offered, $0.175 reference budget):

| cap | $ | allowed | expected under the next-request rule |
|---|---|---|---|
| 100% | $0.1750 | 10 | 10 |
| 90% | $0.1575 | 9 | 9 |
| 75% | $0.1313 | 8 | 8 |
| 50% | $0.0875 | 5 | 5 |
| 25% | $0.0437 | 3 | 3 |
| 10% | $0.0175 | 1 | 1 |
| 0% | $0.0000 | 14 | 14 |

**Ship gate for "quota titration": RED, NOT SHIPPED.** The product measures and
caps; it does not titrate. **Spend capping as shipped: GREEN on QT-1 to QT-6 and
QT-8, YELLOW on QT-7**, because an absent or malformed `usage` leaves the cap
unenforced with no disclosure, and the unpriced-model path contradicts its own
documentation and its own status flag.

---

## Realtime: results (measured as in-path interception)

| test | result |
|---|---|
| RT-1 single event | loop of 3 identical `Bash` calls: HTTP 400 `replay_loop`, end-to-end T6−T0 7.20 ms (first request, includes connection setup), upstream hits 0, ledger refusal record written (`latency_ms` 0, i.e. decision under 1 ms). T0 client send → T1 proxy summarises the body → T3 `DetectLoop` → T4 refusal written → T5 client receives; T6 is "the provider never saw it" |
| RT-2 10 events | 10/10 intercepted, 0 lost, 0 forwarded, 10 records; p50 1.55 / p95 1.83 / p99 1.83 / max 1.83 ms (n=10) |
| RT-2 100 events | 100/100, 0 lost, 0 forwarded, 100 records; p50 0.34 / p95 0.56 / p99 0.94 / max 2.62 ms (n=100) |
| RT-2 1,000 events | 1000/1000, 0 lost, 0 forwarded, 1000 records; p50 0.23 / p95 0.36 / p99 0.56 / max 1.49 ms (n=1000) |
| RT-3 burst | 300 concurrent: 300/300 intercepted, 0 errors, 0 forwarded, wall 84 ms; p50 1.66 / p95 5.19 / p99 6.08 / max 6.68 ms (n=300). No saturation reached at this size |
| RT-4 noise | 200 non-looping histories (2 identical, or distinct): 0 false refusals, 200 forwarded |
| RT-5 priority | **not implemented**; no priority field or semantics exist. NOT APPLICABLE |
| RT-6 out of order | history A A B A: status 200, not refused. The detector counts the adjacent run only, so a reordered history cannot manufacture a loop |
| RT-7 delayed | the "event" is the request itself; there is no staleness window. NOT APPLICABLE |
| RT-8 provider down | breaker 2/3s: sequence 0:502(upstream_failed), 1:502(upstream_failed), 2:503(circuit_open), 3:503(circuit_open), 4:503(circuit_open), 5:503(circuit_open); circuit opened after 2 failures with `Retry-After: 3`; recovered 3.1 s after the provider returned |
| RT-8 proxy down | 5 requests 200, then the proxy killed: 5/5 connection errors; nothing queued, nothing replayed, no false state |
| RT-8 event source / intervention channel killed | the source is the agent and the channel is the same TCP connection; killing either is the proxy-down case. NOT SEPARATELY MEASURABLE |
| RT-9 mechanism | `net/http/httputil` reverse proxy in the request path; decisions synchronous; no subscription, no polling for intervention. The two "live" screens poll (`/replay/status` every 250 ms; statusline per render) and intervene in nothing |

**Ship gate for "realtime" as an event-driven control plane: RED, NOT SHIPPED.**
**In-path guards as shipped: GREEN** on every applicable test, with sub-millisecond
decision latency at p99 up to 1,000 sequential events and 6 ms at p99 under a
300-way burst.

---

## Phase 3: cross-capability test

**NOT MEASURED.** Steps 1 (durable scratch holds work state) and 7 (a fresh agent
retrieves it) have no production path; steps 3 and 5 reduce to the cap and the
in-path guards above, which were measured in isolation. There is nothing to
integrate.

---

## Phase 4: customer black-box test

A fresh agent received the binary, an empty isolated `HOME`, the stub provider,
and only `README.md` and `docs/guide/*` (about 3,400 lines); it was forbidden the
source, the evidence, and the web, and it set no real key. Timings are the
commands' own wall seconds, excluding reading.

| scenario | completed | time | commands | undocumented steps | failures | what the product provides |
|---|---|---|---|---|---|---|
| 1 persist a fact | **no** | 1 s | 6 Replay + 1 shell | guessed which filenames the `agents` scan treats as records (`decisions.log` matched nothing; `facts.jsonl` and a dated `records/` matched as directories, never content) | none as errors; `AGENTS.md` still said "No record files were found by this scan" | nothing: no command, file or MCP tool stores a user-authored fact; the seven MCP tools are read-only lookups |
| 2 retrieve it in a new process | **no** | 0 s | 3 (`agents`, `privacy`, `since`) | none | none as errors; no output carries the fact | nothing: `agents` prints directory labels, `privacy` "has written nothing", `since` "no window" |
| 3 cap a session at $0.05 | **yes** | 15 s | 1 stub + 1 `serve` + 10 curl | none; `--max-session-usd`, the provider-shaped refusal and `x-replay-override` are all in the guide | none | `replay serve --max-session-usd 0.05` refused request 6 (five priced at $0.01 on the stub's default usage) with HTTP 400 `replay_spend_cap`; the override let one through; `/replay/status` counted `refused: 2` |
| 4 react to a 3× repeated tool call | **yes** | 1 s | 1 `serve` + 10 curl | had to discover by experiment that the guard reads tool calls in the request's history, not repeated requests; the `X-Replay-Warning` header, the 400 status and the `replay_loop` type are not in the docs | none | `--loop-warn 2 --loop-block 3`: warning header at 2, 400 `replay_loop` at 3 and 4, 0 reached the provider; 4 plain re-sent requests were not treated as a loop |

Confusion points the customer recorded, verbatim in substance: the README's
"four surfaces" and `replay mcp` read as if an agent could talk state to Replay,
and cannot; the refusal's HTTP status code is nowhere in the docs, so a harness
branching on 402/429 would miss it; `list_cost_usd` prints as
`0.060000000000000005`; and the customer repeated the guide's sentence that an
unpriced model "counts as free" as a foot-gun for a $0.05 budget, when QT-7b
shows the code charges the dearest rate instead. The incorrect assumption the
docs invite, then, is held even by a careful reader.

Its closing judgement: scenarios 3 and 4 "are real, fast to set up, and behave
exactly as the guide describes with small documentation gaps about wire
details"; scenarios 1 and 2 "are outside the product's documented scope entirely;
a customer expecting one would find it absent rather than broken."

---

## Phase 5: independent reproduction

A second, fresh agent received a copy of the harness, the binary, and the
instruction to run it on its own port base without seeing any result file or
this document (`tmp/qual/repro/out/repro.json`). Every scenario ran; nothing
was edited but the port base and the output path.

| scenario | first run (this document) | independent reproduction |
|---|---|---|
| QT-2 first refused index / after-cap forwarded | 3 / 0 | 3 / 0 |
| QT-4 override once / after / fresh session | 200 / 400 / 200 | 200 / 400 / 200 |
| QT-5 allowed at 100/90/75/50/25/10/0% | 10 / 9 / 8 / 5 / 3 / 1 / 14 | 10 / 9 / 8 / 5 / 3 / 1 / 14 |
| QT-6 first refusal index / passes after | 6 / 0 | 6 / 0 |
| QT-7a refused / QT-7b first refused, flag / QT-7c refused | 0 / 1, true / 0 | 0 / 1, true / 0 |
| QT-8 allowed of 15 / overshoot | 16 / 1 | 16 / 1 |
| RT-2 1,000: intercepted / lost / forwarded; p50 / p95 / p99 ms | 1000 / 0 / 0; 0.23 / 0.36 / 0.56 | 1000 / 0 / 0; 0.25 / 0.36 / 0.48 |
| RT-3 300 burst: intercepted / errors; p99 ms | 300 / 0; 6.1 | 300 / 0; 4.5 |
| RT-4 false refusals | 0 | 0 |
| RT-6 A A B A | 200 | 200 |
| RT-8 failures before the circuit opened / recovered after | 2 / 3.1 s | 2 / 3.1 s |
| RT-8 proxy down | 5/5 connection errors | 5/5 connection errors |

Every discrete figure reproduced exactly; the latency distributions sit in the
same sub-millisecond band (p99 within 0.1 ms sequential, 1.6 ms under burst),
which is machine noise, not a disagreement. The reproducer's QT-6 attribution
named the other alternating session as the leader ($0.05 each; a tie broken by
map order), a cosmetic nondeterminism in the attribution sentence, noted for
Pass 2.

---

## Phase 6: mutation

Built from a throwaway worktree of `7f7fa20` (removed afterwards; repository
unchanged), each binary run through the same harness (`tmp/qual/results/mutation.json`):

| build | QT-2 first refused index (real: 3) | RT-1 loop status (real: 400) | RT-1 upstream hit (real: 0) |
|---|---|---|---|
| real | 3 | 400 | 0 |
| `SpendGuard.Check` returns "" | **None** (never refused) | 400 | 0 |
| `DetectLoop` returns nothing | 3 | **200** | **1** (forwarded) |

Each benchmark goes red when the mechanism it claims to measure is removed, and
only that one. Durable scratch has no mechanism to remove.

---

## Phase 7: benchmark

| Capability | Success rate | p50 | p95 | p99 | Failure rate | Isolation | Restart | Adversarial |
|---|---:|---:|---:|---:|---:|---|---|---|
| Durable scratch | NOT MEASURED | — | — | — | — | NOT MEASURED | NOT MEASURED | NOT MEASURED |
| Quota titration | NOT MEASURED (no mechanism) | — | — | — | — | — | — | — |
| Spend cap (what ships) | 100% of over-cap requests refused (20/20, 24/24) | 0.33 ms | 0.35 ms | 0.38 ms | 0 | per session; day cap shared, 1-request overshoot under 4-way concurrency | day total persisted; session totals per process (not measured across restart) | 500s not counted; absent/malformed usage silently unenforced (QT-7) |
| Realtime | NOT MEASURED (no event path) | — | — | — | — | — | — | — |
| In-path guards (what ships) | 1,110/1,110 loop events intercepted, 0 forwarded | 0.23 ms | 0.36 ms | 0.56 ms (1,000 seq); 6.1 ms (300 burst) | 0 | stateless per request | breaker recovers 3.1 s after the provider; proxy death loses in-flight requests | 0 false refusals on 200 noise; reordering cannot forge a loop |

---

## Phase 8: claim clearance

| Claim | Production wired | E2E | Adversarial | Mutation | Independent reproduction | Benchmark | Clearance |
|---|---|---|---|---|---|---|---|
| Durable scratch | no | no | no | no | no | no | **NOT CLEARED** |
| Work continuity | no (closed negative in Rooms VII–IX; repository prose is the carrier) | no | no | no | no | no | **NOT CLEARED** |
| Quota titration | no | no | no | no | no | no | **NOT CLEARED** |
| Spend capping | yes | yes | partial (QT-7 silent) | yes | yes | yes | **PARTIAL** |
| Realtime | no | no | no | no | no | no | **NOT CLEARED** |
| In-path guards (loop, breaker, cap) | yes | yes | yes | yes | yes | yes | **CLEARED** as in-path interception, not as "realtime" |
| Evidence / reconstruction | yes (Release 1.0 gate, `9694f1e`) | yes (34 `TestE2E_*`) | yes (Rooms V–IX) | yes (115/115) | yes (Room V) | partial (match rate, not latency) | **CLEARED** |
| Cost / FinOps | yes | yes | yes (C030/C035/C037) | yes | yes | yes (`replay cost` figures reproduced) | **CLEARED**, with the population caveats the register bounds |

---

## Phase 9: marketing claim audit

No current public sentence claims the three capabilities, so the audit is of the
nearest sentences that exist and of the words the brief proposes.

```text
CURRENT CLAIM       (proposed) "quota titration"
EVIDENCE            QT-3: the only control is forward/refuse; internal/quota is not shipped
ACTUAL BOUNDARY     spend capping at a session or day budget, decided on the next request
REQUIRED REWORDING  "Caps spend per session and per day; refuses the next request when a cap is reached."
```

```text
CURRENT CLAIM       (proposed) "realtime"
EVIDENCE            RT-9: synchronous in-path interception; observation screens poll
ACTUAL BOUNDARY     guards decide inside the request, sub-millisecond; no event bus
REQUIRED REWORDING  "Stops a looping agent, an exhausted budget or a failing provider on the request itself, before it reaches the provider."
```

```text
CURRENT CLAIM       (proposed) "durable scratch" / "work continuity"
EVIDENCE            DS-1: no interface; E4-01, P1 decision; Rooms VII–IX negative
ACTUAL BOUNDARY     none shipped
REQUIRED REWORDING  do not claim it
```

```text
CURRENT CLAIM       docs/guide/commands.md: "--max-session-usd, --max-day-usd: ... A model not in the table counts as free"
EVIDENCE            QT-7b: an unknown model is priced at the dearest known rate as an upper bound and refused; /replay/status says spend_cap_not_enforced: true
ACTUAL BOUNDARY     an unpriced model is capped conservatively, and the status flag says the opposite of what happened
REQUIRED REWORDING  "A model not in the price table is charged at the dearest known rate, so the cap fires early rather than never; /replay/status flags it." And the flag's meaning must be fixed to match. Pass 2 item
```

```text
CURRENT CLAIM       docs/guide/commands.md: "Refuse the next request once the cap is reached"
EVIDENCE            QT-2, QT-4, QT-5, QT-6: exactly that, 0 forwarded after the cap
ACTUAL BOUNDARY     holds; silent when the provider sends no usage (QT-7a)
REQUIRED REWORDING  add: "A response that carries no usage advances no counter; the cap cannot see it."
```

---

## Final answers

**If a customer pays tomorrow and uses only the supported surface, which of the
three will work?**

- **Durable scratch: none of it.** There is no way to write agent state into Replay.
- **Quota titration: none of it.** What works is a hard spend cap: 100% of over-cap
  requests refused in these runs, decided in 0.33 ms at p50, latching for the
  session or the day, with a one-time override. It fails silently when the provider
  returns no usage, and it over-enforces (not under) for a model the table does not
  price.
- **Realtime: none of it as an event-driven control plane.** What works is
  interception on the request path: 1,110 of 1,110 loop events stopped before the
  provider, 0.23 ms p50 / 0.56 ms p99 sequential, 6 ms p99 under a 300-way burst,
  0 false refusals on 200 noise requests, a circuit breaker that opens after N
  provider failures and probes again after the cooldown. If the proxy process dies
  the agent's requests fail; nothing is queued.

**What can we honestly put on the website today?**

"Replay caps an agent's spend per session and per day and refuses the next request
when a cap is reached. It stops an agent that repeats the same tool call, and
holds requests while the provider is failing. These decisions are made on the
request itself, before it reaches the provider, in under a millisecond." Nothing
about scratch, continuity, titration or realtime.

**What must we build before calling Replay 1.0 on these three?**

Nothing, if 1.0 does not claim them, which is the position the repository's own
documents already take. If any is to be claimed: durable scratch needs an
interface, a carrier and the outcome signal E4-01 says is missing; titration needs
a second allocation besides refuse, and a demonstrated behavioural effect; realtime
needs an event path that is not the request. Before any of that, two Pass 2
repairs to what already ships: disclose an unenforced cap when usage is absent or
malformed (QT-7a/c), and make the unpriced-model documentation, status flag and
behaviour agree (QT-7b).

---

## Pass 1 integrity

No production file changed. The mutation builds came from a throwaway worktree,
removed. `git status --porcelain` empty at `7f7fa20`. Orphaned proxies from one
crashed harness run (a port reuse) were killed and the affected scenarios
re-run on fresh ports; the first run's figures for those scenarios were discarded.
