# Beginner lens: `replay doctor` agent discovery

Reviewer stance: installed Replay ten minutes ago. Daily Claude Code user. Has
heard of Codex and Ollama. Does not know what a ledger, a prefix, a lane, a
surface, or a proxy is.

Branch `feat/doctor-agent-discovery`, built from source on 2026-09-09.
Files read only after the output confused me: `cmd/replay/discover.go`,
the `agents` block in `cmd/replay/doctor.go` (lines 67–87).

---

## 1. What I think it is telling me, first read, normal speed

"I found your Claude Code history. There's a lot of it. I also found four other
AI tools on your computer and here is a list of them with, for each one, a
sentence about what's inside and a thing to type. Two of the four I can
actually do something with. Also you haven't turned some proxy thing on."

That is a genuinely good first impression for the top half. `transcripts 123
sessions across 12 projects` told me something true about me in the first three
seconds. I liked that.

Then the `agents` block arrived and I stopped reading for meaning and started
skimming for the bold thing to type, and there wasn't one, because nothing is
bold and everything is the same weight.

What I did **not** get from the read: any sense of what this tool is *for*. The
word "cost" appears exactly once in the whole output, buried mid-sentence in
`replay cost over this directory reports on every one of them`. I installed this
because someone said it tells me what my agent costs. `doctor` never mentions a
dollar. `replay` with no arguments does — and nothing here told me to run that.

---

## 2. The single line that confused me most

```
              next: no reader built yet. The data is there; the dollar scale is unverified
```

What I thought it meant, in order, over about eight seconds:

1. "Reader" — is that a person? A file reader? Am I supposed to build one?
2. "The data is there" — where is *there*? In Grok, or in Replay?
3. "the dollar scale is unverified" — I have no idea. Unverified by whom? Is
   this warning me that the numbers *elsewhere in this output* are unverified,
   or only Grok's? I could not tell whether this was a warning about my money
   or an internal note the author forgot to delete.

It reads like a commit message. It is a developer talking to another developer
about the state of their own codebase, printed into a tool I just installed. I
am not offended by it, but I learned nothing and I felt like I'd walked into
someone else's standup.

Runner-up, and it is close:

```
              next: nothing to run: this is not a spend surface and cannot be priced
```

Two colons in one line. I parsed `nothing to run` as the name of a command
before I parsed it as a sentence.

---

## 3. What I would type next

**Nothing. I would have closed the terminal.**

Not out of frustration — out of not knowing there was a next step worth taking.
Here is the honest mechanism, because "I'd close it" is only useful with the
reason attached.

Doctor gave me six `next:` lines. I counted what happened when I tried to obey
them:

| line | what I'd type | what happened |
|---|---|---|
| `next: replay replay /Users/daniel/.claude/projects/<project>` | pasted it | `replay: stat /Users/daniel/.claude/projects/<project>: no such file or directory` |
| `next: replay codex` | worked | dense, but real |
| `next: nothing to run: ...` | — | nothing |
| `next: no reader built yet. ...` | — | nothing |
| `next: replay burn` | worked | real |
| `next: replay serve, then export ANTHROPIC_BASE_URL=...` | did not dare | see §5 |

The very first `next:` in the output is a **copy-paste that fails**. I pasted
`<project>` literally, because it is inside a path that is otherwise a real
absolute path I could see on my own disk, with no bracket convention explained
anywhere above it. It errored. That is the moment my confidence went, and it
happened at line 5.

By the time I reached the `agents` block I had already learned that the `next:`
lines are not reliably runnable. So the four `next:` lines there — two of which
are genuinely runnable — got skimmed rather than trusted.

**What would have kept me:** the single most valuable command in this tool is
`replay` with no arguments. It prints `total $3306.29 / avoidable $159.43`.
That is the answer to the question I installed the tool to ask. `doctor` — the
command whose stated job is "what to do next" — never mentions it.

---

## 4. Words I did not understand

Every one of these made me pause. Not being generous.

| word | where | what I guessed |
|---|---|---|
| **agent lane** | `a session writes one per agent lane` | a thread? a swimlane? Never defined. It is doing load-bearing work in explaining why 123 sessions is 1737 files, and I did not follow the explanation. |
| **rollout logs** | Codex line | Something to do with deployments? (It's Codex's own word for its transcripts. I did not know that.) |
| **spend surface** | Cursor line | Surface of what. This is the phrase that told me the author has a mental model I have not been given. |
| **priced / cannot be priced** | Cursor line | I think I get it — no token counts, so no dollars — but only after the Grok line, three lines later, listed field names. |
| **reader** | Grok line | See §2. |
| **dollar scale** | Grok line | Genuinely no idea. |
| **rate-limit events** | Codex line | I know what a rate limit is. I do not know why a *log of them* is a selling point. |
| **prompt sizes and eval durations** | Ollama line | "eval" as in evaluate? Is that an Ollama word? |
| **proxy** | proxy section | I know it's a middle-man. I do not know what this one does to my traffic. |
| **ledger** | ledger section | A file of some kind. Section heading with no sentence explaining it. |
| **measured tier** | `next: replay replay ~/.replay/ledger  (measured tier)` | Parenthetical with zero context. Tier of what? Is there an unmeasured tier? Am I on it? |

Eleven terms in twenty lines. That is the review, really.

**The `ledger` section is the worst offender** and it is not even the new
feature. It is three words and a path:

```
ledger        5 sessions recorded under /Users/daniel/.replay/ledger
              next: replay replay /Users/daniel/.replay/ledger  (measured tier)
```

Every other section gets a sentence of explanation. This one gets a noun I do
not know, a path, and a parenthetical I do not know. I have 5 of something and
no idea whether that is good.

---

## 5. What I would not trust

### 5a. The Codex file count is wrong, and I caught it by accident

Doctor:

```
agents        Codex   29 files in /Users/daniel/.codex/sessions
```

`replay codex`, the command doctor told me to run, on the very next line:

```
  610,551,532 tokens billed across 150 Codex session(s)
```

29 files, then 150 sessions. I checked my own disk. `~/.codex/sessions` has 29
files; `~/.codex/archived_sessions` has 121 more. 29 + 121 = 150.

`discover.go:73` looks only at `.codex/sessions`. `codexRoots()` in
`codex.go:23` looks at both — and carries a comment explaining, in detail, that
looking at only the first is a bug this project already fixed once:

> a reader that knows only the first reports a fraction of the corpus as though
> it were all of it. On the machine this was written against that is 27 of 148
> files: a total that is wrong and looks right

The new discovery code reintroduces the exact defect the old code documents
fixing. Doctor is under-reporting my Codex history by 81%.

As a beginner this is the sharpest possible trust failure, because I did not go
looking for it — I ran the two adjacent commands the tool told me to run, in
order, and they disagreed about my own computer. `discover.go`'s own header
comment says the point of the two rules is that "a reader who checks one claim
and finds it hollow stops believing the transcript count on the line above."
That is precisely what happened to me.

### 5b. `123 sessions` vs `115 sessions`

`doctor` says `123 sessions across 12 projects`. `replay` with no arguments says
`across 115 sessions (1723 agent lanes)`. Two numbers for the same directory,
eight apart, in two commands run one minute apart. There may be a good reason
(unpriceable models are excluded — the bare command says so, doctor does not).
I did not know that, and I noticed the mismatch before I noticed the
explanation.

### 5c. The number moves between runs

I ran `doctor` twice. `1737 transcript files`, then `1738`. Probably correct —
I had a session running. But nothing told me the figure was live, so my first
read was "it's guessing."

### 5d. Cursor's count includes a file that is not a transcript

`Cursor 119 files` uses patterns `*.db, *.jsonl, *.sqlite`. One of those 119 is
`~/.cursor/ai-tracking/ai-code-tracking.db`, which is not a conversation
transcript. The line then says `conversation transcripts only` about a count
that isn't only conversation transcripts. Small, but it is the same class of
thing as 5a.

### 5e. Grok's claim, checked — this one holds

Fair is fair. `usage fields are present: inputTokens, cachedReadTokens,
costUsdTicks` — I grepped. 72 of 74 files contain `inputTokens`, and the sample
file has all three fields. That claim is good.

### 5f. The proxy line asks me to reroute my API traffic and tells me nothing

```
proxy         ANTHROPIC_BASE_URL is not set in this shell; the agent talks to the provider directly
              next: replay serve, then export ANTHROPIC_BASE_URL=http://127.0.0.1:4000
```

This is asking me to point Claude Code — with my API credentials — at a program
I installed ten minutes ago. `doctor` does not say that it stays on my machine,
that it forwards requests unmodified, or that nothing is sent anywhere. (The
`--help` text does say "byte-for-byte passthrough". Doctor does not, and doctor
is where the instruction lives.) I would not have run it. Not because I think
it is malicious — because nobody told me, at the point of asking, what it does.

---

## 6. The three different "next" steps: did I understand the difference?

**Partly, and only after re-reading.** Deliberate or half-finished? Honest
answer: **it read as half-finished, and I now think it was deliberate, and that
is a presentation failure rather than a design failure.**

Here is what I actually understood on first read:

- `next: replay codex` — a thing to type. Clear.
- `next: nothing to run: this is not a spend surface and cannot be priced` —
  I got "there is nothing for me here." I did not understand *why*, because
  "spend surface" is not a phrase I know. Verdict: understood the outcome,
  not the reason.
- `next: no reader built yet. The data is there; the dollar scale is
  unverified` — I read this as **"the developer hasn't finished this yet."**
  And that reading contaminated the other three. Once one line in a list says
  "not built", the list stops being a report about *my machine* and becomes a
  progress board for *the author's backlog*.

That is the real damage. There are three genuinely different states here and
they are all worth distinguishing:

1. **Readable now** (Codex, Ollama) — here is your command.
2. **Never readable** (Cursor) — the files exist but contain no numbers; this
   is a permanent fact about Cursor, not a gap in Replay.
3. **Not yet readable** (Grok) — the numbers exist, Replay can't read them yet.

State 2 and state 3 are opposite in meaning — one is "don't wait for this",
the other is "wait for this" — and they are rendered in identical typography,
identical position, identical prefix (`next:`), sorted alphabetically so
Cursor sits *between* the two working ones. Nothing signals that one is a
statement about Cursor and the other is a statement about Replay.

Also confusing: **three tools point at three different commands, but `replay
burn` covers all three.** Ollama's `next` is `replay burn`; I ran it, and it
reported codex, ollama *and* claude-code in one table. So `replay codex` is a
subset of `replay burn`, and nothing said so. I now think I ran the same data
twice.

The word `next:` is also carrying four different jobs — a command, a
non-command, a status report, and a two-step shell instruction. If `next:`
sometimes isn't a next step, I can't scan for it, and scanning for it is the
only thing I was doing.

---

## 7. What I expected and did not find

- **A number in dollars.** This is the headline omission. `doctor` is the
  command a new user runs first (it is the one named in the tagline, "what
  replay can see on this machine and what to do next"), and it never says what
  anything cost. The answer I wanted was one `replay` away and doctor did not
  point at it.
- **One recommended command, marked as the one to run.** Six `next:` lines of
  equal weight is not guidance, it is a menu. I wanted "→ start here".
- **A sentence saying what Replay is,** before it starts listing what it found.
  Three words at the top — "what your coding agent cost" — would have anchored
  every jargon term that followed.
- **Any acknowledgement of the tools it looked for and did not find.** More on
  this below; it is the biggest single finding.
- **Consistency with the tool's own numbers.** See 5a and 5b.
- **`1 sessions across 1 projects`** — I hit this on a fresh home with one
  transcript. Unpluralised. Trivially small, but it is the kind of thing that
  makes me assume the rest was written at the same speed.

---

## THE EMPTY-HOME CASE — reported prominently, as instructed

```
$ HOME=$(mktemp -d) /tmp/rp doctor

replay doctor

transcripts   none found under /var/folders/.../T/tmp.RigtKYkihn/.claude/projects
              run a Claude Code session first, or point replay at another directory
proxy         ANTHROPIC_BASE_URL is not set in this shell; the agent talks to the provider directly
              next: replay serve, then export ANTHROPIC_BASE_URL=http://127.0.0.1:4000
ledger        empty (/var/folders/.../T/tmp.RigtKYkihn/.replay/ledger)
```

Five lines. This is what a genuinely new user sees, and it is much worse than
the populated case. Four separate problems:

**1. The `agents` section vanishes entirely, with no trace.** `doctor.go:73`
guards the whole block on `len(found) > 0`. The code comment defends this — "a
heading over an empty list reads as a broken feature" — and I disagree from the
user's chair. Silence is indistinguishable from *the feature not existing*. I
have Codex installed on my real machine; if my sessions directory happened to be
empty, doctor would never tell me it looked. I would not know the capability was
there. The header comment in `discover.go` says the whole feature exists because
"the tool was measuring one agent on a desk running six" — and on an empty home
it advertises that it measures exactly one. A single line — `agents  none of
Codex, Cursor, Grok, Ollama have written anything under your home yet` — costs
nothing and converts silence into a promise.

**2. `run a Claude Code session first, or point replay at another directory` —
it does not say how to point it.** No flag, no environment variable, no example.
`--help` mentions `$CLAUDE_CONFIG_DIR`; this line, the one actually telling me
to do it, does not. It is an instruction with the operative half missing.

**3. The `ledger` line has no `next:` at all.** When I have 5 ledger sessions I
get a `next:`. When I have zero — the state where I most need to be told what to
do — I get `ledger empty (<path>)` and nothing else. The guidance is inverted:
help arrives only once you no longer need it.

**4. The one remaining `next:` is the one I am least willing to run.** On an
empty home, the *entire actionable content* of doctor is "start a proxy and
reroute your API traffic through it" — with no explanation of what the proxy
does (see 5f). A brand-new user's whole experience of this tool is: it found
nothing, it explained nothing, and the only thing it asked me to do is the one
thing that touches my credentials.

A brand-new user reading these five lines learns: nothing about cost, nothing
about the four other agents Replay can read, nothing about how to point it at
their data, and one instruction they should probably refuse. I would close it.

---

## Narrow-terminal behaviour (COLUMNS=60)

Lines run to 99 characters. The layout is a fixed two-column grid with a
13-space label gutter, and at 60 columns the terminal hard-wraps mid-word:

```
transcripts   123 sessions across 12 projects under /Users/d
aniel/.claude/projects
              1738 transcript files in all: a session writes
 one per agent lane, so
```

The alignment that carries all the structure — labels left, detail indented —
is destroyed, and paths break mid-token so they cannot be copied. `COLUMNS` is
not consulted. In a split pane or a side terminal, which is where a
Claude Code user lives, this is the default experience.

---

## Summary of defects, in the order I would fix them

| # | defect | where |
|---|---|---|
| 1 | Codex discovery misses `archived_sessions`; doctor says 29, `replay codex` says 150 | `discover.go:73` |
| 2 | Empty home: `agents` section silently absent, so the feature is invisible to new users | `doctor.go:73` |
| 3 | First `next:` in the output is a paste that fails (`<project>` placeholder) | `doctor.go:64` |
| 4 | `no reader built yet. The data is there; the dollar scale is unverified` — developer note in user output | `discover.go:84` |
| 5 | Doctor never mentions cost, and never points at bare `replay` | `doctor.go` |
| 6 | Empty ledger gets no `next:`; populated ledger does | `doctor.go:129` |
| 7 | Eleven undefined terms — lane, surface, reader, priced, rollout, measured tier, ledger … | throughout |
| 8 | Cursor's "cannot be priced" and Grok's "not built yet" are opposite meanings in identical typography | `discover.go:73–96` |
| 9 | Proxy instruction asks for credential rerouting with no statement of what the proxy does | `doctor.go:93` |
| 10 | `1 sessions across 1 projects` — unpluralised | `doctor.go:55` |
| 11 | 99-char lines, `COLUMNS` ignored | `doctor.go` |
| 12 | Cursor count includes `ai-code-tracking.db`, not a transcript | `discover.go:92` |

## What is genuinely good, and should not be lost in a rewrite

- Requiring **counted files, not an existing directory**, before naming a tool.
  That rule is right and I would have been annoyed by the alternative.
- Saying **what can be read**, not just that something is there. Cursor's line
  is honest even though its wording defeated me.
- Not shelling out to `brew list` or reading `$PATH`. I did not have to think
  about it, which is the correct amount of thinking to do about it.
- The Grok field-name claim is **verifiable and verifies**. I checked it. That
  is worth a great deal.

The instincts behind this feature are better than its sentences. Almost
everything above is a wording or a presence/absence problem, not an
architecture problem — except #1, which is a real bug.
