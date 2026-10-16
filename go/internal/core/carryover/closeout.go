package carryover

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func (l *Lifecycle) Closeout(state *cyclestate.State, workspace string, cycle int, now time.Time) {
	l.MergeMemo(state, workspace, cycle, now)
	l.MergePrescriptions(state, workspace, cycle, now)
	l.RetireTriageDropped(state, workspace)
}
