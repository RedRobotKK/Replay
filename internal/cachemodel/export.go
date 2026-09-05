package cachemodel

import "time"

// ExportRules renders the compiled table as a rules document.
//
// This exists so the free tier published at a URL is generated from the binary
// rather than maintained beside it. Two copies of a price table drift, and the
// drift is invisible: the published one keeps serving numbers the binary no
// longer agrees with, and nothing fails.
//
// It also makes a claim checkable that would otherwise be marketing. The free
// tier is said to be complete rather than a sample, and a reader can now
// confirm that by diffing this output against the paid feed.
//
// The compiled table is the source. Any installed override is deliberately
// ignored: exporting whatever happened to be installed would publish one
// machine's local state as the product.
func ExportRules() Rules {
	models := make([]ModelRule, 0, len(modelTable))
	for _, r := range modelTable {
		// Export what the table BEHAVES as, not how it happens to be stored.
		//
		// lookup() normalises a zero ReadMult to the standard multiple at read
		// time, so four rows carry 0 in the struct and 0.10 in every answer the
		// binary gives. Copying the raw field, and then `omitempty` dropping
		// it, published a document that priced cache reads on those models at
		// nothing — while the doc comment claimed "the compiled table,
		// exactly". Today those rows are all unpriced so the effect is latent;
		// the moment one is priced, or a consumer reads ReadMult without
		// guarding for zero, the published feed is quietly wrong.
		read := r.price.ReadMult
		if read == 0 {
			read = ReadMultiplier
		}
		models = append(models, ModelRule{
			Match:         r.match,
			MinPrefix:     r.minPrefix,
			InputPerMTok:  r.price.InputPerMTok,
			OutputPerMTok: r.price.OutputPerMTok,
			ReadMult:      read,
			Priced:        r.priced,
		})
	}
	return Rules{
		Schema:   RulesSchema,
		Version:  RulesVersion,
		Provider: "anthropic",
		// Source names where the numbers came from, not where this file was
		// served from. A reader checking a price needs the provider's page.
		Source:    "https://www.anthropic.com/pricing",
		FetchedAt: PriceTableVersion + "T00:00:00Z",
		Models:    models,
	}
}

// ExportedAt is the date stamped on a generated document, kept separate from
// PriceTableVersion so a regeneration does not silently claim the prices were
// rechecked on the day the file was written.
func ExportedAt(now time.Time) string { return now.UTC().Format(time.RFC3339) }
