package cyclestate

type State struct {
	LastUpdated              string                  `json:"lastUpdated"`
	LastCycleNumber          int                     `json:"lastCycleNumber"`
	Version                  int                     `json:"version"`
	CurrentBatch             BatchAccrual            `json:"currentBatch"`
	FailedAt                 []FailedRecord          `json:"failedApproaches,omitempty"`
	CarryoverTodos           []CarryoverTodo         `json:"carryoverTodos,omitempty"`
	SetupCompletedAt         string                  `json:"setupCompletedAt,omitempty"`
	SetupVersion             int                     `json:"setupVersion,omitempty"`
	TriageThroughput         []TriageThroughputEntry `json:"triageThroughput,omitempty"`
	StateRevision            int                     `json:"stateRevision,omitempty"`
	LastAllocatedCycleNumber int                     `json:"lastAllocatedCycleNumber,omitempty"`
}

type TriageThroughputEntry struct {
	Cycle  int `json:"cycle"`
	Floors int `json:"floors"`
}

type BatchAccrual struct {
	CycleAccruedCostUSD float64 `json:"cycleAccruedCostUSD"`
	GoalHash            string  `json:"goalHash,omitempty"`
}

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

type CarryoverTodo struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	Priority       string `json:"priority"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	CyclesUnpicked int    `json:"cycles_unpicked"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
}

type CycleState struct {
	FinalVerdict                    string         `json:"final_verdict,omitempty"`
	GoalHash                        string         `json:"goal_hash,omitempty"`
	GoalText                        string         `json:"goal_text,omitempty"`
	Shipped                         bool           `json:"shipped,omitempty"`
	PreCycleHEAD                    string         `json:"pre_cycle_head,omitempty"`
	CycleID                         int            `json:"cycle_id"`
	Phase                           string         `json:"phase"`
	StartedAt                       string         `json:"started_at"`
	PhaseStartedAt                  string         `json:"phase_started_at"`
	ActiveAgent                     string         `json:"active_agent,omitempty"`
	ActiveWorktree                  string         `json:"active_worktree,omitempty"`
	CompletedPhases                 []string       `json:"completed_phases,omitempty"`
	WorkspacePath                   string         `json:"workspace_path"`
	IntentRequired                  bool           `json:"intent_required"`
	RunID                           string         `json:"run_id,omitempty"`
	WorktreeBaseSHA                 string         `json:"worktree_base_sha,omitempty"`
	AuditRepairAttempts             int            `json:"audit_repair_attempts,omitempty"`
	ShipRecoveryCode                string         `json:"ship_recovery_code,omitempty"`
	ShipRecoveryConflicts           []string       `json:"ship_recovery_conflicts,omitempty"`
	AuditDeclineReason              string         `json:"audit_decline_reason,omitempty"`
	AuditDispatches                 int            `json:"audit_dispatches,omitempty"`
	AuditRepairActive               bool           `json:"audit_repair_active,omitempty"`
	FailureDecisionUntrusted        bool           `json:"failure_decision_untrusted,omitempty"`
	PredicateRepairBrief            string         `json:"predicate_repair_brief,omitempty"`
	PredicateReroutes               int            `json:"predicate_reroutes,omitempty"`
	ExplanationCorrections          []string       `json:"explanation_corrections,omitempty"`
	ExplanationCorrectionActive     bool           `json:"explanation_correction_active,omitempty"`
	ExplanationDocumentationVersion int            `json:"explanation_documentation_version,omitempty"`
	AuditFailReasons                []string       `json:"audit_fail_reasons,omitempty"`
	ShipFailReasons                 []string       `json:"ship_fail_reasons,omitempty"`
	FailedAt                        []FailedRecord `json:"failed_at,omitempty"`
	BookkeepingRegradeAttempted     bool           `json:"bookkeeping_regrade_attempted,omitempty"`
	QuotaWalkCLIs                   []string       `json:"quota_walk_clis,omitempty"`
	Ladder                          *LadderState   `json:"ladder,omitempty"`
}

type LadderState struct {
	Rung         string   `json:"rung,omitempty"`
	Tier         string   `json:"tier,omitempty"`
	Effort       string   `json:"effort,omitempty"`
	Fingerprints []string `json:"fingerprints,omitempty"`
	History      []string `json:"history,omitempty"`
	RepairRound  int      `json:"repair_round,omitempty"`
	AuditFix     string   `json:"audit_fix,omitempty"`
}

func (s State) HighestCycleNumber() int {
	return max(s.LastCycleNumber, s.LastAllocatedCycleNumber)
}
