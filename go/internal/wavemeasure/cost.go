package wavemeasure

import (
	"math"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
)

type CycleSpend struct {
	Shipped      bool
	Spend        cyclespend.Spend
	HandoffWould int
	HandedOff    bool
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
	HandoffStops                   int      `json:"handoff_stops"`
	HandoffWouldStops              int      `json:"handoff_would_stops"`
	FalseStops                     int      `json:"false_stops"`
}

func computeCost(cycles []CycleSpend) Cost {
	var t cyclespend.Totals
	var c Cost
	for _, cy := range cycles {
		t = t.Add(cy.Spend, cy.Shipped)
		c = c.withHandoff(cy)
	}
	c.Started, c.Shipped, c.UnmeasuredCalls = t.Started, t.Shipped, t.Spend.UnmeasuredCalls
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

func (c Cost) withHandoff(cy CycleSpend) Cost {
	c.HandoffWouldStops += cy.HandoffWould
	if cy.HandedOff {
		c.HandoffStops++
	}
	if cy.HandoffWould > 0 && cy.Shipped {
		c.FalseStops++
	}
	return c
}

func rounded(v, scale float64) *float64 {
	r := math.Round(v*scale) / scale
	return &r
}
