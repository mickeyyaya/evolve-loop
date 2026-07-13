package cyclestate

// This file holds the cycle/phase EXECUTION-RESULT value types — the small,
// pure data records a cycle produces as it runs. Like the rest of this leaf
// they have no methods and no dependencies; the orchestrator builds them and
// serializes them into ledger/phase artifacts.

// CycleResult summarises what RunCycle did.
type CycleResult struct {
	Cycle        int
	FinalVerdict string
	PhasesRun    []Phase
	// RetroDecision is the failure-adapter's verdict on the retro branch,
	// populated only when retro ran. Format: "<action>: <reason>".
	RetroDecision string
	// SkippedPhases records non-floor phases (retrospective, memo, the *-scans,
	// router/advisor) whose non-PASS outcome was PREVENTED from overwriting an
	// already-recorded floor-derived FinalVerdict (cycle-802,
	// retro-bridge-timeout-width10). Without this a retro FAIL under quota/timeout
	// pressure clobbered an audit PASS and zeroed the wave. The degrade is
	// preserved here (never silently dropped) and surfaced in the cycle dossier.
	SkippedPhases []SkippedPhase
}

// SkippedPhase is one non-floor phase whose non-PASS verdict was degraded rather
// than allowed to overwrite a floor-derived cycle verdict. Reason carries the
// verdict/label the phase actually produced (FAIL|WARN|SKIPPED).
type SkippedPhase struct {
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
}

// TokenUsage records the LLM token counts attributed to a phase run.
type TokenUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

// Diagnostic is one structured note a phase emits (severity + message).
type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
