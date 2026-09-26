package cyclestate

// CycleResult summarises what RunCycle did.
type CycleResult struct {
	Cycle        int
	FinalVerdict string
	PhasesRun    []Phase
	// TerminationReason is a host-owned terminal disposition that phase artifacts cannot
	// reconstruct safely; empty means the ordinary closeout path.
	TerminationReason string
	// RetroDecision is the failure-adapter's verdict on the retro branch,
	// populated only when retro ran. Format: "<action>: <reason>".
	RetroDecision string
	// SkippedPhases lists phases that did not run, with the skip cause; a phase that ran
	// and had its verdict declined belongs in VerdictsNotAdopted.
	SkippedPhases []SkippedPhase
	// VerdictsNotAdopted lists non-floor phases that ran and returned non-PASS after the
	// floor-derived FinalVerdict was recorded, which they may not overwrite.
	VerdictsNotAdopted []VerdictNotAdopted
	// SystemFailure, when non-nil, marks a pipeline-caused failure on which the batch loop
	// halts instead of re-selecting the task.
	SystemFailure *SystemFailureSignal
	// Remediations records fix-forward rounds as "<gate>: round N -> <verdict>"; provenance
	// only, since the re-run verdict is the one recorded.
	Remediations []string
	// SpineFailOpens records every spine-gate fail-open; repeats accumulate because the
	// count is the signal.
	SpineFailOpens []SpineFailOpen
	// FailReasons surfaces the floor-override explanations in the cycle summary and dossier.
	FailReasons []string
}

// SystemFailureSignal is a system-level failure classification; Halt means the Go floor mandates a loop halt.
// See ADR-0072.
type SystemFailureSignal struct {
	Category string `json:"category"`
	Level    string `json:"level"` // "system"
	Evidence string `json:"evidence"`
	Halt     bool   `json:"halt"`
}

// SkippedPhase is one phase that did not run; Reason is the skip cause, not a verdict.
type SkippedPhase struct {
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
}

// VerdictNotAdopted is one phase that ran whose non-PASS Verdict was not adopted over the floor-derived FinalVerdict.
type VerdictNotAdopted struct {
	Phase   string `json:"phase"`
	Verdict string `json:"verdict"`
}

// SpineFailOpen is one spine-gate fail-open: the Phase entered, its first unsatisfied predecessor, and the verbatim reason.
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
	// Code is the DiagCode* reason a phase's own deterministic gate stamps (empty on an agent's
	// diagnostic); Subject names the item it concerns, such as a top_n card id.
	Code    string `json:"code,omitempty"`
	Subject string `json:"subject,omitempty"`
}

// The triage gate's refusal codes: the reasons triage.Classify itself FAILs a cycle, not the agent's verdict.
const (
	DiagCodeTriageProtectedSurface  = "TRIAGE_PROTECTED_SURFACE"
	DiagCodeTriageTopNEmpty         = "TRIAGE_TOPN_EMPTY"
	DiagCodeTriageCommitmentInvalid = "TRIAGE_COMMITMENT_INVALID"
)

// Severity values of Diagnostic; only SeverityError entries are a phase's reasons for a FAIL verdict.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// ErrorMessages projects diagnostics onto a FAIL's reasons: the error-severity messages in order, nil when none.
func ErrorMessages(diags []Diagnostic) []string {
	var msgs []string
	for _, d := range diags {
		if d.Severity == SeverityError {
			msgs = append(msgs, d.Message)
		}
	}
	return msgs
}

// ErrorCodes projects diagnostics onto their error-severity codes, deduped in order with blanks dropped.
func ErrorCodes(diags []Diagnostic) []string {
	var codes []string
	seen := map[string]bool{}
	for _, d := range diags {
		if d.Severity != SeverityError || d.Code == "" || seen[d.Code] {
			continue
		}
		seen[d.Code] = true
		codes = append(codes, d.Code)
	}
	return codes
}

// Disposition says whose fault a coded refusal is.
type Disposition struct {
	// TaskLevel charges the item's failure_count toward the quarantine ceiling.
	TaskLevel bool
	// RouteConsole hands the refused Subject to the operator on the first hit.
	RouteConsole bool
}

// RefusalDisposition maps a refusal code to its disposition; an unknown or empty code charges nobody.
// TRIAGE_COMMITMENT_INVALID is stamped on I/O faults reading the decision, so it never charges the queue.
func RefusalDisposition(code string) Disposition {
	switch code {
	case DiagCodeTriageProtectedSurface:
		return Disposition{TaskLevel: true, RouteConsole: true}
	case DiagCodeTriageTopNEmpty:
		return Disposition{TaskLevel: true}
	default:
		return Disposition{}
	}
}
