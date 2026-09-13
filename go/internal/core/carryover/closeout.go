package carryover

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Closeout is the cycle-terminal order, encoded once: the memo merge, then
// the prescription merge, then the triage-dropped retirement — retiring
// BEFORE the merges would let a merge resurrect the very id triage just
// dropped (the cycle-1538 reproduction). The caller persists afterwards.
func (l *Lifecycle) Closeout(state *cyclestate.State, workspace string, cycle int, now time.Time) {
	l.MergeMemo(state, workspace, cycle, now)
	l.MergePrescriptions(state, workspace, cycle, now)
	l.RetireTriageDropped(state, workspace)
}
