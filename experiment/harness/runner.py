"""SUPERSEDED by session.py. Do not import this module.

`run_arm` orchestrated arms before the levers were measured. It carries two
defects that were found and fixed elsewhere, and it is imported by nothing:

  - `ex.map` re-raises the first exception its result iterator reaches and
    discards every later result, so one connection reset threw away calls
    already paid for.
  - It had no budget at all. Nothing stopped a run from spending whatever the
    trial count implied.

It also predates policy.py, so it chose no reasoning setting, placed the
variable block wherever the caller asked, and never checked the undocumented
disable parameter.

Use `session.Run`, which enforces all of that by construction. This file stays
as a signpost rather than being deleted, because the failure mode is a future
session finding a plausible-looking runner and using it.
"""


def run_arm(*_a, **_kw):
    raise NotImplementedError(
        "runner.run_arm is superseded by session.Run. It had no spending "
        "ceiling and used ThreadPoolExecutor.map, which discards paid-for "
        "results when any call raises. See session.py.")


def summarise(*_a, **_kw):
    raise NotImplementedError("runner.summarise is superseded by session.Run.summary()")
