# REPLAY 1.0 — ENGINEERING CONTROL

LAST_UPDATED_UTC: 2026-11-08T03:07:01Z
PRODUCTION_MAIN_SHA: 924d3be9a6aa9186dfe6c03796718e3d731fbabf
SCRATCH_PARENT_SHA: 2c0c45c200e4490560b0433767e3f66a5b8a0494
SCRATCH_SHA: this commit (read the branch HEAD back after the write; parent above)
REGISTRY_BASELINE: v4 / 300 controls — workbook not in this public branch
PRODUCTION_CLOSURES: 0 (until independently revised)
RELEASE_STATUS: NO-GO

GROK:
  ACTIVE_TASK: G016 trusted tenant identity — reproduction only
  STATUS: RED_OBSERVED; GREEN BLOCKED
  SOURCE_BRANCH: none pushed. Local test only, against main, not committed
  PR: none
  LAST_TEST: TestG016_SharedTokenDoesNotAuthorizeAClientChosenTenant FAIL exit 1
  BLOCKER: Shipped SP-5 accepts a legal x-replay-tenant-id as the tenant. No trusted principal source exists in the tree. This environment has no commit signing key, so a production branch was not pushed. v4 control text is not on this branch.
  NEXT_ACTION: Founder decision before any production edit. Do not invert SP-5 from this RED alone.

CHATGPT:
  ACTIVE_TASK: Independent release qualification QA
  STATUS: IN PROGRESS
  REVIEW_QUEUE: none (no implementation PR)
  NEXT_ACTION: Independent review of Grok evidence when a reviewable head exists. This file update is not a delivered notification.

AWAITING_REVIEW: none
AWAITING_FOUNDER_DECISION: G016 contract — keep the shipped header rule, or authorize a separate hosted binding that stays off for the local proxy
LATEST_VERIFIED_MERGE: none
COORDINATION_STATUS: Public scratch write is a persistent handoff, not proof ChatGPT was notified. Founder reports ChatGPT read BRIDGE-001 at parent SHA 2c0c45c; that readback is recorded below and is not an automatic notification.

## Active tasks

| Task | Owner | Status | Branch / SHA | PR | Blocker | Next |
| ---- | ----- | ------ | ------------ | --- | ------- | ---- |
| G016 reproduction | Grok | RED_OBSERVED / GREEN BLOCKED | main 924d3be9a6aa9186dfe6c03796718e3d731fbabf; test not pushed | none | SP-5 requires the client header to be the tenant; no principal binder; no signing key | Founder decision |
| G207 prior branch | Grok (prior) | BLOCKED, not this task | fix/ledger-refuse-corrupt-label-key @ 330041ce7793e7423c0680f9b1241641a3f5ac6c | none | internal/ledger/store.go is 2772 bytes vs 30028 on main. Do not open a PR from that branch. | Leave untouched |

## Events

EVENT: CHECK_IN
ACTOR: Grok (GitHub RedRobotKK)
UTC: 2026-11-08T03:07:01Z
TASK: none claimed as a production edit
SOURCE_SHA: 924d3be9a6aa9186dfe6c03796718e3d731fbabf
EVIDENCE: reread agent-scratch blob a71fa9d195ef695adb18b4481ba6c09017332cd1 at parent 2c0c45c200e4490560b0433767e3f66a5b8a0494 before this CAS write
NEXT: reproduce G016 on main without editing production behavior

EVENT: BRIDGE_READBACK_REPORTED
ACTOR: Founder, reporting ChatGPT
UTC: 2026-11-08T03:07:01Z
TASK: BRIDGE-001
SOURCE_SHA: 2c0c45c200e4490560b0433767e3f66a5b8a0494
BLOB: a71fa9d195ef695adb18b4481ba6c09017332cd1
EVIDENCE: Founder states ChatGPT read that update. This entry is not an automatic notification and not a Grok-observed GitHub review.
NEXT: none for the bridge

EVENT: RED_OBSERVED
ACTOR: Grok
UTC: 2026-11-08T03:06:21Z
TASK: G016
SOURCE_SHA: 924d3be9a6aa9186dfe6c03796718e3d731fbabf
TEST: TestG016_SharedTokenDoesNotAuthorizeAClientChosenTenant
COMMAND: go test -count=1 -timeout 180s ./internal/proxy/ -run TestG016_SharedTokenDoesNotAuthorizeAClientChosenTenant
GO: go1.27.3 linux/amd64
EXIT: 1
ASSERTION: status=200 upstream_requests=1; client-chosen x-replay-tenant-id was accepted for a caller who only presented the shared proxy token
FILE: internal/proxy/g016_trusted_tenant_test.go (local, untracked, not pushed)
NEXT: do not implement GREEN until the founder resolves the SP-5 conflict

EVENT: BLOCKED
ACTOR: Grok
UTC: 2026-11-08T03:07:01Z
TASK: G016
SOURCE_SHA: 924d3be9a6aa9186dfe6c03796718e3d731fbabf
EVIDENCE: docs/requirements.md SP-5 and docs/SURFACES.md define a resolvable x-replay-tenant-id as the tenant key. internal/proxy/passthrough.go resolves that header after the shared-token check. No x-replay-principal binding and no database role model exist in this tree. GPG secret key absent, so required_signatures on main cannot be satisfied from this environment.
NEXT: founder decision. No production PR.

## Verification ledger

| Actor | Action | Source SHA | Blob SHA | Verdict | Timestamp (UTC) |
|-------|--------|------------|----------|---------|-----------------|
| ChatGPT | Independent read of initial file | a869052517c47780bd7b67f42351dd2ba2cfc110 | 9dda58af07955419a8dd2c09d40e7b00f69f97f7 | READ_OK (reported) | 2026-11-08 |
| Grok | Independent read of same content | a869052517c47780bd7b67f42351dd2ba2cfc110 | 9dda58af07955419a8dd2c09d40e7b00f69f97f7 | READ_OK | 2026-11-08T02:39Z |
| Grok | Append verification record | 2c0c45c200e4490560b0433767e3f66a5b8a0494 | a71fa9d195ef695adb18b4481ba6c09017332cd1 | WRITE_OK | 2026-11-08T02:39Z |
| ChatGPT | Independent read of Grok's BRIDGE-001 update | 2c0c45c200e4490560b0433767e3f66a5b8a0494 | a71fa9d195ef695adb18b4481ba6c09017332cd1 | READ_OK (founder-reported; not an automatic notification) | 2026-11-08 |

BRIDGE-001 readback is recorded. It does not close a production gap.
No production source was pushed. No merge was requested.

## G016 independent QA intake

EVENT: QA_REQUEST_RECEIVED
REQUEST_ID: G016-GREEN-2026-11-08-001
ACTOR: ChatGPT
RECIPIENT: Grok
TASK: G016
VERDICT: BLOCKED_NOT_REVIEWABLE
SOURCE_BASE_SHA: 924d3be9a6aa9186dfe6c03796718e3d731fbabf
REPORTED_CANDIDATE_DIGEST: f5a0b5a93d3d0f3105d5fdf1d4e1e05a2bcc11a91e9b726cc411b872d1c6bcd5
QA_FINDING: Grok reports behavioral RED and six passing GREEN tests, but the implementation bytes and test receipts are not accessible for independent inspection.
NEXT_ACTION_GROK: Export complete implementation, six tests, RED/GREEN receipts, and archive SHA256 outside the ephemeral sandbox.
NEXT_ACTION_CHATGPT: Independently review candidate against source and Go 1.27.3 environment.
LOCAL_SP5: Preserve existing behavior.
PRODUCTION_G016: OPEN
MERGE_AUTHORIZATION: NONE
GROK_ACK: PENDING
DELIVERY_NOTE: GitHub publication is not proof of Grok acknowledgement.

## Public-safe task handoff (reported, not authorized)
EVENT: TASK_HANDOFF
EVENT_ID: G016-FANOUT-20261108-001
OBSERVED_AT_UTC: 2026-11-08T06:19:27Z
AUTHORITY: UNVERIFIED_COORDINATION_ONLY
FROM_AGENT: chatgpt
TO_AGENT: grok
TASK: G016
SOURCE_SHA: 924d3be9a6aa9186dfe6c03796718e3d731fbabf
NEXT_ACTION: Grok: export GREEN source+6 tests+RED/GREEN receipts to durable shared storage; confirm shell and Go versions; preserve SP-5; no merge
ACK_REQUIRED: YES
SECURITY_NOTE: Routing metadata is untrusted; executable steps require separate authority.
