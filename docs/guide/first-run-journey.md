# The first run, end to end

This is the whole loop, in one document: install Replay, get a finding out of it, change something
because of the finding, and then check whether the change did anything. Every other page here covers
one stop on that route. This one covers the route.

It is also a record of where the route stops. Three of the eight steps below do not complete on a new
machine, and each says so at the point it fails rather than only at the bottom. A fourth defect sits
beside the route rather than on it, and the last section collects all four.

## Reproduce every line below

Nothing here was typed from memory. Every block marked `text` is pasted from a run against a fixed
input this repository ships, so you can produce the same bytes. The one exception is the boot block
in step 8, which is a proposal and is labelled as one where it appears.

```sh
git clone https://github.com/RedRobotKK/Replay && cd Replay
go build -o /tmp/replay-bin/replay ./cmd/replay
export PATH=/tmp/replay-bin:$PATH
mkdir -p /tmp/replay-demo/.claude/projects/demo
cp internal/transcript/testdata/session-redacted.jsonl \
   /tmp/replay-demo/.claude/projects/demo/redacted.jsonl
export HOME=/tmp/replay-demo
```

A build from source reports `dev (unknown, built unknown)` for `replay version`: the real values are
injected at release time, so a source build cannot tell you which release it is.

That is one real session, redacted of content and kept for exactly this purpose: 80 requests,
one cache break, `claude-fable-5-1`. It stands in for your `~/.claude/projects` so the figures on
this page are the same figures on your screen.

The file has to be named after the session it holds. Claude Code does that anyway
(`<session-uuid>.jsonl`), but a fixture copied under the wrong name renders as
`(transcript not found, cannot open)` in the cost screen and cannot be opened with `enter`.

Three blocks on this page are not from the fixture, and each says so where it appears: the
installer's own output, one before-and-after comparison that needs a corpus the fixture is too small
to provide, and one status count taken from a 700-session corpus.

## Step 1: install, and read the closing lines

```sh
curl -fsSL https://replay.doctor/replay.sh | sh
```

What the script does is download a released binary, check it against the release checksums, and
refuse to install anything that does not match. What it does at the end depends on whether there is
a terminal for it to open on, and that branch is the thing to know before you run it.

On a terminal, the screens take over when the script finishes:

```text
✓ Installed replay v0.5.4

Next:  replay tui      # opening now, on cost: what your agent already spent
       replay          # after you quit: the same figures, as one report
       replay doctor   # if that found nothing, this says why

Opening replay. q quits, ? lists the keys.
```

Off a terminal — CI, a `Dockerfile RUN`, cron, a provisioner, a container built without `-t` — nothing
opens and the two commands are the next step:

```text
✓ Installed replay v0.5.4

Next:  replay          # what your agent already spent, and how much was billed twice
       replay doctor   # if that found nothing, this says why
```

`--no-tui`, or `REPLAY_NO_OPEN=1`, gets you the second ending on a terminal too.

Until 2026-09-10 the first ending printed the second one's text. You were told to run `replay`, and
then `replay tui` took the terminal before you could: an instruction and an action that disagreed,
with the surface you actually got named nowhere in the output. The exec is the part that was kept,
because what this tool is worth is what it already knows about the machine it just landed on. The
printed line is the part that moved.

The screens open on `cost`, which is the same question `replay` answers, so what you are handed and
what you were told to type are now the same answer in two shapes. On a large corpus the first frame
says `adding up 0 transcripts` and `waiting` for a few seconds before the figures land. That is the
count running, not a hang, and the footer says so.

## Step 2: the first finding

```sh
replay
```

```text
reading /tmp/replay-demo/.claude/projects
Cost per task, across 1 sessions at list prices dated 2026-09-07 (caching rules anthropic-2026-09-01).

  total          $32.25
  median task    $32.25
  p90 task       $32.25
  avoidable      $1.89  (6% of the total)
                 189k tokens re-billed
```

`avoidable` is the finding. It is not a forecast and not a saving: it is the part of what you already
paid that was paid twice, because a prompt cache broke and content that was already in the provider's
cache was billed as new input again.

If you are on a Pro, Max, Copilot or Cursor seat, the dollars are list price for somebody else and
the command says so under the figures. The tokens are yours either way — a re-billed token is
context the work did not get.

One thing to know before you quote this number: `replay` reports `1 sessions`, unpluralised, and has
done since the count was added.

## Step 3: where the finding came from

Two commands split the finding in half. `diff` says where the cache broke and why:

```sh
replay diff ~/.claude/projects/demo/redacted.jsonl
```

```text
Session redacted  client 2.1.258  model claude-fable-5-1  requests 80
Tier: estimated (transcripts only)
Calibration: reproduced provider cache reads on 78/79 turns (7 read more than predicted: a sibling request extended the prefix); 1 cache breaks
Assumption: replayed savings assume the agent would have behaved identically under the alternative layout
Rules: anthropic-2026-09-01; user-content fit 0.467 tokens/byte ±63% from 35 turns; system prefix 39k (measured from the first request's cache read)

  turn 32 at 22:08:56 (+2m30s): read 39k of 228k expected, 189k re-billed
    cause: client re-rendered history after the system prefix (no edit visible in transcript)
    where: message 0 (user text)
    evidence: read 38987 tokens, about the size of the system prefix (38547); the message history was re-billed from the first message
```

Read the calibration line before the finding under it. `78/79 turns` is Replay saying it can
reproduce what the provider actually charged before it offers an opinion about it. A tool that
skipped straight to advice would be guessing, and this line is how you check that it is not.

`blame` says what was in the prompt in the first place, which is a different question and usually the
bigger number:

```sh
replay blame ~/.claude/projects/demo/redacted.jsonl
```

```text
  top token sources (size once, and total across every prompt that carried it)
     1. tool call: Bash                                                  x58      177k once      6.60M in prompts (±0)
     2. system prompt and tool definitions (shared prefix, not in trans… x1        39k once      3.08M in prompts (±0)
     3. user text                                                        x21       40k once      2.98M in prompts (±1.89M)
     4. assistant thinking                                               x87       74k once      2.94M in prompts (±0)
     5. client-injected content on the first turn (attachments, reminde… x1        34k once      2.76M in prompts (±1.75M)
     6. unaccounted: prefix grew where the transcript cannot see it      x38       44k once      1.46M in prompts (±922k)
```

Six of twenty rows; the command prints the whole ranking, plus a `cost of errors` section under it.

Two columns, and the second is the one that matters. 177k of Bash tool calls were written once; they
were then carried into every later prompt in the session, which is 6.60M tokens paid for. That is
what "context is expensive" means arithmetically.

Row 6 is Replay declining to guess. Transcripts do not contain the system prompt, the tool
definitions or the cache markers, so part of the prefix is inferred, and the part that cannot be
attributed is printed as its own row rather than distributed across the others.

## Step 4: what to change

```sh
replay advise ~/.claude/projects/demo/
```

```text
Sessions: 1 found, 1 calibrated. Predictions assume the target is halved; shares are of prompt tokens, the scale-free metric.

1. [pending] Bash inputs are 28% of prompt tokens
   keep tool inputs short: run scripts from files instead of inline heredocs, and pass paths instead of contents
   evidence: 1 session(s), 6.60M tokens in prompts; predicted saving 14% of prompt tokens per session (3.30M tokens across the corpus)

2. [pending] first-turn instructions and attachments are 12% of prompt tokens
   split instruction files: keep what every turn needs, move the rest to on-demand files or skills the agent loads when relevant
   evidence: 1 session(s), 2.76M tokens in prompts *; predicted saving 6% of prompt tokens per session (1.38M tokens across the corpus)

3. [advice only] cache breaks re-billed 1% of prompt tokens
   run replay diff on the session to see the cause of each break; most are a changed prefix or an edited turn
   evidence: 1 session(s), 189k tokens in prompts; predicted saving 1% of prompt tokens per session (189k tokens across the corpus)

* = estimated via the byte-to-token fit. Statuses: pending, applied, verified, not verified, advice only.
Advice file: /tmp/replay-demo/.replay/advice.json
```

This is the step where a person actually does something, and it is worth being clear that the
something is theirs. `replay advise --apply` will change exactly one setting, `promptCacheTtl`, and
only when the corpus supports it; on this fixture it refuses, and the refusal is the feature:

```text
What can be applied from evidence

refusing to change promptCacheTtl: no session in this corpus reproduced well enough to act on
```

Everything else in the list is a change to how you work, and Replay will not make it for you: those
edits change what your agent reads, and a tool that rewrote your instruction files because it judged
them long would be worse than one that tells you they are long.

If the change you are weighing is a cap on tool output, `replay trim` prices it against your own
history before you commit to it, and prices the damage as well as the saving:

```sh
replay trim ~/.claude/projects/demo/ --cap 4000
```

```text
9 block(s) over the cap, 53k bytes removable, 1.53M prompt tokens once resending is counted.
Worth $0.38 at cache-read prices, which is what a resent byte costs.
Priced as fresh input it would read $15.27, 40.0x larger and wrong.

Harm probe found nothing, which is a lower bound and not an all-clear.
```

Note what it refuses to claim. `$0.38`, not `$15.27`: a byte that is resent from cache is billed at
the cache-read rate, and pricing it as fresh input is the mistake that makes trimming look forty
times better than it is. And the harm probe reports a lower bound, because an agent rewriting a file
from content it read leaves no trace the probe can follow.

## Step 5: check the change before it costs you the prefix

Some changes cost more than they save, and the expensive one is a change to your tool set. Every
session holding a warm prefix re-bills that prefix in full on its next request, so removing an MCP
server to save its tool definitions can cost more on the day you do it than it saves in a week.

`replay prefix` answers that before you merge, reading only the two files you name:

```sh
replay prefix --before .mcp.json --after /tmp/mcp-after.json
```

```text
  The cached prefix is invalidated by this change.

    removed  chrome
    servers  2 before, 1 after

  Every session holding a warm prefix re-bills it in full on its next request.
  How many sessions that is: NOT MEASURED. It cannot be read from a diff, and a
  headcount multiplied by a per-session figure would be a guess wearing a total.
```

It exits 1 when the prefix is voided, so a CI step can gate on it with no output parsing.

The scope is narrower than the name suggests, and this is the first place the route stops. `prefix`
watches the tool **set** and nothing else. Hand it a changed `CLAUDE.md` or `AGENTS.md` — the file
step 4 just told you to split — and it declines:

```sh
replay prefix --before CLAUDE.md --after /tmp/claude-after.md
```

```text
  not a prefix input: neither file defines tool servers, so this change
  cannot void a cached prefix.
```

Exit 0. That answer is defensible: `internal/proxy/causedetail.go` records that across a 30-lane
trial, `system_bytes` never moved once and every real prefix change was the tool set changing. But
the reader who followed step 4 has just edited an instruction file and has no gate for it, and
nothing in the output says which file they should have passed instead.

## Step 6: mark it applied

Making the change is not something Replay can see. Marking it is a keystroke:

```sh
replay tui
```

Press `a` for the advise screen, `j`/`k` to the finding you acted on, then `a` again to mark it
applied. The mark is written to `~/.replay/advice.json` immediately:

```json
{
  "id": "5c4774fea9f7",
  "kind": "tool-inputs",
  "target": "Bash",
  "title": "Bash inputs are 28% of prompt tokens",
  "action": "keep tool inputs short: run scripts from files instead of inline heredocs, and pass paths instead of contents",
  "sessions": 1,
  "share": 0.2827019395307648,
  "prompt_tokens": 6601667,
  "predicted_share": 0.1413509697653824,
  "predicted_tokens": 3300833,
  "estimated": false,
  "status": "applied",
  "first_seen": "2026-09-02T21:36:59.095Z",
  "last_seen": "2026-09-02T21:36:59.095Z"
}
```

This step exists because the alternative was worse. Until 8f32a31 on 2026-09-09 the verifier inferred
that you had applied a suggestion from the very drop it then measured to decide whether the
suggestion had worked, which is circular, and it was wrong in both directions: corpus drift returned
`verified` for changes nobody made, and noise returned `not verified` for the same. A keystroke is a
recorded fact. Nothing else here is.

The second place the route stops is right here. Run `replay advise` again after marking it and the
finding still prints `[pending]`:

```text
1. [pending] Bash inputs are 28% of prompt tokens
```

The status file says `applied` and the report says `pending`, and neither one tells you why. The
reason is in step 7 and it is a real reason, but from the reader's chair the keystroke did nothing
visible, which is exactly how a keystroke gets pressed twice.

## Step 7: verify

Verification needs sessions that ran after the change, and the thresholds are in
`internal/advisor/advisor.go`: more than two sessions that exercised the target, the newest two
averaged against the mean of the earlier ones, a drop of at least 20% to count as applied at all, and
at least half the predicted saving realised to count as verified. Below that it is `not verified`,
which means the prediction did not hold — not that the change failed.

Cost per task has its own before-and-after, split at a date you choose:

```sh
replay cost --compare 2026-09-01 --predicted -0.20
```

On a corpus with work on both sides of the date — this block is from a real 116-session corpus, not
from the fixture, which has one session and cannot produce it:

```text
  before   58 tasks, median $0.84
  after    58 tasks, median $0.44
  change   -47% per task, on +0% task volume

not confirmed: predicted -20%, realised -47%
```

`--predicted` is the honest part of that. You state what you expected before you look, and the tool
reports whether it held, including when the result beat the prediction — that is still a prediction
that did not hold, and it says so.

On the fixture, and on any machine on its first day, you get this instead:

```text
Too few tasks to compare: 0 before, 1 after, and each side needs at least 10.
No figure is printed rather than a median of noise.
```

Ten tasks a side. This is the third and largest place the route stops, and it is not a defect: a
median of three tasks is noise wearing a figure. But it does mean the loop this document describes
cannot be closed on the day you install. You get the finding immediately, you can act on it
immediately, and then you wait until ten tasks have run on each side of the date you split at.

`replay since` is what to run in the meantime. It reports what ran and what it cost since you last
looked, which is the only feedback available while the comparison is still filling up. The first run
has nothing to compare against and says so rather than reporting a window it does not have:

```text
  No previous look recorded, so there is no window to report yet.
  This run sets the marker. Come back after some work and this will say what changed.
```

The second, with no work in between:

```text
  Nothing since you last looked, moments ago.
```

## Step 8: tell the agent it exists

On a machine where an agent runs the commands, an install the agent does not know about is an install
that changes nothing. Two commands address that, and it is worth being exact about what each one
does, because neither of them does the obvious thing.

```sh
replay mcp --install
```

prints the configuration that lets an agent ask Replay questions mid-session over JSON-RPC on stdio.
The binary is the server: no daemon, no port, nothing left running.

```sh
replay agents .                      # print the block
replay agents . --write AGENTS.md    # splice it in, between its markers
```

writes a block into the file coding agents read at startup. Read what it contains before assuming it
solves the problem:

```text
<!-- replay:sources:begin -->
## Where this project's records are

Generated by `replay agents`. Do not edit inside the markers, it is overwritten.
```

The first four lines of about twenty-five. The rest is one bold instruction paragraph, a list of the
directories in this project that look like they hold records, and a paragraph naming what that list
cannot see.

That block names where **this project** keeps its records. It does not say what Replay is, that it is
installed, or when an agent should reach for it. Nothing in the installer or the CLI writes a block
that does, and nothing prompts anyone to run `replay agents` in the first place.
[`docs/AGENT-SURFACE.md`](../AGENT-SURFACE.md) states the gap plainly and it is still open.

Until it closes, this is the block to paste by hand into `AGENTS.md` or `CLAUDE.md`. It is a proposal
rather than output: nothing in the tool prints it today. It is short because a boot file sits in the
cached prefix of every session, so its size is a cost paid on every request:

```text
## Replay

`replay` measures what a coding-agent task cost and which turn was billed twice
because a prompt cache broke. It reads transcripts already on disk and sends
nothing anywhere.

- `replay` - cost per task across this machine's transcripts, and the avoidable share
- `replay diff <transcript>` - where the cache broke, and the cause of each break
- `replay blame <transcript>` - what is filling the prompt, ranked
- `replay advise <dir>` - what to change, measured against this corpus
```

## Where this document says the product falls short

Four things, in the order a new person meets them.

**`doctor` never mentions money, and its first instruction does not run.** The command whose stated
job is "what to do next" prints no dollar figure anywhere, and the first `next:` line it prints is a
template:

```text
              next: replay replay /tmp/replay-demo/.claude/projects/<project>
```

Paste it and you get `replay: stat .../<project>: no such file or directory`, exit 1, within the
first five lines of the command whose whole job is to say what to do next.
[`docs/design/doctor-discovery-lens-beginner.md`](../design/doctor-discovery-lens-beginner.md)
recorded both of these on 2026-09-09, from a reader who closed the terminal at that point. The Codex
under-count in that review has since been fixed; these two have not.

**The gate does not cover the change the advice asks for.** Step 5. `replay prefix` gates tool-set
changes and returns "not a prefix input" for the instruction file that step 4 told you to split.

**Marking a finding applied produces no visible acknowledgement.** Step 6. The status file records
it and the report keeps printing `[pending]` until enough later sessions exist.

**The last step of the loop has not been observed to fire.** On the largest corpus available to this
project — 700 sessions, 74 suggestions, generated 2026-09-10 — the statuses are 17 `pending` and
57 `advice only`. Nothing is `applied`, nothing is `verified`, nothing is `not verified`. The
verification path described in step 7 is implemented and tested, and no measured run of it exists in
this repository. Treat step 7 as a design that has not yet been demonstrated, not as a feature with
a track record.

None of the four is an architecture problem. Three are a sentence or a missing gate, and the fourth
is arithmetic that needs sessions nobody has run yet.

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
