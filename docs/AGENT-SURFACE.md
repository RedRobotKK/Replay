# What PostHog's CLI does that Replay does not

**Status:** analysis, with four gaps that are real.

PostHog shipped a CLI built for agents first and humans second, and the
[docs](https://posthog.com/docs/cli) say so out loud. Reading it against
Replay's own surface turns up four things, three of which are cheap.

## The move worth stealing

Their installer teaches the agent.

```text
npx @posthog/wizard cli add
Installs the CLI and adds instructions to your coding agent
```

That second line is the whole idea. Installing the tool also writes down, where
the agent will read it, what the tool is and when to reach for it. Everything
else follows: the agent knows the tool exists, so the human never has to.

Replay's installer does not do this. It installs a binary and prints a next
command for a person. On a machine where an agent is the one running things,
that is an install the agent does not know happened.

## Four gaps

### 1. Nothing tells the agent Replay exists

`AGENTS.md` in this repository is for agents working **on** Replay. There is no
document for an agent **using** it, and the installer writes none.

The site already publishes one, at
`.well-known/agent-skills/replay-install/SKILL.md`, and `install.sh` does not
mention it. A skill nothing points at is a skill nobody loads.

**Cheap.** The installer offers to append a short block to `AGENTS.md` or
`CLAUDE.md`, with consent, and says exactly what it will write before writing
it. Nothing about it should be automatic: this project's whole position is that
you can see what it did.

### 2. There is no "when to use which"

PostHog documents **When to use the CLI** against **When to use the MCP**, so an
agent holding both does not have to guess.

Replay has three ways in and no such page: read transcripts from disk, sit in
front of the traffic as a proxy, or answer a question through the eight
shortcuts. Which one is right depends on whether the surface writes transcripts,
which is exactly the thing the surface registry knows and nobody else does.

### 3. The FAQ is written for the wrong reader

PostHog's FAQ answers the **agent's** confusions:

```text
My agent doesn't know about the CLI
The api command is unavailable
The tool list looks outdated
```

Replay's docs answer a person's questions. An agent that runs `replay serve`
against a surface Replay cannot parse gets a correct warning and no guidance on
what to do instead.

### 4. One command over many, rather than many flags

`posthog-cli api` is a single shell-friendly front over their whole tool
catalog. Replay has 72 flags across 11 commands and, until the shortcut layer,
no single entry point.

The eight questions are Replay's version of this and they are not finished:
they exist in `internal/tui` and no command runs them.

## What Replay has that this comparison does not flatter away

PostHog's CLI authenticates, stores a personal API token, and talks to their
servers. Replay's argument is the opposite: it runs locally, and the two typed
outbound surfaces are the point rather than an implementation detail.

So the borrowing stops at the shape. Teaching the agent about the tool is worth
copying. Doing it without asking, or writing a token into a config the user did
not read, is not.

---

[Documentation index](README.md) · [Repository README](../README.md)
