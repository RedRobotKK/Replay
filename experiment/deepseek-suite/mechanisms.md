# Mechanism synthesis

No scalar confidence scores. Evidence status only.

## M1 Block-quantised prefix cache with the final block withheld

**Observation.** Cached tokens equal `128 * max(0, floor(n/128) - 1)` across 35
rungs. A prefix under 256 tokens caches nothing.

**Candidate mechanism.** The server stores KV state in fixed blocks and commits
only complete ones, so the trailing partial block is never reusable. The
minus-one term is then not a policy but an artifact of never having a complete
final block for a prefix that ends mid-block.

**Supporting.** Exactness across 35 rungs; every cached count a multiple of 128;
prospective prediction at boundaries chosen to discriminate.

**Alternative explanation.** A deliberate one-block safety margin would produce
identical numbers. The two are indistinguishable from cached counts alone.

**Discriminating experiment.** Construct a prefix whose length is an exact
multiple of 128 **and** which ends on a block boundary in the server's own
tokenisation. Under "partial block" the final block is then complete and should
be served; under "deliberate margin" it should still be withheld. Requires
control over tokenisation we do not currently have.

**Status.** Mechanism NOT_OBSERVED. The formula is SUPPORTED.

**Prior art.** vLLM documents caching only full blocks, with a default block of
16 and an explicit note that a 17-token message may not cache its last token.
The measured behaviour is structurally the same with a different block size, so
this is a measurement of one deployment, not a new mechanism.

## M2 Completion-dependent population

**Observation.** Sequential B hit 3200; concurrent B hit 0, with B provably
dispatched before A returned.

**Candidate mechanism.** The cache entry is committed on request completion, so
requests in flight together cannot see each other's prefix.

**Supporting.** A4 with a wall-clock timeline rather than code ordering; the
earlier arms run independently showed a 0% hit rate for an all-parallel arm at
0.99x the cost of the sequential one.

**Alternative explanation.** Commit could occur at first-token rather than
completion, which would produce the same result at these latencies. Not
separated.

**Discriminating experiment.** Stagger B's dispatch across A's response window
in steps and find the point where the hit appears. If it appears near A's first
token rather than near A's completion, commit is earlier than completion.

**Status.** "Reuse requires the populating request to have completed" is
SUPPORTED. The exact commit point is NOT_OBSERVED.

**Prior art.** The concurrent-miss race and the warm-then-fan remedy are
published, with the fix stated as a single synchronous call before dispatching
the parallel batch. This is a rediscovery with a measurement attached.

## M3 `reasoning_effort` participates in cache identity

**Observation.** Toggling `reasoning_effort` in either direction dropped reuse
to 0 against 3200 controls. An unknown junk key did not. Sampling parameters did
not. Cross-dialect reuse occurs only when the parameter is absent from both
bodies.

**Candidate mechanism.** `reasoning_effort` alters the prompt the server
assembles, for example by inserting or removing a thinking directive, changing
the token stream and therefore the prefix.

**Supporting.** The junk-key arm rules out "the cache key is the raw request
body", because an ignored key would break reuse under that model and did not.

**Alternative explanation.** The parameter is a genuine component of the cache
key, separate from the prompt, while unknown keys are stripped before keying.
A10 does not separate this from the serialisation account.

**Discriminating experiment.** Send `reasoning_effort` with a value that is
semantically a no-op against the populating call's implied setting. If reuse
survives, the key tracks the resulting prompt; if it breaks, the key tracks the
parameter's presence.

**Status.** The effect is SUPPORTED. The mechanism is NOT_OBSERVED.

**Operational consequence.** Mixing reasoning-on and reasoning-off calls over one
shared prefix maintains two separate cache populations. This compounds with C17:
the reasoning-off lever is worth about 1.5x on a cached workload, and toggling it
mid-workload costs a full prefix repopulation.

## M4 Provider cost is reconstructible from response usage alone

**Observation.** 484 responses, four conservation identities, zero violations,
two independent reconstruction paths agreeing to nine decimals, landing inside
the band the balance can distinguish.

**Candidate mechanism.** Usage fields are complete and the published rate card
is applied as written, so the response body is sufficient evidence of cost.

**Alternative explanation.** Agreement within a $0.02 band across a $0.25 total
is a weak constraint. A systematic error under about 8% would not be visible.

**Discriminating experiment.** A workload an order of magnitude larger, so the
band is a smaller fraction of the total.

**Status.** SUPPORTED as a lower bound, provisional on settlement. Precision is
limited by the balance endpoint, not by the reconstruction.
