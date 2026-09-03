package cachemodel

import (
	"strings"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// PriceTableVersion dates the price table. Every dollar figure Buffy prints
// cites it, because prices change and the figure is only as current as the
// table.
const PriceTableVersion = "2026-06-24"

// Price is a model's first-party API list price in US dollars per million
// tokens. Cache reads are a multiple of the input price; on the newest tier
// that multiple is lower than the standard one.
type Price struct {
	InputPerMTok  float64
	OutputPerMTok float64
	ReadMult      float64
}

// readMultNewest applies to the Fable and Mythos 5.1 tier.
const readMultNewest = 0.025

// priceRow pairs a model-id substring with its price. Rows are matched in
// order, so more specific ids come first.
type priceRow struct {
	match string
	price Price
}

var priceTable = []priceRow{
	{"fable-5-1", Price{10, 50, readMultNewest}},
	{"mythos-5-1", Price{10, 50, readMultNewest}},
	{"fable-5", Price{10, 50, ReadMultiplier}},
	{"mythos-5", Price{10, 50, ReadMultiplier}},
	{"opus-5", Price{5, 25, ReadMultiplier}},
	{"opus-4-8", Price{5, 25, ReadMultiplier}},
	{"opus-4-7", Price{5, 25, ReadMultiplier}},
	{"opus-4-6", Price{5, 25, ReadMultiplier}},
	{"sonnet-5", Price{2, 10, ReadMultiplier}},
	{"sonnet-4-6", Price{3, 15, ReadMultiplier}},
	{"haiku-4-5", Price{1, 5, ReadMultiplier}},
}

// PriceFor returns the list price for a model id, and false when the model
// is not in the table. Callers print no dollar figure in that case.
func PriceFor(model string) (Price, bool) {
	m := strings.ToLower(model)
	for _, row := range priceTable {
		if strings.Contains(m, row.match) {
			return row.price, true
		}
	}
	return Price{}, false
}

// ReadMultiplierFor returns the cache-read multiple for a model, falling
// back to the standard multiple for unknown models.
func ReadMultiplierFor(model string) float64 {
	if p, ok := PriceFor(model); ok {
		return p.ReadMult
	}
	return ReadMultiplier
}

// tokensPerMillion converts per-million prices to per-token.
const tokensPerMillion = 1_000_000

// CostUSD prices one request's usage at first-party list rates: uncached
// input at 1x, cache writes at their TTL multiple, cache reads at the
// model's read multiple, output at the output price.
func CostUSD(u transcript.Usage, p Price) float64 {
	writes := float64(u.Create5m)*WriteMultiplierShort + float64(u.Create1h)*WriteMultiplierLong
	if u.Create5m == 0 && u.Create1h == 0 {
		writes = float64(u.CacheCreation) * WriteMultiplierShort
	}
	input := (float64(u.Input) + writes + float64(u.CacheRead)*p.ReadMult) * p.InputPerMTok
	output := float64(u.Output) * p.OutputPerMTok
	return (input + output) / tokensPerMillion
}

// PromptCostUSD prices a simulated prompt: uncached tail, a write at the
// given TTL, and a read, without output tokens.
func PromptCostUSD(tail, write, read int, writeMult float64, p Price) float64 {
	return (float64(tail) + float64(write)*writeMult + float64(read)*p.ReadMult) * p.InputPerMTok / tokensPerMillion
}
