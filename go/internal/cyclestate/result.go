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
	// SkippedPhases records phases that genuinely did NOT run, with the cause
	// (e.g. closeout after an abnormal mid-cycle exit). Surfaced in the cycle
	// dossier as skipped_phases.
	//
	// It is NOT the home for a phase that ran and had its verdict declined —
	// that is VerdictsNotAdopted. Conflating the two is the
	// dossier-retro-skipped-mislabel defect: every FAIL dossier from 1028/1035
	// through 1198 claimed `skipped_phases: [{phase: retro, reason: FAIL}]` while
	// the run dir held retro's report, so the committed record contradicted its
	// own artifacts.
	SkippedPhases []SkippedPhase
	// VerdictsNotAdopted records non-floor phases (retrospective, memo, the
	// *-scans, router/advisor) that RAN and returned non-PASS AFTER a floor-derived
	// FinalVerdict was recorded, so their verdict was PREVENTED from overwriting it
	// (cycle-802, retro-bridge-timeout-width10). Without this a retro FAIL under
	// quota/timeout pressure clobbered an audit PASS and zeroed the wave. The
	// outcome is preserved here (never silently dropped) and surfaced in the cycle
	// dossier as phases_run_verdict_not_adopted — a name that cannot be misread as
	// "this phase did not run".
	VerdictsNotAdopted []VerdictNotAdopted
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
	// SpineFailOpens records every spine-gate fail-open this cycle took: the
	// gate found a mandatory predecessor's handoff artifact missing and
	// proceeded anyway (SpineFloor below enforce, or a non-clean absence).
	// Before cycle-1166 these went to stderr and nowhere else — a width-3 batch
	// emitted 76 of them with no counter, no dossier field and no threshold.
	// Occurrences ACCUMULATE (never collapse repeats): the count IS the signal.
	SpineFailOpens []SpineFailOpen
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

// SkippedPhase is one phase that did NOT run. Reason is the SKIP CAUSE (e.g.
// "abnormal exit in phase build"), not a verdict — a phase that ran belongs in
// VerdictNotAdopted.
type SkippedPhase struct {
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
}

// VerdictNotAdopted is one phase that RAN to completion whose non-PASS verdict was
// NOT adopted as the cycle verdict (the cycle-802 floor guard: a post-verdict
// non-floor phase may not clobber a floor-derived FinalVerdict). Verdict is what
// the phase actually returned (FAIL|WARN|SKIPPED) — the value the old
// SkippedPhase.Reason carried, under a name that no longer implies a skip.
type VerdictNotAdopted struct {
	Phase   string `json:"phase"`
	Verdict string `json:"verdict"`
}

// SpineFailOpen is one spine-gate fail-open event. Phase is the phase that was
// entered anyway; MissingArtifact is the FIRST unsatisfied predecessor anchor
// (the real cause — phase alone cannot group 76 WARNs by cause); Reason is the
// fail-open reason verbatim ("would-block at enforce" vs "digest degraded: …"),
// which is what distinguishes a dialed-down SpineFloor from a degraded read.
type SpineFailOpen struct {
	Phase           string `json:"phase"`
	MissingArtifact string `json:"missing_artifact"`
	Reason          string `json:"reason,omitempty"`
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
