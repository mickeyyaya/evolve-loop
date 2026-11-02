package wavemeasure

import (
	"math"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
)

type CycleSpend struct {
	Shipped      bool
	Spend        cyclespend.Spend
	Verdict      cyclespend.Verdict
	HandoffWould int
	HandedOff    bool
	SpendLimit   string
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
	SpendLimitStops                int      `json:"spend_limit_stops"`
	SpendLimitWouldStops           int      `json:"spend_limit_would_stops"`
	MedianLLMMinutesToVerdict      *float64 `json:"median_llm_minutes_to_verdict"`
	MedianFailFastRatio            *float64 `json:"median_fail_fast_ratio"`
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
	c.MedianLLMMinutesToVerdict, c.MedianFailFastRatio = verdictMedians(cycles)
	return c
}

func verdictMedians(cycles []CycleSpend) (*float64, *float64) {
	var minutes, ratios []float64
	for _, cy := range cycles {
		if cy.Verdict.LLMMinutesToVerdict > 0 {
			minutes = append(minutes, cy.Verdict.LLMMinutesToVerdict)
		}
		if cy.Verdict.FailFastRatio != nil {
			ratios = append(ratios, *cy.Verdict.FailFastRatio)
		}
	}
	return median(minutes, 10), median(ratios, 100)
}

func median(values []float64, scale float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return rounded(values[mid], scale)
	}
	return rounded((values[mid-1]+values[mid])/2, scale)
}

func (c Cost) withHandoff(cy CycleSpend) Cost {
	c.HandoffWouldStops += cy.HandoffWould
	if cy.HandedOff {
		c.HandoffStops++
	}
	switch cy.SpendLimit {
	case cyclespend.ActionStop:
		c.SpendLimitStops++
	case cyclespend.ActionWouldStop:
		c.SpendLimitWouldStops++
	}
	if (cy.HandoffWould > 0 || cy.SpendLimit == cyclespend.ActionWouldStop) && cy.Shipped {
		c.FalseStops++
	}
	return c
}

func rounded(v, scale float64) *float64 {
	r := math.Round(v*scale) / scale
	return &r
}
