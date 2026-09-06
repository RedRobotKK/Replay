# Getting started

This walks through the first ten minutes with Replay: what it can see on your machine, what your
existing sessions already cost, and how to start measuring instead of estimating.

You do not need to change how your agent works, and nothing leaves your machine.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/RedRobotKK/Replay/main/install.sh | sh
```

If you already have Go, `go install github.com/RedRobotKK/Replay/cmd/replay@latest` does the same
job. Either way, check it landed:

```sh
replay version
```

## Get a number out of it

```sh
replay
```

With no arguments Replay finds your agent's transcripts, prices what they cost, and reports the share
of that spend nobody chose. Nothing to configure and nothing to point it at. If it finds no
transcripts it prints the command list instead, which is the honest answer to a machine with nothing
on it yet.

## Find out what Replay can see

```sh
replay doctor
```

`doctor` looks for the things Replay needs and tells you which of them exist: your agent's transcript
directory, whether a proxy variable is set, whether a proxy is already running, and whether a ledger
has been written yet. It finishes by naming the next command worth running. If you only ever run one
Replay command, make it this one.

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

## Where to go next

- [Commands](commands.md) covers every subcommand and the flags on `serve`.
- [Troubleshooting](troubleshooting.md) covers what goes wrong and what it means.
- [Architecture](../architecture/) explains how the replay engine and the proxy actually work.

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
