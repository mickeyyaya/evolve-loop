package pipelinerates

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const (
	runsSource      = ".evolve/runs"
	rollingWaves    = 10
	systemFailLimit = 0.05
)

type Window struct {
	Name     string
	From, To int
}

func (w Window) holds(c Cycle) bool { return c.Wave >= w.From && c.Wave <= w.To }

func Windows(cycles []Cycle) []Window {
	if len(cycles) == 0 {
		return nil
	}
	first, last := cycles[0].Wave, cycles[0].Wave
	for _, c := range cycles {
		first, last = min(first, c.Wave), max(last, c.Wave)
	}
	out := []Window{{Name: fmt.Sprintf("wave:%d", last), From: last, To: last}}
	if from := max(first, last-rollingWaves+1); from < last {
		out = append(out, Window{Name: fmt.Sprintf("waves:%d-%d", from, last), From: from, To: last})
	}
	if first < last-rollingWaves+1 {
		out = append(out, Window{Name: fmt.Sprintf("waves:%d-%d", first, last), From: first, To: last})
	}
	return out
}

func RateIn(cycles []Cycle, w Window, hit func(Cycle) bool) (k, n int) {
	for _, c := range cycles {
		if w.holds(c) {
			n++
			if hit(c) {
				k++
			}
		}
	}
	return k, n
}

func rateReadings(cycles []Cycle, index, suffix, origin string, hit func(Cycle) bool, threshold *float64) []qualitypanel.Reading {
	var out []qualitypanel.Reading
	for _, w := range Windows(cycles) {
		k, n := RateIn(cycles, w, hit)
		out = append(out, qualitypanel.Reading{Index: index, Interval: qualitypanel.Wilson(k, n), Method: qualitypanel.MethodWilson95, N: n, Window: w.Name + suffix, Source: runsSource, Origin: origin, Threshold: threshold})
	}
	return out
}

func Shipped(c Cycle) bool { return c.Shipped }

func ShipRate(cycles []Cycle) []qualitypanel.Reading {
	return rateReadings(cycles, "PQ1", "", "pipelinerates.ShipRate", Shipped, nil)
}

func DeadlineRate(cycles []Cycle) []qualitypanel.Reading {
	limit := systemFailLimit
	deadline := rateReadings(cycles, "PQ4", "", "pipelinerates.DeadlineRate", Cycle.DeadlineEnd, nil)
	system := rateReadings(cycles, "PQ4", "/system-fail", "pipelinerates.DeadlineRate", func(c Cycle) bool { return c.SystemFail }, &limit)
	return append(deadline, system...)
}
