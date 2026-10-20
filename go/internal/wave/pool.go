package wave

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

const (
	TerminationLaneDeadline = "lane deadline"
	todoField               = "todo"
	deadlineStopped         = "stopped"
)

func poolLanes(f wavemeasure.Facts, events []signalcenter.Event) []wavemeasure.Lane {
	var lanes []wavemeasure.Lane
	open := map[string][]int{}
	for _, e := range events {
		at := eventTime(e)
		if !inRun(f, at) {
			continue
		}
		todo := e.Fields[todoField]
		switch e.Code {
		case orchestrator.CodeLaneLaunched, orchestrator.CodeLaneBackfilled:
			open[todo] = append(open[todo], len(lanes))
			lanes = append(lanes, wavemeasure.Lane{Start: at})
		case orchestrator.CodeLaneExited:
			if q := open[todo]; len(q) > 0 {
				lanes[q[0]].End = at
				open[todo] = q[1:]
			}
		}
	}
	return lanes
}

func ApplyLaneDeadlines(cycles []Cycle, events []signalcenter.Event) []Cycle {
	out := append([]Cycle(nil), cycles...)
	for _, e := range events {
		if e.Code != orchestrator.CodeLaneDeadline || e.Fields["action"] != deadlineStopped {
			continue
		}
		at := eventTime(e)
		if i := laneCycle(out, e.Fields[todoField], at); i >= 0 {
			out[i].Termination, out[i].EndedAt = TerminationLaneDeadline, at
		}
	}
	return out
}

func laneCycle(cycles []Cycle, todo string, at time.Time) int {
	found := -1
	for i, c := range cycles {
		if c.Todo != todo || c.Verdict != "" || c.StartedAt.IsZero() || c.StartedAt.After(at) {
			continue
		}
		if found < 0 || c.StartedAt.After(cycles[found].StartedAt) {
			found = i
		}
	}
	return found
}
