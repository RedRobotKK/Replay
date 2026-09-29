# Pricing used by this campaign

**Provenance:** read from `api-docs.deepseek.com/quick_start/pricing` on
2026-09-29 and independently matched against `experiment/harness/pricing.py`
(table dated 2026-09-28). The two agree exactly.

USD per 1M tokens:

| model | regime | cache hit | cache miss | output |
| --- | --- | ---: | ---: | ---: |
| deepseek-flash | peak | 0.006 | 0.30 | 1.20 |
| deepseek-flash | off-peak | 0.003 | 0.15 | 0.60 |
| deepseek-v4-pro | peak | 0.044 | 1.32 | 3.96 |
| deepseek-v4-pro | off-peak | 0.022 | 0.66 | 1.98 |

**Peak** is 01:00-04:00 and 06:00-10:00 UTC, Monday to Friday, excluding Chinese
public holidays. Off-peak is half.

The holiday exclusion is **NOT_OBSERVED** by this campaign and is **ASSUMED**
from the documentation. `pricing.is_peak` does not model holidays and will
over-state cost on one, which is the safe direction and still wrong.

Every dollar figure in this campaign derived from this table is **DERIVED**, never
OBSERVED. The only OBSERVED cost source is `/user/balance`, reported to the cent.
