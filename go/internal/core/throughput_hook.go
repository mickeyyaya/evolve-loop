package core

// ThroughputRecorder observes one shipped cycle: it may mutate state
// in-place (the orchestrator writes state immediately after).
type ThroughputRecorder func(state *State, cycle int, workspacePath string)

// WithThroughputRecorder injects the throughput recorder. Nil keeps the seam
// inert.
func WithThroughputRecorder(r ThroughputRecorder) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.throughputRecorder = r
		}
	}
}

// ThroughputRecorderWired reports whether the recorder is injected.
func (o *Orchestrator) ThroughputRecorderWired() bool { return o.throughputRecorder != nil }

// shippedOutcome reports whether the cycle's final verdict plus HEAD
// movement constitute a real ship.
func shippedOutcome(finalVerdict, preHEAD, postHEAD string) bool {
	if preHEAD == "" || postHEAD == "" || preHEAD == postHEAD {
		return false
	}
	return IsShippingVerdict(finalVerdict)
}

// IsShippingVerdict reports whether a cycle's final verdict is one that can
// represent shipped work.
func IsShippingVerdict(finalVerdict string) bool {
	return finalVerdict == VerdictPASS || finalVerdict == CycleOutcomeShippedViaBuild
}

// hasThroughputCycle prevents replayed closeout from observing the same
// cycle twice.
func hasThroughputCycle(entries []TriageThroughputEntry, cycle int) bool {
	for _, entry := range entries {
		if entry.Cycle == cycle {
			return true
		}
	}
	return false
}
