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

## Where to go next

- [Commands](commands.md) covers every subcommand and the flags on `serve`.
- [Troubleshooting](troubleshooting.md) covers what goes wrong and what it means.
- [Architecture](../architecture/) explains how the replay engine and the proxy actually work.

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
