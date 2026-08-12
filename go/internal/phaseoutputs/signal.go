package phaseoutputs

// signal.go — the survey's projection into the unified signal stream. The
// operator's directive is that phase-output accounting reports to the same
// center every other cycle signal reaches (abnormal-events.jsonl), not only to
// whoever happens to run the CLI. The DECISION of what counts as abnormal
// lives here in the pure layer so the loop emitter, the CLI, and any future
// dashboard reader cannot disagree about it.

// Signal renders one cycle's survey + chain status as a single signal line and
// reports whether the cycle is abnormal: any review-data gap, or a chain state
// that demands operator attention (non-compliance, a failed or corrupt
// recording, an impossible combination). ChainAuditNotRun is NOT abnormal on
// its own — a cycle with no audit has nothing to comply with — and
// ChainPresent is the healthy case.
func Signal(r Result, chain ChainStatus) (details string, abnormal bool) {
	switch chain {
	case ChainAbsent, ChainRecordMissing, ChainRecordCorrupt, ChainInconsistent:
		abnormal = true
	}
	if len(r.Gaps()) > 0 {
		abnormal = true
	}
	return r.SummaryLine() + "; chain: " + string(chain), abnormal
}
