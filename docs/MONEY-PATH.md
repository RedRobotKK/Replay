# The money path

**2026-09-06.** What someone would pay for monthly, why they would keep paying, and what has to
exist before anyone can be charged. Written against the measurements in [`evidence/`](evidence/)
rather than against a pricing template, because the measurements are unusually unkind to the
obvious answer.

## Summary

**A $25 per developer per month subscription does not survive this repository's own numbers, and a
$25 per repository per month subscription does.** The reasoning is arithmetic and it is short.

The one machine measured on 2026-09-06 spent **$3,018.99** at list prices in a month and had
**$150.27** re-billed by broken caches, 5% of the total ([`replay cost`, quoted in
WHAT-YOU-GET.md](WHAT-YOU-GET.md)). That same file puts realistic recovery at 2 to 3 percent rather
than 5, because some breaks have legitimate causes. So the ceiling for the heaviest user in the
corpus is roughly $60 to $90 a month of recoverable spend. To clear $25 a month at a 2.5% recovery
rate a seat has to be spending about **$1,000 a month on metered tokens**, which at the measured
median task cost of **$0.65** is on the order of fifty to seventy agent tasks a day. That is a real
population and it is not most people.

For everyone on a flat seat the recoverable figure is not small, it is **zero dollars**. Claude Max,
Team, Copilot and Cursor produce no invoice line for a broken cache, and the attempt to show that a
break costs a subscriber rate-limit budget instead moved 3.09M tokens and shifted the utilisation
counter by **zero** steps ([titration](evidence/quota-titration-2026-09-06.md)). That is published
as a null result, so there is no second currency to bill against.

Then there is the finding that should end any subscription pitched as "we find your waste". The
cleanest measurement in this repository looked at a fan-out session through a corrected proxy, found
**4.2%** of prompt tokens re-billed by cache breaks, and traced every one of the three breaks to an
MCP connector's tool block arriving mid-session. The recommendation was
[client-side sequencing](evidence/lane-isolation-2026-09-06.md): bind MCP tools before the first
cached request rather than after. That is a free configuration change, it takes one afternoon, and
once it is done the finding does not come back. **A subscription whose value is finding that is a
subscription the customer cancels in month two, correctly.**

So the money is not in diagnosis. Diagnosis is one-time and it is already free. The money is in the
two things that genuinely recur: **configuration drift**, because a team keeps adding servers and
instructions and nobody watches the per-request cost of doing so, and **provider drift**, because
the prices and cache floors that every dollar figure rests on change on the provider's schedule.
Both of those are per-repository problems with a named owner and a budget, which is why the unit is
a repository and not a person.

## 1. The unit of value

**The recurring thing worth paying for is a standing-cost budget for a repository's agent
configuration, enforced on every pull request, priced from measurements the team took itself.**

Here is why that specific thing and not one of the more obvious ones.

Every request an agent makes carries the same fixed payload before any work begins: the system
prompt, the instruction block and every tool definition. The ledger already records `system_bytes`,
`tool_bytes`, `tool_count` and **each tool definition by name and size**
([WHAT-YOU-GET.md](WHAT-YOU-GET.md)). That payload is paid on every turn of every session of every
engineer who shares the configuration. It is the one cost in this product that multiplies by
headcount and by time at once.

It also grows without anyone deciding to grow it. The measured examples are in the ledger already:
three Otter.ai tools finishing their handshake mid-session re-billed **157,080 tokens**, and one
block of **199 tool definitions** arriving cost **38,357**
([lane isolation](evidence/lane-isolation-2026-09-06.md)). Nobody chose either. Somebody added a
connector, and the cost of that decision was invisible at the moment it was made and permanent
afterwards.

This is the shape a subscription needs and the diagnosis does not have:

| | Diagnosis | Standing-cost budget |
|---|---|---|
| How often the answer changes | Once, then it is fixed | Every time the config changes |
| Who it is addressed to | The person who ran it | Whoever opens the next pull request |
| What happens if you stop paying | Nothing, you already know | The number drifts back up unwatched |
| Already free? | Yes, entirely | Does not exist yet |

The last row is the important one and it is the whole design constraint. **Nothing that is free
today can become the paid thing.** Every capability that reads your own data and prints a number
about it stays free forever, for the reason in section 3. The paid thing has to be something that
does not exist, whose absence does not weaken any claim the free tool makes.

Two secondary units are real and smaller:

- **The maintained rules feed**, already designed and already priced over x402
  ([ADR-0013](adr/0013-x402-rules-feed.md), [FUNDING.md](../FUNDING.md)). It is maintenance of
  external facts, so it recurs by construction. It is currently a 503 because the paid document is
  byte-identical to the free one, enforced by a comparison in code.
- **The signed measurement artefact**, which is the deliverable of the position in
  [AUDIT-OUTREACH.md](AUDIT-OUTREACH.md): we offer to measure, we never claim you have the defect.
  The artefact is worth money because it is addressed to somebody other than the person who ran the
  command, and that reader needs provenance they can check rather than a terminal screenshot.

## 2. Who pays

**The buyer is the engineering or platform lead who owns a repository's committed agent
configuration and is answerable for a metered token bill.** Not the developer.

It is a budget line for that person for a reason that has nothing to do with savings. They already
have a number they cannot explain. The API bill went up, several people added MCP servers that
month, and there is no artefact anywhere that connects the two. `replay diff` and `replay context`
can connect them, but only after the money is spent and only on one machine at a time. A budget
enforced at review time is the only version of this that arrives before the spend.

The second buyer is the consultancy or contractor doing agent-efficiency work for a client. They
need the artefact from section 1 because their deliverable is a claim somebody else has to accept,
and this project's whole differentiator is that every number carries how it was obtained. Handing a
client a dated report with tier labels, a corpus size, and the retractions still visible is a
stronger deliverable than a slide, and it is a deliverable no dashboard vendor can produce.

**Who does not pay, stated so it is not quietly assumed later:** the solo developer on a flat seat.
There are no dollars for them to recover, the token argument is real but does not convert to a
purchase order, and the free tool already answers every question they have. They are the audience,
they are not the customer, and the tip jar in `~/.replay/tip.json` is the correct instrument for
them.

## 3. Free and paid

The rule, in one line: **anything that measures your own data on your own machine is free forever,
and nothing that is free today ever becomes paid.**

The reason is not generosity. This product's only durable claim is that every number says how it was
obtained, enforced by the three truth tiers in code. A paywall in front of a measurement makes that
claim unfalsifiable to the person who has not paid, which is the same as not making it. The tool
would still work and it would stop being believable, and believability is the entire asset.

### Free forever

| Capability | Why it can never be gated |
|---|---|
| Every command over your own transcripts: `cost`, `diff`, `blame`, `context`, `advise`, `route`, `learn`, `doctor` | These are the claim. Gating one makes the whole tier system a marketing device |
| `replay serve`, byte-for-byte passthrough, the ledger, secret masking | The proxy sits in a credential path. A payment check in a request path is a new failure mode in the worst possible place |
| The compiled rules table and the free feed at a stable URL | Promised complete in [FUNDING.md](../FUNDING.md) and enforced by `npm run check:rules`, which fails the build if the published file and `replay rules --export` ever differ |
| Every tier label, error bar, refusal and retraction | A refusal is the product. `replay route --to` declining to price an unmeasured pair is the behaviour that earns trust |
| Local operation with no account and no network call | [README.md](../README.md) states this as a footprint guarantee. Entitlement must not touch it, which is what section 4 is about |

### Paid

| Capability | Status | Why it is fair to charge |
|---|---|---|
| The maintained rules feed | Designed, [ADR-0013](adr/0013-x402-rules-feed.md); currently returns 503 by design | Maintenance of facts that change on the provider's schedule. The free table stays complete, so nobody is held hostage |
| `replay gate`, a standing-cost budget enforced in CI | Does not exist | New capability, aimed at a repository rather than a person, and it removes nothing |
| The signed measurement artefact | Does not exist; presentation over data already computed | Its value is that a third party can verify it, which is work the free path does not do |

### The two sentences that have to be corrected first

This is a cost of the plan and it is paid before any revenue, not after.

[SPONSORS.md](../SPONSORS.md) says "Nothing is behind a tier, and nothing will be gated on
sponsorship." [FUNDING.md](../FUNDING.md) says "Nothing in Replay is ever behind it." Both are
scoped in their authors' minds, the first to sponsorship and the second to the rules feed, and
neither reads that way. If a paid `replay gate` ships while those sentences stand, a reader who
finds them concludes the project broke a promise, and they will be right about the wording even
though nothing free was taken away.

The fix is to narrow both to what they meant and to add the rule from the top of this section:
sponsorship gates nothing, the rules feed gates nothing, and nothing that is free today ever becomes
paid. Amend the documents before selling, not in the release notes afterwards.

## 4. Entitlement, and what the repository actually says about it

**Correction first, because it matters more than the design.** The brief for this document said
entitlement decoupled from the payment rail is already stated design intent. It is not. Searched
across the tree, the phrase does not appear and neither does the idea in those terms. What exists is
two adjacent statements:

- [ADR-0012](adr/0012-dual-licensing-deferred.md): "Dual licensing is a legal mechanism, not a
  technical one, and it does not imply licence keys, phone-home, or usage tracking." That keeps the
  option open. It does not take it.
- [ADR-0013](adr/0013-x402-rules-feed.md): the binary reads a 402, reports the terms, and stops. "It
  does not pay. It ships no wallet, stores no key, and has no code path that can move money." That
  decouples **key custody** from the tool. It says nothing about how the tool would recognise a
  buyer.

So this section is a proposal, and it should be recorded as an ADR before it is built rather than
inferred from the two above.

### The mechanism

**A signed entitlement document, installed from a local file, verified offline against a public key
compiled into the binary, expiring on a date the binary reads from the local clock.** No licence
server, no account, no callback, no identifier that leaves the machine.

It works exactly like the pattern already shipped for the paid feed. An agent or a person fetches
the document from wherever they paid, and installs it locally:

```sh
replay entitle --install ./entitlement.json
replay entitle --show
```

`--show` prints the subject, the expiry, and the signature status, so the thing is inspectable
rather than a token that either works or does not.

**Read it the way consent is read.** `internal/consent` already has the discipline this needs, and
reusing it is better than inventing a second one. `readDecision` uses `os.Lstat` so a symlink is an
error and never a grant, refuses any file that is group- or world-writable, refuses anything it
cannot parse exactly, and treats a missing file as undecided rather than as permission. The tests
that hold that line are `TestU2_PermissionMustBeExplicit`, `TestU4_UnparseableIsNeverPermission`,
`TestU5_SymlinkIsRefused` and `TestU6_WorldWritableIsRefused`. An entitlement file is a fourth file
of the same kind and gets the same treatment, sitting beside `update-consent.toml` and
`corpus-consent.toml`.

**Consent is untouched, and that has to be demonstrated rather than asserted.** Entitlement
verification originates no network request, so the count of outbound surfaces the binary can start
stays at two, both of which the user types. A row in [SURFACES.md](SURFACES.md) records the new file
and its permissions, and the existing surfaces test is what proves the claim.

**Failure modes, all of which resolve toward free.** An expired entitlement turns off the paid
capability and prints how to renew. It never degrades a free capability, never blocks a request in
the proxy path, and never fails a build for a reason the user cannot read. If the payment rail
disappears entirely, the current document runs to its expiry and then the tool is exactly the free
tool, which is the same guarantee ADR-0013 already makes about the feed: a paid thing must never
become a hostage.

**Revocation is expiry, and nothing else.** Documents are short-lived, on the order of a month, and
the rail reissues them. There is no revocation list because a revocation list is a server the user
has to trust and a request the binary has to make, and both are exactly what this product sells
against.

### The blocker, which is real and is in the test suite

`cmd/replay/x402_test.go` enforces a module-wide import allowlist. `TestX402_NoSigningCapability`
walks every Go file and fails on any `crypto/` import outside `aes`, `cipher`, `hmac`, `rand` and
`sha256`. `TestX402_AllowlistIsMeaningful` then asserts that `crypto/ed25519`, `crypto/ecdsa`,
`crypto/elliptic`, `crypto/ecdh` and `math/big` are never on that list, with a comment saying that
adding any of them fails the test and "that is the conversation this list exists to force."

This is that conversation.

The property the test protects is that a binary people pipe from `curl` onto machines holding
provider credentials cannot move money. Verifying a signature does not move money. Signing does. So
the allowlist is currently drawn one level too coarse for its own stated purpose, and the fix is to
draw it at the operation rather than the package:

- Permit the import of `crypto/ed25519`.
- Fail the build on any reference to `ed25519.Sign`, `ed25519.GenerateKey`, `ecdsa.Sign` or any
  other construction path, by the same file walk that exists today.
- Keep `TestX402_AllowlistIsMeaningful` as the guard on that narrower line, so the test still has
  something it can fail on.

The alternative is an HMAC token, and it should be rejected explicitly. A symmetric check means the
verification key ships inside the binary, so anyone who reads it can mint entitlements. That is not
a weaker fence, it is a fence with the gate drawn on it.

The signing key lives off the build machine and off CI. It signs entitlements, never releases.
Release signing already exists and is separate: Sigstore keyless through cosign over
`checksums.txt`, verified by `install.sh`, which dies rather than warns when verification fails.

### Why the rail does not matter

Because the binary never talks to it. Stripe, GitHub Sponsors, an invoice paid by bank transfer and
the existing x402 endpoint are all the same to the tool: each produces a signed file, by whatever
means, and the file is what the binary reads. That is what decoupling buys, and it is worth having
for a reason beyond ideology. A solo maintainer will change payment providers at least once, and
this design makes that a website change rather than a release.

## 5. What has to be built, in order

Each step is useful on its own, and the order is chosen so that the parts most likely to be wrong
are tested before the parts that cost the most.

1. **Amend the two sentences in SPONSORS.md and FUNDING.md.** Cost: minutes. Sequence: strictly
   first, because every later step is a promise violation until it is done.
2. **Record the entitlement design as an ADR.** It is a proposal today, not intent, and section 4
   says so. This is also where the allowlist narrowing gets argued in public rather than slipped
   into a commit.
3. **Named-server attribution in `advise`.** Already identified in
   [WHAT-YOU-GET.md](WHAT-YOU-GET.md) as the single highest-leverage unbuilt thing, and it is pure
   presentation over ledger data with no config read and no new permission. It turns "12 tool
   definitions never called are 8% of prompt tokens" into a named server the reader can act on. This
   is free, it ships free, and it is the input the paid gate needs.
4. **`replay budget`, offline.** Emit a signed-by-nothing artefact from a developer's own ledger
   recording the standing per-request cost of the current configuration. Free. This is the file the
   gate compares against.
5. **`replay gate`, the first paid capability.** Exit non-zero when the standing cost in a committed
   budget file has been exceeded. **It reads Replay's own artefact, never the repository's agent
   configuration.** That boundary is not fussiness: `.mcp.json` can hold credentials, and
   [WHAT-YOU-GET.md](WHAT-YOU-GET.md) records that the tool reads no configuration at all except the
   one key `advise --apply` writes, behind a flag you type. A CI job that parses `.mcp.json` to count
   tool definitions would cross that line in the one environment where secrets are most exposed.
   Comparing artefacts costs nothing and crosses nothing.
6. **Entitlement verification.** The mechanism in section 4, gating only step 5. Everything before
   it stays free, so if this step is never finished nothing is lost.
7. **The payment rail.** Whichever one. It comes last because nothing above depends on it.
8. **Make the paid feed actually differ from the free one.** The 503 lifts by itself once the paid
   document carries `documented` against `observed` status per model per field, which is the
   verification layer argued for in [THE-FEED.md](THE-FEED.md). Blocked on corpus width rather than
   on code.

Deliberately not on this list, with reasons:

- **A team dashboard or a shared proxy.** [ADR-0015](adr/0015-single-tenant-state-is-a-boundary.md)
  is unambiguous. Every piece of shared mutable state is scoped to one human, the day spend cap
  behind a shared proxy is an organisation-wide outage waiting to happen, and the metrics listener
  refuses non-loopback binds because its counters name repositories. The tenant dimension comes
  before any commercial layer, and it is a real engineering project. The gate in step 5 is the
  team-shaped product that is architecturally permissible today, because it is stateless and runs on
  the customer's own machine.
- **Gating anything that exists.** Section 3.

## 6. The strongest objection

> **You are charging a subscription for a one-time fix, and your own best measurement proves it.**

Stated at full strength: the corrected fan-out measurement found 4.2% of prompt tokens re-billed,
attributed every one of the three breaks to MCP tool blocks arriving mid-session, and concluded that
the fix is client-side sequencing. That is free, it is permanent, and the document itself says the
98.8% figure it replaced "was about to justify building a prefix compression subsystem" and that
"there is no 98% defect to compress." A tool whose sharpest finding is a configuration change is a
tool you run once. Charging monthly for it is charging rent on a fact.

**The answer, which concedes most of it.**

The objection is correct about diagnosis and that is why nothing diagnostic is paid. It is wrong
about one thing, and the mistake is treating a configuration as a fixed object. It is not fixed. It
is a file in a repository that several people edit, and the measured examples of it changing are in
this repository's own ledger: three connector tools appearing mid-session, then thirty-nine, then a
hundred and ninety-nine. Each of those was somebody making a reasonable local decision with no
visibility into its standing cost. Sequencing the MCP binding once does not stop the next person
adding a server, and the free tool will tell them the cost only after a month of paying it.

So the paid thing is not the finding, it is the **assertion that the finding still holds**, checked
on every pull request, on a number the team measured itself. That recurs for the same reason a test
suite recurs, and nobody argues that a test suite is rent because the bug was fixed once.

Two more objections, weaker but worth answering rather than leaving for someone else to raise.

**"Apache 2.0 means the entitlement check is deleted in one commit."** True, and it does not matter
much. The check is a payment fence for honest buyers, not a DRM scheme, and the buyer in section 2
is a company that will not ship a patched binary through its own compliance process to avoid $25.
The part that a fork genuinely cannot take is the paid feed, because that is data measured against
real traffic and it is not in the repository. [ADR-0012](adr/0012-dual-licensing-deferred.md) reached
the same place from the other direction: revenue has to come from a separate work, and data is the
separate work that is already half built.

**"78 sessions on one machine is not enough to sell a threshold."** Also true, and
[PRODUCT-DIRECTION.md](PRODUCT-DIRECTION.md) makes the sharper version of the point: run over the 40
largest sessions the break-cause study said TTL expiry was 75.2% of re-billed tokens, and run over
all 1,506 transcripts it said layout-addressable causes were 63.6%, opposite conclusions from the
same tool five seconds apart. If a change of sample inverts the answer inside one operator's data, a
threshold fitted to that data is not a threshold. This is exactly why the gate compares a team
against **its own** measured baseline rather than against a number this project supplies. There is
no threshold to be wrong about. The product is the comparison, and the baseline is the customer's.

## The price, straight

**No for $25 per developer per month. Yes for $25 per repository per month.**

Per seat, the arithmetic does not close. A seat needs about $1,000 a month of metered token spend
before a 2.5% recovery covers $25, and on a flat seat the recoverable figure is zero dollars with a
null result standing behind it. Selling per seat means selling to a small slice of a small audience
and hoping they do not do the division.

Per repository, it closes without needing anyone to believe a savings claim at all. The buyer is
approving a line item next to their CI spend, it is shared across everyone who commits to the repo,
and what they get is enforcement of a budget they set themselves. That is a purchase decision that
survives the customer reading every evidence file in this directory, which is the only kind of
purchase decision this project can honestly ask for.

One thing worth stating so the price does not get blamed for the wrong problem. The
[release criteria](../RELEASE-CRITERIA.md) record that a release "is how work reaches the 26 installs
that exist." At 26 installs there is no subscription business at any price, and no amount of pricing
work changes that. **The constraint today is distribution, not the number on the page.** The value of
deciding the money path now is that it says what to build and what never to gate, which is worth
having settled before the install count is large enough for the question to be urgent.

---

[Documentation index](README.md) · [Repository README](../README.md)
