package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

type Phase = cyclestate.Phase

const (
	PhaseStart        = cyclestate.PhaseStart
	PhaseIntent       = cyclestate.PhaseIntent
	PhaseScout        = cyclestate.PhaseScout
	PhaseTriage       = cyclestate.PhaseTriage
	PhaseTDD          = cyclestate.PhaseTDD
	PhaseBuildPlanner = cyclestate.PhaseBuildPlanner
	PhaseSwarmPlan    = cyclestate.PhaseSwarmPlan
	PhaseBuild        = cyclestate.PhaseBuild
	PhaseAudit        = cyclestate.PhaseAudit
	PhaseShip         = cyclestate.PhaseShip
	PhaseRetro        = cyclestate.PhaseRetro
	PhaseDebugger     = cyclestate.PhaseDebugger
	PhaseEnd          = cyclestate.PhaseEnd
)

const (
	VerdictPASS    = cyclestate.VerdictPASS
	VerdictFAIL    = cyclestate.VerdictFAIL
	VerdictWARN    = cyclestate.VerdictWARN
	VerdictSKIPPED = cyclestate.VerdictSKIPPED
)

const (
	CycleOutcomeShippedViaBuild       = cyclestate.CycleOutcomeShippedViaBuild
	CycleOutcomeSkippedAuditAdvisory  = cyclestate.CycleOutcomeSkippedAuditAdvisory
	CycleOutcomeSkippedUnknown        = cyclestate.CycleOutcomeSkippedUnknown
	CycleTerminationTriageNoWork      = cyclestate.CycleTerminationTriageNoWork
	CycleTerminationTddProtectedRoute = cyclestate.CycleTerminationTddProtectedRoute

	CycleTerminationBlockedNeedsConsole = cyclestate.CycleTerminationBlockedNeedsConsole
)

func IsVerdict(s string) bool { return cyclestate.IsVerdict(s) }

type (
	TokenUsage          = cyclestate.TokenUsage
	Diagnostic          = cyclestate.Diagnostic
	CycleResult         = cyclestate.CycleResult
	SkippedPhase        = cyclestate.SkippedPhase
	VerdictNotAdopted   = cyclestate.VerdictNotAdopted
	SpineFailOpen       = cyclestate.SpineFailOpen
	SystemFailureSignal = cyclestate.SystemFailureSignal
)

type BuildExplanationState string

const (
	BuildExplanationLegacy      BuildExplanationState = "legacy"
	BuildExplanationNotYetBuilt BuildExplanationState = "not_yet_built"
	BuildExplanationAvailable   BuildExplanationState = "available"
	BuildExplanationInvalid     BuildExplanationState = "missing_or_invalid"
)

type PhaseRequest struct {
	Cycle                           int               `json:"cycle"`
	AuditRound                      int               `json:"audit_round,omitempty"`
	ProjectRoot                     string            `json:"project_root"`
	Workspace                       string            `json:"workspace"`
	Worktree                        string            `json:"worktree"`
	WorktreeBaseSHA                 string            `json:"worktree_base_sha,omitempty"`
	ExplanationDocumentationVersion int               `json:"explanation_documentation_version,omitempty"`
	Fence                           *WorktreeFence    `json:"worktree_fence,omitempty"`
	WorktreeVerified                bool              `json:"-"`
	RunID                           string            `json:"run_id,omitempty"`
	GoalHash                        string            `json:"goal_hash"`
	Context                         map[string]string `json:"context,omitempty"`
	Signals                         map[string]string `json:"signals,omitempty"`
	PreviousPhase                   string            `json:"previous_phase,omitempty"`
	Env                             map[string]string `json:"env,omitempty"`
	PermissionMode                  string            `json:"permission_mode,omitempty"`

	BuildPlan string `json:"build_plan,omitempty"`

	BuildExplanation      *phaseio.ExplanationView `json:"build_explanation,omitempty"`
	BuildExplanationState BuildExplanationState    `json:"build_explanation_state,omitempty"`
	BuildExplanationError string                   `json:"build_explanation_error,omitempty"`

	Input phaseio.PhaseInput `json:"-"`

	BypassPolicy        bool   `json:"bypass_policy,omitempty"`
	ComposePhases       bool   `json:"compose_phases,omitempty"`
	CorrectionDirective string `json:"correction_directive,omitempty"`
	OperatorDirectives  string `json:"operator_directives,omitempty"`

	Spec *phasespec.PhaseSpec `json:"spec,omitempty"`

	UpstreamSignals map[string]any `json:"upstream_signals,omitempty"`

	ModelRoutingCLI    string  `json:"model_routing_cli,omitempty"`
	ModelRoutingTier   string  `json:"model_routing_tier,omitempty"`
	BudgetScale        float64 `json:"budget_scale,omitempty"`
	Attempt            int     `json:"attempt,omitempty"`
	ModelRoutingEffort string  `json:"model_routing_effort,omitempty"`

	Ladder *retryladder.Try `json:"-"`
}

type PhaseResponse struct {
	Phase        string       `json:"phase"`
	Verdict      string       `json:"verdict"`
	ArtifactsDir string       `json:"artifacts_dir"`
	NextPhase    string       `json:"next_phase,omitempty"`
	CostUSD      float64      `json:"cost_usd"`
	Tokens       TokenUsage   `json:"tokens"`
	DurationMS   int64        `json:"duration_ms"`
	BootMS       int64        `json:"boot_ms,omitempty"`
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`

	Signals map[string]any `json:"signals,omitempty"`

	CommitSHA string `json:"commit_sha,omitempty"`

	Reconciled bool `json:"reconciled,omitempty"`

	ModelSource   string `json:"model_source,omitempty"`
	ResolvedModel string `json:"resolved_model,omitempty"`
}

type PhaseRunner interface {
	Name() string
	Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error)
}

type PersonaProber interface {
	PersonaAvailable() error
}
