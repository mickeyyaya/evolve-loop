package core

// throughput_hook.go — R9.1 (triage capacity): the throughput-recorder seam.
// After a cycle that actually shipped, the orchestrator hands the in-memory
// State to the injected recorder so it can append the cycle's passed
// coverage-floor count to the rolling window (state.json:triageThroughput).
// The window logic itself lives in internal/triagecap (which imports core
// for the entry type — hence the seam: core must not import triagecap).
// Nil (default) is a no-op — byte-identical to the pre-R9 cycle.

// ThroughputRecorder observes one shipped cycle: it may mutate state
// in-place (the orchestrator writes state immediately after).
type ThroughputRecorder func(state *State, cycle int, workspacePath string)

// WithThroughputRecorder injects the R9.1 recorder. Nil keeps the seam inert.
func WithThroughputRecorder(r ThroughputRecorder) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.throughputRecorder = r
		}
	}
}

// ThroughputRecorderWired reports whether the recorder is injected —
// introspection for composition-root wiring tests (mirrors
// FailureAdviserWired).
func (o *Orchestrator) ThroughputRecorderWired() bool { return o.throughputRecorder != nil }

// shippedOutcome reports whether the cycle's final verdict plus HEAD
// movement constitute a real ship — the only cycles whose floor commitments
// are honest throughput evidence. Empty HEADs (git unavailable) are never
// shipped evidence.
func shippedOutcome(finalVerdict, preHEAD, postHEAD string) bool {
	if preHEAD == "" || postHEAD == "" || preHEAD == postHEAD {
		return false
	}
	return finalVerdict == VerdictPASS || finalVerdict == CycleOutcomeShippedViaBuild
}
