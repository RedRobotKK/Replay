# Is a prospective N to N+1 prediction test justified by E005?

**Verdict: NO. The research stops at the retrospective observability result.**

Theory only. No experiment run, no E005 evidence reopened, no predictor tuned.

## 1. The protocol, designed so the reasoning is auditable

Designing it is what exposed the problem, so it is recorded rather than skipped.

**Observables at turn N**, available without the provider diagnostic: hash of the
tool-definition set, hash of the system prefix, elapsed time since the previous
request, lane and sub-agent structure, whether the client rewrote history.

**Outcome at N+1**, from provider usage only: cache-creation tokens against the
prior turn. Binary, does N+1 show a rebuild.

**Required baseline:** the structural-event base rate. `tools_changed` is 1,006
of 1,761 structural events, so a predictor that always answers with the majority
class is already strong. **Beating chance is not the bar; beating the base rate
is.**

**Metric:** held-out balanced accuracy on sessions not used to build the
predictor, with the diagnostic withheld throughout. Precision and recall
reported separately, because the classes are imbalanced and a single figure
would hide which error the predictor makes.

**Stopping rule:** fixed session split declared before any fit. No feature
search after seeing held-out performance.

**Exclusions:** `previous_message_not_found` and `unavailable` boundaries, which
E005 already classes as indeterminate.

**Prediction is not causal inference.** A predictor that works establishes that
information is present in the observables, not that the observables cause the
rebuild.

## 2. Why it is not justified

Four reasons, in descending strength.

### The transition is not observable before it occurs

The state at turn N does not contain turn N+1's tool set. **The thing that
causes a rebuild is the change, and the change happens between N and N+1.**
Predicting it from state at N requires a leading indicator, not the state
itself.

### Where a leading indicator exists, the prediction is already known

There is one: a `ToolSearch` call at turn N loads deferred tools, which changes
the tool set at N+1. That relationship is **mechanical and already documented**.
A predictor that learns it has rediscovered a known cause, not found new
information. The experiment would return a positive result that means nothing.

So the likely outcomes are: trivially positive where a known mechanism exists,
and structurally impossible where it does not. **Neither is informative.**

### There is no intervention point

Prediction earns its keep by enabling action before the cost is incurred. Here
the agent is already mid-execution; by the time turn N is observable, turn N+1
is determined by the client's own behaviour. **A warning with no available
action is a measurement, and Replay already provides the measurement
retrospectively and more accurately.**

### E005 does not motivate it

E005's finding is that materiality is class-dependent, which is a statement
about classification, not about forecasting. **Nothing in E005 indicates
prospective capability.** Moving from "we can say which past transitions
mattered" to "we can predict which future ones will" is a jump, not a
consequence, and this document declines to make it on the strength of an
interesting adjacency.

### The strongest argument for, stated fairly

If materiality could be predicted before a change lands, a developer could avoid
the costly minority of changes rather than discover them afterwards. That is
genuinely more valuable than retrospection. **It is also precisely the claim the
four objections above say cannot be reached from N-state observables.**

## 3. The question that would be worth asking, and is not this one

The useful prospective capability **already ships**. `replay prefix --before F
--after F` takes two versions of a tool-server document and reports whether the
change invalidates the cached prefix, exiting non-zero so CI can gate on it. It
answers "will this change invalidate" **before the change lands**, which is the
actionable moment.

E005's actual contribution to that surface is narrower and sharper than
prediction:

> `replay prefix` currently answers **invalidation**. E005 shows invalidation
> and **materiality** are different: three of four provider divergence classes
> show zero median cache-write movement. A gate that fires on every
> invalidation fires mostly on changes that cost nothing.

**That is a real gap, it is in a shipped surface, and it concerns a decision a
developer actually makes.** It is not a prediction problem.

Whether it is worth closing is a separate judgement and is not made here. It
would also require reopening E005 evidence to test, which this directive
forbids, so it is recorded as an open question rather than pursued.

## 4. Status

### ESTABLISHED BY E005

Unchanged. See `docs/evidence/detector-observables-2026-09-26.md`.

### SUPPORTED HYPOTHESIS

Replay occupies an independent measurement position on the economic arm.

### NOT YET TESTED, AND NOT SCHEDULED

Prospective prediction. **Declined on the reasoning above rather than deferred.**

### CLOSED

The prospective N to N+1 test is not preregistered and will not be run. **The
null result was declared acceptable in advance, and this is a stronger outcome
than a null: the experiment would not have been informative either way.**

Replay stands as a retrospective economic observability instrument. E005
supports that, and nothing here weakens it.
