# Unit economics

The model is `scripts/unit-economics/model.py`, and the inputs it runs on are
`scripts/unit-economics/assumptions.json`. Run it:

```sh
python3 scripts/unit-economics/model.py          # the full report
python3 scripts/unit-economics/model.py --check  # exit 1 while any input is unknown
```

## Why this is code and not a spreadsheet

A spreadsheet lets you type a 3% churn rate into the cell beside a measured
7.94x and the two look identical on the page. A reader cannot tell which one
somebody observed and which one somebody wanted. Every input here carries a
provenance tag, and the report prints it beside the figure:

| Tag | Meaning |
|---|---|
| **M** | measured on this machine or in this repo, with the file cited |
| **D** | a decision we made — true by construction, not by observation |
| **A** | assumed from an adjacent category; **not** measured here |
| **?** | unknown; every figure depending on it is refused |

`--check` exits non-zero while any input is `?`. That is the same rule the rest
of this repository runs on: a model that always exits 0 cannot fail, and a
business plan whose unknowns are invisible is the same defect as a green test
that asserts nothing.

It is deliberately **not** wired into CI as a blocking job. It would be
permanently red, and a check that fails every day is a check somebody switches
off. It is the gate to run before quoting a forecast to anybody.

## What the model says today

Four inputs are unknown — `monthly_churn`, `free_to_paid_conversion`,
`repos_per_paying_account`, `cac_usd` — and all four for one reason: **nothing
has been sold and nobody outside this machine has run the tool.** So the report
prints scenario bands and refuses to call any of them a forecast.

What *is* measured is the value anchor, and it is the strongest part of the
model: cache-blind budget arithmetic runs **7.94x high** over 57,958 requests,
which strands roughly **$437/day** — about **$13,100/month** — of approved,
unusable budget under a $500/day ceiling. A $199 list price is **1.52%** of
that. The anchoring argument for pricing well above $25 is that one figure.

The measured funnel is the other half, and it is why nothing above is a
forecast: attributed installs per hour **0**, distinct install IPs **1**,
external stars **0**.

## What would make it real, in order

1. **one stranger runs it** — `free_to_paid_conversion` becomes definable
2. **ten strangers run it** — conversion gets a denominator
3. **one repository pays** — `cac_usd` becomes measurable
4. **three months of that cohort** — `monthly_churn` becomes real

Update `assumptions.json` as each lands, flipping the provenance tag and citing
the source. When the last `?` is gone, `--check` passes and the figures may be
quoted. Not before.

## A note on two LTV numbers

Prose in this project has quoted LTV both on revenue (`price / churn`) and on
contribution (`price x margin / churn`). The model uses **contribution**, which
is the smaller and the more honest of the two, and states the margin as assumed.
Where an older figure disagrees, the model is authoritative.

---

[Documentation index](README.md) · [Repository README](../README.md)
