#!/usr/bin/env bash
# The installer quotes the calibration corpus. Check it still quotes it correctly.
#
# install.sh closes by telling every new user what the tool was calibrated
# against. Those figures come from the newest docs/evidence/calibration-corpus-*.md,
# and nothing connected the two: the document is regenerated when the corpus is
# re-measured, and the installer line is edited by hand, if anyone remembers.
#
# On 2026-09-08 the line quoted the 2026-09-06 document exactly and carried no
# date. It was not wrong. But a corpus on one laptop grows every day its owner
# works, and the same machine already reported more of both, so undated the
# sentence reads as a standing property of the tool rather than as a snapshot
# and a reader has no way to tell which they are being given.
#
# Figures are deliberately absent from this comment. Stating the newer ones here
# to explain the problem is what tripped TestFrozenFD8 when it was tried in
# install.sh: a number backed by no evidence document, which is the defect this
# whole area exists to prevent.
#
# So this checks two things: that the figures match the newest published corpus
# document, and that the line carries that document's date. Either can drift on
# its own and neither is visible by reading one file.
set -euo pipefail

cd "$(dirname "$0")/.."

newest="$(ls -1 docs/evidence/calibration-corpus-*.md 2>/dev/null | sort | tail -1)"
[ -n "$newest" ] || { echo "no calibration-corpus document found" >&2; exit 2; }

date_from_name="$(basename "$newest" .md | sed 's/^calibration-corpus-//')"

# The document's own summary line: "across N transcripts, from M distinct sessions".
line="$(grep -m1 -o 'across [0-9,]* transcripts, from [0-9,]* distinct sessions' "$newest" || true)"
[ -n "$line" ] || { echo "could not read the figures out of $newest" >&2; exit 2; }

doc_transcripts="$(printf '%s' "$line" | sed 's/^across \([0-9,]*\) transcripts.*/\1/' | tr -d ,)"
doc_sessions="$(printf '%s' "$line" | sed 's/.*from \([0-9,]*\) distinct sessions$/\1/' | tr -d ,)"

# The installer's claim.
ins="$(grep -m1 -o 'Calibrated against [0-9,]* sessions\\n*across [0-9,]* transcripts' install.sh || true)"
[ -n "$ins" ] || { echo "install.sh no longer states a corpus figure in the expected shape" >&2; exit 2; }

ins_sessions="$(printf '%s' "$ins" | sed 's/^Calibrated against \([0-9,]*\) sessions.*/\1/' | tr -d ,)"
ins_transcripts="$(printf '%s' "$ins" | sed 's/.*across \([0-9,]*\) transcripts$/\1/' | tr -d ,)"

fail=0
if [ "$ins_sessions" != "$doc_sessions" ] || [ "$ins_transcripts" != "$doc_transcripts" ]; then
  echo "install.sh states ${ins_sessions} sessions / ${ins_transcripts} transcripts;" >&2
  echo "  ${newest} says ${doc_sessions} sessions / ${doc_transcripts} transcripts." >&2
  fail=1
fi

# The date has to be there, and it has to be the document's.
if ! grep -q "as of ${date_from_name}" install.sh; then
  echo "install.sh does not date its corpus figures 'as of ${date_from_name}'." >&2
  echo "  An undated snapshot reads as a standing property of the tool." >&2
  fail=1
fi

[ "$fail" -eq 0 ] || { echo "corpus-figures-check: FAILED" >&2; exit 1; }
echo "corpus-figures-check: ok (${doc_sessions} sessions / ${doc_transcripts} transcripts, as of ${date_from_name})"
