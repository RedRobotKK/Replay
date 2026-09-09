# tui-audit

Renders every TUI screen and checks it for layout artifacts.

```sh
scripts/tui-audit/run.sh
```

9 screens × 2 locales × 4 widths = 72 renders, plus 9 colour checks.

## What it checks

- **Overflow** — no rendered line wider than the terminal it claims to fit,
  measured East-Asian-aware, because Ambiguous characters are one cell in
  `en_US` and two in `ja_JP` and this ships to both.
- **Palette** — every SGR emitted is one of the seven codes `color.go` defines.
- **Leakage** — every sequence opened is closed, so colour never crosses a row.
- **Control characters and trailing whitespace.**

## What passes today

Clean at 80, 120 and 200 columns in both locales. No overflow, no palette drift,
no unclosed colour, no control characters.

## What is frozen rather than fixed

**18 overflows below 80 columns**, and the count is pinned so it cannot quietly
grow. The layout is a constant, not a measurement: `shortcuts.go` declares
`BudgetCols = 80` and nothing reads `COLUMNS` or calls `TIOCGWINSZ`.

This is an unimplemented design rather than an unknown defect.
`storyboard.go` scene 25, *"Terminal narrower than 80"*, already specifies the
remedy — *"columns dropped in a fixed order: wire, endpoint, surface"* — into an
8/9/9 layout. Implementing it should drive the count to zero, and this check
will say so and ask for the frozen number to be lowered.

Shown to fail: `BudgetCols` 80 → 96 produces 16 overflows at the 80-column gate
and exits 1.
