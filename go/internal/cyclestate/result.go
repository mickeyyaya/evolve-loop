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
