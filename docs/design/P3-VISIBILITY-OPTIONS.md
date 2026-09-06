# P3, live context visibility: four options and no recommendation

**Status: options, not a decision.** Nothing here is approved and none of it is
scheduled. It exists so the argument can be had on facts instead of on taste.

## The question

A person running agents cannot see, while it is happening, what is filling their
context window and what that is costing. Everything Replay says about context
today is said afterwards, from a transcript, at a terminal they had to go to.

Two prior documents already answer neighbouring questions and are not re-derived
here.

- `docs/DASHBOARD-DESIGN.md`, on branch `design/live-dashboard`, specifies a
  `replay serve` dashboard with 25 states, a width-safe formatter, and a rule
  that nothing renders unless the surface registry backs it. That is option A
  below, already designed.
- [`architecture/mcp-server.md`](../architecture/mcp-server.md) specifies a tool
  surface for agents rather than people, and its own cut list is the sharpest
  writing in this repository about what an agent should not be handed. That is
  option C below, already designed.

What neither settles is where live context composition should surface and who
reads it. That is what is open.

## Summary

Four options, in one line each.

| | Option | Surfaces where | Consumer | Push or pull |
|---|---|---|---|---|
| A | Finish the `serve` dashboard | A second terminal you left running | Human, watching | Push, on a timer |
| B | Put it in the status line | The agent's own UI, one line | Human, working | Push, per render |
| C | A tool the agent calls | The agent's own context | The agent | Pull, on demand |
| D | Publish the data, build no consumer | `/replay/status`, a file, a scrape | Anything | Pull, by whoever |

**The single sharpest tradeoff.** The composition data lives in the proxy. The
window-fullness data lives in the client. Neither half alone answers the
question, and each option picks a side rather than joining them. Options A and D
sit on the composition side and cannot say how full the window is. Option B sits
on the fullness side and cannot say what is in it. Option C is the only one that
starts on both sides at once, and it is the only one where the consumer can act
without a human reading anything, which is also the reason it is the most
dangerous of the four.

## What is true today, with citations

Read this section before arguing about the options. Three of the four have been
proposed here before on assumptions that do not hold.

**Replay does not know how large the context window is.** The price table
carries input price, output price and a read multiplier, and nothing else
(`internal/cachemodel/anthropic.go:118`). `analysis.ContextEntry.Share` says so
in its own comment: the share is over the attributed total, not over the window,
because "the window is not known here and claiming it would be a guess"
(`internal/analysis/context.go:29`). No percentage-full figure is available to
the proxy at any price.

**Replay does not know what is in the context window either. It knows what
entered it.** `/replay/status` fills its `context` field from
`analysis.EnteredContext` (`internal/proxy/state.go:506`), and that attribution
never subtracts cleared or compacted content. `replay context` names the same
limit in its own doc comment (`cmd/replay/context.go:15`).

**But the proxy does hold the exact composition of the request it just
forwarded, per block, and writes it to disk.** `SummarizeRequest` walks the
whole messages array (`internal/ledger/summarize.go:96`), reduces each block to
a kind, a label and a byte count (`internal/transcript/types.go:63`), and lands
the result on `Prompt.Messages` (`internal/ledger/record.go:126`), which is
embedded in the ledger record (`internal/ledger/record.go:42`) and appended as
one JSON line per request (`internal/ledger/store.go:86`). **A true "what is in
this prompt right now" is a sum over the last ledger line for a lane. Nothing
computes it.** That is the one genuinely new measurement any of these options
could add, and it is small.

The catch is the denominator. Blocks carry bytes, and the provider reports
prompt tokens for the request as a whole. Splitting one total across many blocks
by byte share is the byte-to-token fit, which this repository already reports as
carrying error bars up to plus or minus 159% across sessions
([`architecture/mcp-server.md`](../architecture/mcp-server.md)). A live
composition is therefore honest in bytes and estimated in tokens, and every
option below inherits that.

**Per-lane data exists and is keyed by agent id.** `context_by_lane`,
`re_reads_by_lane` and `what_if_by_lane` are on `/replay/status`
(`internal/proxy/state.go:593`, assembled at `internal/proxy/state.go:680`),
keyed by the `x-claude-code-agent-id` header with `""` for the main loop
(`internal/proxy/server.go:65`). [The lane isolation
evidence](../evidence/lane-isolation-2026-09-06.md) is what forced that shape:
one session-wide field made 31 of 34 recorded cache-break events things that
never happened. **Any live surface that renders one number for a fan-out session
repeats that defect in the display layer.**

**A pull surface already exists and nothing consumes it.** `/replay/status`
returns the per-session and per-lane JSON (`internal/proxy/server.go:130` and
`internal/proxy/server.go:439`), `/replay/metrics` returns Prometheus text, and
`--metrics-listen` binds a second read-only listener whose mux structurally
cannot proxy (`cmd/replay/serve.go:52`,
`internal/proxy/metrics_listener.go:36`). Both are guarded only by an `Origin`
and `Sec-Fetch-Mode` check plus an optional token
(`internal/proxy/server.go:427`), and `docs/SURFACES.md:102` records that as a
known gap. The metrics are aggregate only (`docs/SURFACES.md:96`).

**The status line already exists and is wired into the agent's own UI.**
`replay statusline` reads the JSON Claude Code hands it on stdin, opens no files
and makes no network call (`cmd/replay/statusline.go:25`). It refuses to price a
model that is not in the table (`cmd/replay/statusline.go:129`).

**Two things about the status line matter more than the feature.** It already
decodes `context_window.used_percentage` (`cmd/replay/statusline.go:36`) and
renders it nowhere: grep the tree and that field appears twice, both inside the
struct declaration. And nothing in this repository establishes that Claude Code
sends either `context_window` or `prompt_cache` at all. There is no evidence
file for the status line input schema, and every test fixture in
`cmd/replay/statusline_test.go` was written here. **The whole avoidable-spend
figure rests on a payload shape this project asserted and never captured.** That
is a checkable question rather than an accusation, and it is question 1 below.

**A forwarded surface has a hard ceiling.** Anything that is not `/v1/messages`
or `/v1/chat/completions` is forwarded byte for byte and never parsed
(`internal/proxy/server.go:475`, and `noteUnparsed` at
`internal/proxy/server.go:1203`). For those paths Replay has a path string, a
byte count, a latency and a status code. **No option below can show context
composition on a forwarded surface. Any that claims to is inventing
capability.**

**The roadmap defers a web dashboard "until a user asks"**
(`docs/ROADMAP.md:71`). None of the four options is a web dashboard, and that is
deliberate.

## Option A: finish the `serve` dashboard

### A: what it is

The surface already specified in `docs/DASHBOARD-DESIGN.md` on branch
`design/live-dashboard`, extended with a context panel: what is in the current
prompt, by tool, per lane, repainted on a timer.

### A: the case for it, put by someone who believes it

The proxy is the only place in this system that sees the truth. It has the
provider's own usage numbers, the request structure, the cache outcome, the
guards and the surface registry, all in one process, at the moment it happens.
Every other option is a straw into that process. Building the straw before
building the thing it drinks from is how you ship four half-views that disagree.

And the hard part is done. The state inventory is 25 entries deep, it already
contains the two states nobody would think to draw, a cap configured but not
enforced and a path forwarded blind, and it already carries the rule that stops
the display outrunning the evidence. Adding a context panel to a designed
dashboard is a panel. Starting somewhere else is starting over.

### A: who consumes it

A person who chose to run `replay serve` in a terminal and can see that
terminal.

### A: what it needs that does not exist

The width-safe formatter and the non-TTY path, both specified and unbuilt. A
per-request composition rollup, which is new but small. Nothing else: the
per-lane data is already on `/replay/status`.

### A: what it costs

The largest of the four. The design's own build order runs to five stages and
stage one is a formatter with a test that fails on any ambiguous-width
character.

### A: which surfaces it works on

Every surface, at the honesty level each allows. Parsed paths get composition.
Forwarded paths get a row saying they were forwarded blind and that nothing
Replay offers applies to them, which is the most useful thing anyone has said
about them.

### A: the objection that kills it

**Nobody is looking at that terminal.** The person is looking at their agent. A
dashboard in a window you have to remember to check is a log with a repaint
loop, and the premise of P3 is that the feedback has to arrive while there is
still a session left to change. Replay has already learned this once: the status
line exists precisely because the invoice arriving a month after the mistake is
the problem, and a dashboard in another window is a smaller version of the same
delay. If that objection holds, the cost of option A buys a surface that gets
checked after the fact, which is what the CLI already does better.

## Option B: put it in the status line

### B: what it is

Extend `replay statusline` so the one line it already renders says what is
filling the window, not only what the session cost. Something of the shape
"72% full, 41% of it tool results".

### B: the case for it, put by someone who believes it

This is the only surface in the argument that is already in front of the user's
eyes, and it already ships. The plumbing is done, the debounce is done, the
price table is wired in, and the refusal discipline is already there: it will
not price a model it does not know. It costs about 6ms per render and opens no
files.

More to the point, it is the only option where the number arrives at the moment
the decision is made, without anyone choosing to go and look. That is not a
smaller version of the dashboard. It is the thing the dashboard is trying to
approximate.

### B: who consumes it

The human, in the window they are already working in.

### B: what it needs that does not exist

**A join that has never been made.** The status line runs in the client and
knows fullness. The proxy runs elsewhere and knows composition. To say both in
one line, the status line has to read the proxy, which means opening a file or
making a request on every render, and "opens no files, makes no network call" is
its stated contract today (`cmd/replay/statusline.go:25`).

**A key for that join.** The proxy attributes by the `x-claude-code-session-id`
header (`internal/proxy/server.go:64`); Claude Code hands its status line a
`session_id`. Whether those are the same string is not established anywhere in
this repository.

**Confirmation the payload exists at all.** See question 1.

### B: what it costs

The smallest of the four if the join is a file read, and it stops being small
the moment anyone argues about the per-render budget.

### B: which surfaces it works on

**Claude Code only, and this option has to say so out loud.** That is a client
limitation rather than a provider one: the mechanism is Claude Code's
`statusLine` settings hook and its private JSON payload. There is no equivalent
for a client pointed at an OpenAI-compatible endpoint, and for a forwarded
surface there is neither a hook nor anything to put in it.

### B: the objection that kills it

**One line will not hold it, and the useful part is the breakdown.** "72% full"
is the number the client already shows. The value Replay adds is which three
tools filled it, and that does not fit beside spend and cache health on a line
that also has to leave room for the user's own status line content. Truncate it
and you have shipped a worse version of a number Claude Code already renders.

A second objection stands behind it. Making the status line read the proxy turns
a component with no I/O into one with a dependency that can be missing, stale or
slow, on a 300ms cadence, and the failure mode of a status line is that it
disappears without telling anyone.

## Option C: a tool the agent calls

### C: what it is

The MCP surface designed in
[`architecture/mcp-server.md`](../architecture/mcp-server.md), with a
session-scoped tool that answers "what is in my context now and what did it
cost". The consumer is the agent.

### C: the case for it, put by someone who believes it

Every other option shows a human a number and hopes they act. This one hands it
to the party that can act immediately and at no cost. The agent already holds
the tools that would fix the problem, and it is mid-task, which is the only
moment the fix is cheap. A human reading "41% tool results" has to decide to
compact, remember how, and interrupt themselves. An agent reading the same thing
can stop re-reading the file it already read.

It is also the only option with a worked answer to the thing that makes it
dangerous. `mcp-server.md` cut the boolean gate, cut the memory audit, cut
agent-initiated corpus submission and cut the self-tuning loop, each for a
stated reason, and the session-scoped path needs no index at all: the median
session parses in 0.00s.

### C: who consumes it

The agent. A human sees it only if the agent quotes it.

### C: what it needs that does not exist

There is no `replay mcp` command. Nothing is built. On top of that, the tool has
to answer from live proxy state rather than from a transcript, because the
transcript for the current turn is not written yet and the proxy is where the
current turn is.

### C: what it costs

A new front door, a new transport, and a new trust boundary, plus the review
that all three earn.

### C: which surfaces it works on

MCP is client-side, so it works for any client that speaks MCP, which is more
than one vendor. But the answer it returns is only as good as what the proxy
could parse, so on a forwarded path the honest return value is "this session ran
on a path I do not read".

### C: the objection that kills it

**Every byte it returns is uploaded into the context it is measuring.** An MCP
tool result enters the next request. A tool whose job is to say your context is
too full does so by adding to it, on every call, forever, and the agent decides
how often to call. `mcp-server.md` states this as the design's own most
important section.

The second objection is worse, and it is also already written down there. **The
agent's only observable is spend, so its gradient points at doing less work.** An
agent that can see its own context cost and act on it will act to reduce the
number it can see. It cannot see whether the work got worse. The bill falls, the
outcome degrades, and the instrument is structurally incapable of noticing.

## Option D: publish the data and build no consumer

### D: what it is

Treat the pull surface as the product. Harden `/replay/status`, add the live
composition to it, document the shape, and let the status line, an editor
plugin, a script or a person's own dashboard read it. Optionally write the same
thing to a per-session file under `~/.replay` so a reader needs no port.

### D: the case for it, put by someone who believes it

Most of this is already built and nobody noticed. `/replay/status` carries
per-session and per-lane context today. `--metrics-listen` gives it its own
address with a mux that structurally cannot proxy. What is missing is not a
surface, it is one field and a documented contract.

Replay's differentiator is the measurement, not the pixels. Three of the four
options here are arguments about a renderer, and a renderer is the part with the
shortest half-life and the most opinions. Ship the number, ship the shape, and
let the people who disagree about the renderer each build their own. It is also
the only option that does not require picking a consumer, which is the decision
this group is least equipped to make without users.

### D: who consumes it

Whoever wants to, including options A, B and C later. This option is a
prerequisite for the other three as much as a rival to them.

### D: what it needs that does not exist

The live per-request composition, same as everyone. A documented schema:
`/replay/status` carries no schema version at all, which
`internal/proxy/state.go:584` records as the reason its published field shapes
could not be changed when their meaning was corrected. And the authentication
gap in `docs/SURFACES.md:102` has to close before anyone is encouraged to point
more readers at it.

### D: what it costs

The least new code. The most argument about compatibility, forever, because a
published shape with consumers cannot be changed.

### D: which surfaces it works on

Every surface, and it is the only option that can be fully honest about the
forwarded ones without a renderer to design: the JSON simply says what it does
not know.

### D: the objection that kills it

**"Someone will build the consumer" is a wish, and this repository has already
run the experiment.** The pull surface has existed for some time, carries
per-lane context, and has exactly zero consumers, including inside this project.
The status line does not read it. `doctor` probes only `/replay/healthz`.
Publishing harder is not obviously different from publishing.

A second objection has teeth if the group leans toward Prometheus specifically.
**Context composition is keyed by tool label, and tool labels are model-supplied
and unbounded.** `MaxContextLabel` truncates to 48 runes and its comment records
observed labels reaching 424 (`internal/analysis/context.go:43`), and this
project has already refused to make a free-text field a metrics label for
exactly this reason: `CauseDetail` is "deliberately NOT a metrics label" because
a metric label set must stay a bounded vocabulary
(`internal/ledger/record.go:162`). A per-tool context series would hand anyone
who can get text into a tool name control over a cardinality explosion. JSON
survives that. Prometheus does not.

## What the group has to decide

Each of these has a checkable answer. None is a matter of preference.

### 1. Does Claude Code actually send `prompt_cache` and `context_window` to a status line?

Everything in option B, and the credibility of the avoidable-spend figure
already shipped, rests on it. There is no evidence file, and every fixture in
`cmd/replay/statusline_test.go` was written by this project.

**How to check:** set `statusLine` to a command that dumps stdin to a file, run
one session that includes a cache miss, read the file. One session settles it.

**If the answer is no**, option B collapses to whatever the client does send,
`cmd/replay/statusline.go:36` is dead code, and the shipped avoidable figure
needs a retraction note in the same style as [the lane isolation
evidence](../evidence/lane-isolation-2026-09-06.md).

### 2. Is the proxy's session key the same string as the client's session id?

Options B and C both need to join proxy-side composition to client-side
fullness, and there is only one candidate join key:
`x-claude-code-session-id` (`internal/proxy/server.go:64`) against whatever
Claude Code calls the session in the status line payload.

**How to check:** run one session through `replay serve`, take the session id
from the dump in question 1, and look for a ledger file named after it.

**If they differ**, both options need a discovered join or a header the proxy
stamps, and the cost of option B rises above the cost of option A.

### 3. Is a byte-share split of prompt tokens good enough to render live?

Every option renders the same underlying quantity, and that quantity is
estimated. The provider gives one prompt-token total per request; the split
across blocks comes from the byte-to-token fit, which carries error bars up to
plus or minus 159% across sessions.

**How to check:** replay a corpus of sessions, compare the byte-share split
against the per-turn figures the engine already reconstructs, and publish the
distribution of the error rather than its mean.

**If the error is wide at the per-tool level**, then no option may render a
percentage without a tier label, and the honest live surface is bytes and ranks
rather than shares. That changes what all four options draw, which is why it has
the widest blast radius and should be answered first.

---

[Documentation index](../README.md) · [Repository README](../../README.md)
