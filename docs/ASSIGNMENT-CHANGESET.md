# Prepared changeset: RedRobot KK to Uncanny Valley Inc.

**NOT APPLIED. Apply on the day the assignment executes, not before.**

The documents follow the assignment. Editing them first would assert an
ownership that does not yet exist, which is the same defect class this
repository exists to find.

Every current value below was read from the tree at `08c0b93`.

---

## What this changeset does not do

- It does not change the **Change Date**. `2029-09-06` stands, and so does the
  conversion to Apache 2.0. Rights already granted under BUSL 1.1 with RedRobot
  KK as licensor cannot be retracted by assigning the copyright. **Investors
  must be told the asset becomes Apache-licensed in 2029**, in diligence, not
  after.
- It does not move the **module path**. That is a decision, not an edit. See
  the section at the end.
- It does not touch `redrobot.jp`, the RedRobot name, or anything else the KK
  keeps.

---

## 0. Prerequisite: who the assignor's shareholders are

**RedRobot K.K.'s two shareholders, Daniel Saito and Minoru Saito, are one
natural person holding two nationalities, two passports and two sets of bank
accounts.** Recorded here because every question below turns on it, and because
a reader who sees two names on a shareholder register will otherwise reach the
wrong conclusion about what this transaction is.

### What that removes

No minority consent. No fairness opinion. No related-party negotiation, because
there is no second party. No valuation is needed to protect anyone else's
stake, so the transfer can be made at book value or for nominal consideration
without anyone being prejudiced by the price.

Authorization on both sides is mechanical: written consent in lieu of a
meeting, signed once for the K.K. as assignor and once for Uncanny Valley Inc.
as assignee. The same human signing both sides is not a defect when there is no
counterparty to be disadvantaged. It still has to be papered, because the
authorization is what makes the transfer effective, not the intent behind it.

### What it adds, and this is the larger item

**A shareholder schedule that lists two names for one human is a false
statement about ownership.** Financing documents carry representations that the
capitalization disclosed is accurate and complete. A $300k instrument and a $3M
round both make that representation, and the party giving it is you. An
investor who later discovers that the two shareholders of the predecessor
entity were the same person will not read it as a harmless formality; they will
read it as a misstatement in the schedule they priced off, and the remedy sits
in the indemnity.

The fix is cheap and has to happen before the first instrument is signed, not
after:

1. **Consolidate the K.K. register, or disclose the identity.** Either the
   shares are recorded as held by one person under one legal name, or the
   register keeps both entries and the diligence response states plainly that
   they are the same individual. Either is defensible. Silence is not.
2. **Pick one legal identity and use it in every Uncanny Valley document.**
   Founder shares, the 83(b) election, the assignment, the board consents, the
   signature blocks. Mixing the two names across those documents puts a
   chain-of-title question exactly where the asset transfers, which is the one
   place the repository's own standard says a record must be unambiguous.
3. **Whichever identity is the K.K.'s registered representative director signs
   for the K.K.** That is fixed by the Japanese commercial register and is not
   a choice. If it differs from the identity chosen in (2), the assignment
   should name both and state the relationship, so the two signatures reconcile
   on the face of the document.

### The item that needs a specialist, not counsel-in-general

Two nationalities across a Japanese company and a Delaware company is a
**cross-border tax question before it is a corporate one.** If either identity
is a US person, ownership of a Japanese corporation carries a separate annual
information-return obligation and a controlled-foreign-corporation regime, both
independent of whether the K.K. ever distributes anything, and both with
penalties that do not depend on tax being owed. The assignment itself is a
transfer of intangible property out of a Japanese entity, which has its own
treatment on the Japanese side.

**None of that is resolved by a Delaware formation agent, and none of it is
resolved by this changeset.** It needs a US-Japan cross-border tax adviser, and
it should be asked before the assignment executes rather than at the first
filing deadline after it. The corporate steps in sections 1 to 6 are unaffected
and can be prepared in parallel.


---

## 1. `LICENSE`

Two lines. The BUSL parameter block is the operative text.

```diff
 Parameters

-Licensor:             RedRobot KK
+Licensor:             Uncanny Valley Inc.

 Licensed Work:        Replay
-                      The Licensed Work is (c) 2026 RedRobot KK
+                      The Licensed Work is (c) 2026 Uncanny Valley Inc.
```

Unchanged, deliberately: `Change Date: 2029-09-06`, `Change License: Apache
License, Version 2.0`, and the Additional Use Grant.

---

## 2. `NOTICE`

The trademark sentence is the one that needs judgement rather than a rename.
**"Replay" and "RedRobot" are different marks with different destinations.**

```diff
 Project Replay
-Copyright 2026 RedRobotKK
+Copyright 2026 Uncanny Valley Inc.

-This product includes software developed by RedRobotKK and contributors.
+This product includes software developed by Uncanny Valley Inc. and
+contributors. Portions copyright 2026 RedRobot KK, assigned.

 Licensed under the Business Source License 1.1 (SPDX: BUSL-1.1). See LICENSE.
 On the Change Date stated there, this version converts to the Apache License,
 Version 2.0.

-"Replay", "RedRobot" and the RedRobot logo are trademarks of RedRobot KK. The
-licence grants no rights in them; a fork must carry its own name.
+"Replay" is a trademark of Uncanny Valley Inc. "RedRobot" and the RedRobot
+logo remain trademarks of RedRobot KK. The licence grants no rights in any of
+them; a fork must carry its own name.
```

**The "portions copyright, assigned" line matters.** It preserves the true
history rather than rewriting it, which is the same reason the evidence files
keep their retractions. A reader who finds a 2026 commit attributed to the KK
should find the record saying so.

**Decision required:** if `RedRobot` is to be wound down, the mark moves too and
the second sentence collapses into the first. Counsel decides; this changeset
assumes the KK survives.

---

## 3. `CITATION.cff`

```diff
 authors:
   - given-names: Daniel
     family-names: Saito
-    affiliation: Red Robot K.K.
-    website: "https://redrobot.jp"
+    affiliation: Uncanny Valley Inc.
+    website: "https://replay.doctor"
```

`repository-code` changes only if the org moves. See the last section.

---

## 4. `docs/adr/0016-business-source-license.md`

The ADR records a decision that was true when made. **Do not edit the decision
table.** Append a superseding note instead, in the style ADR-0012 already uses
when it was reversed by ADR-0016.

```diff
 | Licensor | RedRobot KK |
```

stays exactly as written, and below the table:

```markdown
**Licensor assigned 2026-XX-XX.** Replay was assigned by RedRobot KK to
Uncanny Valley Inc. The licensor of record is now Uncanny Valley Inc.; the
parameters above are the terms as originally granted and are preserved because
every copy distributed before that date carries them. The Change Date is
unaffected: 2029-09-06 stands, and the conversion to Apache 2.0 with it.
```

---

## 5. Everything else that names the KK

Searchable, and to be swept in the same commit:

| File | What changes |
|---|---|
| `README.md` | the maintainer block, "founder of Red Robot K.K., Tokyo" |
| `FUNDING.md`, `SPONSORS.md` | entity references; funding rails are personal and may stay |
| `docs/SURFACES.md` | any entity reference in the data-handling table |
| `docs/MONEY-PATH.md` | the K.K. statutory cost line becomes a Delaware line |

**Do not sweep `docs/evidence/` or `docs/research/`.** Those files record what
was true on their date. Rewriting them would be falsifying a record, and the
repository's own standard forbids it.

---

## 6. The module path. A decision, not an edit.

`go.mod:1` is `module github.com/RedRobotKK/Replay`, and that path appears in
**233 Go files**.

**Option A: keep the path.** Assignment does not require renaming a GitHub org.
Zero breakage, zero import churn, and the path becomes a historical artifact
like any vendor prefix. Costs nothing technically; looks inconsistent with the
new entity.

**Option B: move to `github.com/UncannyValley/Replay`.** Clean identity, and it
breaks every import. GitHub redirects the URL but a Go module path is an
identity, not a URL: existing installs, `go install` lines, and anything
vendoring it are affected.

**The precedent is in your own CHANGELOG.** The Buffy-to-Replay rename changed
the module path and was done deliberately **before any tagged release, so no
published artifact was affected**. That option is gone. Releases are published
now.

**Recommendation: Option A at assignment, Option B only at a major version**, if
at all. The entity that owns the copyright and the string in the import path do
not have to match, and pretending otherwise costs every user a migration for a
cosmetic gain.

---

## Order of operations

1. Assignment executes. Counsel confirms the effective date.
2. Apply sections 1 to 5 in **one commit**, message naming the assignment and
   its date.
3. Module path decision recorded as an ADR whether or not it changes anything.
   A decision not to rename is still a decision and should be findable.
4. Trademark filings updated to reflect the new owner of "Replay".

## What needs an answer before this can be applied

**Blocking, from section 0:**

- Which single legal identity holds the founder shares in Uncanny Valley Inc.
  Everything else in the corporate file keys off it.
- Consolidate the K.K. register, or disclose the identity in diligence. Pick
  one before the $300k instrument is signed.
- Cross-border tax adviser engaged, and the US-person question answered, before
  the assignment executes.

**Non-blocking, from the sections above:**

- Does RedRobot KK survive, or wind down? Decides the NOTICE trademark sentence.
- Effective date of the assignment, for the ADR note.
- Module path: A or B.
- Do the funding rails (`buymeacoffee.com/saitodaniel`,
  `github.com/sponsors/saitodaniel`) stay personal or move to the company?
