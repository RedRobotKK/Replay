# 0005. License the project under Apache 2.0

**Status:** Accepted
**Date:** 2026-09-02

## Context

The repository was scaffolded under the Business Source License 1.1 as the v4 PRD specified, with conversion to Apache 2.0 after three years. Both adversarial reviews flagged the choice: BSL is not an open-source license, it deters corporate contributors and sharing, and the project's stated goal is reach and attention, not a licensing moat. The core analysis (calibration, blame, diff, replay) will be commoditized regardless; any commercial tier can be licensed separately later.

## Decision

The project is licensed under the Apache License, Version 2.0, from this commit onward, with a NOTICE file carrying the copyright line. Contributions are accepted under the same license per its Section 5. The README may call the project open source.

## Consequences

- Anyone may use, modify, and redistribute Replay, including in commercial products. That is intended.
- The patent grant in Section 3 protects users and contributors.
- A future commercial offering must be a separate work; it cannot retroactively restrict this code.
- The BSL parameters that were drafted (Additional Use Grant, Change Date) are void; no release was ever published under them.

## Alternatives considered

**BSL 1.1 as drafted.** Rejected for the reasons above.

**MIT.** Acceptable, but Apache 2.0's explicit patent grant is worth the slightly longer text for a tool that sits in front of API traffic and may attract corporate adopters.
