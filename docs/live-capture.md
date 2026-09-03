# Recording a live session

How to put Replay in front of a real Claude Code session and get measured-tier data. Nothing here needs a release: it builds from a clone.

## Install

```sh
git clone https://github.com/RedRobotKK/Replay.git
cd Replay
make ci        # lint, test, build; should be green before you trust anything
make build     # ./bin/replay
```

Put it on your path if you like: `sudo install -m 0755 ./bin/replay /usr/local/bin/replay`.

`go install github.com/RedRobotKK/Replay/cmd/replay@latest` works only once the GitHub repository itself is renamed to `Replay`; until then, clone.

## Check what Replay can see

```sh
replay doctor
```

It reports the transcripts it found, what is answering at `ANTHROPIC_BASE_URL`, and whether the ledger has anything in it, with the next command for each. Run it again any time something looks wrong; it distinguishes "nothing is listening" (the agent will fail) from "another gateway answers" (the agent is fine, Replay just records nothing).

## Record

Two shells. In the first:

```sh
replay serve
```

It prints the address it bound and the ledger directory. Leave it running.

In the second, the one you start the agent from:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:4000
export ENABLE_TOOL_SEARCH=true   # see the caveat below
claude
```

Work normally for a few turns. `replay doctor` in a third shell should now say Replay is answering and the ledger is filling up.

Already behind another gateway? Chain through it instead of replacing it:

```sh
replay serve --upstream https://your-gateway.example
```

## Read what it recorded

```sh
replay ~/.replay/ledger/            # measured tier: what the provider actually charged
replay diff ~/.replay/ledger/       # every cache break, located and classified
replay blame ~/.replay/ledger/      # what is eating prompt tokens
curl -s localhost:4000/replay/status | jq   # live per-session totals and what-if rows
```

The tier line at the top of each report says `measured` rather than `estimated`. That is the whole point of running the proxy: transcripts alone cannot see the system prompt and tool definitions, so those figures are inferred; the ledger sees the bytes.

## Stop

Ctrl-C. The proxy closes connections that carry no turn immediately and gives a turn in flight up to five seconds, so it exits promptly and with status 0.

## Caveats worth knowing before you start

- **Credentials pass straight through.** Replay never stores or logs them. It binds loopback only and rejects browser origins. Add `--token` if you want a shared secret on top.
- **The ledger holds no message text**, only structure, sizes, labels, and usage. That is by design; `replay redact` exists for when you want to share a transcript.
- **MCP tool search.** With a non-first-party base URL, Claude Code disables MCP tool search unless `ENABLE_TOOL_SEARCH=true` is set. Replay forwards `tool_reference` blocks unchanged, so setting it is safe.
- **Nothing is modified by default.** Policies, masking, and guards are all opt-in flags. Plain `replay serve` forwards bytes unchanged and only watches.
- **`REPLAY_DISABLED=1`** makes `serve` refuse to start; unset `ANTHROPIC_BASE_URL` to bypass Replay entirely.

## Then: spike 4

The one open question that needs live traffic is whether adding the provider's context-editing parameter leaves Claude Code's behavior intact.

```sh
replay serve --context-edit-trigger 200000
```

Run a session of ten or more turns that gets past the trigger. It is admissible only when the client already enabled the context-management beta and set no `context_management` of its own, so it may simply never apply — the log says either way, and the ledger records the provider's applied edits and cleared tokens. Success is a session that completes normally with the parameter present.
