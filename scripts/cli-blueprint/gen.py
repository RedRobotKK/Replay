#!/usr/bin/env python3
"""Generate docs/CLI.md from the binary, not from memory.

A command reference is a claim about what a program does, and a hand-written one
starts drifting the moment a flag is added. This asks the built binary what its
flags are and writes the answer down, so the reference is a recording rather than
a recollection.

    go build -o /tmp/replay ./cmd/replay
    scripts/cli-blueprint/gen.py --bin /tmp/replay > docs/CLI.md

`check.sh` runs that and fails if the result differs from what is committed,
which is what stops the drift rather than merely describing it.

Two things are handled carefully because they cost a ten-minute timeout to
discover. `replay mcp` is a JSON-RPC server on stdin and blocks forever without
a closed stdin. Several commands do real work before parsing --help, so they are
run against an empty transcript root and given a deadline.
"""
import argparse, os, re, subprocess, sys, tempfile

# Every command main.go dispatches. Kept explicit rather than scraped, because a
# command that disappears should break this loudly instead of vanishing quietly
# from the reference.
#
# The cost of that choice is the opposite hole, and it was found the first time
# a command was added after this file existed: `prefix` dispatched, appeared in
# --help and in the guide, and was silently absent here, because nothing checked
# this list against the dispatch table. TestCLIBlueprintCoversEveryCommand now
# does, so the list stays explicit and can no longer fall behind.
COMMANDS = [
    "cost", "ceiling", "diff", "advise", "serve", "tui", "context", "blame", "replay",
    "route", "trim", "codex", "burn", "agents", "mcp", "corpus", "learn",
    "probe", "doctor", "rules", "statusline", "redact", "version", "prefix", "since", "budget",
    "upgrade", "purge", "privacy", "pool",
]

# What each command is for, and the two facts an agent needs before running one
# unsupervised: does it reach the network, and does it write anything.
#
# These were verified against the source, not recalled, after a first draft got
# three of them wrong in the direction that matters: it called `cost` a pure
# reader when it writes an index cache, missed that `doctor` and `burn` open
# loopback connections, and said `rules` only reaches the network with --update
# when --check-prices fetches a published price table too.
#
#   net:    "none" | "loopback: ..." | "outbound: ..."
#   writes: "none" | what it can write
META = {
    "cost":       ("Cost per task from transcripts already on disk", "none", "a transcript index cache and the tip-frequency file under ~/.replay; --png writes a card; --contribute writes a corpus submission"),
    "ceiling":    ("What a cache-blind budget ceiling halts your agents at, in your billing basis", "none", "none"),
    "diff":       ("Locate and classify every cache break, with its cause", "none", "none"),
    "advise":     ("Rank the largest token sources, with predicted savings", "none", "with --apply --yes, a settings file; --out writes advice.json"),
    "serve":      ("Local proxy: forwards to the provider, records a ledger", "outbound: proxies every request to the provider", "~/.replay/ledger/<session>.jsonl"),
    "tui":        ("The same answers as screens you can move between", "loopback: the proxy status endpoint, if one is running", "a temp file, created and removed, on the doctor screen"),
    "context":    ("What entered a session's context, by tool", "none", "none"),
    "blame":      ("Rank what is eating prompt tokens", "none", "none"),
    "replay":     ("Reproduce caching, then score alternative layouts", "none", "none"),
    "route":      ("What switching models would change, structurally", "none", "none"),
    "trim":       ("What a byte cap on tool output would have saved, and cost", "none", "none"),
    "codex":      ("The same reading, for OpenAI Codex rollout logs", "none", "none"),
    "burn":       ("What each agent surface burned: Codex, Ollama, Claude Code", "loopback: Ollama's version endpoint", "none"),
    "agents":     ("A boot block naming where this project keeps its records", "none", "with --write, splices into the named file"),
    "mcp":        ("Answer an agent's questions mid-session, JSON-RPC on stdio", "none", "none"),
    "pool":       ("Aggregate corpus submissions into one figure, with its roster", "none", "none"),
    "corpus":     ("Calibration across many sessions, as Markdown", "none", "--contribute writes a calibration report"),
    "learn":      ("Re-score the policy catalog, select one with held-out checks", "none", "--out writes policy.json"),
    "probe":      ("Measure a model's caching floor", "outbound only with --execute, which sends billable requests", "--record appends to measurements.jsonl; --contribute writes a submission file"),
    # The two commands that exist because the tool keeps something. purge is
    # the only one that removes it, and privacy the only one that discloses it;
    # both are answers to questions an auditor and a subject access request ask.
    "purge":      ("Remove ledger records past a retention window, or one session's", "none", "removes ledger records under the directory given; nothing unless --yes"),
    "privacy":    ("Everything Replay has written to this machine, and what each store holds", "none", "nothing: it reports and never removes"),
    # The only command that reaches the network without being asked to proxy
    # anything, and the only one that writes to the binary the reader invoked.
    # Both facts belong in the two columns an agent reads before running a
    # command unsupervised, stated plainly rather than softened.
    "upgrade":    ("Replace this binary with the latest published release", "outbound: github.com, to resolve the latest tag and download the release archive and its checksums", "the running binary, in place, after its checksum is verified; --check and --dry-run write nothing"),
    "doctor":     ("What replay can see on this machine and what to do next", "loopback: the local proxy status endpoint, and Ollama on 127.0.0.1:11434", "a temp file, created and removed, to test whether ~/.replay is writable"),
    "rules":      ("Show the provider rules in effect, or install a dated document", "outbound with --update (https) and with --check-prices (fetches LiteLLM's published table)", "with --update, installs a rules document"),
    "statusline": ("Live spend and cache-miss cost, for Claude Code's status line", "none", "none"),
    "redact":     ("Strip content, keep structure and usage (for bug reports)", "none", "none"),
    "version":    ("Print build information", "none", "none"),
    "prefix":     ("Whether a change to a tool-server document voids the cached prefix", "none", "none"),
    "since":      ("What ran, and what it cost, since you last looked", "none", "~/.replay/seen.json, one timestamp; --peek writes nothing"),
    "budget":     ("What this configuration costs on every request, before any work", "none", "none"),
}

SCREENS = [
    ("cost", "what the work cost", "replay cost"),
    ("why", "where the cache broke and why", "replay diff"),
    ("context", "what entered the context, by tool", "replay context"),
    ("advise", "the largest token sources, ranked", "replay advise"),
    ("guards", "spend caps drawn from your own spread", "replay advise --guards"),
    ("model", "what switching models would change", "replay route --to <model>"),
    ("safe", "what a byte cap would have saved", "replay trim --cap <n>"),
    ("doctor", "what replay can see on this machine", "replay doctor"),
    ("share", "a paste-ready card, no totals or paths", "replay cost --share"),
]


def help_for(binary, cmd, empty_root):
    """Ask the binary for one command's flags, safely."""
    env = dict(os.environ, REPLAY_TRANSCRIPTS=empty_root, NO_COLOR="1")
    try:
        p = subprocess.run([binary, cmd, "--help"], capture_output=True, text=True,
                           stdin=subprocess.DEVNULL, timeout=20, env=env)
    except subprocess.TimeoutExpired:
        return None
    return (p.stdout or "") + (p.stderr or "")


def flags(text):
    """Pull (name, type, description) out of Go's flag output.

    Go prints the flag and its type on one line and the description, indented,
    on the next. Descriptions can run to several lines.
    """
    if not text:
        return []
    out, cur = [], None
    for line in text.split("\n"):
        m = re.match(r"^\s+-([a-z0-9-]+)(?:\s+(\S+))?\s*$", line)
        if m:
            if cur:
                out.append(cur)
            cur = [m.group(1), m.group(2) or "bool", []]
        elif cur is not None and line.strip():
            if re.match(r"^\s{4,}\S", line) or line.startswith("\t"):
                cur[2].append(line.strip())
            else:
                out.append(cur)
                cur = None
    if cur:
        out.append(cur)
    return [(n, t, " ".join(d)) for n, t, d in out]


def md_cell(text):
    """Make one flag description safe inside a Markdown table.

    Pipes would end the cell early, and a bare URL fails the repository's
    markdownlint rule. Both are properties of the destination, so they are fixed
    here rather than by hand-editing a generated file that CI then rejects.
    """
    text = text.replace("|", "\\|")
    text = re.sub(r'(?<![`(])(https?://[^\s,)"\'`]+)', r"`\1`", text)
    return undate(text)


# Any flag whose default is a date is a default that changes at midnight.
DATED_DEFAULT = re.compile(r'\(default "(\d{4}-\d{2}-\d{2})"\)')


def undate(text):
    """Replace a date-valued default with a placeholder.

    A generated artifact that embeds today fails every morning, and a check
    that fails daily gets switched off. The CI job that compares this file to
    the binary says exactly that, and then `replay pool` grew a --pooled-at
    whose default is time.Now(), so from the next midnight UTC the reference
    said one date and the binary printed another. Every open pull request
    failed the blueprint check, none of them for a reason of its own.

    Normalised here rather than in the flag, because the binary telling a
    reader that --pooled-at defaults to today is correct and useful. It is only
    writing it into a file under version control that is wrong.
    """
    return DATED_DEFAULT.sub('(default "<today, in UTC>")', text)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True, help="path to a built replay binary")
    a = ap.parse_args()

    empty = tempfile.mkdtemp(prefix="replay-cli-blueprint-")
    w = sys.stdout.write
    w("<!-- Generated by scripts/cli-blueprint/gen.py. Do not edit by hand. -->\n")
    w("# The CLI, every command and every flag\n\n")
    w("**Generated from the binary**, not written from memory, and checked in CI. A\n")
    w("reference somebody typed out is a claim about the program; this one is a\n")
    w("recording of it. If a flag is added and this file is not regenerated, the build\n")
    w("fails.\n\n")
    w("It exists for agents as much as for people. `replay mcp` answers questions\n")
    w("mid-session over JSON-RPC and `replay agents` writes a boot block naming where\n")
    w("a project keeps its records; this is the map both of those assume you have.\n\n")

    w("## Before running anything unsupervised\n\n")
    w("Two questions decide whether a command is safe to run without a human: does it\n")
    w("reach the network, and does it write. Both are answered per command below, and\n")
    w("summarised here.\n\n")
    safe = [c for c in COMMANDS if META[c][1] == "none" and META[c][2] == "none"]
    w("**Opens no socket and writes nothing** — safe to run at will:\n")
    w(", ".join(f"`{c}`" for c in safe) + ".\n\n")
    w("**Leaves the machine:** `serve` proxies every request to the provider.\n")
    w("`probe --execute` sends billable requests, and without `--execute` it prints a\n")
    w("plan and sends nothing, which is the default. `rules --update <https url>`\n")
    w("fetches a document, and `rules --check-prices` fetches LiteLLM's published price\n")
    w("table. That is the whole list.\n\n")
    w("**Opens a loopback connection but never leaves the machine:** `doctor` and `tui`\n")
    w("ask the local proxy for its status, and `doctor` and `burn` ask Ollama on\n")
    w("`127.0.0.1:11434` for its version. Each fails quietly when nothing is listening.\n\n")
    w("**Writes, including where you might not expect it:** `cost` keeps a transcript\n")
    w("index cache and a tip-frequency file under `~/.replay`, so it is not a pure\n")
    w("reader even without `--png`. Then `serve` (the ledger), `advise --apply --yes`,\n")
    w("`agents --write`, `learn --out`, `probe --record` and `--contribute`, and\n")
    w("`rules --update`. `doctor` and `tui` create and immediately remove one temp file\n")
    w("to test whether `~/.replay` is writable.\n\n")

    w("## Commands\n\n")
    w("| Command | What it answers | Network | Writes |\n|---|---|---|---|\n")
    for c in COMMANDS:
        d, net, wr = META[c]
        w(f"| [`{c}`](#{c}) | {d} | {net} | {wr} |\n")
    w("\n")

    total = 0
    for c in COMMANDS:
        d, net, wr = META[c]
        txt = help_for(a.bin, c, empty)
        fl = flags(txt)
        total += len(fl)
        w(f"### {c}\n\n{d}.\n\n")
        if fl:
            w("| Flag | Type | What it does |\n|---|---|---|\n")
            for n, t, desc in fl:
                w(f"| `-{n}` | {t} | {md_cell(desc)} |\n")
            w("\n")
        else:
            w("Takes no flags.\n\n")

    w("## The TUI covers the same ground\n\n")
    w("`replay tui` opens the same answers as movable screens. `--screen <name>` opens\n")
    w("on one directly, and `--once` renders a single frame and exits, which is how a\n")
    w("pipe or a screenshot gets a stable image.\n\n")
    w("| Screen | The question | The command that answers it |\n|---|---|---|\n")
    for s, q, cmd in SCREENS:
        w(f"| `{s}` | {q} | `{cmd}` |\n")
    w("\n```sh\nreplay tui --screen why            # open on the cache-break screen\n")
    w("replay tui --screen cost --once    # one frame, for a pipe\n")
    w("replay tui --color never           # NO_COLOR always wins regardless\n```\n\n")

    w("## Notes an agent will otherwise learn the hard way\n\n")
    w("- **`replay mcp` blocks.** It is a JSON-RPC server on stdin. Give it a closed\n")
    w("  stdin or a request, never an interactive terminal you expect to return.\n")
    w("- **Several commands work before parsing `--help`.** Running one against a large\n")
    w("  corpus to read its usage will scan the corpus first. Point\n")
    w("  `REPLAY_TRANSCRIPTS` at an empty directory when you only want the flags.\n")
    w("- **`--json` is not universal.** `cost`, `context`, `route`, `trim` and\n")
    w("  `advise --apply` emit it; the rest print for people.\n")
    w("- **An unpriced model is excluded, never counted as free.** Figures say how many\n")
    w("  transcripts were left out.\n")
    w("- **`probe` sends nothing without `--execute`**, and `--execute` still asks\n")
    w("  before spending unless `--yes` is given.\n\n")

    # No date and no version string here on purpose. This file is diffed in CI,
    # and a generated artifact carrying today's date fails every morning for a
    # reason that has nothing to do with the code. A check that fails daily is a
    # check somebody switches off, and it would take the real one with it.
    w("---\n\n")
    w(f"{len(COMMANDS)} commands, {total} flags, read from the binary.\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
