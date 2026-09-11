# Getting started

This walks through the first ten minutes with Replay: what it can see on your machine, what your
existing sessions already cost, and how to start measuring instead of estimating.

You do not need to change how your agent works, and nothing leaves your machine.

## Install

```sh
curl -fsSL https://redrobot.jp/replay.sh | sh
```

If you already have Go, `go install github.com/RedRobotKK/Replay/cmd/replay@latest` does the same
job. Either way, check it landed:

```sh
replay version
```

### What the installer does when it finishes

On a terminal, the screens open as soon as the install completes. The script `exec`s `replay tui`,
so it owns your terminal until you press `q`, and it opens on `cost` — the same question `replay`
answers below. The closing lines say so before it happens:

```text
Next:  replay tui      # opening now, on cost: what your agent already spent
       replay          # after you quit: the same figures, as one report
       replay doctor   # if that found nothing, this says why
```

Off a terminal, nothing opens and those two commands are the next step. That covers CI, a
`Dockerfile RUN`, cron, a provisioner, and a container built without `-t`: the script probes
`/dev/tty` rather than stdin, because under `curl ... | sh` stdin is the script and says nothing
about whether a person is watching.

`--no-tui`, or `REPLAY_NO_OPEN=1`, declines the open on a terminal too. `CI` being set declines it
already.

## Get a number out of it

```sh
replay
```

With no arguments Replay finds your agent's transcripts, prices what they cost, and reports the share
of that spend nobody chose. Nothing to configure and nothing to point it at. If it finds no
transcripts it prints the command list instead, which is the honest answer to a machine with nothing
on it yet.

The avoidable line comes in two currencies, and the second one is probably yours:

```text
  avoidable      $150.27  (5% of the total)
                 31.4M tokens re-billed
```

**If you are on a subscription seat — Claude Pro or Max, Copilot, Cursor — the dollars are not your
money.** You are not billed per token, so those are list prices for someone who is, and the report
says so under the figures rather than leaving you to work out that the headline was addressed to
somebody else. The tokens are still yours: a re-billed token is context the work did not get, and
rate-limit budget spent on nothing. That is why the same finding is stated both ways.

`replay cost` prints the same report on its own, and takes the same default — no directory needed. It
names on stderr which root it read, so a report never leaves you guessing what it was over. Give it a
path when you want a different one.

## Find out what Replay can see

```sh
replay doctor
```

`doctor` looks for the things Replay needs and tells you which of them exist: your agent's transcript
directory, whether a proxy variable is set, whether a proxy is already running, and whether a ledger
has been written yet. It finishes by naming the next command worth running. If you only ever run one
Replay command, make it this one.

It reports **two** transcript figures where there are two, and the second only when it differs from
the first. Claude Code writes one transcript per session and one more per sub-agent lane, so a session
that fanned out contributes several files. `doctor` counts sessions; `cost` reads files. Both numbers
are right, and until they were printed side by side with the reason, seeing 91 from one command and
1494 from the next one second later looked like a bug.

## Read a session you have already paid for

Point Replay at a transcript directory. On Claude Code that is usually under
`~/.claude/projects/`, one directory per project.

```sh
replay ~/.claude/projects/your-project/
```

Replay reads what the agent already wrote. It reproduces the provider's caching turn by turn, prints
how closely that reproduction matched the provider's own numbers, and only then scores alternative
context layouts against what actually ran.

The order matters. A tool that tells you what you should have done, without first showing it can
explain what you did, is guessing. The calibration line at the top is Replay proving it understands
your session before it offers an opinion about it.

Add `--dollars` for a list-price column. The output names the date of the price table it used,
because prices change and other platforms charge differently.

## Understand estimated and measured

Every figure Replay prints carries one of two labels, and the difference is not cosmetic.

**Estimated** means the number came from a transcript. Transcripts do not contain the system prompt,
the tool definitions, or the cache markers, so some of the prompt has to be inferred. Useful, and
honest about being inferred.

**Measured** means the number came off the wire. For that, Replay has to be in the request path.

## Move from estimated to measured

```sh
replay serve
```

That starts a local proxy on `127.0.0.1:4000`. In the shell that runs your agent:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:4000
```

Now run your agent as you normally would. By default the proxy forwards every request and response byte for byte, including streaming and
cache markers, and never stores or logs your credential. Two opt-in features do modify traffic and
say so: `--mask` rewrites secrets in the body, and `--context-edit-trigger` adds the provider's
context-management parameter. Both are off unless you turn them on. What it writes is a ledger: block kinds, sizes, labels,
timings and usage, with no message text, under `~/.replay/ledger` with owner-only permissions.

Then run the same analysis against the ledger instead of the transcripts:

```sh
replay ~/.replay/ledger/
```

Same commands, measured tier.

If you want out, unset `ANTHROPIC_BASE_URL` and the proxy is bypassed entirely. `REPLAY_DISABLED=1`
stops it starting at all.

## How you actually talk to it

Four surfaces, and most people use two. Each one is a single command or a single
line of config.

**The command line.** Nothing to set up. `replay` with no arguments reads the
transcripts already on disk and prints cost per task.

**The status line, which is where most people will meet it.** It renders while
you work, every few hundred milliseconds, and carries live spend, cache health,
and the rate-limit window closest to binding with time until it resets. On a
subscription seat that window is the useful number, because the dollars are
somebody else's.

```sh
replay statusline --install    # prints the settings.json snippet
```

**MCP, so an agent can ask mid-session rather than being told afterwards.** The
binary IS the server: there is no daemon, no port and nothing hosted. Your agent
starts it as a child process, talks JSON-RPC over its pipes, and stops it when
the session ends.

```sh
replay mcp --install           # prints the config, with this binary's real path
```

```json
{ "mcpServers": { "replay": { "command": "replay", "args": ["mcp"] } } }
```

Seven tools. Five answer from the table compiled into the binary and touch no
network at all; the two that are facts about a remote release say so rather than
answering from a value that would go stale.

**The boot file, where the reader is another program.** `AGENTS.md` is where
several coding agents look at startup and `CLAUDE.md` is where one of them does.

```sh
replay agents .                      # print the block
replay agents . --write AGENTS.md    # splice it in, between its markers
```

It writes where this project keeps its records, so an agent asked "what was
sent" knows to look somewhere other than the first directory it thinks of. The
block is capped and drops file names on purpose: it sits in the cached prefix of
every session, so its size is a recurring cost paid against the breaks it
prevents.

**And one that is not an interface to the tool at all.** `replay cost --share
--png card.png` renders an image you post. That is an interface to other people,
and it carries a rate, a break count and a session count, never a path, a
project name or a spend total.

## Where to go next

- [The first run, end to end](first-run-journey.md) carries this walkthrough through to acting on a
  finding and checking whether the change worked, with real output at every step and a plain account
  of the three places that route stops.
- [Commands](commands.md) covers every subcommand and the flags on `serve`.
- [Troubleshooting](troubleshooting.md) covers what goes wrong and what it means.
- [Architecture](../architecture/) explains how the replay engine and the proxy actually work.

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
