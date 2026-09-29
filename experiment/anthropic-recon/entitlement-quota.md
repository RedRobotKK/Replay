# Anthropic product, entitlement and quota surfaces

Reconnaissance map for the Anthropic optimization campaign. Documentation
research only. No paid API call was made to produce this file.

**Fetch date for every row below: 2026-09-29** unless a row states otherwise.
These values change. Re-fetch before citing any figure in a public claim.

Evidence classes used, one per row:

| Class | Meaning |
| :- | :- |
| DOCUMENTED | Anthropic's own published documentation, with the URL that was fetched |
| OBSERVED | Visible in this repository |
| DERIVED | Arithmetic over DOCUMENTED or OBSERVED rows, with the arithmetic shown |
| HYPOTHESIS | A candidate mechanism, not a finding |
| UNKNOWN | Not published, or could not be verified from published sources |

This file records no HYPOTHESIS rows by design. Where a mechanism is not
published it is recorded as UNKNOWN. Per the repository's own contract at
`docs/ANTHROPIC-EFFICIENCY-CONTRACT.md` rule A11, provider internals stay
hypotheses and are never promoted to findings without an experiment.

Note on domains: the documentation moved during 2026. `docs.claude.com`
301-redirects to `platform.claude.com/docs` (API and platform) and to
`code.claude.com/docs` (Claude Code). `support.anthropic.com` redirects to
`support.claude.com`. Every URL in the Sources section is the post-redirect
address that was actually fetched.

---

## 0. Proposed quota vocabulary

The word "quota" collapses at least eight distinct mechanisms that reset on
different clocks, are enforced by different systems, and respond to different
optimizations. The campaign needs these kept apart. The following vocabulary is
proposed for this repository.

| Term | Definition | Unit | Reset or scope | Enforcement signal | Evidence for the underlying mechanism |
| :- | :- | :- | :- | :- | :- |
| USAGE_LIMIT | Subscription entitlement consumed by doing work. Not denominated in dollars to the user. | Opaque internal unit, not published | Rolling five-hour window; separate weekly window | `You've hit your session limit`, `You've hit your weekly limit` | DOCUMENTED, https://code.claude.com/docs/en/errors and https://code.claude.com/docs/en/costs |
| CONTEXT_LIMIT | Maximum tokens in one request's context window. A capacity bound, not an entitlement. | Tokens | Per request | Auto-compact warning; API 400 | DOCUMENTED, https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work and https://platform.claude.com/docs/en/about-claude/pricing |
| RATE_LIMIT | Throughput ceiling on the API, per organization, per model, per minute. | RPM, ITPM, OTPM | Token bucket, continuously replenished | HTTP 429 with `retry-after` | DOCUMENTED, https://platform.claude.com/docs/en/api/rate-limits |
| SPEND_LIMIT | Dollar ceiling on billed usage, set by Anthropic tier or by an admin. | USD per calendar month | 00:00 UTC on the first of the month, or admin action | HTTP 429 `enforced_spend_limit_reached` (tier cap) or HTTP 400 `invalid_request_error` (self-set) | DOCUMENTED, https://platform.claude.com/docs/en/api/rate-limits |
| CREDIT_LIMIT | Balance of prepaid or metered usage credits that extend work past a USAGE_LIMIT, billed at API rates. | USD | Monthly billing period; auto-reload optional | `You've hit your monthly spend limit` and variants | DOCUMENTED, https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans |
| MODEL_LIMIT | A limit scoped to one model family rather than to the account as a whole. | Two kinds: (a) subscription model-family usage window, (b) per-model API RPM/ITPM/OTPM | (a) rolling window, (b) per minute | `You've hit your Opus limit`, `You've hit your Sonnet limit`; per-model 429 | DOCUMENTED, https://code.claude.com/docs/en/errors and https://platform.claude.com/docs/en/api/rate-limits |
| FEATURE_LIMIT | A ceiling attached to one product feature rather than to tokens. | Feature-specific | Feature-specific | Feature-specific | DOCUMENTED, examples below |
| CONCURRENCY_LIMIT | Ceiling on simultaneous in-flight work. | Count of queued or running items | Continuous | Batch queue rejection | DOCUMENTED for Batch API; UNKNOWN for subscription surfaces |

Two further terms are needed because the surfaces do not reduce to the eight
above.

| Term | Definition | Evidence |
| :- | :- | :- |
| ENTITLEMENT_REGIME | The pairing of an authentication method with a billing model, which together decide which limit classes apply at all. A seat-based subscription is a different regime from a Console API key even on the same machine and the same model. | DOCUMENTED, https://code.claude.com/docs/en/costs states "If your organization mixes sign-in methods, each developer is metered according to the one they authenticated with" |
| ACCESS_CONTROL | An admin toggle that removes a model or a surface entirely, rather than metering it. Not a quota. | DOCUMENTED, https://support.claude.com/en/articles/15694740-manage-model-access-for-your-organization |

**The relationship between USAGE_LIMIT and every other class is UNKNOWN.**
Anthropic does not publish the unit that USAGE_LIMIT is denominated in, nor the
function that maps tokens to that unit. This is the single largest gap in the
map and Section 6 records why it blocks optimization claims.

---

## A. Product surfaces

### A1. Plans

| Plan | Exists | Price | Included usage, as published | Evidence | URL |
| :- | :- | :- | :- | :- | :- |
| Claude Free | Yes | $0 | Baseline; five-hour rolling session window; Projects limited to 5 | DOCUMENTED | https://claude.com/pricing |
| Claude Pro | Yes | $17/month billed annually ($200 up front), $20 billed monthly | "at least 5x more usage per 5-hour session than Free"; weekly caps apply | DOCUMENTED | https://claude.com/pricing |
| Claude Max 5x | Yes | $100/month | 5x Pro per five-hour session | DOCUMENTED | https://support.claude.com/en/articles/11049741-what-is-the-max-plan |
| Claude Max 20x | Yes | $200/month | 20x Pro per five-hour session | DOCUMENTED | https://support.claude.com/en/articles/11049741-what-is-the-max-plan |
| Claude Team, Standard seat | Yes | $20/seat/month annually, $25 monthly | "1.25x the Pro plan's per-session usage allowance" | DOCUMENTED | https://support.claude.com/en/articles/9266767-what-is-the-team-plan |
| Claude Team, Premium seat | Yes | $100/seat/month annually, $125 monthly | "6.25x the Pro plan's per-session usage allowance" | DOCUMENTED | https://support.claude.com/en/articles/9266767-what-is-the-team-plan |
| Claude Enterprise, usage-based (current) | Yes | US$20/seat/month billed annually, plus usage at API rates | Seat fee covers platform access only. "The seat fee only covers access to the platform and doesn't include any usage." No plan or seat-level usage limits. | DOCUMENTED | https://support.claude.com/en/articles/9797531-what-is-the-enterprise-plan |
| Claude Enterprise, seat-based (legacy) | Yes, for organizations not yet migrated | Not published on the public pricing page | Standard and Premium seats with per-seat included usage limits, plus usage credits | DOCUMENTED | https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans |
| Claude Console / API | Yes | Per token, see pricing table | No included allowance; SPEND_LIMIT and RATE_LIMIT by usage tier | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |

Discrepancy recorded rather than resolved: the extraction of
`https://claude.com/pricing` returned "From $100 per month" for both Max 5x and
Max 20x, while the Max help article gives "Max 5x: $100 per month" and "Max 20x:
$200 per month". The help-article figures are the more specific statement and
are used above. The pricing-page reading is treated as an extraction artifact,
not as a second published price. Anyone citing a Max price should re-fetch both.

Team seat minimums: minimum 2 members, maximum 150 seats before an Enterprise
upgrade is required (DOCUMENTED,
https://support.claude.com/en/articles/9266767-what-is-the-team-plan).
Enterprise seat minimums: 20 seats self-serve, 50 seats sales-assisted
(DOCUMENTED,
https://support.claude.com/en/articles/9797531-what-is-the-enterprise-plan).

### A2. Surfaces, and whether their accounting differs

Do not assume one quota model across this table. The differences are the point.

| Surface | Exists | What it is | Usage accounting | Evidence | URL |
| :- | :- | :- | :- | :- | :- |
| Claude.ai (web) | Yes | Chat surface | Draws the shared subscription USAGE_LIMIT pool | DOCUMENTED: "Your usage of all different Claude product surfaces (claude.ai, Claude Code, Claude Desktop) counts towards the same usage limit." | https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work |
| Claude Desktop | Yes | Desktop app | Same shared pool. Does not read `apiKeyHelper`, `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN`; uses OAuth, except desktop sessions on a third-party inference configuration | DOCUMENTED | https://code.claude.com/docs/en/iam |
| Claude Code (CLI) | Yes | Agentic coding CLI | Metered by whichever credential authenticated. Subscription: seat USAGE_LIMIT. Console: per token to the org, in an auto-created "Claude Code" workspace. Cloud provider: per token to the cloud account | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Claude Code in IDEs (VS Code extension, Cursor, JetBrains) | Yes | Same engine, IDE host | Same credential rules as the CLI. VS Code extension shows attribution and behavior flags in an Account & usage dialog, without the Loops rows | DOCUMENTED | https://code.claude.com/docs/en/costs, https://support.claude.com/en/articles/11845131-use-claude-code-with-your-team-or-enterprise-plan |
| Claude Code on the web / cloud sessions | Yes | Hosted sessions | "Cloud sessions always use your subscription credentials." `ANTHROPIC_API_KEY` set in the cloud environment does not override them | DOCUMENTED | https://code.claude.com/docs/en/iam |
| Claude Cowork | Yes | Agentic knowledge work beyond coding; local VM sessions or cloud sessions (beta) | Shares the seat allowance with chat and Claude Code on Team and Enterprise. Being merged with chat into one conversation. "Everything you do with Claude counts toward your plan's usage limits." Token intensity is higher than chat | DOCUMENTED | https://code.claude.com/docs/en/costs, https://support.claude.com/en/articles/16761823-claude-cowork-and-chat-are-one-claude, https://support.claude.com/en/articles/14782391-claude-enterprise-consumption-guide |
| Claude in Chrome | Yes | Browser extension. Cowork in Chrome available on Max and Team, rolling out to Pro; Enterprise availability is admin-controlled | Per-surface accounting UNKNOWN. Admin controls are documented | DOCUMENTED (existence and availability), UNKNOWN (accounting) | https://support.claude.com/en/articles/13455879-use-claude-cowork-on-team-and-enterprise-plans, https://support.claude.com/en/articles/13065128-claude-in-chrome-admin-controls |
| Connectors (MCP servers) | Yes | Tool integrations | On Claude Code, `/usage` attributes recent usage to individual MCP servers; a server's share counts only requests that consumed one of its tool results. MCP tool definitions are deferred by default | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Skills | Yes | On-demand instruction bundles | Attributed separately in the `/usage` breakdown. Load on demand rather than at session start | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Subagents | Yes | Isolated child contexts | "The subagent's own requests still draw on your usage." Attributed separately in `/usage` | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Agent teams | Yes | Multiple Claude Code instances, each with its own context window | "approximately 7x more tokens than standard sessions when teammates run in plan mode". Disabled by default behind `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Projects | Yes | Grouped conversations with shared context | FEATURE_LIMIT on Free: 5 projects. "Content in projects is cached and counts less against your limits when reused." | DOCUMENTED | https://claude.com/pricing, https://support.claude.com/en/articles/9797557-usage-limit-best-practices |
| Artifacts | Yes | Published pages | "Artifact creation and usage" is named as a factor that affects usage limits. Separate admin guide exists for Team and Enterprise | DOCUMENTED | https://support.claude.com/en/articles/9797557-usage-limit-best-practices |
| Claude Design | Yes | Design surface | Tracked as its own product dimension in Enterprise analytics (designs, edits, DAU/WAU/MAU) | DOCUMENTED | https://platform.claude.com/docs/en/api/admin/analytics |
| Office Agent, Claude Science, Claude Tag (Claude in Slack) | Yes | Additional surfaces | Each appears as a separate product dimension in the Enterprise Analytics API. Claude Tag has a per-channel SPEND_LIMIT | DOCUMENTED | https://platform.claude.com/docs/en/api/admin/analytics, https://code.claude.com/docs/en/errors |
| Web search, web fetch, research | Yes | Server-side tools | On the API these are priced separately from tokens: web search $10 per 1,000 searches, web fetch no additional charge. Named as a usage factor on subscriptions | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing, https://support.claude.com/en/articles/9797557-usage-limit-best-practices |
| Code execution tool | Yes | Server-side container | Billed by execution time, not tokens. 1,550 free container-hours per organization per month, then $0.05 per hour per container. Free when used with web search or web fetch | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing |
| Claude Managed Agents | Yes | Server-hosted agent sessions | Billed on two dimensions: tokens at standard rates, plus session runtime at $0.08 per session-hour, metered only while status is `running`. Batch discount and partner cloud platforms do not apply | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing |
| Claude Console | Yes | API management surface | Not metered itself. Hosts SPEND_LIMIT, RATE_LIMIT, workspaces, usage charts | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |
| Claude apps gateway | Yes | Self-hosted gateway, corporate SSO, routes inference to a configured provider | Has its own per-user spend limits, enforced by the gateway, with its own error messages | DOCUMENTED | https://code.claude.com/docs/en/costs |

---

## B. Authentication and entitlement

The controlling statement, quoted exactly:

> "If your organization mixes sign-in methods, each developer is metered
> according to the one they authenticated with."

DOCUMENTED, https://code.claude.com/docs/en/costs

### B1. Metering by authentication method

| Authentication method | Who is billed | Limit classes that apply | Where spend is visible | Where spend is capped | Evidence | URL |
| :- | :- | :- | :- | :- | :- | :- |
| Claude.ai consumer subscription (Pro, Max) | The individual | USAGE_LIMIT (5h + weekly), MODEL_LIMIT (Opus, Sonnet families), CONTEXT_LIMIT, CREDIT_LIMIT once usage credits are on, SPEND_LIMIT on credits only if the user sets one | `/usage` in Claude Code; Settings > Usage on claude.ai | Personal monthly spend limit on usage credits, at claude.ai/settings/usage | DOCUMENTED | https://code.claude.com/docs/en/costs, https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans |
| Claude for Teams | The organization, per seat | USAGE_LIMIT per member on a rolling five-hour window and a weekly window, sized by seat tier. "Usage limits on Team plans are per-member, rather than applied to the team as a whole." Plus org, group and individual SPEND_LIMIT on credits | Spend report in org analytics, CSV, daily refresh with a one-day delay | Spend limits in claude.ai admin settings; credits are pre-purchased on Team | DOCUMENTED | https://code.claude.com/docs/en/costs, https://support.claude.com/en/articles/9266767-what-is-the-team-plan, https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans |
| Seat-based Enterprise (legacy) | The organization, per seat | Same as Team: per-seat USAGE_LIMIT by Standard or Premium seat tier, plus usage credits with org, seat-tier, group and individual SPEND_LIMIT | Spend report CSV, plus Enterprise Analytics API | claude.ai admin settings > Usage | DOCUMENTED | https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans |
| Usage-based Enterprise (current) | The organization, per token | No USAGE_LIMIT and no included token allowance. "All usage across Claude, Claude Code, and Cowork is billed separately at standard API rates." SPEND_LIMIT at org, group and user level is the only ceiling | Spend report plus Enterprise Analytics API | Spend limits in admin settings | DOCUMENTED | https://support.claude.com/en/articles/9797531-what-is-the-enterprise-plan, https://support.claude.com/en/articles/14782391-claude-enterprise-consumption-guide |
| Claude Console API key | The API organization | RATE_LIMIT (RPM, ITPM, OTPM per model per minute), SPEND_LIMIT (tier monthly cap plus optional self-set), workspace RATE_LIMIT and workspace SPEND_LIMIT. No USAGE_LIMIT | Console usage page, Claude Code dashboard | Workspace spend limits; org spend limit on the Billing page | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits, https://code.claude.com/docs/en/costs |
| Console sign-in without an API key (Anthropic profile, OAuth) | The API organization | Same as API key. Difference is credential storage and refresh, not metering | Same as API key | Same as API key | DOCUMENTED | https://code.claude.com/docs/en/iam |
| Workload Identity Federation profile | The API organization | Same as API key | Same | Same | DOCUMENTED | https://code.claude.com/docs/en/iam |
| Amazon Bedrock | Your AWS account | Provider RATE_LIMIT and provider budget controls. Anthropic-side USAGE_LIMIT does not apply | Your cloud billing console | Your cloud's budget controls | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Google Cloud Agent Platform (Vertex) | Your Google Cloud account | Same shape as Bedrock | Your cloud billing console | Your cloud's budget controls | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Microsoft Foundry | Your Azure account, via Azure Marketplace in Claude Consumption Units at $0.01 per CCU | Provider limits plus CCU metering. US Data Zone Standard applies the 1.1x data residency multiplier | Azure Cost Management, aggregated CCU | Azure budget controls | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing, https://code.claude.com/docs/en/costs |
| Claude Platform on AWS | Your AWS account via AWS Marketplace in CCUs | Rate limits on the platform apply, but the organization is placed on the Start tier and moves up on AWS Marketplace invoice history. Per-workspace rate limit configuration and fast mode are not available | Claude Console via AWS Console, plus AWS Cost Explorer | Same monthly spend caps as the API tiers | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits, https://platform.claude.com/docs/en/about-claude/pricing |
| Claude apps gateway (self-hosted, corporate SSO) | Whichever provider the gateway routes to | Gateway per-user SPEND_LIMIT, enforced by the gateway, with its own reset schedule | Gateway telemetry, OTLP | Gateway spend limits | DOCUMENTED | https://code.claude.com/docs/en/costs |
| `CLAUDE_CODE_OAUTH_TOKEN` (one-year token from `claude setup-token`) | The subscription it was minted from | Subscription USAGE_LIMIT. "requires a Pro, Max, Team, or Enterprise plan" | `/usage` | Same as the subscription | DOCUMENTED | https://code.claude.com/docs/en/iam |

### B2. Credential precedence, which decides the regime

Claude Code chooses one credential in this documented order. This is the
mechanism by which a machine silently changes entitlement regime, so it is
recorded verbatim in order.

1. Cloud provider credentials, when `CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX` or `CLAUDE_CODE_USE_FOUNDRY` is set
2. `ANTHROPIC_AUTH_TOKEN`
3. `ANTHROPIC_API_KEY`
4. `apiKeyHelper` script output
5. `CLAUDE_CODE_OAUTH_TOKEN`
6. Anthropic profile and federation credentials
7. Subscription OAuth credentials from `/login`, the default for Pro, Max, Team and Enterprise

A signed-in Claude apps gateway session sits outside the list and outranks all of it.

DOCUMENTED, https://code.claude.com/docs/en/iam

Consequence, DOCUMENTED on the same page: "If you have an active Claude
subscription but also have `ANTHROPIC_API_KEY` set in your environment, Claude
Code uses the API key once you approve it." A benchmark run on a machine with a
stray environment variable measures a different entitlement regime than the
operator intended. Verify with `/status`, which marks the credential not in use.

### B3. Cache lifetime differs by regime

This is an entitlement-linked behaviour difference, not a pricing one, and it
matters for any caching optimization.

| Regime | Default prompt cache lifetime | Evidence | URL |
| :- | :- | :- | :- |
| Subscription (Pro, Max, Team, Enterprise seat) | One hour | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Subscription while drawing on usage credits | Drops to five minutes | DOCUMENTED | https://code.claude.com/docs/en/costs |
| API key or cloud provider | Five minutes by default | DOCUMENTED | https://code.claude.com/docs/en/costs |

The one-hour lifetime can be kept while on usage credits by choosing the TTL
explicitly (DOCUMENTED, same page).

---

## C. Quota dimensions, inventoried separately

### C1. USAGE_LIMIT, subscription surfaces

| Dimension | Window | Scope | Published magnitude | Evidence | URL |
| :- | :- | :- | :- | :- | :- |
| Session limit | Rolling five hours | Account, across all models, across claude.ai, Desktop, Claude Code and Cowork | Relative only: Pro "at least 5x" Free; Max 5x is 5x Pro; Max 20x is 20x Pro; Team Standard 1.25x Pro; Team Premium 6.25x Pro | DOCUMENTED | https://claude.com/pricing, https://support.claude.com/en/articles/9266767-what-is-the-team-plan |
| Weekly limit | Weekly, resetting at a fixed time assigned to the account | Account, across all models | Not published in absolute terms | DOCUMENTED that it exists; UNKNOWN magnitude | https://support.claude.com/en/articles/11049741-what-is-the-max-plan |
| Opus limit | Its own window | Opus model family only | Not published | DOCUMENTED that it exists; UNKNOWN magnitude | https://code.claude.com/docs/en/errors |
| Sonnet limit | Its own window | Sonnet model family only | Not published | DOCUMENTED that it exists; UNKNOWN magnitude | https://code.claude.com/docs/en/errors |
| Fable weekly allocation on Pro and Max | Weekly | Fable models | "50% of weekly limits via credits" as extracted from the pricing page. Reading is low confidence and should be re-verified against the plan page directly | DOCUMENTED with a caveat | https://claude.com/pricing |
| Unit of account | n/a | n/a | Not published. "How much you can do depends on the length and complexity of your conversations, the model you choose, and the features you use, so there's no fixed message count." | UNKNOWN | https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work |

Distinguishing behaviour, DOCUMENTED at https://code.claude.com/docs/en/costs:
the session and weekly limits are shared across all models, so switching model
with `/model` does not restore access. The Opus and Sonnet limits are
model-family scoped, so switching outside that family does restore access. This
is the operational test that tells a USAGE_LIMIT from a MODEL_LIMIT.

Published factors that increase USAGE_LIMIT consumption (DOCUMENTED,
https://support.claude.com/en/articles/9797557-usage-limit-best-practices):
message length, file attachment size, current conversation length, tool usage
including Research and web search, model choice, effort level, artifact creation
and usage, and multi-step tasks. Also documented: "Content in projects is cached
and counts less against your limits when reused." The magnitude of "counts less"
is UNKNOWN.

Documented change, dated by Anthropic's own announcement rather than by this
fetch: Claude Code's five-hour rate limits were doubled for Pro, Max, Team and
seat-based Enterprise, and the peak-hours limit reduction on Claude Code was
removed for Pro and Max (DOCUMENTED,
https://www.anthropic.com/news/higher-limits-spacex). Treat any figure predating
that announcement as stale.

### C2. RATE_LIMIT, Claude API

Per organization, per model, per minute. Token bucket, continuously replenished
rather than reset at fixed intervals.

| Tier | RPM (Opus 5.5 / Opus 5 / Sonnet 5.5 / Sonnet 5 / Haiku 4.5) | ITPM | OTPM | Fable 5.x RPM / ITPM / OTPM |
| :- | :- | :- | :- | :- |
| Start | 1,000 | 2,000,000 | 400,000 | 1,000 / 500,000 / 100,000 |
| Build | 5,000 | 5,000,000 | 1,000,000 | 2,000 / 1,500,000 / 300,000 |
| Scale | 10,000 | 10,000,000 | 2,000,000 | 4,000 / 4,000,000 / 800,000 |
| Custom | Arranged with the account team | | | |

DOCUMENTED, https://platform.claude.com/docs/en/api/rate-limits

Cache-aware ITPM, DOCUMENTED on the same page and load-bearing for any caching
claim: `input_tokens` and `cache_creation_input_tokens` count toward ITPM;
`cache_read_input_tokens` does **not**, for most models. Claude Haiku 3.5 is the
documented exception and does count cache reads. Anthropic's own worked example:
a 2,000,000 ITPM limit with an 80 percent cache hit rate processes 10,000,000
total input tokens per minute.

Also DOCUMENTED on that page:

- New organizations may start in an **Evaluation tier** below the published Start tier while account history is established.
- Acceleration limits produce 429s on a sharp usage increase, independent of the tier table.
- Rate limits are shared across `inference_geo` values; `us` and `global` draw from one pool.
- OTPM counts actual output tokens produced. `max_tokens` does not factor in, so raising `max_tokens` has no rate-limit cost.
- Rate limits are per model, so different models can be driven to their respective limits simultaneously.
- Full header set for observing limits: `anthropic-ratelimit-{requests,tokens,input-tokens,output-tokens}-{limit,remaining,reset}`, plus `anthropic-priority-*` on Priority Tier and `anthropic-fast-*` in fast mode. `anthropic-workspace-id` names the workspace a request counted against.
- Token headers report the most restrictive limit currently in effect, so a header value can come from a workspace limit rather than the organization limit.

Separate per-organization rate limits, DOCUMENTED on the same page:

| Surface | Limit |
| :- | :- |
| Managed Agents, create endpoints | 300 requests per minute |
| Managed Agents, read endpoints | 1,200 requests per minute |
| Files API | Own per-organization limit, shared across upload, list, retrieve, download and delete |
| Fast mode on Opus 5.5, Opus 5, Opus 4.8 | Dedicated limits separate from standard Opus limits |

### C3. CONCURRENCY_LIMIT

| Surface | Limit | Tier values | Evidence | URL |
| :- | :- | :- | :- | :- |
| Message Batches API | Maximum batch requests in the processing queue | Start 200,000; Build 300,000; Scale 500,000 | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |
| Message Batches API | Maximum batch requests per batch | 100,000 on every tier | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |
| Message Batches API | Own RPM, shared across all models | Start 1,000; Build 2,000; Scale 4,000 | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |
| Concurrent sessions on a subscription | Not published | UNKNOWN | | |
| Concurrent subagents or agent teammates | Not published as a hard ceiling. Agent teams are gated behind an env flag and guidance is to keep teams small | UNKNOWN as a limit; DOCUMENTED as guidance | https://code.claude.com/docs/en/costs |

### C4. SPEND_LIMIT

| Level | Value or mechanism | Reset | Error shape | Evidence | URL |
| :- | :- | :- | :- | :- | :- |
| API tier monthly cap, Start | $500 USD | 00:00 UTC, first of the month | HTTP 429, `error_code: enforced_spend_limit_reached`, no `retry-after` | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |
| API tier monthly cap, Build | $1,000 USD | Same | Same | DOCUMENTED | Same |
| API tier monthly cap, Scale | $200,000 USD | Same | Same | DOCUMENTED | Same |
| API tier monthly cap, Custom | No cap; arranged with the account team | n/a | n/a | DOCUMENTED | Same |
| Self-set organization spend limit | Any value at or below the tier cap | Same | HTTP 400, `invalid_request_error`, message begins "You have reached your specified API usage limits" | DOCUMENTED | Same |
| Workspace spend limit | Set per workspace | Same | HTTP 400 with the workspace variant of that message | DOCUMENTED | Same |
| Claude Code workspace | Checked separately. Over-limit Claude Code requests can instead receive a 429 carrying a `retry-after` header | Same | 429 with `retry-after` | DOCUMENTED | Same |
| Personal usage-credit spend limit (Pro, Max) | User-set monthly cap; auto-reload optional; daily redemption limit $2000 | Monthly | `You've hit your monthly spend limit` | DOCUMENTED | https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans |
| Organization spend limit (Team, Enterprise) | Admin-set monthly cap across the whole organization | Monthly billing period | `You've hit your org's monthly spend limit` | DOCUMENTED | https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans |
| Seat-tier spend limit (seat-based Enterprise) | Dollar amount or unlimited, per Standard and Premium tier | Monthly | Variant of the above | DOCUMENTED | Same |
| Group spend limit | Per-user monthly limit inherited by group members | Monthly | `You've hit your team's shared budget` is the documented pooled-budget message | DOCUMENTED | https://support.claude.com/en/articles/14782391-claude-enterprise-consumption-guide, https://code.claude.com/docs/en/errors |
| Individual member spend limit | Per-member monthly cap. "Individual limits always override group limits, regardless of which is higher." | Monthly | `You've hit your individual spend limit` | DOCUMENTED | https://support.claude.com/en/articles/14782391-claude-enterprise-consumption-guide |
| Claude Tag channel spend limit | Per Slack channel, monthly | Monthly | `You've hit your channel's monthly spend limit` | DOCUMENTED | https://code.claude.com/docs/en/errors |
| Claude apps gateway per-user spend limit | Set on the self-hosted gateway | Gateway-defined period | Gateway message | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Claude Code `--max-budget-usd` | Client-side per-session budget flag | Per session | Client-side | DOCUMENTED | https://code.claude.com/docs/en/costs |

### C5. CREDIT_LIMIT

| Attribute | Pro and Max | Team | Seat-based Enterprise | Usage-based Enterprise |
| :- | :- | :- | :- | :- |
| Exists | Yes | Yes | Yes | Not applicable. No included allowance to exceed |
| Billed at | "standard API rates" | "standard API rates" | "standard API rates" | All usage is at API rates by definition |
| Purchase model | Auto-reload on a threshold; daily redemption limit $2000 | Owners pre-purchase credits | Monthly billing on actual usage | n/a |
| Who turns it on | The individual, Settings > Usage | Owners, Primary Owners, or a custom role with Billing set to "Can manage" | Same | n/a |
| At the limit | Blocked until the next billing period or until limits are adjusted | Same | Same | Blocked at the spend limit |
| Evidence | DOCUMENTED | DOCUMENTED | DOCUMENTED | DOCUMENTED |

URLs: https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans,
https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans,
https://support.claude.com/en/articles/9797531-what-is-the-enterprise-plan

Exact quote on the blocking behaviour, DOCUMENTED: "If your account is
configured for usage credits and you exceed your set spend limit, you won't be
able to use Claude, Cowork, or Claude Code again until the next billing period,
or until your limits are adjusted."

### C6. CONTEXT_LIMIT

| Item | Value | Evidence | URL |
| :- | :- | :- | :- |
| Team plan context window | 200k tokens | DOCUMENTED | https://support.claude.com/en/articles/9266767-what-is-the-team-plan |
| Newer paid-plan models | Up to 1M tokens | DOCUMENTED | https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work |
| Long context pricing | Claude 4.6 and later include the full 1M token context window at standard pricing. "A 900k-token request is billed at the same per-token rate as a 9k-token request." | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing |
| Tokenizer change | Claude 4.7 and later use a newer tokenizer that "produces approximately 30% more tokens for the same text" | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing |
| Auto-compact window | Client-side threshold at which Claude Code summarizes older history. Explicitly "not a usage limit" | DOCUMENTED | https://code.claude.com/docs/en/costs |

The tokenizer row is a trap for any cross-model efficiency comparison. A token
count measured on Sonnet 4.6 and a token count measured on a 4.7-or-later model
are not the same unit. This is the concrete form of contract rule A5.

### C7. MODEL_LIMIT and ACCESS_CONTROL

| Mechanism | What it does | Who sets it | Evidence | URL |
| :- | :- | :- | :- | :- |
| Opus and Sonnet family usage windows | Model-family scoped USAGE_LIMIT. Switching families restores work | Anthropic | DOCUMENTED | https://code.claude.com/docs/en/errors |
| Per-model API rate limits | Applied separately per model, so models can be driven concurrently | Anthropic, by tier | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |
| Combined model buckets | Fable 5.1 and Fable 5 share one limit; Mythos 5.1 and Mythos 5 share a separate one; Opus 4.8/4.7/4.6/4.5 share one; Sonnet 4.6 and 4.5 share one. Opus 5.5, Opus 5, Sonnet 5.5 and Sonnet 5 each have their own | Anthropic | DOCUMENTED | https://platform.claude.com/docs/en/api/rate-limits |
| Organization model access | Organization settings > Models. Disabling a model affects every member including Primary Owners. The model picker shows only accessible models | Primary Owners, Owners, or a custom role with Identity and Access | DOCUMENTED | https://support.claude.com/en/articles/15694740-manage-model-access-for-your-organization |
| Organization effort cap | An effort level ceiling per model, set at the organization level | Same | DOCUMENTED | Same |
| Role-level model access and effort limits | Apply only to members whose role is Custom. User, Admin and Owner roles get every model enabled at org level, up to the org effort cap | Enterprise admins | DOCUMENTED | Same |
| Model availability by plan | Free: Sonnet and Haiku. Pro, Max: Sonnet, Haiku, Opus, Fable. Team, Enterprise: all models | Anthropic | DOCUMENTED | https://claude.com/pricing |

### C8. FEATURE_LIMIT

| Feature | Limit | Evidence | URL |
| :- | :- | :- | :- |
| Projects on Free | 5 | DOCUMENTED | https://claude.com/pricing |
| Code execution container hours | 1,550 free hours per organization per month, then $0.05 per hour per container; 5-minute minimum execution time | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing |
| Web search | $10 per 1,000 searches on the API. Each search counts as one use regardless of result count. Errored searches are not billed | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing |
| Web fetch | No additional charge beyond token costs | DOCUMENTED | https://platform.claude.com/docs/en/about-claude/pricing |
| Agent teams | Off by default, gated by `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Cowork cloud sessions | On by default on Team; off by default on Enterprise, enablable and restrictable to groups via custom roles | DOCUMENTED | https://support.claude.com/en/articles/13455879-use-claude-cowork-on-team-and-enterprise-plans |
| Cowork itself | Disableable org-wide via Organization settings > Cowork | DOCUMENTED | Same |
| `/usage-credits` command | Not available with API key authentication | DOCUMENTED | https://code.claude.com/docs/en/costs |
| `CLAUDE_CODE_OAUTH_TOKEN` scope | Model requests only. Cannot establish Remote Control sessions or fetch claude.ai connectors | DOCUMENTED | https://code.claude.com/docs/en/iam |
| Anthropic profile sessions | claude.ai connectors and `/schedule` are unavailable while a profile or federation credential is selected | DOCUMENTED | https://code.claude.com/docs/en/iam |
| `/insights` | Analyzes up to 200 previously unseen sessions per run; skips very short ones; local sessions only | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Enterprise Analytics API date span | Maximum 31-day span on usage and cost endpoints; history back to 2026-01-01 | DOCUMENTED | https://platform.claude.com/docs/en/api/admin/analytics |
| Spend report CSV range | MTD, Last Month, Last 90 Days, or a custom range up to 90 days back | DOCUMENTED | https://support.claude.com/en/articles/12883420-view-usage-analytics-for-team-and-enterprise-plans |

---

## N. Enterprise specifics

### N1. The two Enterprise regimes, verified

This distinction is the one most likely to be reported wrong, so both sides are
quoted.

| Attribute | Usage-based Enterprise (current) | Seat-based Enterprise (legacy) |
| :- | :- | :- |
| Seat structure | "Single Enterprise seat type (per-user/month, billed annually)" | Standard and Premium seats |
| What the seat fee buys | "The seat fee only covers access to the platform and doesn't include any usage." | Access plus an included per-seat usage allowance |
| Usage billing | "All usage across Claude, Claude Code, and Cowork is billed separately at standard API rates." | Included allowance first, then usage credits at standard API rates |
| Usage limits | "no plan or seat-level usage limits" | Per-seat USAGE_LIMIT on a rolling five-hour window and a weekly window |
| Usage credits | Not applicable | Yes, with org, seat-tier, group and individual spend limits |
| Migration | Current model | "these seat types, usage limits, and usage credit options remain as they are until the organization migrates to the current usage-based billing model" |
| Evidence | DOCUMENTED | DOCUMENTED |

URLs: https://support.claude.com/en/articles/9797531-what-is-the-enterprise-plan,
https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans,
https://support.claude.com/en/articles/11845131-use-claude-code-with-your-team-or-enterprise-plan

Recorded inconsistency, not resolved: the Enterprise consumption guide describes
Enterprise as a "per-seat, usage-based model" where "your org's consumption pool
is shared across all users," while the Claude Code costs page says that on Teams
and Enterprise plans "usage draws from each member's seat allowance" sized by
seat tier. These two descriptions are consistent only if the costs page is
describing the legacy seat-based regime and the consumption guide the current
usage-based one. That reading is plausible but is not stated on either page, so
it is recorded as UNKNOWN rather than asserted.

### N2. Spend limit hierarchy

| Level | Applies to | Who can set it | Precedence rule | Evidence |
| :- | :- | :- | :- | :- |
| Organization | Hard ceiling across all users | Owners, Primary Owners, custom role with Billing "Can manage" | Always applies | DOCUMENTED |
| Seat tier (seat-based Enterprise only) | All users on Standard or on Premium | Owners, Primary Owners | Dollar amount or unlimited | DOCUMENTED |
| Group | Per-user monthly limit inherited by group members | Admins | Overridden by an individual limit | DOCUMENTED |
| Individual | One member | Owners, Primary Owners | "Individual limits always override group limits, regardless of which is higher." Removing a member's limit still leaves them subject to org and seat-tier limits | DOCUMENTED |

URLs: https://support.claude.com/en/articles/14782391-claude-enterprise-consumption-guide,
https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans

### N3. Admin controls and analytics

| Control or report | What it does | Plans | Evidence | URL |
| :- | :- | :- | :- | :- |
| Spend report CSV | Per-user, per-model token usage and estimated spend. Fields include user email, account UUID, product type, model, request count, prompt tokens, completion tokens, `total_net_spend_usd` (after discounts) and gross spend. Refreshes daily with a one-day delay | Team, Enterprise. Team Owners and Primary Owners; Enterprise Owners, Primary Owners and Admins | DOCUMENTED | https://support.claude.com/en/articles/12883420-view-usage-analytics-for-team-and-enterprise-plans |
| Analytics dashboard | Weekly active members, PRs created in Code, Cowork sessions, adoption, stickiness, skills usage, connector activity, per-product breakdowns for Chat, Code, Design and Cowork | Team, Enterprise | DOCUMENTED | Same |
| Enterprise Analytics API | Five endpoint families: `/summaries`, `/usage_report`, `/user_usage_report`, `/cost_report`, `/user_cost_report`, `/users`. Org-level API key with `read:analytics` scope, created by a Primary Owner at claude.ai/analytics/api-keys. Data lag typically 1 day and may be revised by a few percent. History from 2026-01-01. Max 31-day span on usage and cost | Enterprise only | DOCUMENTED | https://platform.claude.com/docs/en/api/admin/analytics |
| Analytics API dimensions | product, model, context window, inference region, speed (fast or standard), RBAC group, Claude Tag category, cost type (tokens, code execution, web search), token type (uncached input, cached input, output, cache creation) | Enterprise | DOCUMENTED | Same |
| Claude Code Analytics API | Daily per-user Claude Code metrics, with an Admin API key | Console (API) organizations | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Console Claude Code dashboard | Spend and accepted lines per member | Console | DOCUMENTED | Same |
| `modelPricing` managed setting | Rewrites the cost figures Claude Code shows developers to contracted rates. "The setting changes what Claude Code reports, not what Anthropic charges." Ignored in user, project and local settings and in `--settings` | Any, via managed settings. Requires v2.1.242+ | DOCUMENTED | Same |
| `forceLoginMethod`, `forceLoginOrgUUID` | Restrict which login method and which Anthropic organization developers may use. Enforcement differs by login path; `claude setup-token` and `/install-github-app` enforce only `forceLoginMethod` | Managed settings | DOCUMENTED | https://code.claude.com/docs/en/iam |
| OpenTelemetry export | Per-user token and cost metrics into the operator's own stack, near real time. "works on every setup and is the only option that streams per-user token and cost metrics" | All regimes | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Role-based permissions, custom roles, SCIM, audit logs, compliance API, customer-managed encryption keys, US-only inference, HIPAA-ready configuration | Enterprise controls | Enterprise | DOCUMENTED | https://support.claude.com/en/articles/9797531-what-is-the-enterprise-plan |

### N4. Published Enterprise budgeting figures

| Figure | Value | Evidence | URL |
| :- | :- | :- | :- |
| Average Claude Code cost per developer per active day | "around $13" | DOCUMENTED | https://code.claude.com/docs/en/costs |
| Average Claude Code cost per developer per month | "$150-250" | DOCUMENTED | Same |
| Share of users below $30 per active day | 90 percent | DOCUMENTED | Same |
| Background token usage when idle | "typically under $0.04 per session" | DOCUMENTED | Same |
| Agent teams token multiplier | "approximately 7x more tokens than standard sessions when teammates run in plan mode" | DOCUMENTED | Same |
| Recommended TPM per user by team size | 1-5 users: 200k-300k; 5-20: 100k-150k; 20-50: 50k-75k; 50-100: 25k-35k; 100-500: 15k-20k; 500+: 10k-15k | DOCUMENTED | Same |
| Recommended RPM per user by team size | 1-5: 5-7; 5-20: 2.5-3.5; 20-50: 1.25-1.75; 50-100: 0.62-0.87; 100-500: 0.37-0.47; 500+: 0.25-0.35 | DOCUMENTED | Same |

Per the repository contract rule A15, every figure in this table enters the
repository as provider guidance with a source and a date, not as a Replay
finding.

---

## 5. Repository observations

| Observation | Where | Class |
| :- | :- | :- |
| The repository already binds any quota claim to measuring the entitlement surface it affects (rule A1), forbids describing a context reduction as a quota saving (A2), forbids summing cost and quota (A3), and requires per-surface evidence before a Claude Code result is applied to claude.ai, Cowork, Team, Enterprise, Bedrock, Vertex or Foundry (A4) | `docs/ANTHROPIC-EFFICIENCY-CONTRACT.md` | OBSERVED |
| The repository records that on a subscription seat the dollar figures shown are list prices "for somebody else", and that whether a re-billed token draws down a rate-limit window was measured as a null result | `docs/ANTHROPIC-EFFICIENCY-CONTRACT.md`, rule A3 rationale | OBSERVED |
| The repository's rule A11 already classifies undocumented quota, cache and compaction mechanics as UNKNOWN, which this map upholds | `docs/ANTHROPIC-EFFICIENCY-CONTRACT.md` | OBSERVED |
| The efficiency contract was written 2026-09-29, the same day as this map, before any Anthropic optimization work | `docs/ANTHROPIC-EFFICIENCY-CONTRACT.md` header | OBSERVED |
| `docs/anthropic-recon/` was empty before this file | filesystem | OBSERVED |

No independent measurement was performed for this map. Every DOCUMENTED row is a
published claim by the vendor, not a verified behaviour.

---

## 6. UNKNOWN register

A known gap is more useful than a silent one. Each row names what would close it.

| # | UNKNOWN | Why it matters | What would close it |
| :- | :- | :- | :- |
| U1 | **The unit that subscription USAGE_LIMIT is denominated in.** Anthropic publishes only relative multipliers (5x, 20x, 1.25x, 6.25x) and states there is no fixed message count. The mapping from tokens, requests or wall time to consumed allowance is not published | This blocks every quantitative optimization claim on a subscription seat. Without the unit there is no denominator | Instrumented measurement against the plan usage bars, or publication by Anthropic. Never inference |
| U2 | **The absolute size of the weekly limit, the Opus limit and the Sonnet limit on any plan** | A weekly-limit optimization cannot be sized | Same as U1 |
| U3 | **Whether cached input tokens consume subscription USAGE_LIMIT at a reduced rate, as they do for API ITPM and API price** | Caching is the single largest documented lever on the API side. Whether it transfers to the subscription side is the highest-value open question in the campaign | Controlled measurement on a subscription seat. The documented claim that project content "counts less against your limits when reused" is suggestive but unquantified and is about Projects, not prompt caching |
| U4 | **Whether the `/usage` plan bars and the Enterprise spend report use the same underlying counter** | The two visible instruments may not measure the same quantity. `/usage` is explicitly "approximate and computed from local session history on this machine" and excludes other devices and claude.ai | Cross-check a controlled workload against both instruments |
| U5 | **The five-hour window's exact semantics.** Whether it is a fixed bucket starting at first use or a continuously rolling window | Changes whether batching work into a window helps or hurts | Measurement, or publication |
| U6 | **Concurrency ceilings on subscription surfaces**, including concurrent sessions, subagents and agent teammates | Agent-team and subagent strategies assume no hard ceiling | Measurement |
| U7 | **How Claude in Chrome usage is accounted**, and whether it is separable from the shared pool | A surface in scope with no published accounting | Publication, or measurement |
| U8 | **Whether the Enterprise consumption guide's "shared consumption pool" and the Claude Code costs page's "each member's seat allowance" describe the same regime or two different ones** | Determines whether an Enterprise optimization targets a pooled or a per-seat ceiling | Anthropic clarification, or inspection of a live Enterprise org's admin settings |
| U9 | **The magnitude of "counts less" for cached Project content** | Named by Anthropic as a lever with no number attached | Measurement |
| U10 | **How the merged Cowork-and-chat experience meters usage during rollout.** Anthropic states "while the new experience rolls out, usage may be measured slightly differently" between accounts with and without access | Any measurement taken during the rollout may not be comparable across accounts or across dates | Wait for rollout completion, or record account rollout state with every measurement |
| U11 | **Whether Anthropic's doubled five-hour Claude Code limits apply uniformly across seat tiers**, and the effective date relative to any figure already in this repository | Stale figures | Re-fetch the announcement and the plan pages together |
| U12 | **Priority Tier eligibility, pricing and commitment terms.** Only its response headers are documented on the rate limits page | An Enterprise-scale lever that is not mapped | Fetch the Priority Tier documentation directly |
| U13 | **Evaluation tier limits.** Anthropic states new organizations may start below the published Start tier but does not publish those starting values | A benchmark run from a new organization may be measuring the Evaluation tier without knowing it | Read the org's own Rate limits page in Console; never assume the Start tier |
| U14 | **Acceleration limit thresholds.** Documented as existing and as producing 429s on sharp usage increases, with no published trigger | A throughput experiment can trip this and read the 429 as a tier limit | Ramp traffic gradually and log the distinction; Anthropic does not publish the threshold |
| U15 | **Whether any of the documented behaviours are enforced as stated.** Every DOCUMENTED row here is a vendor claim, not a verified behaviour | Per the repository's own standard, a reported behaviour is not evidence until the check is shown to exercise the property | Measurement, on the specific surface, in the specific regime |

---

## Sources

Every URL below was fetched on 2026-09-29 for this file. Where an older address
redirected, the post-redirect address is listed.

- [Rate limits, Claude Platform Docs](https://platform.claude.com/docs/en/api/rate-limits)
- [Pricing, Claude Platform Docs](https://platform.claude.com/docs/en/about-claude/pricing)
- [Enterprise Analytics API, Claude Platform Docs](https://platform.claude.com/docs/en/api/admin/analytics)
- [Manage costs effectively, Claude Code Docs](https://code.claude.com/docs/en/costs)
- [Authentication, Claude Code Docs](https://code.claude.com/docs/en/iam)
- [Error reference, Claude Code Docs](https://code.claude.com/docs/en/errors)
- [Plans and pricing](https://claude.com/pricing)
- [How do usage and length limits work?](https://support.claude.com/en/articles/11647753-how-do-usage-and-length-limits-work)
- [Usage limit best practices](https://support.claude.com/en/articles/9797557-usage-limit-best-practices)
- [Models, usage, and limits in Claude Code](https://support.claude.com/en/articles/14552983-models-usage-and-limits-in-claude-code)
- [What is the Max plan?](https://support.claude.com/en/articles/11049741-what-is-the-max-plan)
- [What is the Team plan?](https://support.claude.com/en/articles/9266767-what-is-the-team-plan)
- [What is the Enterprise plan?](https://support.claude.com/en/articles/9797531-what-is-the-enterprise-plan)
- [Claude Enterprise consumption guide](https://support.claude.com/en/articles/14782391-claude-enterprise-consumption-guide)
- [Manage usage credits for paid Claude plans](https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans)
- [Manage usage credits for Team and seat-based Enterprise plans](https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans)
- [Use Claude Code with your Team or Enterprise plan](https://support.claude.com/en/articles/11845131-use-claude-code-with-your-team-or-enterprise-plan)
- [View usage analytics for Team and Enterprise plans](https://support.claude.com/en/articles/12883420-view-usage-analytics-for-team-and-enterprise-plans)
- [Use Claude Cowork on Team and Enterprise plans](https://support.claude.com/en/articles/13455879-use-claude-cowork-on-team-and-enterprise-plans)
- [Claude Cowork and chat are one Claude](https://support.claude.com/en/articles/16761823-claude-cowork-and-chat-are-one-claude)

Referenced from search result summaries but not fetched in full, so treated as
lower confidence and flagged where used:

- [Higher usage limits and a SpaceX compute deal](https://www.anthropic.com/news/higher-limits-spacex)
- [Manage model access for your organization](https://support.claude.com/en/articles/15694740-manage-model-access-for-your-organization)
- [Claude in Chrome admin controls](https://support.claude.com/en/articles/13065128-claude-in-chrome-admin-controls)

Local file read for the OBSERVED section:

- `docs/ANTHROPIC-EFFICIENCY-CONTRACT.md`
