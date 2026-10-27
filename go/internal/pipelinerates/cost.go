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
		var h handoffs
		for _, c := range cycles {
			if w.holds(c) {
				t = t.Add(c.Spend, c.Shipped)
				h = h.add(c)
			}
		}
		per, _ := t.PerShipped()
		rounds, _ := t.AuditRoundsPerCycle()
		out = append(out,
			census(w.Name, per.LLMMinutes, t.Shipped),
			census(w.Name+"/fresh-tokens", per.FreshTokens, t.Shipped),
			census(w.Name+"/cache-read-tokens", per.CacheReadTokens, t.Shipped),
			census(w.Name+"/audit-rounds", rounds, t.Started),
			census(w.Name+"/unmeasured-calls", float64(t.Spend.UnmeasuredCalls), t.Started),
			census(w.Name+"/handoff-would-stops", float64(h.would), t.Started),
			census(w.Name+"/handoff-stops", float64(h.stops), t.Started),
			census(w.Name+"/false-stops", float64(h.false), t.Started),
			census(w.Name+"/spend-limit-would-stops", float64(h.spendWould), t.Started),
			census(w.Name+"/spend-limit-stops", float64(h.spendStops), t.Started))
	}
	return out
}

func census(window string, v float64, n int) qualitypanel.Reading {
	return qualitypanel.Reading{Index: costIndex, Interval: qualitypanel.Interval{Value: v, Lo: v, Hi: v}, Method: qualitypanel.MethodCensus, N: n, Window: window, Source: runsSource, Origin: "pipelinerates.CostPerShip"}
}

type handoffs struct{ would, stops, false, spendWould, spendStops int }

func (h handoffs) add(c Cycle) handoffs {
	spendWould := c.SpendLimit == cyclespend.ActionWouldStop
	h.would += c.HandoffWould
	h.stops += boolInt(c.HandedOff)
	h.false += boolInt((c.HandoffWould > 0 || spendWould) && c.Shipped)
	h.spendWould += boolInt(spendWould)
	h.spendStops += boolInt(c.SpendLimit == cyclespend.ActionStop)
	return h
}
