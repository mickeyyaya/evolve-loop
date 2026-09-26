// Package policy loads .evolve/policy.json, the user-owned rule layer that bounds
// the autonomous pipeline, and resolves each block against its compiled defaults.
// See docs/architecture/packages/internal-policy.md.
package policy

import "github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"

// Pin is a hard per-phase dispatch pin; an empty field leaves that dimension unpinned.
type Pin struct {
	CLI   string `json:"cli,omitempty"`
	Model string `json:"model,omitempty"`
}

// FloorGate is one entry of the `floor` closeout-gate array, such as "dossier-closeout".
// See ADR-0055.
type FloorGate struct {
	ID                 string `json:"id"`
	Description        string `json:"description,omitempty"`
	EnforcedSinceCycle int    `json:"enforced_since_cycle,omitempty"`
}

// Policy is the user-controlled rule set from .evolve/policy.json; a nil block means compiled defaults.
type Policy struct {
	// MandatoryPhases can only add to the mandatory set; the integrity floor still applies on top.
	MandatoryPhases []string       `json:"mandatory_phases,omitempty"`
	Pins            map[string]Pin `json:"pins,omitempty"`
	// ShipFloor lists the phases a shipping plan must run; empty means the router's default, and FloorPhases always adds "audit".
	ShipFloor []string `json:"ship_floor,omitempty"`
	// Floor lists closeout gates, not phases (contrast ShipFloor).
	Floor []FloorGate `json:"floor,omitempty"`
	// FailureFloor tunes only the LLM-learning layer; the deterministic failure floor is not configurable.
	FailureFloor *FailureFloor `json:"failure_floor,omitempty"`
	// SystemFailurePolicy is the failure decision table, distinct from FailureFloor.
	SystemFailurePolicy *SystemFailurePolicy `json:"failure_policy,omitempty"`
	// GC cannot relax the hard retention rules: quarantine is manual-only, and the ledger and live runs are never touched.
	GC       *gcpolicy.Policy `json:"gc,omitempty"`
	Fanout   *FanoutPolicy    `json:"fanout,omitempty"`
	Observer *ObserverPolicy  `json:"observer,omitempty"`

	Boot               *BootPolicy               `json:"boot,omitempty"`
	CIWatch            *CIWatchPolicy            `json:"ci_watch,omitempty"`
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
	ACS                *ACSConfig                `json:"acs,omitempty"`
	Paths              *PathsConfig              `json:"paths,omitempty"`
	Worktree           *WorktreePolicy           `json:"worktree,omitempty"`
	Integrity          *IntegrityPolicy          `json:"integrity,omitempty"`
	Sandbox            *SandboxPolicy            `json:"sandbox,omitempty"`
	Fleet              *FleetPolicy              `json:"fleet,omitempty"`
	Chain              *ChainPolicy              `json:"chain,omitempty"`
	GoalStall          *GoalStallPolicy          `json:"goal_stall,omitempty"`
	ObservationMask    *ObservationMaskPolicy    `json:"observation_mask,omitempty"`
	// Overlays: nil applies the compiled default; a block with empty Rules opts out of all overlays.
	Overlays *OverlaysPolicy `json:"overlays,omitempty"`
}
