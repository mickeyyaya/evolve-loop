package cyclestate

// State is a subset view of .evolve/state.json; the orchestrator's WriteState drops every key it does not model.
type State struct {
	LastUpdated     string          `json:"lastUpdated"`
	LastCycleNumber int             `json:"lastCycleNumber"`
	Version         int             `json:"version"`
	CurrentBatch    BatchAccrual    `json:"currentBatch"`
	FailedAt        []FailedRecord  `json:"failedApproaches,omitempty"`
	CarryoverTodos  []CarryoverTodo `json:"carryoverTodos,omitempty"`
	// SetupCompletedAt and SetupVersion are the onboarding marker; `evolve setup complete`
	// writes them by lossless raw merge, never WriteState.
	SetupCompletedAt string `json:"setupCompletedAt,omitempty"`
	SetupVersion     int    `json:"setupVersion,omitempty"`
	// TriageThroughput is the rolling window of coverage floors passed per recent PASS cycle,
	// which bounds triage's floor commitments; internal/triagecap owns its ops.
	TriageThroughput []TriageThroughputEntry `json:"triageThroughput,omitempty"`
	// StateRevision increments once per Storage.UpdateState read-modify-write;
	// a gap or repeat exposes a writer that bypassed the lock.
	StateRevision int `json:"stateRevision,omitempty"`
	// LastAllocatedCycleNumber is the highest cycle number ever minted (LastCycleNumber is the highest
	// completed); UpdateState bumps it before a run starts, so a crashed run burns its number.
	LastAllocatedCycleNumber int `json:"lastAllocatedCycleNumber,omitempty"`
}

// TriageThroughputEntry is one PASS cycle in the triage-capacity window and the coverage floors it passed.
type TriageThroughputEntry struct {
	Cycle  int `json:"cycle"`
	Floors int `json:"floors"`
}

// BatchAccrual tracks per-dispatcher-invocation cost.
type BatchAccrual struct {
	CycleAccruedCostUSD float64 `json:"cycleAccruedCostUSD"`
	GoalHash            string  `json:"goalHash,omitempty"`
}

// FailedRecord is one non-PASS cycle outcome in state.json failedApproaches[], the input failureadapter decides from.
type FailedRecord struct {
	TS                string   `json:"ts,omitempty"`
	Cycle             int      `json:"cycle"`
	Verdict           string   `json:"verdict"`
	Classification    string   `json:"classification,omitempty"`
	RecordedAt        string   `json:"recordedAt,omitempty"`
	ExpiresAt         string   `json:"expiresAt,omitempty"`
	AuditReportPath   string   `json:"auditReportPath,omitempty"`
	AuditReportSHA256 string   `json:"auditReportSha256,omitempty"`
	GitHead           string   `json:"gitHead,omitempty"`
	TreeStateSHA      string   `json:"treeStateSha,omitempty"`
	Defects           []string `json:"defects,omitempty"`
	Retrospected      bool     `json:"retrospected"`
	Summary           string   `json:"summary,omitempty"`
}

// CarryoverTodo is operator-queued work surfaced cycle-to-cycle.
type CarryoverTodo struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	Priority       string `json:"priority"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	// CyclesUnpicked is never incremented; per-task failure memory is the inbox item's failure_count.
	// Kept for wire compatibility; add no new reads.
	CyclesUnpicked int `json:"cycles_unpicked"`
	// ExpiresAt (RFC3339) is inherited from the originating FailedRecord so the loop-start prune
	// ages the todo out; empty means it is never auto-pruned.
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// CycleState mirrors .evolve/cycle-state.json; later-added fields are omitempty so older checkpoints round-trip unchanged.
type CycleState struct {
	// FinalVerdict is the floor-gated disposition at the last completed phase;
	// post-audit resume keeps it rather than defaulting to PASS.
	FinalVerdict string `json:"final_verdict,omitempty"`
	// GoalHash and GoalText carry the original request across quota pauses.
	// Absent on legacy checkpoints; resume resolves those conservatively.
	GoalHash string `json:"goal_hash,omitempty"`
	GoalText string `json:"goal_text,omitempty"`
	// Shipped is this cycle's own ship latch, set when its ship phase PASSes review; it is
	// never inferred from main HEAD, which sibling lanes move too.
	Shipped bool `json:"shipped,omitempty"`
	// PreCycleHEAD keeps the closeout baseline across a pause after Ship;
	// recapturing it on resume would lose that completed ship.
	PreCycleHEAD    string   `json:"pre_cycle_head,omitempty"`
	CycleID         int      `json:"cycle_id"`
	Phase           string   `json:"phase"`
	StartedAt       string   `json:"started_at"`
	PhaseStartedAt  string   `json:"phase_started_at"`
	ActiveAgent     string   `json:"active_agent,omitempty"`
	ActiveWorktree  string   `json:"active_worktree,omitempty"`
	CompletedPhases []string `json:"completed_phases,omitempty"`
	WorkspacePath   string   `json:"workspace_path"`
	IntentRequired  bool     `json:"intent_required"`
	// RunID is the ULID RunCycle mints for this run, also stamped on every ledger entry it emits.
	RunID string `json:"run_id,omitempty"`
	// WorktreeBaseSHA is the cycle worktree's HEAD at creation, persisted so crash-resume can run
	// the build-commit normalize; empty makes that normalize a no-op.
	WorktreeBaseSHA string `json:"worktree_base_sha,omitempty"`
	// AuditRepairAttempts counts in-cycle audit repairs dispatched; it is persisted because an
	// in-memory count would reset on resume and grant a failing cycle unlimited retries.
	AuditRepairAttempts int `json:"audit_repair_attempts,omitempty"`
	// ShipRecoveryCode is the ship error a recovery rebuilds from, cleared by the ship latch;
	// persisted so the live loop and a resume seed the standing-findings brief alike.
	ShipRecoveryCode string `json:"ship_recovery_code,omitempty"`
	// AuditDeclineReason is the retry envelope's reason when an audit FAIL got no repair grant;
	// it alone marks a retro-routed tdd/build re-entry as owed the audit's standing findings.
	AuditDeclineReason string `json:"audit_decline_reason,omitempty"`
	// AuditDispatches counts audit dispatches, not completions, to retire a superseded round's verdicts.
	// It is bumped before the pre-phase checkpoint write, so a resume retires an interrupted round.
	AuditDispatches int `json:"audit_dispatches,omitempty"`
	// AuditRepairActive is true only inside a granted repair round; the monotonic
	// AuditRepairAttempts cannot gate the repair brief without leaking it into later re-entries.
	AuditRepairActive bool `json:"audit_repair_active,omitempty"`
	// ExplanationDocumentationVersion is the Build explanation contract version this cycle
	// activated; zero grandfathers legacy checkpoints.
	ExplanationDocumentationVersion int `json:"explanation_documentation_version,omitempty"`
	// AuditFailReasons are the runner's own error diagnostics behind an audit FAIL, read here by the coherence
	// floor (never from a workspace file); persisted so a crash resumes without a false halt.
	AuditFailReasons []string `json:"audit_fail_reasons,omitempty"`
	// ShipFailReasons is the ship-phase twin of AuditFailReasons, so a ship rejection after a green audit
	// reads as a coherent task failure, not a forged verdict; persisted for the same crash-resume reason.
	ShipFailReasons []string `json:"ship_fail_reasons,omitempty"`
	// FailedAt mirrors State.FailedAt so the evidence dossier can count same-class recurrence
	// at the retro decision.
	FailedAt []FailedRecord `json:"failed_at,omitempty"`
	// BookkeepingRegradeAttempted bounds the bookkeeping-regrade re-audit to once per cycle; it lives here,
	// not in a workspace file an agent could delete, so neither an agent nor a crash-resume grants it twice.
	BookkeepingRegradeAttempted bool `json:"bookkeeping_regrade_attempted,omitempty"`
}
