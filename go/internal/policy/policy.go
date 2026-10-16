// Package policy loads .evolve/policy.json, the user-owned rule layer that bounds
// the autonomous pipeline, and resolves each block against its compiled defaults.
// See docs/architecture/packages/internal-policy.md.
package policy

import "github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"

type Pin struct {
	CLI   string `json:"cli,omitempty"`
	Model string `json:"model,omitempty"`
}

type FloorGate struct {
	ID                 string `json:"id"`
	Description        string `json:"description,omitempty"`
	EnforcedSinceCycle int    `json:"enforced_since_cycle,omitempty"`
}

type Policy struct {
	MandatoryPhases     []string             `json:"mandatory_phases,omitempty"`
	Pins                map[string]Pin       `json:"pins,omitempty"`
	CLIRouting          *CLIRouting          `json:"cli_routing,omitempty"`
	InboxPriority       *InboxPriorityPolicy `json:"inbox_priority,omitempty"`
	ShipFloor           []string             `json:"ship_floor,omitempty"`
	Floor               []FloorGate          `json:"floor,omitempty"`
	FailureFloor        *FailureFloor        `json:"failure_floor,omitempty"`
	SystemFailurePolicy *SystemFailurePolicy `json:"failure_policy,omitempty"`
	GC                  *gcpolicy.Policy     `json:"gc,omitempty"`
	Fanout              *FanoutPolicy        `json:"fanout,omitempty"`
	Observer            *ObserverPolicy      `json:"observer,omitempty"`

	Boot               *BootPolicy               `json:"boot,omitempty"`
	CIWatch            *CIWatchPolicy            `json:"ci_watch,omitempty"`
	PracticeWatch      *PracticeWatchPolicy      `json:"practice_watch,omitempty"`
	PR                 *PRPolicy                 `json:"pr,omitempty"`
	Bridge             *BridgePolicy             `json:"bridge,omitempty"`
	QuotaReset         *QuotaResetConfig         `json:"quota_reset,omitempty"`
	Dispatch           *DispatchConfig           `json:"dispatch,omitempty"`
	CLIHealth          *CLIHealthConfig          `json:"cli_health,omitempty"`
	Workflow           *WorkflowPolicy           `json:"workflow,omitempty"`
	Retry              *RetryPolicy              `json:"retry,omitempty"`
	Swarm              *SwarmPolicy              `json:"swarm,omitempty"`
	FailureDisposition *FailureDispositionPolicy `json:"failure_disposition,omitempty"`
	Gates              *GatesPolicy              `json:"gates,omitempty"`
	Chronicle          *ChroniclePolicy          `json:"chronicle,omitempty"`
	Router             *RouterPolicy             `json:"router,omitempty"`
	ReportBudget       *ReportBudgetPolicy       `json:"report_budget,omitempty"`
	RetroAutofile      *RetroAutofilePolicy      `json:"retro_autofile,omitempty"`
	MergeGate          *MergeGatePolicy          `json:"merge_gate,omitempty"`
	ParallelEvaluate   *ParallelEvaluatePolicy   `json:"parallel_evaluate,omitempty"`
	ContextFill        *ContextFillPolicy        `json:"context_fill,omitempty"`
	Research           *ResearchPolicy           `json:"research,omitempty"`
	RegressionTIA      *RegressionTIAPolicy      `json:"regression_tia,omitempty"`
	Classify           *ClassifyPolicy           `json:"classify,omitempty"`
	Catalog            *CatalogPolicy            `json:"catalog,omitempty"`
	Recovery           *RecoveryPolicy           `json:"recovery,omitempty"`
	DocsFloor          *DocsFloorPolicy          `json:"docs_floor,omitempty"`
	CommentFloor       *CommentFloorPolicy       `json:"comment_floor,omitempty"`
	CoverChanged       *CoverChangedPolicy       `json:"cover_changed,omitempty"`
	ACS                *ACSConfig                `json:"acs,omitempty"`
	Paths              *PathsConfig              `json:"paths,omitempty"`
	Worktree           *WorktreePolicy           `json:"worktree,omitempty"`
	Checkpoint         *CheckpointPolicy         `json:"checkpoint,omitempty"`
	Integrity          *IntegrityPolicy          `json:"integrity,omitempty"`
	Sandbox            *SandboxPolicy            `json:"sandbox,omitempty"`
	Fleet              *FleetPolicy              `json:"fleet,omitempty"`
	Preflight          *PreflightPolicy          `json:"preflight,omitempty"`
	Chain              *ChainPolicy              `json:"chain,omitempty"`
	GoalStall          *GoalStallPolicy          `json:"goal_stall,omitempty"`
	ObservationMask    *ObservationMaskPolicy    `json:"observation_mask,omitempty"`
	Overlays           *OverlaysPolicy           `json:"overlays,omitempty"`
	Wave               *WavePolicy               `json:"wave,omitempty"`
	AttemptPostmortem  *AttemptPostmortemPolicy  `json:"attempt_postmortem,omitempty"`
}
