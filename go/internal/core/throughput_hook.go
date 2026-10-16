package core

type ThroughputRecorder func(state *State, cycle int, workspacePath string)

func WithThroughputRecorder(r ThroughputRecorder) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.throughputRecorder = r
		}
	}
}

func (o *Orchestrator) ThroughputRecorderWired() bool { return o.throughputRecorder != nil }

func shippedOutcome(finalVerdict, preHEAD, postHEAD string) bool {
	if preHEAD == "" || postHEAD == "" || preHEAD == postHEAD {
		return false
	}
	return IsShippingVerdict(finalVerdict)
}

func IsShippingVerdict(finalVerdict string) bool {
	return finalVerdict == VerdictPASS || finalVerdict == CycleOutcomeShippedViaBuild
}

func hasThroughputCycle(entries []TriageThroughputEntry, cycle int) bool {
	for _, entry := range entries {
		if entry.Cycle == cycle {
			return true
		}
	}
	return false
}
