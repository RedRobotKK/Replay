# Surface taxonomy, part 1: the request path

**What this is:** every place between a client's bytes arriving at the proxy
and those bytes leaving for the provider where Replay touches, reads, delays,
refuses or alters the traffic. Byte-for-byte passthrough is treated here as a
*setting*, not an identity: each row states the default, what has to hold for
the surface to be doing its job, what it looks like when it is not, and whether
its state can be moved.

**Scope.** The request path only: `internal/proxy/server.go` `handle` and
everything it calls, plus the listener that admits the request and the
transport that emits it. The response tap, the ledger's own storage and
analysis behaviour, and per-provider response differences are **out of scope —
see parts 2, 3 and 4.**

**Method.** Every default, pass condition and fail condition below is cited to
`file:line` in `~/Development/Replay-clean` at the working tree of 2026-09-09.
Where a pass condition is enforced by a test, the test function is named. Where
it is not, the row says **UNTESTED**, because an untested pass condition is the
finding. Cells reading *not determined* are places the code did not answer the
question; they are left rather than guessed.

**Judging alterations.** [ADR-0011](../adr/0011-opt-in-request-rewriting.md)
stages request rewriting as: **stage 1** offline only, sends nothing; **stage
2** deterministic, semantics-preserving transforms, live, off by default;
**stage 3** a user-supplied rewriter, gated on evidence stage 2 produces. Every
optimisation proposed at the end names the stage it sits in. A stage below 1 —
*reduces* what Replay touches — is written **stage 0**.

---

## Headline findings

**F1. A whole guard is tested and unreachable.** `Config.PreFlight`
(`internal/proxy/server.go:103`) is never assigned. The only construction of
`proxy.Config` in the shipped binary is `cmd/replay/serve.go:136`, and
`PreFlight` does not appear in it, in any flag, in any environment variable, or
anywhere in `docs/`. `analysis.EvaluatePreFlightPolicy` returns `false`
whenever `optInActive` is false (`internal/analysis/predictor.go:46-48`), and
`Straddles` likewise (`internal/analysis/predictor.go:91-93`). So
`internal/proxy/preflight.go` — 120 lines, a refusal kind
(`internal/proxy/server.go:1017`) and a `preflight_deficit` counter — **can
never refuse and can never warn in a build a user runs.** Eight tests exercise
it (`internal/proxy/preflight_test.go:49,79,99,116,140,167,190,224`); all eight
call `s.preFlight` directly. This is the most valuable undocumented surface in
the request path: a guard that is green in CI and absent in production.

**F2. The package doc contradicts the package.** `internal/proxy/server.go:5-6`
states: *"Nothing here rewrites a request body or removes a client header."*
The same file rewrites the request body in three places
(`server.go:511-514` masking, `server.go:540-543` context-edit,
`server.go:545-552` include-usage) and removes two client headers
(`server.go:211` `x-replay-token`, `server.go:561` `Accept-Encoding`). Recorded
as a disagreement between code and doc, not as a bug in either.

**F3. One default-on mutation re-serializes the whole body.** Every other
mutation path splices: masking replaces byte ranges in place
(`internal/masking/mask.go:128-130`), the context-edit policy inserts one
member before the closing brace and keeps every other byte where it was
(`internal/policy/contextedit.go:103-122`). `withUsageReporting`
(`internal/proxy/server.go:1278-1296`) does not: it unmarshals into
`map[string]json.RawMessage` and re-marshals, so the top-level object comes
back in Go's sorted key order with normalised whitespace. It is gated on
`openai && !s.cfg.NoPolicy` (`server.go:545`) — **not** on
`policyConfigured()` (`server.go:829`) — so a plain `replay serve` with no
flags at all does this to every streaming `/v1/chat/completions` request whose
client left `stream_options` unset. `docs/architecture/proxy-protocol.md:35`
says *"Nothing in the body is re-serialized in passthrough mode; bytes in are
bytes out"* and `:36` says *"Every byte the client sent stays in place."* For
this family, by default, both are false.

**F4. One client-set header disables four guards.** `x-replay-override`
(`server.go:140`, read at `server.go:945`) with **any** non-empty value passes
the spend cap (`server.go:947`), the error budget (`server.go:954`), the loop
block (`server.go:964`) and the pre-flight ceiling
(`internal/proxy/preflight.go:112`). `preflight.go:55-58` states as a design
principle that consent is *not* read from a request header, "because that would
let any client switch the guard on or off" — and then reads the override from a
request header four lines later. The value is logged as a reason; nothing
validates it. Documented at `docs/architecture/proxy-protocol.md:27`.

**F5. Guards are gated on a readable, non-empty body.** `server.go:525` guards
the whole block on `readable && len(body) > 0`. A POST to `/v1/messages` with
an empty body is forwarded with no cap, no loop check and no masking. A
non-empty body the summarizer rejects still reaches `guard`
(`server.go:537`) but with a zero `rec.Prompt`, so the loop detector
(`guards.go:288-290`) and pre-flight (`preflight.go:63`) are no-ops, and if the
client sent no session header `rec.SessionID` stays `""`
(`server.go:534-536` only fills it from `SessionHash`, which an unsummarized
request does not have) — so all such traffic accrues against one empty-string
key in the spend guard's table.

**F6. Nothing about the request is streamed.** `server.go:500` reads the entire
body into memory before a byte is forwarded, on **every path**, including paths
this build declares it cannot read (`server.go:478-482`). The 64 MB ceiling is
`server.go:57`.

---

## 1. Ingress: listener and admission

| Surface | Primitive it acts on | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **S1 Loopback enforcement** | the bind address | `127.0.0.1:4000` (`cmd/replay/serve.go:28`) | `New` returns an error for a non-loopback host (`server.go:182`, `isLoopback` `server.go:1095-1104`) | binds a routable interface; anyone on the network can spend the key | Yes — `-listen` | a socket path skips the check entirely (`server.go:182` short-circuits on `isUnixAddr`); `localhost` is accepted by name, not by resolution (`server.go:1099`) | None. Do not touch |
| | | | Tested: `TestRefusesNonLoopbackListen` (`server_test.go:452`) | | | | |
| **S2 Socket mode** | the socket file's permission bits | off; TCP is the default | `chmod 0600` after bind (`uds.go:38,98`) | umask-derived mode; another user connects and spends the key | Yes — `-listen unix://…` | opts into filesystem-enforced authorization instead of address-implied | None |
| | | | Tested: `TestU2_TheSocketIsOwnerOnly` (`uds_test.go:166`) | | | | |
| **S3 Socket directory check** | the containing directory's mode | off | refuse when `perm&0o022 != 0` (`uds.go:120-124`) | a group- or world-writable dir lets someone unlink and re-bind, receiving the API key | No, once a path is chosen | — | None |
| | | | Tested: `TestU3_ARefusedSocketDirectory` (`uds_test.go:199`) | | | | |
| **S4 Stale-socket handling** | an existing file at the socket path | off | symlink refused, non-socket refused, live socket refused, dead socket removed (`uds.go:139-158`) | takes over a running proxy's socket, or deletes a user's file | No | — | None |
| | | | Tested: `TestU4`, `TestU5`, `TestU6`, `TestU10` (`uds_test.go:228,252,299,422`) | | | | |
| **S5 Socket path length** | `sun_path` | off | refuse over 103 bytes with the reason named (`uds.go:43,78-82`) | opaque `EINVAL` at bind | No | — | None |
| | | | Tested: `TestU9_APathTooLongForTheKernelIsRefusedByName` (`uds_test.go:387`) | | | | |
| **S6 Metrics listener** | a second inbound socket | off (`serve.go:52`, `""`) | loopback-or-unix enforced (`metrics_listener.go:47-62`); its mux carries three routes and **no `/`**, so it structurally cannot proxy (`metrics_listener.go:36-42`) | a second port that could forward to the provider — a complete bypass of S1/S2 | Yes — `-metrics-listen` | opens a read surface exposing model names, token counts and list-price dollars, unauthenticated unless `-token` is set | None |
| | | | Tested: `TestMT2_TheMetricsListenerCannotProxy`, `TestMT5_TheMetricsListenerIsLoopbackOnly` (`metrics_listener_test.go:110,207`) | | | | |
| **S7 Read-header timeout** | how long a client may take to send headers | 30 s (`server.go:44`, set at `server.go:245`) | a stalled client is cut before it pins a goroutine forever | slowloris on the loopback listener | No — a constant | there is **no** read or write timeout for the body: a client that sends headers and then trickles a body is bounded by nothing (`server.go:243-249`) | Low |
| | | | **UNTESTED** that the field is set. Tests use the constant only as a timing bound | | | | |
| **S8 Idle-connection tracking** | accepted connections carrying no request | on (`server.go:248`) | `closeUnusedConns` closes them so Ctrl-C is not a 5 s hang (`server.go:351-386`) | shutdown waits `ReadHeaderTimeout` on pooled agent connections | No | — | None |
| | | | Tested: `TestShutdownClosesConnectionsThatNeverSentARequest` (`server_test.go:1962`) | | | | |
| **S9 Browser-origin refusal** | `Origin`, `Sec-Fetch-Mode` | on, unconditional (`hostguard.go:78`) | either header present → 403 | a web page reaches the proxy via DNS rebinding | No | **no longer the whole defence.** S9b below is the part that does not need the browser's cooperation | None |
| | | | Tested: `TestBrowserOriginAndTokenChecks` (`server_test.go:390`); `Sec-Fetch-Mode` covered by `TestS6c` (`hostguard_test.go`) | | | | |
| **S9b `Host` validation** | `Host` | on for TCP, 2026-09-10 (`hostguard.go:82`) | empty, a loopback IP literal, `localhost`, or a name under `.localhost` | a rebound request carries the attacker's own name, which S9 alone could not see: a plain `<form>` post sends no `Origin` | No | — | None |
| | | | Tested: `TestS6a`, `TestS6b`, `TestS6e`, `TestS6g` (`hostguard_test.go`). Exempt on a Unix socket, which has no DNS and whose clients send a placeholder authority — `TestS6f` | | | | |
| **S10 Listener token** | `x-replay-token` | off (`serve.go:55`; `REPLAY_TOKEN` at `serve.go:113`) | when set, a mismatch is 401 (`server.go:432-435`) | any local process can spend against the proxy | Yes — `-token`/env | turns local address-implied trust into a shared secret | None |
| | | | Tested: `TestBrowserOriginAndTokenChecks` (`server_test.go:390`) | | | | |
| **S11 Health exemption** | `/replay/healthz` | narrowed 2026-09-10 | S9 and S9b apply | `health` now calls `notLocal`, so a browser origin and a foreign `Host` are both 403. It still does **not** take the token | No | the fingerprinting gap is closed; what remains is a local non-browser process learning "something answers ok here", which `connect(2)` already tells it | None. The token is withheld on purpose: `doctor` probes this endpoint with none (`doctor.go:260`) and cannot learn one it did not set. Tested: `TestS6c`, `TestS6d` |
| **S12 Path classification** | `r.URL.Path` | on | `isMessages` / `isChatCompletions` decide whether anything at all applies (`server.go:472-475`) | traffic silently unprotected | No | both use `strings.HasSuffix` (`server.go:981,1000`), so `/anything/v1/messages` is parsed, guarded, masked and policy-applied, and forwarded to the upstream at that path. `/v1/messages/count_tokens` is deliberately excluded | Low |
| | | | Tested: `TestCountTokensIsForwardedButNotRecorded` (`server_test.go:437`) | | | | |

## 2. Body intake

| Surface | Primitive | Default | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **S13 Whole-body buffering** | `r.Body` | on, unconditional (`server.go:500`) | the body is fully in memory before `s.rp.ServeHTTP` at `server.go:651` | the proxy adds latency proportional to body size and holds a copy of every in-flight request | No | applies even to paths `noteUnparsed` has just declared unreadable (`server.go:478-482`) | **High** — see O2. Outbound framing is *not* altered: `setBody` does not touch `r.TransferEncoding`, so a chunked request stays chunked |
| | | | **UNTESTED** that any byte is withheld until the body completes | | | | |
| **S14 Request size ceiling** | body length | 64 MiB (`server.go:57`) | over the cap → 413 (`server.go:505-508`) | unbounded memory per connection | No — a constant | — | Low |
| | | | **UNTESTED**: no test references `MaxRequestBytes` or `StatusRequestEntityTooLarge` | | | | |
| **S15 Read failure** | a truncated client body | on | read error → 400 (`server.go:500-504`) | a partial body is summarized and forwarded as if complete | No | — | None |
| | | | **UNTESTED**: no test references "could not read request body" | | | | |
| **S16 Body re-installation** | `r.Body`, `r.ContentLength`, `r.GetBody` | on (`setBody`, `server.go:778-782`) | `GetBody` is non-nil, which is what makes S47 retries possible (`retry.go:72`) | retries silently disabled | No | called four times per request (`server.go:509,513,542,550`); each allocates a fresh reader over the same slice | Low |
| | | | Tested indirectly: `TestRetriesResendUntilSuccessAndAreRecorded` (`server_test.go:1060`) | | | | |
| **S17 Unparsed-path warning** | first sight of an unknown POST path | on (`server.go:478-482`) | one `NOT PARSED` line per path + `replay_unparsed_requests_total` (`server.go:1240-1250`) | the operator infers protection from a running proxy | No | — | None |
| | | | Tested: `TestUnparsedTrafficIsCountedAndAnnouncedOnce` (`unparsed_test.go:20`) — unit only, not over HTTP | | | | |

## 3. Derived identity and privacy

| Surface | Primitive | Default | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **S18 Prefix hashing** | tools + system prompt, as raw JSON | on for readable bodies | `sha256` over the raw values, whitespace included, first 16 hex (`ledger/summarize.go:91,127-133`; OpenAI at `ledger/openai.go:65`) | a hash that moves for a byte-identical prefix — the sibling gate (S49) and pre-flight (S34) both key on it | No | **Ordering:** masking runs at `server.go:511-514`, *before* summarize at `server.go:525-532`, so the prefix hash is over the **masked** body. Turning `-mask` on changes the sibling key for identical client bytes | Medium — a cheaper stable identity would cut per-request hashing of the whole tool block |
| | | | Tested: `TestPrefixChangeIsNamedAsBreakCause` (`server_test.go:855`); lane scoping `prefixlane_test.go:32,68,94` | | | | |
| **S19 Session identity fallback** | `x-claude-code-session-id`, else `SessionHash` | on (`server.go:483`, fallback `server.go:534-536`) | a client that sends no session header still gets one stable id per conversation | every unattributed request collapses onto one key; the spend cap then meters all of them together | No | `SessionHash` is derived from `system` + the first message (`summarize.go:93`), so it exists only for summarized bodies | Low |
| | | | **UNTESTED in `internal/proxy`**: no proxy test omits `HeaderSessionID`. Only `ledger/openai_session_test.go:25` and `ledger/provider_conformance_test.go:361` assert the hash is non-empty | | | | |
| **S20 Label hashing** | tool-call arguments and file paths | on for readable bodies | paths are HMAC'd with the ledger key and only the extension survives; every other argument is dropped (`summarize.go:33-64`); block text is stripped (`summarize.go:117-122`) | request content reaches the ledger | No | this is the privacy boundary the whole ledger rests on, and it is computed **on the request goroutine** | Medium — HMAC per tool-use block per request |
| **S21 Session typing** | prompt byte total | on when `-policy-file` is set | `promptSize` sums system + tools + every block (`server.go:898-906`) feeding `learn.TypeFromBytes` (`server.go:859`) | a session lands in the wrong arm of the trial | No | evaluated once, at a session's first request | Low |
| | | | Tested: `TestPolicyIsChosenBySessionType` (`server_test.go:1456`) | | | | |
| **S22 Lane identity** | `x-claude-code-agent-id` | on (`server.go:483`) | pre-flight compares a lane against **its own** previous prefix, not the session's (`preflight.go:76`, `state.go:931-943`) | a fan-out session refuses lanes that changed nothing — the exact instrument error `preflight.go:24-30` records, where 31 of 34 measured events never happened | No | a client that omits the header puts every sub-agent in one lane | Low |
| | | | Tested: `TestPreFlight_ASiblingLaneDoesNotTriggerARefusal` (`preflight_test.go:190`) | | | | |

## 4. Body mutation

| Surface | Primitive | Default | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **S23 Secret masking** | string literals in a Messages body | **off** (`serve.go:74`) | matched secrets are replaced by vault placeholders *in place* — byte ranges spliced, never re-serialized (`masking/mask.go:128-130`); only `/v1/messages` (`server.go:511`) | a credential leaves the machine | Yes — `-mask`, `-mask-patterns`, `-mask-entropy`; forced off by `REPLAY_NO_POLICY` (`serve.go:88-95`) | the provider receives different bytes from the ones the client sent, and S18's prefix hash moves with them | Low. Do not trade correctness here |
| | | | Tested: `TestMaskingReplacesSecretsBeforeEgressAndKeepsThemLocal` (`server_test.go:1650`), `TestMaskingIsDeterministicOnTheWire` (`server_test.go:1712`) | | | | |
| **S24 Mask fail-secure** | a vault write failure | on whenever S23 is on | the region is blind-scrubbed and the **safe** body forwarded; the error is still returned (`masking/mask.go:120-127`, caller `server.go:668-682`) | the one place in the program that must not fail open would put a positively-identified credential on the wire | No | `rec.MaskDegraded` is set (`server.go:665`) so the ledger, not only stderr, records it | None |
| | | | Tested: `TestServerMaskDoesNotForwardASecretWhenTheVaultFails` (`maskfailclosed_test.go:33`). **`rec.MaskDegraded` itself UNTESTED** — no test references the field | | | | |
| **S25 Unmasked-path warning** | `/v1/chat/completions` with `-mask` on | on (`server.go:515-522`) | one `NOT MASKED` line per path + `replay_unmasked_requests_total` (`server.go:1257-1266`) | an operator running `-mask` believes that traffic is redacted; it is forwarded in clear | No | — | None |
| | | | **UNTESTED**: `noteUnmasked` and the string `NOT MASKED` appear in no test file | | | | |
| **S26 Context-edit splice** | one top-level member of the request object | **off** (`serve.go:69`, trigger 0) | inserted before the final `}` with every client byte kept in place, validated with `json.Valid`, reverted to the original on any doubt (`policy/contextedit.go:103-122`); only when the client sent the beta header and set no `context_management` (`contextedit.go:89-97`); skipped for unsummarized bodies (`server.go:805-810`); logged with sha256 before/after, never content (`server.go:822`) | the provider receives a parameter the client did not ask for, or an invalid body | Yes — `-context-edit-trigger`/`-keep`, or `-policy-file`; killed by `REPLAY_NO_POLICY` even against a persisted pin (`server.go:829-831`) | this is the reference shape for an admissible live transform: pinned, splice-only, fail-open, hash-logged | Low — already minimal |
| | | | Tested: `TestContextEditPolicyIsAppliedRecordedAndPinned` (`server_test.go:929`), `TestUnparsedRequestsNeverCarryThePolicy` (`server_test.go:1412`), `TestPoliciesOffOverrideAPersistedPin` (`server_test.go:1385`) | | | | |
| **S27 Policy pinning** | a session's first request | on whenever S26 is configured | a persisted pin wins over the flag, which wins over the file; the decision is written before the request goes out (`server.go:837-871`) and never revisited (`server.go:796-801`) | a config change mid-session makes one conversation incoherent | Yes — the pins file, the policy file, the revert file | **Two synchronous disk operations run on the request goroutine at each session's first request**: `learn.LoadFile` re-reads and re-parses the policy JSON (`server.go:914`, `learn/learn.go:502-511`) and `Store.SetPin` does open/write/close (`server.go:867`, `ledger/store.go:411-430`) | **Medium** — see O3 |
| | | | Tested: `TestPolicyFileIsReadAtSessionStartAndPinsSurviveRewritesAndRestarts` (`server_test.go:1268`), `TestPolicyFlagWinsOverFileAndStaleFileAppliesNothing` (`server_test.go:1330`) | | | | |
| **S28 Include-usage injection** | the whole top-level object of an OpenAI-compatible body | **ON by default** (`server.go:545`: `openai && !s.cfg.NoPolicy`) | a streaming request with no `stream_options` gets `{"include_usage":true}`; a client that set it keeps its own value; an unparseable body is returned as it arrived (`server.go:1278-1296`) | without it this family reports no usage on a stream, so the spend cap, the error budget and every cost figure see a free request | Yes — only `REPLAY_NO_POLICY=1` turns it off. **There is no flag** | **It re-serializes.** `json.Unmarshal` into a map then `json.Marshal` emits keys in sorted order with normalised whitespace, so no client byte is guaranteed to stay in place. It is the only mutation path in the request that does not splice, and the only one on by default | **High** — see O1 |
| | | | Tested: `openai_usage_test.go:17,38,50,58` — behaviour only. **UNTESTED**: that unrelated keys keep their order or the body its bytes; and no test asserts `rec.Policy == "openai-include-usage"` over the wire | | | | |

## 5. Guards

| Surface | Primitive | Default | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **S29 Circuit breaker** | consecutive provider failures | **off** (`serve.go:63`, 0 failures) | while open, requests are refused locally with `Retry-After` before the body is even read (`server.go:485-491`) | the agent burns its own retries against a provider already saying no | Yes — `-breaker-failures`, `-breaker-cooldown` (30 s default, `serve.go:31`) | refusals are deliberately never fed back to `Observe`, or a 503 refusal would re-arm the cooldown forever (`server.go:1039-1043`) | None |
| | | | Tested: `TestBreakerHoldsRequestsAfterProviderFailures` (`server_test.go:581`), `TestBreakerOpensAndProbes` (`guards_test.go:101`) | | | | |
| **S30 Probe release** | the single half-open probe | on with S29 | a probe that never reached an outcome is given back (`server.go:493-498`) | every request refused until restart | No | — | None |
| | | | **UNTESTED at the handler**: only `Breaker.Release` in isolation (`guards_test.go:101`) | | | | |
| **S31 Spend cap** | session and UTC-day tokens and list-price dollars | **off** (`serve.go:56-59`, all 0) | refuses the *next* request once a cap is reached, never interrupts one in flight (`server.go:946-952`, `guards.go:146-170`); the day total survives a restart (`guards.go:427-471`) | a cap that resets on restart, which is worse than no cap because the operator believes it | Yes — four flags; bypassed by S35 | an unpriced model contributes nothing, so a dollar cap silently never fires; `CapNotEnforced` exists to say so (`guards.go:86-95`, surfaced at `server.go:449`) | Low |
| | | | Tested: `TestSpendCapRefusesNextRequestNotCurrent` (`server_test.go:463`), `TestDayCapSurvivesARestart` (`guards_test.go:219`), `TestDollarCapOnAnUnpricedModelIsReportedNotSilentlyIgnored` (`guards_test.go:190`) | | | | |
| **S32 Error budget** | the share of a session's prompt tokens carrying error content | **off** (`serve.go:60`) | evaluated only above 10 000 prompt tokens, over **all** lanes summed (`guards.go:247,254-263`, `state.go:317-336`) | a numerator from one lane over a session-wide denominator — a ratio between two populations, refusing live traffic | Yes — `-error-budget`; bypassed by S35 | — | Low |
| | | | Tested: `TestErrorBudgetRefusesBeforeSpendCapAndHonorsOverride` (`server_test.go:1167`), `TestErrorBudgetCountsEveryLaneOfTheSession` (`errorbudget_test.go:14`) | | | | |
| **S33 Loop detector** | the tail run of identical tool calls | **off** (`serve.go:61-62`) | counts only the *tail*, so a legitimate repeat earlier cannot block a session forever; identity is the content-free HMAC call key, so the body is not re-parsed (`guards.go:288-314`) | either no detection, or a session blocked by history | Yes — `-loop-warn`, `-loop-block`; bypassed by S35 | the warn case adds `x-replay-warning`, the only header Replay adds to a response (`server.go:145,968`) | Low |
| | | | Tested: `TestLoopGuardWarnsThenBlocks` (`server_test.go:516`), `TestDetectLoop` (`guards_test.go:58`) | | | | |
| **S34 Pre-flight ceiling** | an estimated deficit from a changed prefix | **off, and unreachable** — see F1 | a diverged lane over the ceiling is refused; a ceiling inside the ±15 % error band warns instead of refusing (`preflight.go:91-110`, `analysis/predictor.go:56,90-95`) | refusing on noise, or the fact arriving one request after the money is spent | **No — there is no flag, env var or config path that sets `Config.PreFlight`** | setting `OptInActive` is the only thing that would make the code do anything at all | **High as a gap, not as a speed-up** — see O4 |
| | | | Tested: eight functions in `preflight_test.go`, all calling `s.preFlight` directly. **No HTTP-path test, and no wiring** | | | | |
| **S35 Override header** | `x-replay-override` | on, unconditional | absent → guards enforce; present with any value → S31, S32, S33 and S34 are all bypassed once and the value is logged as the reason (`server.go:945-969`, `preflight.go:112-115`) | any client can turn four guards off for itself | Yes — by the **client**, per request | this is the one place a request header changes Replay's behaviour, and `preflight.go:55-58` argues against exactly that pattern | None. It is a deliberate escape hatch; the finding is that its scope is all four guards at once |
| | | | Tested: the override arm of `server_test.go:463,516,1167` and `preflight_test.go:167` | | | | |

## 6. Refusal path

| Surface | Primitive | Default | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **S36 Refusal response shape** | the local answer | on | the provider's error JSON so a provider-aware client renders it, plus `Retry-After` when the breaker set one (`server.go:1083-1093`, kinds at `server.go:1011-1017`) | an agent shows a bare status the user cannot act on | No | — | None |
| | | | Tested: `TestRefusalStillAnswersTheClient` (`refusal_test.go:44`) | | | | |
| **S37 Refusal ledger record** | one record written off the bookkeeping path | on when a store exists | written from `recordRefusal` (`server.go:1068-1081`), deliberately **not** through the response tap, or a 503 refusal would re-arm the breaker forever | a guard firing is invisible except as a counter and a stopped log | No | — | None |
| | | | Tested: `TestRefusalIsRecordedOnTheLedger` (`refusal_test.go:101`), `TestRefusalRecordCarriesNoContent` (`refusal_test.go:132`), `TestRefusalIsNeverObservedAsAProviderFailure` (`refusal_test.go:66`) | | | | |
| **S38 Day-cap attribution** | who spent the day's budget | on with a day cap | names the largest surviving session, or says the accounting cannot support a name after eviction (`guards.go:181-225`) | blaming a small lane for someone else's overrun | No | — | None |
| | | | Tested: `spend_attribution_test.go:33,62,95,123,147,172` | | | | |

## 7. Egress shaping

| Surface | Primitive | Default | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **S39 Token stripping** | `x-replay-token` on the outbound | on (`server.go:211`) | never reaches the provider | Replay's own listener secret is sent to a third party | No | — | None |
| | | | Tested: `TestBrowserOriginAndTokenChecks` (`server_test.go:390`, assertion at `:409`) | | | | |
| **S40 Host rewrite** | `r.Out.Host` | on (`server.go:206`) | set to the upstream host | the provider receives the loopback Host | No | — | None |
| **S41 Forwarding-header restore** | `Forwarded`, `X-Forwarded-{For,Host,Proto}` | on (`server.go:136,212-218`) | a client's own values reach the provider; Replay adds none of its own | the reverse proxy's rewrite silently drops the client's bytes | No | — | None |
| | | | Tested: `TestClientForwardingHeadersAreKept` (`server_test.go:1426`) asserts **`X-Forwarded-For` only**. **`Forwarded`, `X-Forwarded-Host` and `X-Forwarded-Proto` UNTESTED** | | | | |
| **S42 Accept-Encoding deletion** | a client request header | off; on whenever `-rehydrate` has a rehydrator (`server.go:557-562`) | removed so the response arrives uncompressed and can be rewritten as it passes | a compressed body cannot be rehydrated, and the placeholders reach the user | Yes — implied by `-mask` + `-rehydrate` (default true, `serve.go:77`) | **the client asked for compression and does not get it**; this is a removed client header, contra `server.go:5-6` | Medium — costs bandwidth and decompression on every rehydrated session |
| | | | Tested: `TestRehydrationRestoresPlaceholdersWithinScope` (`server_test.go:1812`, assertion at `:1854`) | | | | |
| **S43 Transport compression** | Go's automatic `Accept-Encoding: gzip` | disabled (`server.go:191-194`) | the client's own encoding preference is what reaches the provider; nothing decompresses behind its back | Go adds a header the client did not send and transparently decodes | No | — | None |
| | | | Tested indirectly: `TestGzipResponseIsForwardedCompressedAndParsed` (`server_test.go:350`) | | | | |
| **S44 Environment proxy** | `HTTP_PROXY` / `HTTPS_PROXY` | inherited (`server.go:186`) | requests reach the host the operator configured | with an `http://` upstream, the full request and its credential headers go in plaintext to whatever `HTTP_PROXY` names | Yes — by the environment, invisibly | the outbound host is **not** necessarily the one configured | None. Recorded at `docs/SURFACES.md:70-78` |
| **S45 Upstream timeouts** | dial, TLS handshake, response headers, idle pool | 30 s / 30 s / 10 min / 90 s (`server.go:44-50`, applied `server.go:185-193`) | a frontier turn running for minutes is not cut; there is deliberately **no** overall request timeout — the client owns that | either a hung connection or a killed long turn | No — constants | — | Low |
| **S46 Flush interval** | streamed writes | `-1`, flush every write (`server.go:223`) | streamed events reach the client as they arrive | a buffered stream that arrives in bursts | No | — | Low |
| | | | Tested: `TestStreamingIsFlushedIncrementally` (`server_test.go:310`) | | | | |
| **S47 Retry transport** | a failed round trip | **off** (`serve.go:66`, 0 attempts) | resends only on a retryable status or a **dial** failure, only while `GetBody` is non-nil, and only below the reverse proxy so no response byte can have reached the client (`retry.go:67-101,152-159`) | a request already billed is sent again | Yes — `-retries`, `-retry-base`, `-retry-max` | a reset after the body was written, or a header timeout, is deliberately *not* resent because it may already have billed (`retry.go:152-158`) | Low |
| | | | Tested: `TestRetriesResendUntilSuccessAndAreRecorded` (`server_test.go:1060`), `TestRetriesStopOnClientErrorsExhaustionAndLongRetryAfter` (`server_test.go:1095`), `TestRetriesResendWhenTheProviderCannotBeReached` (`server_test.go:1355`) | | | | |
| **S48 Retry-After ceiling** | the provider's own wait | on with S47 | a `Retry-After` under `-retry-max` replaces the backoff; one over it **ends** the retries (`retry.go:107-118`) | the client waits longer than its user will sit through | Yes — `-retry-max` (30 s default) | — | Low |
| | | | Tested: `TestRetryAfterParsesSecondsAndDates` (`server_test.go:1125`) | | | | |
| **S49 Sibling hold** | a request whose prefix is in flight and not yet cached | **off** (`serve.go:65`, `MaxWait` 0) | a parallel sub-agent waits, bounded by `MaxWait` or its own cancellation, then reads the cache instead of writing it (`server.go:563-575`, `siblings.go:66-96`); a leader whose response begins releases the rest (`siblings.go:100-113`) | every parallel sub-agent pays the cache-write price | Yes — `-hold-siblings` (suggested 10 s, `siblings.go:26`) | **This is the one surface that deliberately delays the request path.** It keys on `PrefixHash` (S18), so its behaviour moves with masking | **This is already the request path's optimisation feature.** Its measured effect is `rec.HeldMS` |
| | | | Tested: `TestSiblingsAreHeldUntilTheFirstResponseBegins` (`siblings_test.go:119`), `TestSiblingsWaitIsBoundedAndFailuresRelease` (`siblings_test.go:182`) | | | | |

---

## Counts

- **49 surfaces** on the request path.
- **21 alterable** without a code change (S1, S2, S3\*, S6, S10, S23, S24\*, S26, S27, S28, S29, S31, S32, S33, S35, S42, S44, S47, S48, S49, and S12 via the request path a client chooses). *\*S3 and S24 are alterable only by choosing a different socket path or vault state, not by a switch.*
- **11 have an untested pass condition**: S7 (the timeout field), S9 (`Sec-Fetch-Mode`), S13 (nothing streamed), S14 (413), S15 (400), S19 (`SessionHash` fallback), S24 (`MaskDegraded`), S25 (`NOT MASKED`), S28 (byte stability), S30 (handler probe release), S41 (three of four forwarding headers).
- **1 surface is fully tested and fully unwired**: S34.

## Where the code and the docs disagree

| Claim | Where | What the code does |
|---|---|---|
| "Nothing here rewrites a request body or removes a client header" | `internal/proxy/server.go:5-6` | rewrites at `server.go:511-514,540-543,545-552`; removes headers at `server.go:211,561` |
| "Nothing in the body is re-serialized in passthrough mode; bytes in are bytes out" | `docs/architecture/proxy-protocol.md:35` | true of S23 and S26; **false of S28**, which is on by default for `/v1/chat/completions` |
| "With a live policy enabled, the only change is one top-level member spliced… Every byte the client sent stays in place" | `docs/architecture/proxy-protocol.md:36` | true of S26. S28 sets `rec.Policy` (`server.go:551`) and is therefore reported as a policy, but does not splice |
| "consent is `s.cfg.PreFlight`, set by the operator" | `internal/proxy/preflight.go:57-58` | no operator can set it — F1 |
| "It does not read consent from a request header, because that would let any client switch the guard on or off" | `internal/proxy/preflight.go:55-57` | the override header at `preflight.go:112` does exactly that for the refusal |
| the pre-flight guard, its refusal kind and its counter | absent from `docs/SURFACES.md` and every ADR | exists in code, is unreachable |

## Proposed alterations, each with the test that decides it

**O1 — make `withUsageReporting` splice instead of re-serialize. Build this first.**
*Stage:* this is not a new rewrite; it makes an existing default-on rewrite
deterministic and byte-preserving, so it belongs at ADR-0011 **stage 2's
standard applied retroactively** — a transform whose output can be proven equal
to its input plus one named member.
*Change:* insert `,"stream_options":{"include_usage":true}` before the final
`}` using the same mechanism as `policy/contextedit.go:103-122`, and keep the
`json.Valid` fail-open.
*Test (goes red today):*
`TestIncludeUsageIsSplicedAndLeavesEveryOtherByteInPlace` — feed a body whose
top-level keys are in deliberately non-alphabetical order with irregular
whitespace (`{"stream":true,  "model":"x", "messages":[]}`), assert the output
is **byte-for-byte** the input with the one member inserted before the closing
brace, and assert a second body that differs only in key order produces a
correspondingly different output. Today's `json.Marshal` implementation
normalises both to the same sorted form, so the equality assertion fails.
*Why first:* it is the only default-on request mutation that is not
byte-preserving, it restores `proxy-protocol.md:35-36` to true, and the failing
test is one table entry.

**O2 — do not buffer bodies on paths this build cannot read.**
*Stage:* **0.** It strictly reduces what Replay touches; no ADR-0011 gate
applies.
*Change:* at `server.go:474-500`, when `!readable`, skip the `io.ReadAll` and
hand `r.Body` straight to the reverse proxy.
*Test (goes red today):* `TestAnUnreadablePathIsStreamedNotBuffered` — a POST to
`/v1/responses` whose client writes the first half of the body, then blocks
until an upstream stub signals it has received those bytes, then writes the
rest. Today the handler is inside `io.ReadAll` and the stub never sees a byte,
so the test deadlocks to its timeout.
*Caveat to state in the change:* `MaxRequestBytes` (S14) no longer applies to
those paths, and neither does the 413. That is a real trade and belongs in the
commit message, not in a footnote.

**O3 — cache the policy file behind an mtime check.**
*Stage:* **0.** No bytes change; only when a file is read.
*Change:* `server.go:914` re-reads and re-parses the policy JSON at every
session's first request. Hold the parsed `learn.Result` with the file's
`ModTime` and size, and re-read only when either moves. The existing pinning
contract is unaffected: a session already keeps its first decision
(`server.go:796-801`), and `TestPolicyFileIsReadAtSessionStartAndPinsSurvive…`
(`server_test.go:1268`) must stay green unchanged.
*Test:* introduce a `policyLoader func(string) (learn.Result, time.Time, error)`
field on `Server` defaulting to the current call, then
`TestPolicyFileIsParsedOncePerGeneration` — start 20 sessions against one
unchanged file and assert the loader ran once; touch the file with a new
`ModTime` and assert it ran a second time. Today the counter reads 20.

**O4 — wire the pre-flight ceiling, or delete it.**
*Stage:* **2** if wired — a refusal is a live behaviour change and must be
off by default, which `PolicyState`'s zero value already guarantees
(`analysis/predictor.go:22-25`).
*Change:* add `-preflight-ceiling` (int, 0 = off) setting
`analysis.PolicyState{CeilingTokens: n, OptInActive: n > 0}` at
`cmd/replay/serve.go:136`.
*Test (goes red today):*
`TestPreFlightRefusesOverTheCeilingOverHTTP` — serve with the flag, send a
first request establishing a lane's prefix, then a second with a changed tool
set whose estimated deficit is over the ceiling, and assert a 400 carrying
`replay_preflight_deficit`. All eight existing tests call `s.preFlight`
directly, so nothing today proves the guard is reachable from `handle`; with no
flag, this test cannot even be written.
*The alternative is honest too:* if nobody intends to ship the ceiling, delete
`preflight.go`, its refusal kind and its tests. A guard that is green in CI and
absent in the binary is a check that cannot fail.

**Not proposed.** A faster prefix hash (S18), a cheaper label HMAC (S20) and
skipping the second `setBody` allocation (S16) are all real costs, but this
document has no measurement of them and `internal/proxy/latency_bench_test.go`
does not isolate any of the three. Proposing them without a stateable
pass condition would be the thing ADR-0011 exists to prevent.

---

## Limits of this document

Read-only against the working tree of 2026-09-09; nothing was executed inside
the repository and no test was run. Test coverage was established by opening
each named test and confirming what it asserts, not by inference from names;
the **UNTESTED** cells were established by grep across
`internal/proxy/*_test.go` and `cmd/replay/*_test.go` for the identifier or the
literal string, and each is stated as "no test file references X" rather than
as "the behaviour is wrong". The response tap, the ledger's storage and
retention behaviour, and per-provider response parsing are out of scope — see
parts 2, 3 and 4.

---

[Design](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
