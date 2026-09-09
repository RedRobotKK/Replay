# replay: triage a finding, on a terminal that uses the screen it was given.
#
# Rendered with `vhs docs/demo/triage.tape`, which writes docs/demo/triage.gif.
#
# Two things this exists to show, both of which landed on 2026-09-09:
#
#   1. The advise screen is a triage tool. j/k moves a cursor, enter opens the
#      evidence, `a` marks a finding applied and it persists to the advice file
#      `replay advise` already keeps.
#   2. It sizes itself to the terminal. This tape runs at 40 rows deliberately;
#      the same screen on 24 shows fewer findings and still fits, because the
#      height is now read rather than assumed.
#
# The corpus is this machine's own. Nothing here is a mock, which is why the
# figures move between recordings — 1,691 transcripts to 1,738 in one day.

Output docs/demo/triage.gif

Set Shell "bash"
Set FontSize 15
Set Width 1100
Set Height 720
Set Padding 18
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
# A real terminal size, and the point of the recording: 40 rows, not 24.
Set WindowBar Colorful

# The binary built from this tree, not whatever is on PATH. The first
# recording silently used an installed v0.5.4 and showed the cost screen with
# "0 transcripts" — a demo of a release that predates everything it was meant
# to show. A tape that does not pin its binary records the wrong software.
Type "./docs/demo/.replay-demo-bin tui --screen advise"
Sleep 400ms
Enter
# The screen reads from advice.json, so it arrives in milliseconds rather than
# the 7.7 seconds it took when every keypress re-analysed 1,738 transcripts.
Sleep 2s

# Move through the findings. The cursor is visible and the columns do not
# shift as it moves — the marker is two cells wide on every row.
Type "j"
Sleep 900ms
Type "j"
Sleep 900ms
Type "k"
Sleep 900ms

# Open the evidence. This key was advertised on the screen for an hour with
# nothing behind it; now it answers which transcripts, how much, and how the
# saving was derived.
Enter
Sleep 2500ms

# Back to the list, and mark the finding applied. The decision is written
# through a sibling and renamed, so a half-written file cannot lose the
# decisions already made.
Escape
Sleep 1200ms
Type "a"
Sleep 1800ms

Type "q"
Sleep 600ms
