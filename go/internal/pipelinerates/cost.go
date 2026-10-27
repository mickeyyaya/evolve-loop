package pipelinerates

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const costIndex = "PQ5"

func CostPerShip(cycles []Cycle) []qualitypanel.Reading {
	var out []qualitypanel.Reading
	for _, w := range Windows(cycles) {
		var t cyclespend.Totals
		for _, c := range cycles {
			if w.holds(c) {
				t = t.Add(c.Spend, c.Shipped)
			}
		}
		per, _ := t.PerShipped()
		rounds, _ := t.AuditRoundsPerCycle()
		out = append(out,
			census(w.Name, per.LLMMinutes, t.Shipped),
			census(w.Name+"/fresh-tokens", per.FreshTokens, t.Shipped),
			census(w.Name+"/cache-read-tokens", per.CacheReadTokens, t.Shipped),
			census(w.Name+"/audit-rounds", rounds, t.Started),
			census(w.Name+"/unmeasured-calls", float64(t.Spend.UnmeasuredCalls), t.Started))
	}
	return out
}

func census(window string, v float64, n int) qualitypanel.Reading {
	return qualitypanel.Reading{Index: costIndex, Interval: qualitypanel.Interval{Value: v, Lo: v, Hi: v}, Method: qualitypanel.MethodCensus, N: n, Window: window, Source: runsSource, Origin: "pipelinerates.CostPerShip"}
}
