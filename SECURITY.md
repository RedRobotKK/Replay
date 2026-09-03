# Security Policy

Replay will sit between developers and their model provider and will hold API keys, session tokens, and plaintext of masked secrets in memory. We treat every report seriously.

## Reporting a vulnerability

**Do not open a public issue.** Use one of:

1. GitHub private vulnerability reporting: <https://github.com/RedRobotKK/Replay/security/advisories/new>
2. Email: <security@redrobot.jp>

Include the version (`replay version`), platform, a reproduction, and the impact as you understand it. You will get an acknowledgement within 3 working days and a status update at least every 7 days until resolution.

## Scope

In scope: anything in this repository, including the daemon, its build and release pipeline, and the documentation where it would lead a user into an unsafe configuration.

Out of scope: vulnerabilities in the model providers, IDEs, or agents Replay talks to. Report those upstream.

## Supported versions

Until 1.0, only the latest minor release receives security fixes.

## Disclosure

We follow coordinated disclosure. We ask for 90 days from acknowledgement before public disclosure, and we will credit reporters in the release notes unless they prefer otherwise.
