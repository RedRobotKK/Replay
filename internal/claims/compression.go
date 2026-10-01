package claims

// The Epistemic Compression Matrix.
//
// One row per site where evidence distinctions converge before reaching a
// user-visible claim. The campaign that produced it started from a confirmed
// pair in the cost path and asked whether the same shape exists elsewhere.
//
// The question a row answers is NOT "does information disappear here". Almost
// every aggregation discards something and most of it is correct to discard.
// The question is whether a distinction the CONTRACT requires to survive has
// been lost on the way to a claim. So Intentional is a field, and a site is
// only a defect once the contract has been established and found to require
// what the site does not preserve.

// Compression classifies one convergence site.
type Compression string

const (
	// Lossless: every distinction the contract requires survives.
	Lossless Compression = "LOSSLESS"
	// IntentionallyCollapsed: distinctions are lost and the contract says so,
	// usually with the reason in a comment at the site.
	IntentionallyCollapsed Compression = "INTENTIONALLY_COLLAPSED"
	// UnintentionallyCollapsed: the contract requires a distinction that the
	// site does not preserve. This is the only value that names a defect.
	UnintentionallyCollapsed Compression = "UNINTENTIONALLY_COLLAPSED"
	// CompressionUnmeasured: a candidate that was found and not attacked.
	CompressionUnmeasured Compression = "UNMEASURED"
	// CompressionNoEndpoint: no user-visible claim depends on this site, so
	// there is nothing for a collapse to damage.
	CompressionNoEndpoint Compression = "NO_ENDPOINT"
	// CompressionNotApplicable: inspected and found not to be a convergence.
	CompressionNotApplicable Compression = "NOT_APPLICABLE"
)

// Site is one row of the matrix.
type Site struct {
	// ID is stable so a row can be cited from a commit or an evidence file.
	ID string
	// Where is file:line.
	Where string
	// InputDistinctions are the states present in the evidence arriving here.
	InputDistinctions []string
	// ProductionRepr is how the implementation represents them internally.
	ProductionRepr string
	// UserVisibleRepr is what a reader of the output can tell apart.
	UserVisibleRepr string
	// OracleDistinctions are the states an independent oracle needs.
	OracleDistinctions []string
	// Lossy records whether anything is lost at all, before asking whether
	// that matters.
	Lossy bool
	// Intentional records whether the contract declares the loss. Checked at
	// the site, not assumed from the fact that it compiles.
	Intentional bool
	// ContractSays is the evidence for Intentional: a quoted comment, an ADR,
	// or the absence of either.
	ContractSays string
	// Claim is the registered claim this site governs, if any.
	Claim string
	// Positive and Negative are the controls, where the site was attacked.
	Positive string
	Negative string
	// Result is the classification.
	Result Compression
	// Why is required.
	Why string
}

// Matrix is the ledger. Rows marked UNMEASURED were found by the discovery
// sweep and have NOT been attacked; they are listed so the sweep's output is
// auditable rather than summarised away.
var Matrix = []Site{
	{
		ID:    "EC-01",
		Where: "internal/transcript/wire.go:134 (*WireUsage).Usage()",
		InputDistinctions: []string{
			"provider sent no TTL breakdown (CacheBreak == nil)",
			"provider sent a breakdown of 0/0",
			"provider sent an explicit 5m or 1h split",
		},
		ProductionRepr:     "transcript.Usage{Create5m, Create1h int} with NO companion presence field",
		UserVisibleRepr:    "a dollar figure. The first two states are identical by the time anything prices them",
		OracleDistinctions: []string{"OBSERVED split", "ABSENT split", "ZERO split"},
		Lossy:              true,
		Intentional:        true,
		ContractSays: "documented at both sites. TTLOf: \"Without a breakdown the provider default applies\". " +
			"writeEquivalent: \"or at the short multiplier when no split was reported\"",
		Claim:    "RPL-C033",
		Positive: "an explicit 5m split and an explicit 1h split price differently, so the distinction survives where the provider supplies it",
		Negative: "absent and present-0/0 are byte-identical downstream, confirmed by EC1",
		Result:   IntentionallyCollapsed,
		Why:      "The collapse is declared in code at both sites, so it is not a silent bug. What is NOT declared anywhere a reader can see is that an absent breakdown is priced on an ASSUMED short TTL: the same 10,000 cache-creation tokens price at $0.037500 assumed-short and $0.060000 if the provider says 1h, a 60% difference decided by a field the provider may simply not have sent.",
	},
	{
		ID:                 "EC-02",
		Where:              "internal/analysis/replay.go:65 Tally.AddAt",
		InputDistinctions:  []string{"request priced", "request unpriceable"},
		ProductionRepr:     "t.Requests++ unconditionally; the price branch adds dollars only on success, with NO counter on failure",
		UserVisibleRepr:    "costUnit.Requests counts ALL requests beside a CostUSD covering only the priced ones",
		OracleDistinctions: []string{"OBSERVED", "UNKNOWN", "PARTIAL"},
		Lossy:              true,
		Intentional:        false,
		ContractSays:       "nothing at the site. The contrast is the evidence: burn.go:582 and costusage.go:157 BOTH keep per-record priced/unpriced counters, costusage.go:160 saying outright that \"an unpriced record sits inside a session that is otherwise priced and would contribute a silent zero\". The failure mode was identified and fixed in two paths and not in the one `replay cost` uses",
		Claim:              "RPL-C031",
		Positive:           "",
		Negative:           "",
		Result:             CompressionUnmeasured,
		Why:                "This is the ROOT of RPL-C031 rather than a separate finding: the session-granularity counter in cost.go cannot see a record-level exclusion because AddAt never records one. Found by the sweep, not yet attacked with its own oracle.",
	},
	{
		ID:                 "EC-03",
		Where:              "cmd/replay/route.go:281 `if !okF || !okT { continue }`",
		InputDistinctions:  []string{"turn priceable on both models", "turn unpriceable on either"},
		ProductionRepr:     "skipped with no counter and no comment",
		UserVisibleRepr:    "a per-turn saving whose numerator covers priced turns and whose denominator is c.total, every turn",
		OracleDistinctions: []string{"OBSERVED", "PARTIAL"},
		Lossy:              true,
		Intentional:        false,
		ContractSays:       "nothing, and the function discloses carefully elsewhere: the TTL-split comment sits immediately above this skip",
		Result:             CompressionUnmeasured,
		Why:                "A granularity mismatch that dilutes the per-turn figure whenever any turn is skipped. Candidate, not attacked.",
	},
	{
		ID:                 "EC-04",
		Where:              "internal/analysis/trim.go:148 `model := \"claude-opus-5\"`",
		InputDistinctions:  []string{"session names a model", "session names none"},
		ProductionRepr:     "a default model id is substituted",
		UserVisibleRepr:    "a dollar estimate priced against a model the session never used",
		OracleDistinctions: []string{"OBSERVED", "NO_ENDPOINT"},
		Lossy:              true,
		Intentional:        false,
		ContractSays:       "the repository argues AGAINST this at burn.go:355: \"A guessed model is a wrong figure with no way for a reader to see it is wrong\"",
		Result:             CompressionUnmeasured,
		Why:                "Directly contradicts a rule the codebase states elsewhere in its own words. Candidate, not attacked.",
	},
	{
		ID:                 "EC-05",
		Where:              "internal/transcript/types.go:164 Request.IDFromMessage",
		InputDistinctions:  []string{"id is the provider's request id", "id is the provider's message id"},
		ProductionRepr:     "a bool set once at claudecode.go:336",
		UserVisibleRepr:    "nothing. The field has no read site anywhere in non-test code",
		OracleDistinctions: []string{"provenance of the identifier"},
		Lossy:              true,
		Intentional:        false,
		ContractSays:       "the field's own doc says it exists so \"a consumer that needs the request id specifically can tell that it does not have one\". No consumer does",
		Result:             CompressionUnmeasured,
		Why:                "A dead provenance field: written, never read. Not a collapse of two states into one so much as a distinction recorded and then ignored. Candidate.",
	},
	{
		ID:                 "EC-06",
		Where:              "internal/analysis/diff.go:51",
		InputDistinctions:  []string{"lane-serial", "lane-overlap", "unmeasured"},
		ProductionRepr:     "lane-overlap is converted to cachemodel.CauseNotMeasured",
		UserVisibleRepr:    "a count of causes-not-measured, indistinguishable from any other not-measured cause",
		OracleDistinctions: []string{"OBSERVED serial", "OBSERVED overlap", "UNMEASURED"},
		Lossy:              true,
		Intentional:        true,
		ContractSays:       "the Correlation constants document that overlap means per-event attribution is NOT MEASURED rather than guessed, which is exactly what the conversion encodes",
		Result:             IntentionallyCollapsed,
		Why:                "The collapse is the contract: overlap genuinely cannot be attributed, so folding it into not-measured is correct. What is lost is WHY it was not measured, and the raw three-state value survives verbatim in exactly one place, the proxy's live log line.",
	},
	{
		ID:                 "EC-07",
		Where:              "cmd/replay/cost.go:932 `if !*asJSON`",
		InputDistinctions:  []string{"this machine also runs other agent surfaces", "it does not"},
		ProductionRepr:     "otherSurfacesNote computed, then gated off the JSON path",
		UserVisibleRepr:    "human readers are told spend exists outside the figure; JSON consumers are not",
		OracleDistinctions: []string{"PARTIAL coverage", "COMPLETE coverage"},
		Lossy:              true,
		Intentional:        true,
		ContractSays:       "the gate is explicit and deliberate in code",
		Result:             IntentionallyCollapsed,
		Why:                "Deliberate, and its consequence is that the machine-readable surface cannot tell a partial corpus from a complete one. Recorded because intent at the site does not settle whether the omission is right at the boundary.",
	},
	{
		ID:                 "EC-08",
		Where:              "internal/advisor, internal/learn/graduate.go, internal/analysis/payback.go",
		InputDistinctions:  []string{"an action was observed", "an outcome was measured"},
		ProductionRepr:     "Status and Decision are separate types; graduate.go requires an independently computed outcome before a saving can graduate",
		UserVisibleRepr:    "both survive distinctly",
		OracleDistinctions: []string{"ACTION", "OUTCOME"},
		Lossy:              false,
		Intentional:        false,
		ContractSays:       "advisor.go:90 narrates the 2026-09-23 fix: the two were one field, \"and that is precisely how a status the verifier had inferred came back as a human decision\"",
		Result:             Lossless,
		Why:                "A clean negative result. The action-as-outcome collapse the sweep went looking for is not present: it existed, it was found, and the fix holds. Worth recording precisely because the campaign should report where the boundary holds.",
	},
}
