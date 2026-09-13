package cachemodel

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// PricingDigest names the numbers this build actually prices with.
//
// WHY THIS IS NOT RulesVersion.
//
// RulesVersion names the provider's published rule document. On 2026-09-12 two
// builds read the same transcript directory on the same machine and reported
// $4,088.49 and $11,969.37, both stamped "anthropic-2026-09-01" (#284). The
// label was not lying: the provider's document had not changed. What changed
// was this code. Six models the older build declined to price became priced,
// and the unknown-model read multiple became a named rule and moved (#271,
// #253). A pool that added those two submissions together would be summing
// figures produced by different arithmetic under one name, and no reader could
// have noticed.
//
// Rewriting RulesVersion to cover the code would have been the wrong repair. It
// is quoted on the website and in dated evidence files, and redefining a
// published field underneath the people already citing it is a second defect on
// top of the first. So the label keeps its meaning and this is a different
// fact, published beside it.
//
// WHY IT IS COMPUTED RATHER THAN DECLARED.
//
// The alternative was a constant somebody bumps when they change a price. That
// is a discipline, and #284 is what a discipline looks like the first time
// somebody is busy: every one of these changes went through review, and the
// label stayed still through all of them. A digest over the inputs cannot be
// forgotten, because forgetting is not one of the things it can do.
//
// WHAT IT COVERS, AND WHY EACH ONE.
//
//   - Every row's match string, in order. Rows match by substring in order, so
//     order alone decides whether "sonnet-4-5" resolves to its own row or to
//     the bare "sonnet" one. That reprices a corpus with no number changed.
//   - Every row's minimum cacheable prefix. A prefix under the floor does not
//     cache at all, silently, so the floor changes the re-billed figure.
//   - Every row's input price, output price and cache-read multiple.
//   - Whether the row is priced at all. This is the #284 case in one bit.
//   - The unknown-model fallback, all of it. An unrecognised model is the
//     common case in a contributed corpus, and this is what one costs.
//   - THE LOADED RULES DOCUMENT, when one is installed. Replay does not always
//     price from the compiled table: `replay rules` loads a document, Override
//     installs it for the process, and activeRow consults it first. A digest
//     that read only the compiled table would name numbers the report did not
//     use, which is worse than the defect it was built to fix. #284 left a
//     reader unable to tell two builds apart; this would have told them
//     something false.
//   - The document's AccountDiscount. It multiplies every input and output
//     price on the way out, it is the one number Replay cannot observe because
//     the operator declares it, and a negotiated rate pooled into a list-price
//     aggregate is a private discount silently deflating everyone else's
//     comparison.
//
// It deliberately does not cover PriceTableVersion, PriceTableCheckedAt or
// RulesVersion. Those are dates and labels that travel in their own fields; a
// digest that moved when a comment's date moved would cry wolf, and a reader
// comparing two submissions wants to know whether the arithmetic differed, not
// whether somebody re-read a web page.
//
// The tests beside this file mutate each covered input and require the digest
// to move. A digest that cannot change is not evidence of anything.
func PricingDigest() string {
	var b strings.Builder

	// A version prefix on the serialisation itself. If the set of covered
	// inputs ever grows, every digest changes, which is correct: a build that
	// prices on an input the previous build did not consider is a different
	// build. It also stops a future reader assuming two equal digests from
	// different Replay versions mean the same thing.
	b.WriteString("replay.pricing.v1\n")

	for _, m := range modelTable {
		writeRow(&b, m)
	}
	b.WriteString("unknown\n")
	writeRow(&b, unknownModel)

	// The compiled table is always written, even when a document overrides it,
	// because the document is an overlay rather than a replacement: activeRow
	// falls through to the table for any model the document does not name.
	// Both are in force, so both are in the digest.
	writeOverride(&b)

	sum := sha256.Sum256([]byte(b.String()))
	// Twelve hex characters. It sits beside rulesVersion on a terminal report
	// and in a submission a person reads, so it has to be short enough to
	// compare by eye; 48 bits is far more than enough to separate the handful
	// of builds a pool will ever hold, and this is a change detector rather
	// than a security boundary. Nothing authenticates itself with it.
	return "p" + hex.EncodeToString(sum[:])[:12]
}

// writeRow serialises one row unambiguously.
//
// The separators are why this is a function rather than a Sprintf. Without a
// delimiter that cannot appear inside a field, {"sonnet", 1024} and
// {"sonnet1", 024} serialise to the same bytes, and the digest would be blind
// to an edit that moved a model between tiers. Match strings are model ids, so
// they carry no newline or tab, and the test that swaps two rows is what proves
// the framing survives reordering.
func writeRow(b *strings.Builder, m modelRow) {
	b.WriteString(m.match)
	b.WriteByte('\t')
	b.WriteString(strconv.Itoa(m.minPrefix))
	b.WriteByte('\t')
	// 'g' with -1 precision round-trips a float64 exactly, so two tables that
	// differ in the last representable bit of a price serialise differently.
	// 'f' with a fixed precision would round 0.025 and 0.0250000001 together,
	// and the cache-read multiple is exactly where a small difference is a
	// large one: it multiplies every cached token in the corpus.
	b.WriteString(strconv.FormatFloat(m.price.InputPerMTok, 'g', -1, 64))
	b.WriteByte('\t')
	b.WriteString(strconv.FormatFloat(m.price.OutputPerMTok, 'g', -1, 64))
	b.WriteByte('\t')
	b.WriteString(strconv.FormatFloat(m.price.ReadMult, 'g', -1, 64))
	b.WriteByte('\t')
	b.WriteString(strconv.FormatBool(m.priced))
	b.WriteByte('\n')
}

// writeOverride serialises the loaded rules document, or records that there is
// none.
//
// "none" is written rather than nothing, so that a build with no document and a
// build whose document happened to serialise to zero bytes cannot collide.
func writeOverride(b *strings.Builder) {
	overrideMu.RLock()
	r := override
	overrideMu.RUnlock()

	b.WriteString("override\n")
	if r == nil {
		b.WriteString("none\n")
		return
	}

	// The version label and the discount, then every row. Version is included
	// even though it is a label rather than a number: two documents that price
	// identically today but were published under different versions are still
	// two documents, and a reader comparing submissions wants to see that.
	b.WriteString(r.Version)
	b.WriteByte('\t')
	b.WriteString(strconv.FormatFloat(r.AccountDiscount, 'g', -1, 64))
	b.WriteByte('\n')

	for _, m := range r.Models {
		// Fields in the same order and framing as writeRow, plus Provider and
		// the effective-date bounds, which select whether a row applies at all.
		b.WriteString(m.Match)
		b.WriteByte('\t')
		b.WriteString(m.Provider)
		b.WriteByte('\t')
		b.WriteString(strconv.Itoa(m.MinPrefix))
		b.WriteByte('\t')
		b.WriteString(strconv.FormatFloat(m.InputPerMTok, 'g', -1, 64))
		b.WriteByte('\t')
		b.WriteString(strconv.FormatFloat(m.OutputPerMTok, 'g', -1, 64))
		b.WriteByte('\t')
		b.WriteString(strconv.FormatFloat(m.ReadMult, 'g', -1, 64))
		b.WriteByte('\t')
		b.WriteString(strconv.FormatBool(m.Priced))
		b.WriteByte('\t')
		b.WriteString(m.EffectiveFrom)
		b.WriteByte('\t')
		b.WriteString(m.EffectiveUntil)
		b.WriteByte('\n')
	}
}
