package wave

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func (r WaveRead) LaneLastEvent(root, lane string, now time.Time) time.Time {
	var last time.Time
	note := func(at time.Time) {
		if at.After(last) {
			last = at
		}
	}
	for _, e := range r.Loop {
		if at := eventTime(e); orchestrator.IsLaneLifecycle(e.Code) && e.Fields[todoField] == lane && !at.After(now) {
			note(at)
		}
	}
	for _, c := range r.Cycles {
		if c.Todo == lane {
			note(orchestrator.CycleFreshest(paths.RunWorkspace(root, c.ID), now))
		}
	}
	return last
}
