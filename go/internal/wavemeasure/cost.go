package wavemeasure

import (
	"math"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
)

type CycleSpend struct {
	Shipped bool
	Spend   cyclespend.Spend
}

type Cost struct {
	Started                        int      `json:"started_cycles"`
	Shipped                        int      `json:"shipped_cycles"`
	LLMMinutesPerShippedCycle      *float64 `json:"llm_minutes_per_shipped_cycle"`
	FreshTokensPerShippedCycle     *float64 `json:"fresh_tokens_per_shipped_cycle"`
	CacheReadTokensPerShippedCycle *float64 `json:"cache_read_tokens_per_shipped_cycle"`
	UnmeasuredCalls                int      `json:"unmeasured_calls"`
	AuditRoundsPerCycle            *float64 `json:"audit_rounds_per_cycle"`
	ConvergenceRate                *float64 `json:"convergence_rate"`
}

func computeCost(cycles []CycleSpend) Cost {
	var t cyclespend.Totals
	for _, cy := range cycles {
		t = t.Add(cy.Spend, cy.Shipped)
	}
	c := Cost{Started: t.Started, Shipped: t.Shipped, UnmeasuredCalls: t.Spend.UnmeasuredCalls}
	if per, ok := t.PerShipped(); ok {
		c.LLMMinutesPerShippedCycle = rounded(per.LLMMinutes, 10)
		c.FreshTokensPerShippedCycle = rounded(per.FreshTokens, 1)
		c.CacheReadTokensPerShippedCycle = rounded(per.CacheReadTokens, 1)
	}
	if rounds, ok := t.AuditRoundsPerCycle(); ok {
		c.AuditRoundsPerCycle = rounded(rounds, 100)
	}
	if rate, ok := t.ConvergenceRate(); ok {
		c.ConvergenceRate = &rate
	}
	return c
}

func rounded(v, scale float64) *float64 {
	r := math.Round(v*scale) / scale
	return &r
}
