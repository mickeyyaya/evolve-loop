package cyclestate

// This file holds the on-disk cycle/state DTOs — the value types serialized to
// .evolve/state.json and .evolve/cycle-state.json. They are byte-identity
// critical (the JSON tags and field order define the on-disk wire shape; the
// ledger SHA-chain and resume path depend on stable bytes). Pure data: no
// methods, no I/O, no dependency on any other internal package. Persistence and
// mutation logic live in package core (the Storage/Ledger ports) and
// internal/triagecap (the throughput window ops).

// State struct {...} mirrors the persisted .evolve/state.json (a SUBSET view —
// the orchestrator's WriteState drops unmodeled keys).
type State struct {
	LastUpdated     string          `json:"lastUpdated"`
	LastCycleNumber int             `json:"lastCycleNumber"`
	Version         int             `json:"version"`
	CurrentBatch    BatchAccrual    `json:"currentBatch"`
	FailedAt        []FailedRecord  `json:"failedApproaches,omitempty"`
	CarryoverTodos  []CarryoverTodo `json:"carryoverTodos,omitempty"`
	// SetupCompletedAt / SetupVersion are the first-run onboarding marker
	// (RFC3339 stamp + version) written by `evolve setup complete`. Empty
	// SetupCompletedAt means setup has never run → `evolve loop` prints a
	// one-line non-blocking nudge. omitempty keeps pre-setup state.json
	// byte-clean. NOTE: this struct is a SUBSET view of state.json (the
	// orchestrator's WriteState would drop unmodeled keys), so the setup
	// marker is written via a lossless raw-merge — never via WriteState.
	SetupCompletedAt string `json:"setupCompletedAt,omitempty"`
	SetupVersion     int    `json:"setupVersion,omitempty"`
	// TriageThroughput is the rolling window (last 5 floor-bearing PASS
	// cycles) of coverage floors passed per cycle — the observed builder
	// throughput that bounds triage's per-cycle floor commitments (R9,
	// inbox coverage-floor-overpacking). Ops live in internal/triagecap.
	TriageThroughput []TriageThroughputEntry `json:"triageThroughput,omitempty"`
	// StateRevision counts writes made through Storage.UpdateState (CA.3
	// OCC): ++ per serialized RMW. 0 (omitted) = never touched by
	// UpdateState; a gap/repeat in the sequence betrays a writer that
	// bypassed the lock. Additive field — single-mode byte-stable.
	StateRevision int `json:"stateRevision,omitempty"`
	// LastAllocatedCycleNumber is the CA.4 allocation lease: the highest
	// cycle number ever minted (≠ LastCycleNumber, the highest COMPLETED).
	// Bumped atomically via UpdateState before a run starts; a crashed run
	// burns its number. Additive field — single-mode byte-stable.
	LastAllocatedCycleNumber int `json:"lastAllocatedCycleNumber,omitempty"`
}

// TriageThroughputEntry is one observed cycle in the triage-capacity rolling
// window: a PASS cycle and how many coverage floors it committed and passed.
type TriageThroughputEntry struct {
	Cycle  int `json:"cycle"`
	Floors int `json:"floors"`
}

// BatchAccrual tracks per-dispatcher-invocation cost.
type BatchAccrual struct {
	CycleAccruedCostUSD float64 `json:"cycleAccruedCostUSD"`
	GoalHash            string  `json:"goalHash,omitempty"`
}

// FailedRecord captures a non-PASS cycle outcome — what the bash side
// writes into state.json:failedApproaches[]. JSON tags use camelCase to
// match the on-disk shape (see legacy/scripts/dispatch/subagent-run.sh +
// failure-classifications.sh). The Classification + ExpiresAt fields are
// consumed by failureadapter to decide RETRY/BLOCK/PROCEED.
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
	CyclesUnpicked int    `json:"cycles_unpicked"`
	// ExpiresAt (RFC3339) is the TTL stamp inherited from the FailedRecord that
	// created this todo, mirroring failedApproaches. It lets the loop-start prune
	// (failurelog.PruneExpiredCarryoverTodos) age the array out instead of letting
	// it grow unboundedly. Empty ⇒ legacy/untimestamped ⇒ never auto-pruned.
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// CycleState mirrors .evolve/cycle-state.json (transient per-cycle).
type CycleState struct {
	CycleID         int      `json:"cycle_id"`
	Phase           string   `json:"phase"`
	StartedAt       string   `json:"started_at"`
	PhaseStartedAt  string   `json:"phase_started_at"`
	ActiveAgent     string   `json:"active_agent,omitempty"`
	ActiveWorktree  string   `json:"active_worktree,omitempty"`
	CompletedPhases []string `json:"completed_phases,omitempty"`
	WorkspacePath   string   `json:"workspace_path"`
	IntentRequired  bool     `json:"intent_required"`
	// RunID is the CA.5 event-sourced run identity: the ULID RunCycle mints
	// for this run, also stamped on every ledger entry the run emits.
	// Additive omitempty field — pre-CA.5 cycle-state files decode/encode
	// unchanged.
	RunID string `json:"run_id,omitempty"`
	// WorktreeBaseSHA is the per-cycle worktree HEAD at creation == the cycle
	// base. Persisted so the crash-resume path (RunCycleFromPhase) can run the
	// cycle-156 build-commit normalize, which RunCycle previously drove from a
	// run-local variable. Empty (omitted) for pre-field checkpoints and
	// worktree-less cycles → the normalize degrades to a no-op.
	WorktreeBaseSHA string `json:"worktree_base_sha,omitempty"`
	// AuditFailReasons: the error-severity diagnostics behind an audit FAIL
	// verdict recorded by the runner's OWN gates (set in-process at the
	// recordFloorVerdictFailure chokepoint; cleared on every audit re-dispatch).
	// The ADR-0072 coherence floor reads THIS field — orchestrator memory, never
	// an agent-writable workspace file — to tell a DIAGNOSED gate-downgrade
	// (coherent task-FAIL → retro + continue) from an unexplained forged verdict
	// (halt). Additive omitempty; persisted so a crash between the verdict
	// record and cycle finalization resumes without a false halt (the resume
	// path already trusts cycle-state.json wholesale).
	AuditFailReasons []string `json:"audit_fail_reasons,omitempty"`
}
