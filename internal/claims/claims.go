package claims

// Register is the claim surface. Discovered from README, CLI help, ADRs,
// docs/ and the code itself, not supplied by hand from marketing copy.
//
// Ordering is by subject, not by confidence. A DELIBERATE_NON_CLAIM sits
// beside an ESTABLISHED one on purpose: the decision not to assert something
// is part of the claim surface and is as load-bearing as an assertion.
var Register = []Claim{
	// ---------------------------------------------------------- measurement
	{
		ID:       "RPL-C001",
		Text:     "Replay reads agent transcripts already on disk and attributes prompt-cache behaviour per turn, without provider cooperation.",
		Asserted: []string{"README.md:13", "cmd/replay/main.go:639"},
		Scope:    "Claude Code, Codex, Grok, Ollama transcripts on local disk. Not Cursor, AnythingLLM or OpenClaw, which have no readable usage field.",
		Establishes: []string{
			"a per-turn record can be reconstructed from local artifacts alone",
		},
		DoesNotEstablish: []string{
			"that the reconstruction matches the provider's own accounting",
			"that any surface other than the four named ones can be read",
			"anything about surfaces whose usage field is absent or always zero",
		},
		Vocabulary: "surface.Class",
		Oracle:     "fixture corpora under internal/surface/testdata classified by an audit performed independently of the classifier",
		Tests:      []string{"TestFixtureCorporaClassifyAsTheAuditFound"},
		Result:     Bounded,
		Why:        "Holds for four surfaces and is explicitly refused for three others. The bound is the claim.",
	},
	{
		ID:          "RPL-C004",
		Text:        "Every figure Replay prints carries a truth tier: measured, estimated or structural.",
		Asserted:    []string{"README.md:293", "docs/adr/0002-replay-engine-and-truth-tiers.md:9"},
		Scope:       "printed output of the CLI",
		Establishes: []string{"a reader can tell how a number was obtained"},
		DoesNotEstablish: []string{
			"that a 'measured' figure is a billed figure",
			"that the tier is correct, only that one is present",
		},
		Vocabulary: "",
		Oracle:     "partial. The price basis is shown to declare its own provenance and age; exhaustive per-figure tier coverage is NOT established",
		Tests:      []string{"TestC011_ThePriceBasisDeclaresItsOwnProvenanceAndAge"},
		Result:     Unresolved,
		Why:        "This campaign established that the price basis declares its version, its check date and its staleness. It did NOT enumerate every printed figure and confirm each carries a tier. The claim is plausible and untested at the level it is stated.",
	},

	// ------------------------------------------------- the billing boundary
	{
		ID:       "RPL-C011",
		Text:     "The 'measured' truth tier means the provider reported the USAGE, not that the provider reported the CHARGE.",
		Asserted: []string{"README.md:299"},
		Scope:    "all dollar figures, including those obtained through replay serve",
		Establishes: []string{
			"token counts in the measured tier came from the provider's own counters",
		},
		DoesNotEstablish: []string{
			"that the dollar figure beside them is what the provider billed",
			"that the price table matched the account's negotiated rate",
			"that the figure is reconcilable with an invoice",
		},
		Vocabulary: "",
		Oracle:     "independent reimplementation of the cost arithmetic in the test, not an import of CostLegsUSD",
		Tests: []string{
			"TestC011_IndependentArithmeticAgreesWithProduction",
			"TestC011_MeasuredUsageTimesLocalPriceIsNotABill",
			"TestC011_ThePriceBasisDeclaresItsOwnProvenanceAndAge",
			"TestC012_NoProviderBillingEndpointExists",
		},
		Result: Bounded,
		Why:    "Usage is provider-reported. Price is a local table. Their product is reconstructed, and the word 'measured' covers only the first factor.",
	},
	{
		ID:          "RPL-C012",
		Text:        "Replay can establish what a provider actually billed.",
		Asserted:    nil,
		Scope:       "all providers",
		Establishes: nil,
		DoesNotEstablish: []string{
			"anything. No code path reaches a provider invoice, balance or usage-report endpoint",
		},
		Vocabulary: "",
		Oracle:     "static scan of every outbound HTTP destination in non-test code",
		Tests:      []string{"TestC012_NoProviderBillingEndpointExists"},
		Result:     NoEndpoint,
		Why:        "Structural, not incidental. Every dollar figure in the repository is usage times a locally held price table. The only second source, LiteLLM, is documented in-code as an observer and not an authority.",
	},
	{
		ID:          "RPL-C024",
		Text:        "Attributed prompt tokens sum back to the provider's reported prompt total, with any excess carried in a named re-billed row rather than absorbed.",
		Asserted:    []string{"internal/analysis/conservation_test.go:12"},
		Scope:       "blame and reconcile output",
		Establishes: []string{"no token is silently dropped or double-counted in attribution"},
		DoesNotEstablish: []string{
			"that the split between buckets is correct. Conservation pins the total and says nothing about the division",
		},
		Vocabulary: "",
		Oracle:     "hand-computed totals in the conservation fixtures",
		Tests:      []string{"TestConserve_EveryBilledTokenLandsInExactlyOneBucket", "TestConserve_EachLaneConservesIndependently"},
		Result:     Bounded,
		Why:        "A necessary condition that the repository's own tests already state is not sufficient.",
	},

	// ------------------------------------------------------------- refusals
	{
		ID:               "RPL-C005",
		Text:             "Anything Replay cannot measure it declines to print, and says why in the place the number would have gone.",
		Asserted:         []string{"README.md:21"},
		Scope:            "all printed figures",
		Establishes:      []string{"absence of a figure is distinguishable from a figure of zero"},
		DoesNotEstablish: []string{"that every refusal is correct, only that refusals are reachable and distinguishable"},
		Vocabulary:       "surface.Class, surface.WriteContract",
		Oracle:           "scripts/refusal-reachability neutralises each refusal and requires the suite to notice",
		Tests:            []string{"TestC005_RefusalIsDistinguishableFromZero"},
		Result:           Bounded,
		Why:              "Reachability of refusals is machine-checked. Correctness of each refusal is per-site and is not established globally.",
	},
	{
		ID:               "RPL-C008",
		Text:             "Absence, zero and unknown are three different values, and Replay never collapses them.",
		Asserted:         []string{"README.md:603", "docs/adr/0018-provenance-is-a-field.md"},
		Scope:            "cache counters on every surface",
		Establishes:      []string{"a missing counter is reported as missing, not as zero"},
		DoesNotEstablish: []string{"that a reported zero is a true zero. That depends on the provider's contract, which may be unknown"},
		Vocabulary:       "surface.WriteContract, surface.Observables.WriteFieldPresent",
		Oracle:           "fixtures that omit a field versus fixtures that set it to zero, compared on output",
		Tests:            []string{"TestC008_AbsentFieldAndZeroFieldDiffer"},
		Result:           Established,
		Why:              "Positive, negative and mutation controls all discriminate. WriteFieldPresent and PrefixMeasured exist for exactly this.",
	},

	// ------------------------------------------------- correlation and joins
	{
		ID:          "RPL-C020",
		Text:        "Records from two different sessions are never joined to each other.",
		Asserted:    []string{"cmd/replay/requestjoin_test.go:16"},
		Scope:       "cross-file and cross-session joins",
		Establishes: []string{"a locally synthesised id is never used as a join key"},
		DoesNotEstablish: []string{
			"that a provider id collision across accounts would be caught",
			"that a true cross-session body mismatch is rejected. No test constructs session A's request paired with session B's response; the coverage is id-collision and arrival-order only. NAMED GAP",
		},
		Vocabulary: "",
		Oracle:     "two independently constructed sessions, cross-wired by hand in the fixture",
		Tests:      []string{"TestRJ1_SynthesisedIDsAreNotJoinedAcrossFiles", "TestRJ2_ProviderIDsStillJoin"},
		Result:     Bounded,
		Why:        "Synthesised-id collision is covered and provider ids still join. A true cross-session body mismatch remains UNTESTED and is recorded as a named gap rather than quietly closed.",
	},
	{
		ID:               "RPL-C021",
		Text:             "Replay offers no cross-surface grand total.",
		Asserted:         []string{"cmd/replay/burn.go:20"},
		Scope:            "replay burn",
		Establishes:      []string{"figures with incommensurable units are not added"},
		DoesNotEstablish: []string{"that per-surface figures are themselves comparable"},
		Vocabulary:       "",
		Oracle:           "type-level: the summary type exposes no addition",
		Tests:            []string{"TestC021_NoCrossSurfaceTotalIsReachable"},
		Result:           Established,
		Why:              "Enforced by the shape of the type rather than by a convention, and a mutation that adds a total is caught.",
	},

	// ------------------------------------------- the deliberate non-claims
	{
		ID:               "RPL-C013",
		Text:             "Replay establishes realized savings.",
		Asserted:         nil,
		Scope:            "all output",
		Establishes:      nil,
		DoesNotEstablish: []string{"anything. The product explicitly refuses to forecast or claim savings"},
		Vocabulary:       "",
		Oracle:           "scan of user-facing strings for savings and forecast vocabulary",
		Tests:            []string{"TestC013_NoSavingsClaimReachesTheUser"},
		Result:           DeliberateNonClaim,
		Why:              "README:523 and the agent skill both forbid it. Payback computes a projection from two reconstructed costs under an unverifiable counterfactual, and labels it an assumption.",
	},
	{
		ID:          "RPL-C015",
		Text:        "Replay can establish that acting on its advice changed anything.",
		Asserted:    nil,
		Scope:       "advisor findings",
		Establishes: nil,
		DoesNotEstablish: []string{
			"outcome for KindCacheBreaks or KindHotFile, the two kinds carrying the largest predicted value",
		},
		Vocabulary: "advisor.Status",
		Oracle:     "the refusal is in the product: track returns AdviceOnly before any scoring runs",
		Tests:      []string{"TestC015_TheTwoLargestKindsAreRefusedScoring", "TestC015_VerifierInferenceIsNotAReaderDecision"},
		Result:     NotMeasured,
		Why:        "The verifier refuses by design, because a share that moves with the work cannot be attributed to a user action. The seeded-intervention pilot then failed its gating question.",
	},
	{
		ID:               "RPL-C016",
		Text:             "Replay improves agent task outcomes.",
		Asserted:         nil,
		Scope:            "durable work state",
		Establishes:      nil,
		DoesNotEstablish: []string{"anything in either direction"},
		Vocabulary:       "",
		Oracle:           "R10 hidden-oracle benchmark",
		Tests:            []string{"TestC016_NoTaskImprovementClaimIsAsserted"},
		Result:           NotMeasured,
		Why:              "R1 established retrieval, not outcome. The R10 trial on 2026-09-30 stopped at a preregistered ceiling and the treatment arm never ran.",
	},

	// -------------------------------------------------------- the footprint
	{
		ID:               "RPL-C022",
		Text:             "The binary makes no network request except for commands the user explicitly types.",
		Asserted:         []string{"README.md:407", "plugins/replay/skills/replay-doctor/SKILL.md:9"},
		Scope:            "the shipped binary",
		Establishes:      []string{"no telemetry, no beacon, no first-run call"},
		DoesNotEstablish: []string{"anything about the hosted installer, which is checked separately"},
		Vocabulary:       "",
		Oracle:           "static enumeration of outbound destinations in non-test code",
		Tests:            []string{"TestC022_OutboundDestinationsAreAnEnumeratedSet"},
		Result:           Bounded,
		Why:              "Checkable statically. A dynamic proof would need a sandboxed run and is not attempted here.",
	},
}
