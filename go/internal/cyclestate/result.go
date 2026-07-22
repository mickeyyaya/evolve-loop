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
	// SystemFailure, when non-nil, marks that this cycle's failure was
	// classified as SYSTEM-level (ADR-0072): the pipeline itself — not the
	// task's code — is the cause (verdict-incoherence, infra-systemic,
	// non-progress). The batch loop reads it to HALT + escalate instead of
	// re-selecting the same inbox task. Nil ⇒ an ordinary task-level outcome
	// (never-stop: retry/defer/quarantine as usual).
	SystemFailure *SystemFailureSignal
	// Remediations records graduated fix-forward rounds (operator directive
	// 2026-07-21): each entry is "<gate>: round N -> <verdict>" for a
	// deterministic gate that FAILed, received one bounded builder fix, and
	// was re-run. Provenance only — the re-run verdict is what recorded; a
	// remediated cycle is never a silent PASS.
	Remediations []string
	// FailReasons surfaces the floor-override explanations (the untruncated
	// audit-fail-reason.json / CycleState.AuditFailReasons content) in the
	// cycle summary and dossier — cycle-1022's lesson: the reason WAS recorded
	// on disk while every operator-facing surface stayed silent.
	FailReasons []string
}

// SystemFailureSignal records a system-level failure classification (ADR-0072).
// Category is the failure_policy category (e.g. "verdict-incoherence"); Halt is
// true when the Go floor mandates a loop halt regardless of orchestrator
// judgment. Evidence is the deterministic proof (e.g. the coherence signal).
type SystemFailureSignal struct {
	Category string `json:"category"`
	Level    string `json:"level"` // "system"
	Evidence string `json:"evidence"`
	Halt     bool   `json:"halt"`
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
