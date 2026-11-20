package pipelinerates

import (
	"slices"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type RecoveryInput struct {
	Dossiers       []*dossier.Dossier
	Window, Source string
	Cycle          int
}

type RecoveryResult struct {
	Recoveries  int
	Hours       []float64
	MedianHours float64
	OQ4         qualitypanel.Reading
}

func (r RecoveryResult) Events(stage qualitypanel.Stage) []signalcenter.Event {
	return qualitypanel.Events(r.OQ4, stage)
}

func MeasureRecovery(in RecoveryInput) RecoveryResult {
	var res RecoveryResult
	failures, open := 0, map[string]string{}
	for _, d := range byCycle(in.Dossiers) {
		for _, task := range *d.Tasks {
			firstFailureEnd, failing := open[task]
			switch {
			case recoveryretro.Shipped(d) && failing:
				res.Recoveries++
				if h, ok := hoursBetween(firstFailureEnd, d.EndedAt); ok {
					res.Hours = append(res.Hours, h)
				}
				delete(open, task)
			case d.FinalVerdict == dossier.VerdictFail && !failing:
				failures++
				open[task] = d.EndedAt
			}
		}
	}
	res.MedianHours = qualitypanel.Median(res.Hours)
	res.OQ4 = qualitypanel.Reading{
		Index: "OQ4", Interval: qualitypanel.Wilson(res.Recoveries, failures), Method: qualitypanel.MethodWilson95, N: failures,
		Window: in.Window, Source: in.Source, Cycle: in.Cycle, Origin: "pipelinerates.MeasureRecovery",
		Extra: map[string]string{"median_hours": strconv.FormatFloat(res.MedianHours, 'f', 3, 64)},
	}
	return res
}

func byCycle(all []*dossier.Dossier) []*dossier.Dossier {
	seen := map[int]bool{}
	var out []*dossier.Dossier
	for _, d := range all {
		if d != nil && d.Tasks != nil && !seen[d.Cycle] {
			seen[d.Cycle] = true
			out = append(out, d)
		}
	}
	slices.SortStableFunc(out, func(a, b *dossier.Dossier) int { return a.Cycle - b.Cycle })
	return out
}

func hoursBetween(from, to string) (float64, bool) {
	start, errStart := time.Parse(time.RFC3339, from)
	end, errEnd := time.Parse(time.RFC3339, to)
	return end.Sub(start).Hours(), errStart == nil && errEnd == nil
}
