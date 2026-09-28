"""DeepSeek pricing, dated, with the time-of-day dimension the provider uses.

Read from the published table on 2026-09-28. Every figure produced here is
DERIVED: it is arithmetic over observed usage and a published rate, never a
provider-reported charge. /user/balance is the only OBSERVED cost source and its
resolution floor is near $0.01, which is coarser than most single requests.
"""
import datetime as dt

TABLE_DATE = "2026-09-28"
# USD per 1M tokens: (cache_hit, cache_miss, output)
PEAK = {"deepseek-flash": (0.006, 0.30, 1.20),
        "deepseek-v4-pro": (0.044, 1.32, 3.96)}
OFFPEAK = {"deepseek-flash": (0.003, 0.15, 0.60),
           "deepseek-v4-pro": (0.022, 0.66, 1.98)}

def is_peak(when_utc):
    """Peak is 01:00-04:00 and 06:00-10:00 UTC, Mon-Fri.

    Chinese public holidays are excluded from peak by the provider and are NOT
    modelled here, so this returns True on a holiday that is actually off-peak.
    It over-states cost, which is the safe direction, and it is wrong on those
    days. A dated holiday list would fix it.
    """
    if when_utc.weekday() >= 5:
        return False
    h = when_utc.hour + when_utc.minute / 60
    return (1 <= h < 4) or (6 <= h < 10)

def cost_usd(model, fresh_in, cache_read, out, when_utc=None):
    """DERIVED cost, or None when any input is NOT_MEASURED."""
    if fresh_in is None or out is None:
        return None
    when = when_utc or dt.datetime.now(dt.timezone.utc)
    table = PEAK if is_peak(when) else OFFPEAK
    key = next((k for k in table if k in (model or "")), None)
    if key is None:
        return None                     # unpriced model stays unpriced
    hit, miss, outp = table[key]
    return (fresh_in / 1e6 * miss + (cache_read or 0) / 1e6 * hit + out / 1e6 * outp)
