package cachemodel

import "github.com/RedRobotKK/Replay/internal/transcript"

// CostLegs is one request's list-price cost split the way the provider bills:
// uncached input, cache writes, cache reads, output.
//
// Token Reduction Is Not Cost Reduction (arXiv:2607.12161) is the reason this
// exists as a printed split rather than a comment on CostUSD. A cut that
// shrinks text already in the cached prefix saves at the read multiple, not
// at the input price. Ranking interventions by token share cannot see that.
type CostLegs struct {
	Uncached float64
	Write    float64
	Read     float64
	Output   float64
}

// Total is the four legs. CostUSD is this number.
func (l CostLegs) Total() float64 {
	return l.Uncached + l.Write + l.Read + l.Output
}

// CostLegsUSD prices one request's usage as four billed legs.
func CostLegsUSD(u transcript.Usage, p Price) CostLegs {
	in := p.InputPerMTok / tokensPerMillion
	return CostLegs{
		Uncached: float64(u.Input) * in,
		Write:    writeEquivalent(u) * in,
		Read:     float64(u.CacheRead) * p.ReadMult * in,
		Output:   float64(u.Output) * p.OutputPerMTok / tokensPerMillion,
	}
}
