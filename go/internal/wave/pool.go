package wave

import (
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

const (
	todoField       = "todo"
	liveField       = "live"
	deadlineStopped = "stopped"
)

type loopRun struct {
	pid      int
	from, to time.Time
}

func loopRunOf(rec Record, next *Record) loopRun {
	run := loopRun{pid: rec.PID, from: rec.StartedAt, to: runEnd(rec, next)}
	if at, ok := gcpolicy.ParseLogRunID(rec.RunID); ok && at.Before(run.from) {
		run.from = at
	}
	return run
}

func (r loopRun) holds(e signalcenter.Event) bool {
	return (r.pid == 0 || e.PID == r.pid) && r.spans(eventTime(e))
}

func (r loopRun) spans(at time.Time) bool {
	return !at.Before(r.from) && (r.to.IsZero() || !at.After(r.to))
}

func poolLanes(run loopRun, events []signalcenter.Event) []wavemeasure.Lane {
	var lanes []wavemeasure.Lane
	open := map[string][]int{}
	for _, e := range events {
		if !run.holds(e) {
			continue
		}
		at, todo := eventTime(e), e.Fields[todoField]
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

func PoolLive(rec Record, next *Record, events []signalcenter.Event) (int, bool) {
	run := loopRunOf(rec, next)
	live, found := 0, false
	for _, e := range events {
		if n, err := strconv.Atoi(e.Fields[liveField]); err == nil && run.holds(e) {
			live, found = n, true
		}
	}
	return live, found
}

func ApplyLaneDeadlines(cycles []Cycle, events []signalcenter.Event) []Cycle {
	out := append([]Cycle(nil), cycles...)
	for _, e := range events {
		if e.Code != orchestrator.CodeLaneDeadline || e.Fields["action"] != deadlineStopped {
			continue
		}
		at := eventTime(e)
		if i := laneCycle(out, e.Fields[todoField], at); i >= 0 && out[i].Verdict == "" && out[i].Termination == "" {
			out[i].Termination, out[i].EndedAt = cyclestate.CycleTerminationLaneDeadline, at
		}
	}
	return out
}

func laneCycle(cycles []Cycle, todo string, at time.Time) int {
	found := -1
	for i, c := range cycles {
		if c.Todo != todo || c.StartedAt.IsZero() || c.StartedAt.After(at) {
			continue
		}
		if found < 0 || c.StartedAt.After(cycles[found].StartedAt) {
			found = i
		}
	}
	return found
}

func ReadRunCycles(root string, floor, ceiling int) ([]Cycle, []signalcenter.Event, []string) {
	cycles, warnings := ReadCycles(root, floor, ceiling)
	loop, loopWarnings := ReadLoopSignals(root)
	return ApplyLaneDeadlines(cycles, loop), loop, append(warnings, loopWarnings...)
}
